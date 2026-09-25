package lifecycle

import "time"

// Operation is called only by the broker identity after dispatch authorization.
// A request cannot extend its own lease, and a delayed completion cannot cancel
// a newer request. Expiry bounds the effect of a lost broker completion.
func (s *Store) Operation(workspace, request string, active bool) error {
	if !uuidRE.MatchString(workspace) || !uuidRE.MatchString(request) {
		return ErrAuthority
	}
	tx, e := s.DB.Begin()
	if e != nil {
		return e
	}
	defer tx.Rollback()
	if active {
		var state string
		if e = tx.QueryRow(`SELECT state FROM workspaces WHERE id=?`, workspace).Scan(&state); e != nil || state != "running" {
			return ErrAuthority
		}
		var previous string
		_ = tx.QueryRow(`SELECT request_id FROM activity_leases WHERE workspace_id=?`, workspace).Scan(&previous)
		if previous == request {
			return nil
		}
		if _, e = tx.Exec(`INSERT INTO activity_leases VALUES(?,?,?) ON CONFLICT(workspace_id) DO UPDATE SET request_id=excluded.request_id,expires_at=excluded.expires_at`, workspace, request, stamp(s.Now().Add(3*time.Minute))); e != nil {
			return e
		}
		if _, e = tx.Exec(`UPDATE workspaces SET last_activity=?,warning_at='' WHERE id=?`, stamp(s.Now()), workspace); e != nil {
			return e
		}
	} else {
		if _, e = tx.Exec(`DELETE FROM activity_leases WHERE workspace_id=? AND request_id=?`, workspace, request); e != nil {
			return e
		}
	}
	return tx.Commit()
}
