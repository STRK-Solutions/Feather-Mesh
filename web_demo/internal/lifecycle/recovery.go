package lifecycle

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"time"

	"golang.org/x/sys/unix"
)

// ProcessLock is held by the daemon and every mutating operator command. Never
// unlink the lock file: replacing the inode would permit two exclusive owners.
func ProcessLock(database string) (*os.File, error) {
	fd, err := unix.Open(database+".controller.lock", unix.O_RDWR|unix.O_CREAT|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0600)
	if err != nil {
		return nil, err
	}
	f := os.NewFile(uintptr(fd), database+".controller.lock")
	var st unix.Stat_t
	if err = unix.Fstat(fd, &st); err != nil || st.Mode&unix.S_IFMT != unix.S_IFREG || st.Mode&0077 != 0 || st.Uid != uint32(os.Getuid()) || st.Nlink != 1 {
		f.Close()
		return nil, errors.New("unsafe controller lock")
	}
	if err = unix.Flock(fd, unix.LOCK_EX|unix.LOCK_NB); err != nil {
		f.Close()
		return nil, errors.New("controller is active; stop the service before an operator mutation")
	}
	return f, nil
}

type RecoveryReview struct {
	JobID             string `json:"job_id"`
	RequestSHA256     string `json:"request_sha256"`
	State             string `json:"state"`
	WorkspaceID       string `json:"workspace_id"`
	Method            string `json:"method"`
	CurrentGeneration int64  `json:"current_generation"`
	TargetGeneration  int64  `json:"target_generation"`
	CurrentSlot       string `json:"current_slot"`
	TargetSlot        string `json:"target_slot"`
}

func (s *Store) ReviewRecovery(id string) (RecoveryReview, error) {
	var out RecoveryReview
	j, err := s.Job(id)
	if err != nil {
		return out, err
	}
	w, err := s.Workspace(j.Request.WorkspaceID)
	if err != nil {
		return out, err
	}
	var hash string
	if err = s.DB.QueryRow(`SELECT request_hash FROM lifecycle_jobs WHERE id=?`, id).Scan(&hash); err != nil {
		return out, err
	}
	return RecoveryReview{id, hash, j.State, w.ID, j.Request.Method, w.Generation, j.TargetGeneration, w.SlotID, j.TargetSlot}, nil
}

// AbortJob is an explicit, hash-bound resolution. It never retries a start or
// FEAM operation. Stopped bytes and uncertain reset slots remain recoverable.
func (c *Controller) AbortJob(ctx context.Context, id, expectedHash string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	review, err := c.Store.ReviewRecovery(id)
	if err != nil {
		return err
	}
	if review.RequestSHA256 != expectedHash || !regexp.MustCompile(`^[0-9a-f]{64}$`).MatchString(expectedHash) {
		return ErrConflict
	}
	if review.State == "failed" {
		var previous string
		if err = c.Store.DB.QueryRow(`SELECT request_hash FROM recovery_audit WHERE job_id=?`, id).Scan(&previous); err == nil && previous == expectedHash {
			return nil
		}
		return ErrConflict
	}
	if review.State != "accepted" && review.State != "executing" && review.State != "unknown" {
		return ErrConflict
	}
	j, err := c.Store.Job(id)
	if err != nil {
		return err
	}
	w, err := c.Store.Workspace(review.WorkspaceID)
	if err != nil {
		return err
	}
	if w.Generation > j.TargetGeneration {
		return ErrConflict
	}
	// The daemon must preserve this operator resolution across a process loss;
	// it may neither dispatch nor finish a job with this durable outcome marker.
	if err = c.Store.Mark(id, "unknown", "operator_abort_pending"); err != nil {
		return err
	}
	review.State = "unknown"
	var revokeErr error
	if c.Capabilities != nil {
		revokeErr = c.Capabilities.Revoke(ctx, w)
	}
	if err = c.stopVerified(ctx, w); err != nil {
		return err
	}
	if j.TargetGeneration != w.Generation {
		target := w
		target.Generation, target.SlotID = j.TargetGeneration, j.TargetSlot
		if err = c.stopVerified(ctx, target); err != nil {
			return err
		}
	}
	if revokeErr != nil {
		return revokeErr
	}
	tx, err := c.Store.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var currentState string
	if err = tx.QueryRow(`SELECT state FROM lifecycle_jobs WHERE id=? AND request_hash=?`, id, expectedHash).Scan(&currentState); err != nil || currentState != review.State {
		return ErrConflict
	}
	next := max(w.Generation, j.TargetGeneration) + 1
	if _, err = tx.Exec(`UPDATE slots SET state='retained',retained_at=? WHERE workspace_id=? AND state='reserved'`, stamp(c.Store.Now()), w.ID); err != nil {
		return err
	}
	if _, err = tx.Exec(`UPDATE lifecycle_jobs SET state='failed',outcome='operator_aborted_no_replay' WHERE id=?`, id); err != nil {
		return err
	}
	if _, err = tx.Exec(`UPDATE workspaces SET state='stopped',generation=? WHERE id=?`, next, w.ID); err != nil {
		return err
	}
	if _, err = tx.Exec(`UPDATE slots SET generation=? WHERE id=? AND state='assigned'`, next, w.SlotID); err != nil {
		return err
	}
	if _, err = tx.Exec(`INSERT INTO recovery_audit VALUES(?,?,?,?,?)`, id, expectedHash, w.ID, next, stamp(c.Store.Now())); err != nil {
		return err
	}
	if _, err = tx.Exec(`INSERT INTO snapshot_outbox SELECT ?,activation FROM deployment WHERE 1 ON CONFLICT(workspace_id) DO UPDATE SET activation=excluded.activation`, w.ID); err != nil {
		return err
	}
	return tx.Commit()
}

type RetentionNotice struct {
	WorkspaceID  string `json:"workspace_id"`
	LastActivity string `json:"last_activity"`
	NotifiedAt   string `json:"notified_at"`
	DeleteAfter  string `json:"delete_after"`
}

// WarnRetention records a minimum three-day warning, including after a long
// intentionally offline period. Returning notices does not itself prove email
// delivery; the portal presents them and the operator must preserve that record.
func (s *Store) WarnRetention() ([]RetentionNotice, error) {
	tx, err := s.DB.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	now := s.Now()
	if _, err = tx.Exec(`UPDATE slots SET retained_at=? WHERE state='retained' AND retained_at=''`, stamp(now)); err != nil {
		return nil, err
	}
	if _, err = tx.Exec(`DELETE FROM retention_notices WHERE workspace_id IN (SELECT id FROM workspaces WHERE state='running' OR last_activity!=retention_notices.last_activity)`); err != nil {
		return nil, err
	}
	rows, err := tx.Query(`SELECT id,last_activity FROM workspaces WHERE state='stopped' AND last_activity<=? AND id NOT IN(SELECT workspace_id FROM retention_notices)`, stamp(now.Add(-27*24*time.Hour)))
	if err != nil {
		return nil, err
	}
	var pending []RetentionNotice
	for rows.Next() {
		var n RetentionNotice
		if err = rows.Scan(&n.WorkspaceID, &n.LastActivity); err != nil {
			rows.Close()
			return nil, err
		}
		last, e := time.Parse(time.RFC3339Nano, n.LastActivity)
		if e != nil {
			rows.Close()
			return nil, e
		}
		due := last.Add(30 * 24 * time.Hour)
		if minimum := now.Add(3 * 24 * time.Hour); minimum.After(due) {
			due = minimum
		}
		n.NotifiedAt, n.DeleteAfter = stamp(now), stamp(due)
		pending = append(pending, n)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	for _, n := range pending {
		if _, err = tx.Exec(`INSERT INTO retention_notices VALUES(?,?,?,?)`, n.WorkspaceID, n.LastActivity, n.NotifiedAt, n.DeleteAfter); err != nil {
			return nil, err
		}
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return s.RetentionNotices()
}
func (s *Store) RetentionNotices() ([]RetentionNotice, error) {
	rows, err := s.DB.Query(`SELECT workspace_id,last_activity,notified_at,delete_after FROM retention_notices ORDER BY delete_after`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []RetentionNotice{}
	for rows.Next() {
		var n RetentionNotice
		if err = rows.Scan(&n.WorkspaceID, &n.LastActivity, &n.NotifiedAt, &n.DeleteAfter); err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

type ReclaimPlan struct {
	Protocol       string `json:"protocol"`
	ID             string `json:"id"`
	DeploymentID   string `json:"deployment_id"`
	WorkspaceID    string `json:"workspace_id"`
	Generation     int64  `json:"generation"`
	SlotID         string `json:"slot_id"`
	Path           string `json:"path"`
	FilesystemUUID string `json:"filesystem_uuid"`
	Mode           string `json:"mode"`
	CreatedAt      string `json:"created_at"`
}
type ReclaimReview struct {
	Plan   ReclaimPlan `json:"plan"`
	SHA256 string      `json:"sha256"`
}
type ReclaimReceipt struct {
	Protocol       string `json:"protocol"`
	ID             string `json:"id"`
	PlanSHA256     string `json:"plan_sha256"`
	FilesystemUUID string `json:"filesystem_uuid"`
	Empty          bool   `json:"empty"`
	CompletedAt    string `json:"completed_at"`
}

// Remove discards only the exact stopped, template-verified container. Volume
// removal and forced deletion are never requested.
func (d *Docker) Remove(ctx context.Context, w Workspace) error {
	exists, running, err := d.Inspect(ctx, w)
	if err != nil {
		return err
	}
	if !exists {
		return nil
	}
	if running {
		return errors.New("running generation cannot be reclaimed")
	}
	status, _, err := d.request(ctx, "DELETE", "/containers/"+name(w)+"?force=false&v=false", nil)
	if err != nil {
		return err
	}
	if status != 204 && status != 404 {
		return errors.New("runtime removal uncertain")
	}
	exists, _, err = d.Inspect(ctx, w)
	if err != nil {
		return err
	}
	if exists {
		return errors.New("runtime removal not verified")
	}
	return nil
}

// RecordedWorkspace recovers the old grant version from its immutable template;
// a current gateway grant is deliberately not used to authorize an old mount.
func (s *Store) RecordedWorkspace(current Workspace, generation int64, slot string) (Workspace, error) {
	w := current
	w.Generation, w.SlotID = generation, slot
	var raw, hash string
	err := s.DB.QueryRow(`SELECT payload,sha256 FROM runtime_templates WHERE workspace_id=? AND generation=?`, w.ID, generation).Scan(&raw, &hash)
	if errors.Is(err, sql.ErrNoRows) {
		return w, nil
	}
	if err != nil {
		return w, err
	}
	sum := sha256.Sum256([]byte(raw))
	if hex.EncodeToString(sum[:]) != hash {
		return w, errors.New("recorded template hash mismatch")
	}
	var template struct{ Labels map[string]string }
	if json.Unmarshal([]byte(raw), &template) != nil {
		return w, errors.New("invalid recorded template")
	}
	version, err := strconv.ParseInt(template.Labels["feam.grant_version"], 10, 64)
	if err != nil || version < 1 {
		return w, errors.New("old generation grant version missing")
	}
	w.AssignmentVersion = version
	return w, nil
}

// PrepareReclaim requires a stopped site and preserves a durable exact scope
// before any root helper may remove bytes. It never frees a slot on its own.
func (c *Controller) PrepareReclaim(ctx context.Context, slot Slot) (ReclaimReview, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	var out ReclaimReview
	var desired string
	if err := c.Store.DB.QueryRow(`SELECT desired FROM deployment`).Scan(&desired); err != nil || desired != "stopped" {
		return out, errors.New("stop deployment intent before slot reclamation")
	}
	var state, owner, retained string
	var generation int64
	if err := c.Store.DB.QueryRow(`SELECT state,workspace_id,generation,retained_at FROM slots WHERE id=?`, slot.ID).Scan(&state, &owner, &generation, &retained); err != nil {
		return out, err
	}
	var pending string
	if err := c.Store.DB.QueryRow(`SELECT id FROM slot_reclaims WHERE slot_id=? AND state='pending'`, slot.ID).Scan(&pending); err == nil {
		return c.Store.ReclaimReview(pending)
	} else if !errors.Is(err, sql.ErrNoRows) {
		return out, err
	}
	w, err := c.Store.Workspace(owner)
	if err != nil {
		return out, err
	}
	if w.State != "stopped" && w.State != "deleted" {
		return out, errors.New("workspace must be drained and stopped")
	}
	var jobs int
	if err = c.Store.DB.QueryRow(`SELECT count(*) FROM lifecycle_jobs WHERE workspace_id=? AND state IN('accepted','executing','unknown')`, owner).Scan(&jobs); err != nil || jobs != 0 {
		return out, ErrConflict
	}
	mode := "retained"
	if state == "assigned" && w.SlotID == slot.ID {
		mode = "inactive"
		var last, due string
		if err = c.Store.DB.QueryRow(`SELECT last_activity,delete_after FROM retention_notices WHERE workspace_id=?`, owner).Scan(&last, &due); err != nil || last != w.LastActivity {
			return out, errors.New("inactive workspace has no current advance warning")
		}
		expiry, e := time.Parse(time.RFC3339Nano, due)
		if e != nil || c.Store.Now().Before(expiry) {
			return out, errors.New("retention warning period has not elapsed")
		}
	} else if state == "retained" {
		var confirmed int
		if err = c.Store.DB.QueryRow(`SELECT count(*) FROM lifecycle_jobs WHERE workspace_id=? AND (old_slot=? OR target_slot=?) AND action IN('workspace.reset','workspace.delete') AND state IN('completed','failed')`, owner, slot.ID, slot.ID).Scan(&confirmed); err != nil {
			return out, err
		}
		when, e := time.Parse(time.RFC3339Nano, retained)
		if confirmed == 0 && (e != nil || c.Store.Now().Before(when.Add(30*24*time.Hour))) {
			return out, errors.New("retained generation is not eligible for reclamation")
		}
	} else {
		return out, ErrConflict
	}
	if c.Capabilities != nil {
		if err = c.Capabilities.Revoke(ctx, w); err != nil {
			return out, err
		}
	}
	remover, ok := c.Runtime.(interface {
		Remove(context.Context, Workspace) error
	})
	if !ok {
		return out, errors.New("verified container removal unavailable")
	}
	rows, err := c.Store.DB.Query(`SELECT old_generation FROM lifecycle_jobs WHERE workspace_id=? AND old_slot=? UNION SELECT target_generation FROM lifecycle_jobs WHERE workspace_id=? AND target_slot=?`, owner, slot.ID, owner, slot.ID)
	if err != nil {
		return out, err
	}
	generations := map[int64]bool{generation: true}
	for rows.Next() {
		var gen int64
		if err = rows.Scan(&gen); err != nil {
			rows.Close()
			return out, err
		}
		generations[gen] = true
		if len(generations) > 512 {
			rows.Close()
			return out, errors.New("generation history exceeds one cleanup batch")
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return out, err
	}
	for gen := range generations {
		old, e := c.Store.RecordedWorkspace(w, gen, slot.ID)
		if e != nil {
			return out, e
		}
		if err = c.stopVerified(ctx, old); err != nil {
			return out, err
		}
		if err = remover.Remove(ctx, old); err != nil {
			return out, err
		}
	}
	if !regexp.MustCompile(`^slot-(0[1-9]|1[0-2])$`).MatchString(slot.ID) || slot.Path != "/home/feam-service-data/"+slot.ID || slot.FilesystemUUID == "" {
		return out, errors.New("fixed recorded slot required")
	}
	plan := ReclaimPlan{"feam.slot-reclaim.v1", ID(), w.DeploymentID, w.ID, generation, slot.ID, slot.Path, slot.FilesystemUUID, mode, stamp(c.Store.Now())}
	raw, _ := json.Marshal(plan)
	sum := sha256.Sum256(raw)
	hash := hex.EncodeToString(sum[:])
	tx, err := c.Store.DB.BeginTx(ctx, nil)
	if err != nil {
		return out, err
	}
	defer tx.Rollback()
	if _, err = tx.Exec(`INSERT INTO slot_reclaims(id,slot_id,workspace_id,generation,mode,payload,sha256,state) VALUES(?,?,?,?,?,?,?,'pending')`, plan.ID, slot.ID, w.ID, generation, mode, string(raw), hash); err != nil {
		return out, err
	}
	if mode == "inactive" {
		if _, err = tx.Exec(`UPDATE workspaces SET state='cleanup' WHERE id=?`, w.ID); err != nil {
			return out, err
		}
	}
	if err = tx.Commit(); err != nil {
		return out, err
	}
	return ReclaimReview{plan, hash}, nil
}
func (s *Store) ReclaimReview(id string) (ReclaimReview, error) {
	var out ReclaimReview
	var raw string
	err := s.DB.QueryRow(`SELECT payload,sha256 FROM slot_reclaims WHERE id=?`, id).Scan(&raw, &out.SHA256)
	if err != nil {
		return out, err
	}
	sum := sha256.Sum256([]byte(raw))
	if hex.EncodeToString(sum[:]) != out.SHA256 {
		return out, errors.New("reclaim scope changed")
	}
	err = json.Unmarshal([]byte(raw), &out.Plan)
	return out, err
}
func (c *Controller) CompleteReclaim(id string) error {
	if !uuidRE.MatchString(id) {
		return ErrConflict
	}
	path := filepath.Join("/run/feam/reclamation", id+".json")
	var parent unix.Stat_t
	if err := unix.Lstat(filepath.Dir(path), &parent); err != nil || parent.Mode&unix.S_IFMT != unix.S_IFDIR || parent.Uid != 0 || parent.Mode&0022 != 0 {
		return errors.New("root-owned reclamation receipt directory required")
	}
	// Use fstat on an O_NOFOLLOW descriptor, not a pathname permission check.
	fd, err := unix.Open(path, unix.O_RDONLY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	if err != nil {
		return err
	}
	f := os.NewFile(uintptr(fd), path)
	defer f.Close()
	var stat unix.Stat_t
	if err = unix.Fstat(fd, &stat); err != nil || stat.Mode&unix.S_IFMT != unix.S_IFREG || stat.Uid != 0 || stat.Mode&0022 != 0 || stat.Size > 4096 {
		return errors.New("root-owned bounded reclamation receipt required")
	}
	var receipt ReclaimReceipt
	decoder := json.NewDecoder(f)
	decoder.DisallowUnknownFields()
	if err = decoder.Decode(&receipt); err != nil {
		return err
	}
	if decoder.Decode(new(any)) != io.EOF {
		return errors.New("trailing reclamation receipt data")
	}
	return c.completeReclaim(receipt)
}
func (c *Controller) completeReclaim(receipt ReclaimReceipt) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	review, err := c.Store.ReclaimReview(receipt.ID)
	if err != nil {
		return err
	}
	if receipt.Protocol != "feam.slot-reclaimed.v1" || receipt.PlanSHA256 != review.SHA256 || receipt.FilesystemUUID != review.Plan.FilesystemUUID || !receipt.Empty {
		return ErrConflict
	}
	if _, err = time.Parse(time.RFC3339Nano, receipt.CompletedAt); err != nil {
		return err
	}
	raw, _ := json.Marshal(receipt)
	sum := sha256.Sum256(raw)
	tx, err := c.Store.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var state string
	if err = tx.QueryRow(`SELECT state FROM slot_reclaims WHERE id=?`, receipt.ID).Scan(&state); err != nil {
		return err
	}
	if state == "completed" {
		return nil
	}
	if state != "pending" {
		return ErrConflict
	}
	p := review.Plan
	var slotState, slotOwner string
	var slotGeneration int64
	if err = tx.QueryRow(`SELECT state,workspace_id,generation FROM slots WHERE id=?`, p.SlotID).Scan(&slotState, &slotOwner, &slotGeneration); err != nil || slotOwner != p.WorkspaceID || slotGeneration != p.Generation {
		return ErrConflict
	}
	if p.Mode == "inactive" {
		if slotState != "assigned" {
			return ErrConflict
		}
		result, e := tx.Exec(`UPDATE workspaces SET generation=generation+1,state='stopped',last_activity=?,warning_at='' WHERE id=? AND state IN ('cleanup','stopped') AND generation=?`, stamp(c.Store.Now()), p.WorkspaceID, p.Generation)
		if e != nil {
			return e
		}
		changed, e := result.RowsAffected()
		if e != nil || changed != 1 {
			return ErrConflict
		}
		if _, err = tx.Exec(`UPDATE slots SET generation=generation+1,retained_at='' WHERE id=? AND state='assigned'`, p.SlotID); err != nil {
			return err
		}
		if _, err = tx.Exec(`DELETE FROM retention_notices WHERE workspace_id=?`, p.WorkspaceID); err != nil {
			return err
		}
		if _, err = tx.Exec(`INSERT INTO snapshot_outbox SELECT ?,activation FROM deployment WHERE 1 ON CONFLICT(workspace_id) DO UPDATE SET activation=excluded.activation`, p.WorkspaceID); err != nil {
			return err
		}
	} else {
		if p.Mode != "retained" || slotState != "retained" {
			return ErrConflict
		}
		if _, err = tx.Exec(`UPDATE workspaces SET slot_id=NULL WHERE id=? AND slot_id=? AND state='deleted'`, p.WorkspaceID, p.SlotID); err != nil {
			return err
		}
		if _, err = tx.Exec(`UPDATE slots SET state='free',kind='spare',workspace_id=NULL,generation=NULL,retained_at='' WHERE id=? AND state='retained'`, p.SlotID); err != nil {
			return err
		}
	}
	if _, err = tx.Exec(`UPDATE slot_reclaims SET state='completed',receipt_sha256=? WHERE id=?`, hex.EncodeToString(sum[:]), receipt.ID); err != nil {
		return err
	}
	return tx.Commit()
}
