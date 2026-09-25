package lifecycle

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/STRK-Solutions/Feather-Mesh/web_demo/internal/control"
)

// Authority queries the gateway owner on every admission and dispatch. Returning
// unavailable is a denial; a client-supplied identity is never an authorization.
type Authority interface {
	Check(context.Context, Request, Workspace) error
	Snapshot(context.Context, Workspace, int64) error
}
type Runtime interface {
	Inspect(context.Context, Workspace) (bool, bool, error)
	Start(context.Context, Workspace) error
	Stop(context.Context, Workspace) error
}
type CapabilityLifecycle interface {
	Prepare(context.Context, Workspace) error
	Activate(context.Context, Workspace) error
	Revoke(context.Context, Workspace) error
}
type Controller struct {
	Store        *Store
	Authority    Authority
	Runtime      Runtime
	Capabilities CapabilityLifecycle
	mu           sync.Mutex
}

func (c *Controller) Admit(ctx context.Context, r Request) (Result, error) {
	w, e := c.Store.Workspace(r.WorkspaceID)
	if e != nil {
		return Result{}, e
	}
	if e = c.Authority.Check(ctx, r, w); e != nil {
		return Result{}, ErrAuthority
	}
	return c.Store.admit(r, w.OwnerID != r.ActorID)
}

// Work serializes local runtime effects. The database also prevents concurrent
// jobs across requests. Any loss of a runtime reply leaves durable uncertainty;
// recovery only inspects labeled resources before deciding what completed.
func (c *Controller) Work(ctx context.Context, id string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	j, e := c.Store.Job(id)
	if e != nil {
		return e
	}
	if j.Outcome == "operator_abort_pending" {
		return errors.New("operator abort requires explicit completion")
	}
	if j.State == "completed" || j.State == "failed" {
		return nil
	}
	w, e := c.Store.Workspace(j.Request.WorkspaceID)
	if e != nil {
		return e
	}
	target := w
	target.Generation = j.TargetGeneration
	target.SlotID = j.TargetSlot
	if j.State == "executing" || j.State == "unknown" {
		return c.reconcile(ctx, j, w, target)
	}
	if j.Request.Method == "workspace.start" || j.Request.Method == "workspace.reset" {
		var desired string
		if e = c.Store.DB.QueryRow(`SELECT desired FROM deployment WHERE id=? AND activation=?`, j.Request.DeploymentID, j.Request.ActivationGeneration).Scan(&desired); e != nil || desired != "running" {
			_ = c.Store.Mark(id, "unknown", "deployment_stopped")
			return ErrAuthority
		}
		if e = c.Authority.Check(ctx, j.Request, w); e != nil {
			_ = c.Store.Mark(id, "unknown", "authorization_changed")
			return ErrAuthority
		}
	}
	if e = c.Store.Mark(id, "executing", ""); e != nil {
		return e
	}
	if j.Request.Method != "workspace.start" {
		var capabilityErr error
		if c.Capabilities != nil {
			capabilityErr = c.Capabilities.Revoke(ctx, w)
		}
		if e = c.Runtime.Stop(ctx, w); e != nil {
			_ = c.Store.Mark(id, "unknown", "runtime_stop_unknown")
			return e
		}
		if capabilityErr != nil {
			return capabilityErr
		}
	}
	if j.Request.Method == "workspace.start" || j.Request.Method == "workspace.reset" {
		if e = c.Authority.Check(ctx, j.Request, w); e != nil {
			_ = c.Store.Mark(id, "unknown", "authorization_changed")
			return ErrAuthority
		}
		_, running, err := c.Runtime.Inspect(ctx, target)
		if err != nil {
			return err
		}
		if running {
			return c.reconcile(ctx, j, w, target)
		}
		if c.Capabilities != nil {
			if e = c.Capabilities.Prepare(ctx, target); e != nil {
				_ = c.Store.Mark(id, "unknown", "configuration_prepare_failed")
				return e
			}
		}
		if e = c.Runtime.Start(ctx, target); e != nil {
			_ = c.Store.Mark(id, "unknown", "runtime_start_unknown")
			return e
		}
	}
	return c.reconcile(ctx, j, w, target)
}
func (c *Controller) reconcile(ctx context.Context, j Job, old, target Workspace) error {
	_, running, e := c.Runtime.Inspect(ctx, target)
	if e != nil {
		_ = c.Store.Mark(j.ID, "unknown", "runtime_inspection_unavailable")
		return e
	}
	wantsRunning := j.Request.Method == "workspace.start" || j.Request.Method == "workspace.reset"
	if wantsRunning {
		var desired string
		if err := c.Store.DB.QueryRow(`SELECT desired FROM deployment WHERE id=? AND activation=?`, j.Request.DeploymentID, j.Request.ActivationGeneration).Scan(&desired); err != nil || desired != "running" {
			_ = c.stopVerified(ctx, target)
			if c.Capabilities != nil {
				_ = c.Capabilities.Revoke(ctx, target)
			}
			_ = c.Store.Mark(j.ID, "unknown", "deployment_stopped_no_replay")
			return ErrAuthority
		}
		if e = c.Authority.Check(ctx, j.Request, old); e != nil {
			_ = c.Runtime.Stop(ctx, target)
			_ = c.Store.Mark(j.ID, "unknown", "authorization_changed")
			return ErrAuthority
		}
		if !running {
			_ = c.Store.Mark(j.ID, "unknown", "explicit_reconciliation_required")
			return errors.New("runtime did not reach expected state; no replay")
		}
		if readiness, ok := c.Runtime.(interface {
			Ready(context.Context, Workspace) error
		}); ok {
			if e = readiness.Ready(ctx, target); e != nil {
				return e
			}
		}
		if j.Request.Method == "workspace.reset" {
			_, oldRunning, e := c.Runtime.Inspect(ctx, old)
			if e != nil || oldRunning {
				return errors.New("old generation must be verified stopped")
			}
		}
	} else if running {
		return errors.New("stop outcome not verified")
	}
	if !wantsRunning && c.Capabilities != nil {
		if e = c.Capabilities.Revoke(ctx, target); e != nil {
			return e
		}
	}
	if e = c.Store.Finish(j); e != nil {
		return e
	}
	target.State = "stopped"
	if wantsRunning {
		target.State = "running"
	}
	if j.Request.Method == "workspace.delete" {
		target.State = "deleted"
	}
	if e = c.Authority.Snapshot(ctx, target, j.Request.ActivationGeneration); e != nil {
		return errors.New("lifecycle committed; gateway snapshot requires reconciliation")
	}
	if wantsRunning && c.Capabilities != nil {
		if e = c.Capabilities.Activate(ctx, target); e != nil {
			return errors.New("lifecycle committed; capability reconciliation required")
		}
	}
	_, e = c.Store.DB.Exec(`DELETE FROM snapshot_outbox WHERE workspace_id=?`, target.ID)
	if e != nil {
		return e
	}
	return nil
}

type HTTPAuthority struct {
	Client *http.Client
	URL    string
}

func (a HTTPAuthority) Check(ctx context.Context, r Request, w Workspace) error {
	if r.ActorID != w.OwnerID {
		if r.Method != "workspace.stop" && r.Method != "workspace.reset" && r.Method != "workspace.delete" {
			return ErrAuthority
		}
		var out control.Snapshot
		if e := a.exchange(ctx, "POST", "/v1/authorize", control.Check{AccountID: r.ActorID, AuthVersion: r.AuthorizationVersion}, &out); e != nil || out.Account.Role != "admin" {
			return ErrAuthority
		}
		var assignments control.Assignments
		if e := a.exchange(ctx, "GET", "/v1/assignments/"+w.ID, nil, &assignments); e != nil {
			return e
		}
		if assignments.Workspace.Generation != w.Generation || assignments.Workspace.GrantVersion != w.AssignmentVersion || assignments.Workspace.DeploymentID != r.DeploymentID || assignments.Workspace.ActivationGeneration != r.ActivationGeneration {
			return ErrAuthority
		}
		return nil
	}
	in := map[string]any{"account_id": r.ActorID, "auth_version": r.AuthorizationVersion, "workspace_id": w.ID, "workspace_generation": w.Generation, "grant_version": w.AssignmentVersion, "deployment_id": r.DeploymentID, "activation_generation": r.ActivationGeneration}
	return a.post(ctx, "/v1/authorize", in)
}
func (a HTTPAuthority) Snapshot(ctx context.Context, w Workspace, activation int64) error {
	return a.post(ctx, "/v1/workspaces/snapshot", map[string]any{"id": w.ID, "owner_id": w.OwnerID, "hostname": w.Hostname, "ready": w.State != "deleted", "generation": w.Generation, "grant_version": w.AssignmentVersion, "deployment_id": w.DeploymentID, "activation_generation": activation})
}
func (a HTTPAuthority) post(ctx context.Context, path string, in any) error {
	return a.exchange(ctx, "POST", path, in, nil)
}
func (a HTTPAuthority) exchange(ctx context.Context, method, path string, in, out any) error {
	body, e := json.Marshal(in)
	if e != nil {
		return e
	}
	req, e := http.NewRequestWithContext(ctx, method, a.URL+path, bytes.NewReader(body))
	if e != nil {
		return e
	}
	req.Header.Set("Content-Type", "application/json")
	res, e := a.Client.Do(req)
	if e != nil {
		return e
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return ErrAuthority
	}
	if out != nil {
		return json.NewDecoder(io.LimitReader(res.Body, 64<<10)).Decode(out)
	}
	_, _ = io.Copy(io.Discard, io.LimitReader(res.Body, 64<<10))
	return nil
}

// Revoke is an independently authenticated gateway command: it stops only the
// workspace owned by this account, even after normal authorization is disabled.
// It never starts/recreates or grants an admin access to private file contents.
func (c *Controller) Revoke(ctx context.Context, account string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, e := c.Store.DB.Exec(`INSERT OR IGNORE INTO revoked_accounts VALUES(?)`, account); e != nil {
		return e
	}
	var id string
	if e := c.Store.DB.QueryRow(`SELECT id FROM workspaces WHERE owner_id=?`, account).Scan(&id); e != nil {
		return e
	}
	w, e := c.Store.Workspace(id)
	if e != nil {
		return e
	}
	var capabilityErr error
	if c.Capabilities != nil {
		capabilityErr = c.Capabilities.Revoke(ctx, w)
	}
	if e = c.Runtime.Stop(ctx, w); e != nil {
		return e
	}
	ids, e := c.Store.Pending()
	if e != nil {
		return e
	}
	for _, jobID := range ids {
		j, e := c.Store.Job(jobID)
		if e != nil {
			return e
		}
		if j.Request.WorkspaceID == w.ID && j.TargetGeneration != w.Generation {
			target := w
			target.Generation = j.TargetGeneration
			target.SlotID = j.TargetSlot
			if e = c.Runtime.Stop(ctx, target); e != nil {
				return e
			}
			_, running, e := c.Runtime.Inspect(ctx, target)
			if e != nil || running {
				return errors.New("pending generation stop not verified")
			}
		}
	}
	_, running, e := c.Runtime.Inspect(ctx, w)
	if e != nil {
		return e
	}
	if running {
		return errors.New("revocation stop not verified")
	}
	_, e = c.Store.DB.Exec(`UPDATE workspaces SET state='stopped' WHERE id=?`, id)
	return errors.Join(e, capabilityErr)
}
func (c *Controller) Run(ctx context.Context) {
	t := time.NewTicker(time.Second)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			ids, e := c.Store.Pending()
			if e == nil {
				for _, id := range ids {
					_ = c.Work(ctx, id)
				}
			}
			_ = c.FlushSnapshots(ctx)
			_ = c.StopIdle(ctx)
			_ = c.RenewCapabilities(ctx)
			_, _ = c.Store.WarnRetention()
		}
	}
}

func (c *Controller) RenewCapabilities(ctx context.Context) error {
	if c.Capabilities == nil {
		return nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	var desired string
	if e := c.Store.DB.QueryRow(`SELECT desired FROM deployment`).Scan(&desired); e != nil {
		return e
	}
	if desired != "running" {
		return nil
	}
	rows, e := c.Store.DB.Query(`SELECT id FROM workspaces WHERE state='running'`)
	if e != nil {
		return e
	}
	var ids []string
	for rows.Next() {
		var id string
		if e = rows.Scan(&id); e != nil {
			rows.Close()
			return e
		}
		ids = append(ids, id)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return e
	}
	for _, id := range ids {
		w, e := c.Store.Workspace(id)
		if e != nil {
			return e
		}
		if e = c.Capabilities.Activate(ctx, w); e != nil {
			return e
		}
	}
	return nil
}

func (c *Controller) FlushSnapshots(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	rows, e := c.Store.DB.Query(`SELECT workspace_id,activation FROM snapshot_outbox`)
	if e != nil {
		return e
	}
	type pending struct {
		id         string
		activation int64
	}
	var work []pending
	for rows.Next() {
		var p pending
		if e = rows.Scan(&p.id, &p.activation); e != nil {
			rows.Close()
			return e
		}
		work = append(work, p)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return e
	}
	for _, p := range work {
		w, e := c.Store.Workspace(p.id)
		if e != nil {
			return e
		}
		if e = c.Authority.Snapshot(ctx, w, p.activation); e != nil {
			return e
		}
		if w.State == "running" && c.Capabilities != nil {
			if e = c.Capabilities.Activate(ctx, w); e != nil {
				return e
			}
		}
		if _, e = c.Store.DB.Exec(`DELETE FROM snapshot_outbox WHERE workspace_id=?`, p.id); e != nil {
			return e
		}
	}
	return nil
}
func (c *Controller) StopIdle(ctx context.Context) error {
	due, e := c.Store.Due()
	if e != nil {
		return e
	}
	var dep, desired string
	var activation int64
	if e = c.Store.DB.QueryRow(`SELECT id,activation,desired FROM deployment`).Scan(&dep, &activation, &desired); e != nil {
		return e
	}
	if desired == "stopped" {
		rows, e := c.Store.DB.Query(`SELECT id FROM workspaces WHERE state='running'`)
		if e != nil {
			return e
		}
		var ids []string
		for rows.Next() {
			var id string
			if e = rows.Scan(&id); e != nil {
				rows.Close()
				return e
			}
			ids = append(ids, id)
		}
		e = rows.Err()
		rows.Close()
		if e != nil {
			return e
		}
		due = nil
		for _, id := range ids {
			w, e := c.Store.Workspace(id)
			if e != nil {
				return e
			}
			due = append(due, w)
		}
	}
	for _, w := range due {
		r := Request{Protocol: "feam.web.v1", RequestID: ID(), IdempotencyKey: ID(), DeploymentID: dep, ActivationGeneration: activation, WorkspaceID: w.ID, ExpectedGeneration: w.Generation, ActorID: w.OwnerID, AuthorizationVersion: 1, Method: "workspace.stop"}
		out, e := c.Store.Admit(r)
		if e == ErrConflict {
			continue
		}
		if e != nil {
			return e
		}
		if e = c.Work(ctx, out.JobID); e != nil {
			return e
		}
	}
	return nil
}
