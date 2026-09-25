package control

import "context"

// Restore is an explicit operator action, never a bootstrap/converge side
// effect. The old revocation must have finished before a fresh policy revision
// can make this identity eligible again. Roles, subject bindings and grants
// remain unchanged; old local authorization and CSRF versions remain invalid.
func (s *Store) Restore(ctx context.Context, actor, target string, version int64) error {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	a, err := scanAccount(tx.QueryRow(accountSelect+` WHERE id=?`, actor))
	if err != nil || a.Role != "admin" || a.Status != "active" {
		return ErrForbidden
	}
	result, err := tx.Exec(`UPDATE accounts SET status='pending',edge_ready=0,auth_version=auth_version+1,updated_at=? WHERE id=? AND status='disabled' AND auth_version=? AND EXISTS(SELECT 1 FROM revocations WHERE account_id=accounts.id AND auth_version=accounts.auth_version AND status='completed')`, now(), target, version)
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
	if err = bump(tx); err != nil {
		return err
	}
	if err = audit(tx, actor, target, "restore", "pending_edge_reconciliation"); err != nil {
		return err
	}
	return tx.Commit()
}
