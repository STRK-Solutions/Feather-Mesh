package control

import (
	"context"
	"regexp"
)

type Grant struct {
	Bundle string `json:"bundle"`
	Digest string `json:"digest"`
}
type Assignments struct {
	Account   Account   `json:"account"`
	Workspace Workspace `json:"workspace"`
	Grants    []Grant   `json:"grants"`
}

type GrantUpdate struct {
	WorkspaceID  string `json:"workspace_id"`
	OwnerID      string `json:"-"`
	GrantVersion int64  `json:"grant_version"`
}

// Assignments is read only by authenticated services. Immutable release bytes
// and withdrawal floors are independently checked with the pipeline owner.
func (s *Store) Assignments(ctx context.Context, id string) (Assignments, error) {
	tx, e := s.DB.BeginTx(ctx, nil)
	if e != nil {
		return Assignments{}, e
	}
	defer tx.Rollback()
	var out Assignments
	out.Workspace, e = scanWorkspace(tx.QueryRow(workspaceSelect+` WHERE id=?`, id))
	if e != nil {
		return out, e
	}
	out.Account, e = scanAccount(tx.QueryRow(accountSelect+` WHERE id=?`, out.Workspace.OwnerID))
	if e != nil || out.Account.Status != "active" {
		return out, ErrForbidden
	}
	rows, e := tx.Query(`SELECT bundle,release_digest FROM grants WHERE account_id=? AND status='active' ORDER BY bundle`, out.Account.ID)
	if e != nil {
		return out, e
	}
	out.Grants = []Grant{}
	for rows.Next() {
		var g Grant
		if e = rows.Scan(&g.Bundle, &g.Digest); e != nil {
			rows.Close()
			return out, e
		}
		out.Grants = append(out.Grants, g)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return out, e
	}
	return out, tx.Commit()
}

// SetGrant is an audited account policy change, not a release publication. The
// gateway must verify an immutable current release with the pipeline first.
func (s *Store) SetGrant(ctx context.Context, actor, target, bundle, digest string, expected int64) error {
	if expected < 1 || !regexp.MustCompile(`^[a-z][a-z0-9-]{0,62}$`).MatchString(bundle) || digest != "" && !regexp.MustCompile(`^[0-9a-f]{64}$`).MatchString(digest) {
		return ErrConflict
	}
	tx, e := s.DB.BeginTx(ctx, nil)
	if e != nil {
		return e
	}
	defer tx.Rollback()
	admin, e := scanAccount(tx.QueryRow(accountSelect+` WHERE id=?`, actor))
	if e != nil || admin.Role != "admin" || admin.Status != "active" {
		return ErrForbidden
	}
	account, e := scanAccount(tx.QueryRow(accountSelect+` WHERE id=?`, target))
	if e != nil || account.Role != "user" || account.Status != "active" {
		return ErrForbidden
	}
	w, e := scanWorkspace(tx.QueryRow(workspaceSelect+` WHERE owner_id=?`, target))
	if e != nil || w.GrantVersion != expected {
		return ErrConflict
	}
	if _, e = tx.Exec(`UPDATE grants SET status='revoked' WHERE account_id=? AND bundle=? AND status='active'`, target, bundle); e != nil {
		return e
	}
	if digest != "" {
		if _, e = tx.Exec(`INSERT INTO grants(id,account_id,bundle,release_digest,version,status,actor_id,effective_at) VALUES(?,?,?,?,?,'active',?,?)`, ID(), target, bundle, digest, expected+1, actor, now()); e != nil {
			return e
		}
	}
	// Deny new terminal admission before the old mounted release is removed.
	if _, e = tx.Exec(`UPDATE workspaces SET grant_version=grant_version+1,ready=0 WHERE id=?`, w.ID); e != nil {
		return e
	}
	if _, e = tx.Exec(`UPDATE accounts SET auth_version=auth_version+1,updated_at=? WHERE id=?`, now(), target); e != nil {
		return e
	}
	if _, e = tx.Exec(`INSERT INTO grant_updates VALUES(?,?,'pending') ON CONFLICT(workspace_id) DO UPDATE SET grant_version=excluded.grant_version,status='pending'`, w.ID, expected+1); e != nil {
		return e
	}
	if e = audit(tx, actor, target, "dataset_grant_changed", bundle+":"+digest); e != nil {
		return e
	}
	return tx.Commit()
}

func (s *Store) PendingGrantUpdates(ctx context.Context) ([]GrantUpdate, error) {
	rows, e := s.DB.QueryContext(ctx, `SELECT g.workspace_id,w.owner_id,g.grant_version FROM grant_updates g JOIN workspaces w ON w.id=g.workspace_id WHERE g.status='pending' ORDER BY g.workspace_id`)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	var all []GrantUpdate
	for rows.Next() {
		var update GrantUpdate
		if e = rows.Scan(&update.WorkspaceID, &update.OwnerID, &update.GrantVersion); e != nil {
			return nil, e
		}
		all = append(all, update)
	}
	return all, rows.Err()
}

// A stale controller acknowledgment must never clear a newer policy change.
func (s *Store) GrantUpdateComplete(ctx context.Context, id string, version int64) error {
	r, e := s.DB.ExecContext(ctx, `UPDATE grant_updates SET status='completed' WHERE workspace_id=? AND grant_version=? AND status='pending'`, id, version)
	if e != nil {
		return e
	}
	n, e := r.RowsAffected()
	if e == nil && n != 1 {
		return ErrConflict
	}
	return e
}
func (s *Store) WorkspaceList(ctx context.Context) ([]Workspace, error) {
	rows, e := s.DB.QueryContext(ctx, workspaceSelect+` ORDER BY id`)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	var all []Workspace
	for rows.Next() {
		w, e := scanWorkspace(rows)
		if e != nil {
			return nil, e
		}
		all = append(all, w)
	}
	return all, rows.Err()
}

// ReleasePins includes old versions until every pending mount change is
// acknowledged. A disabled account's active grants are conservatively retained.
func (s *Store) ReleasePins(ctx context.Context) ([]Grant, error) {
	rows, e := s.DB.QueryContext(ctx, `SELECT DISTINCT g.bundle,g.release_digest FROM grants g JOIN workspaces w ON w.owner_id=g.account_id WHERE g.status='active' OR EXISTS(SELECT 1 FROM grant_updates u WHERE u.workspace_id=w.id AND u.status='pending') OR EXISTS(SELECT 1 FROM revocations r WHERE r.account_id=g.account_id AND r.status='pending') ORDER BY g.bundle,g.release_digest`)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	all := []Grant{}
	for rows.Next() {
		var v Grant
		if e = rows.Scan(&v.Bundle, &v.Digest); e != nil {
			return nil, e
		}
		all = append(all, v)
	}
	return all, rows.Err()
}

// RevokeBundleGrants is idempotent for a promotion whose response was lost.
// Stream polling observes auth_version immediately; the durable outbox removes
// mounts even when no dashboard request survives to trigger its first dispatch.
func (s *Store) RevokeBundleGrants(ctx context.Context, actor, bundle string) error {
	if !regexp.MustCompile(`^[a-z][a-z0-9-]{0,62}$`).MatchString(bundle) {
		return ErrConflict
	}
	tx, e := s.DB.BeginTx(ctx, nil)
	if e != nil {
		return e
	}
	defer tx.Rollback()
	a, e := scanAccount(tx.QueryRow(accountSelect+` WHERE id=?`, actor))
	if e != nil || a.Status != "active" || a.Role != "admin" {
		return ErrForbidden
	}
	rows, e := tx.Query(`SELECT DISTINCT w.id,w.owner_id,w.grant_version FROM grants g JOIN workspaces w ON w.owner_id=g.account_id WHERE g.bundle=? AND g.status='active'`, bundle)
	if e != nil {
		return e
	}
	var updates []GrantUpdate
	for rows.Next() {
		var u GrantUpdate
		if e = rows.Scan(&u.WorkspaceID, &u.OwnerID, &u.GrantVersion); e != nil {
			rows.Close()
			return e
		}
		updates = append(updates, u)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return e
	}
	if _, e = tx.Exec(`UPDATE grants SET status='revoked' WHERE bundle=? AND status='active'`, bundle); e != nil {
		return e
	}
	for _, u := range updates {
		if _, e = tx.Exec(`UPDATE workspaces SET ready=0,grant_version=grant_version+1 WHERE id=?`, u.WorkspaceID); e != nil {
			return e
		}
		if _, e = tx.Exec(`UPDATE accounts SET auth_version=auth_version+1,updated_at=? WHERE id=?`, now(), u.OwnerID); e != nil {
			return e
		}
		if _, e = tx.Exec(`INSERT INTO grant_updates VALUES(?,?,'pending') ON CONFLICT(workspace_id) DO UPDATE SET grant_version=excluded.grant_version,status='pending'`, u.WorkspaceID, u.GrantVersion+1); e != nil {
			return e
		}
		if e = audit(tx, actor, u.OwnerID, "dataset_bundle_withdrawn", bundle); e != nil {
			return e
		}
	}
	return tx.Commit()
}
