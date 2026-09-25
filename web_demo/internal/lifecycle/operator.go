package lifecycle

import (
	"context"
	"database/sql"
)

func quiescent(tx *sql.Tx) error {
	var desired string
	if err := tx.QueryRow(`SELECT desired FROM deployment`).Scan(&desired); err != nil {
		return err
	}
	var busy int
	if err := tx.QueryRow(`SELECT (SELECT count(*) FROM workspaces WHERE state NOT IN ('stopped','deleted')) + (SELECT count(*) FROM lifecycle_jobs WHERE state IN ('accepted','executing','unknown')) + (SELECT count(*) FROM slot_reclaims WHERE state='pending')`).Scan(&busy); err != nil {
		return err
	}
	if desired != "stopped" || busy != 0 {
		return ErrConflict
	}
	return nil
}

// AssignStopped extends an existing deployment without reinitializing it.
// The caller holds ProcessLock and supplies an exact configured assignment.
func (s *Store) AssignStopped(w Workspace) error {
	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	err = quiescent(tx)
	if err == nil {
		var deployment string
		err = tx.QueryRow(`SELECT id FROM deployment`).Scan(&deployment)
		if err == nil && deployment != w.DeploymentID {
			err = ErrConflict
		}
	}
	if err != nil {
		return err
	}
	if err = s.assign(tx, w); err != nil {
		return err
	}
	if _, err = tx.Exec(`INSERT INTO snapshot_outbox SELECT ?,activation FROM deployment`, w.ID); err != nil {
		return err
	}
	return tx.Commit()
}

// RestoreAccount removes only the controller's completed revocation barrier.
// Gateway restoration and actual edge reconciliation remain separate checks.
func (c *Controller) RestoreAccount(ctx context.Context, id string, generation int64) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	w, err := c.Store.Workspace(id)
	if err != nil {
		return err
	}
	if w.Generation != generation || w.State != "stopped" {
		return ErrConflict
	}
	_, running, err := c.Runtime.Inspect(ctx, w)
	if err != nil {
		return err
	}
	if running {
		return ErrConflict
	}
	tx, err := c.Store.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err = quiescent(tx); err != nil {
		return err
	}
	result, err := tx.Exec(`DELETE FROM revoked_accounts WHERE account_id=?`, w.OwnerID)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return ErrConflict
	}
	return tx.Commit()
}

// UpgradeImage gives the stopped workspace a fresh container generation while
// preserving its assigned volume, grants, and prior containers. A lost reply
// cannot advance it twice because both the old image and generation are bound.
func (c *Controller) UpgradeImage(ctx context.Context, id string, generation int64, oldImage, image string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !digestRE.MatchString(image) || image == oldImage {
		return ErrConflict
	}
	w, err := c.Store.Workspace(id)
	if err != nil {
		return err
	}
	if w.Generation != generation || w.ImageID != oldImage || w.State != "stopped" {
		return ErrConflict
	}
	_, running, err := c.Runtime.Inspect(ctx, w)
	if err != nil {
		return err
	}
	if running {
		return ErrConflict
	}
	tx, err := c.Store.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err = quiescent(tx); err != nil {
		return err
	}
	result, err := tx.Exec(`UPDATE workspaces SET image_id=?,generation=generation+1 WHERE id=? AND generation=? AND image_id=? AND state='stopped'`, image, id, generation, oldImage)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return ErrConflict
	}
	if _, err = tx.Exec(`UPDATE slots SET generation=? WHERE id=? AND state='assigned'`, generation+1, w.SlotID); err != nil {
		return err
	}
	if _, err = tx.Exec(`INSERT INTO snapshot_outbox SELECT ?,activation FROM deployment WHERE 1 ON CONFLICT(workspace_id) DO UPDATE SET activation=excluded.activation`, id); err != nil {
		return err
	}
	return tx.Commit()
}
