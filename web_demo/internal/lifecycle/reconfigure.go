package lifecycle

import (
	"context"
	"errors"

	"github.com/STRK-Solutions/Feather-Mesh/web_demo/internal/control"
)

type AssignmentAuthority interface {
	Assignments(context.Context, string) (control.Assignments, error)
}

func (a HTTPAuthority) Assignments(ctx context.Context, id string) (control.Assignments, error) {
	var out control.Assignments
	e := a.exchange(ctx, "GET", "/v1/assignments/"+id, nil, &out)
	return out, e
}

// Reconfigure stops all recorded generations before changing the mount policy.
// It preserves private bytes and never resumes the terminal or an uncertain job.
// A durable gateway outbox retries after any lost reply.
func (c *Controller) Reconfigure(ctx context.Context, id string, version int64) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	a, ok := c.Authority.(AssignmentAuthority)
	if !ok {
		return ErrAuthority
	}
	assignments, e := a.Assignments(ctx, id)
	if e != nil || assignments.Workspace.GrantVersion != version {
		return ErrAuthority
	}
	w, e := c.Store.Workspace(id)
	if e != nil {
		return e
	}
	if assignments.Account.ID != w.OwnerID || assignments.Workspace.DeploymentID != w.DeploymentID || version < w.AssignmentVersion {
		return ErrAuthority
	}
	var reclaimPending int
	if e = c.Store.DB.QueryRow(`SELECT count(*) FROM slot_reclaims WHERE workspace_id=? AND state='pending'`, id).Scan(&reclaimPending); e != nil {
		return e
	}
	if reclaimPending != 0 {
		return ErrConflict
	}
	if version == w.AssignmentVersion {
		return c.Authority.Snapshot(ctx, w, assignments.Workspace.ActivationGeneration)
	}
	var capabilityErr error
	if c.Capabilities != nil {
		capabilityErr = c.Capabilities.Revoke(ctx, w)
	}
	if e = c.stopVerified(ctx, w); e != nil {
		return e
	}
	ids, e := c.Store.Pending()
	if e != nil {
		return e
	}
	maxGeneration := w.Generation
	for _, jobID := range ids {
		j, err := c.Store.Job(jobID)
		if err != nil {
			return err
		}
		if j.Request.WorkspaceID != id {
			continue
		}
		if j.TargetGeneration > maxGeneration {
			maxGeneration = j.TargetGeneration
		}
		if j.TargetGeneration != w.Generation {
			target := w
			target.Generation, target.SlotID = j.TargetGeneration, j.TargetSlot
			if e = c.stopVerified(ctx, target); e != nil {
				return e
			}
		}
	}
	if capabilityErr != nil {
		return capabilityErr
	}
	tx, e := c.Store.DB.BeginTx(ctx, nil)
	if e != nil {
		return e
	}
	defer tx.Rollback()
	if _, e = tx.Exec(`UPDATE slots SET state='retained' WHERE workspace_id=? AND state='reserved'`, id); e != nil {
		return e
	}
	if _, e = tx.Exec(`UPDATE lifecycle_jobs SET state='failed',outcome='grant_changed_no_replay' WHERE workspace_id=? AND state IN ('accepted','executing','unknown')`, id); e != nil {
		return e
	}
	if _, e = tx.Exec(`UPDATE workspaces SET generation=?,assignment_version=?,state='stopped' WHERE id=?`, maxGeneration+1, version, id); e != nil {
		return e
	}
	if _, e = tx.Exec(`UPDATE slots SET generation=? WHERE id=? AND state='assigned'`, maxGeneration+1, w.SlotID); e != nil {
		return e
	}
	if _, e = tx.Exec(`INSERT INTO snapshot_outbox VALUES(?,?) ON CONFLICT(workspace_id) DO UPDATE SET activation=excluded.activation`, id, assignments.Workspace.ActivationGeneration); e != nil {
		return e
	}
	if e = tx.Commit(); e != nil {
		return e
	}
	w, e = c.Store.Workspace(id)
	if e != nil {
		return e
	}
	return c.Authority.Snapshot(ctx, w, assignments.Workspace.ActivationGeneration)
}

func (c *Controller) stopVerified(ctx context.Context, w Workspace) error {
	if e := c.Runtime.Stop(ctx, w); e != nil {
		return e
	}
	_, running, e := c.Runtime.Inspect(ctx, w)
	if e != nil {
		return e
	}
	if running {
		return errors.New("generation stop not verified")
	}
	return nil
}
