package budget

import (
	"context"
	"crypto/ed25519"
	"database/sql"
	"errors"
	"time"

	"github.com/STRK-Solutions/Feather-Mesh/web_demo/internal/db"
)

var runMigrations = []db.Migration{{Version: 1, SQL: `
CREATE TABLE run_allocations(id TEXT PRIMARY KEY,document_hash TEXT NOT NULL,amount INTEGER NOT NULL CHECK(amount>0),state TEXT NOT NULL CHECK(state IN ('active','paused')));
CREATE TABLE requests(id TEXT PRIMARY KEY,account_id TEXT NOT NULL,idempotency_key TEXT NOT NULL,request_hash TEXT NOT NULL,auth_version INTEGER NOT NULL,workspace_generation INTEGER NOT NULL,reserved INTEGER NOT NULL CHECK(reserved>0),state TEXT NOT NULL CHECK(state IN ('reserved','dispatched','unknown','settled')),provider_id TEXT NOT NULL DEFAULT '',cost INTEGER,created_at TEXT NOT NULL,reconciled_at TEXT,UNIQUE(account_id,idempotency_key));
`}}

type Run struct {
	DB       *sql.DB
	Document SignedAllocation
}
type Reservation struct {
	ID          string `json:"request_id"`
	Account     string `json:"account_id"`
	Key         string `json:"idempotency_key"`
	Hash        string `json:"request_hash"`
	AuthVersion int64  `json:"auth_version"`
	Generation  int64  `json:"workspace_generation"`
	Amount      int64  `json:"reserved_usd_micros"`
	State       string `json:"state"`
	ProviderID  string `json:"provider_id"`
	Cost        *int64 `json:"cost_usd_micros,omitempty"`
}
type Usage struct {
	Amount        int64 `json:"allocation_usd_micros"`
	Settled       int64 `json:"settled_usd_micros"`
	Reserved      int64 `json:"reserved_usd_micros"`
	Unknown       int64 `json:"unknown_usd_micros"`
	Alert         int   `json:"alert_percent"`
	Paused        bool  `json:"paused"`
	DailyLimit    int64 `json:"daily_limit_usd_micros,omitempty"`
	DailyExposure int64 `json:"daily_exposure_usd_micros,omitempty"`
}

func InitializeRun(path string, s SignedAllocation, key ed25519.PublicKey, deployment string, generation int64) (*Run, error) {
	if err := s.Verify(key, deployment, generation, time.Now()); err != nil {
		return nil, err
	}
	d, err := db.Initialize(path, runMigrations)
	if err != nil {
		return nil, err
	}
	if _, err = d.Exec(`INSERT INTO run_allocations VALUES(?,?,?,'active')`, s.Allocation.ID, s.Digest(), s.Allocation.Amount); err != nil {
		d.Close()
		return nil, err
	}
	return &Run{d, s}, nil
}
func OpenRun(path string, s SignedAllocation, key ed25519.PublicKey, deployment string, generation int64) (*Run, error) {
	if err := s.Verify(key, deployment, generation, time.Now()); err != nil {
		return nil, err
	}
	d, err := db.Open(path, runMigrations)
	if err != nil {
		return nil, err
	}
	var id, hash string
	var amount int64
	if err = d.QueryRow(`SELECT id,document_hash,amount FROM run_allocations`).Scan(&id, &hash, &amount); err != nil || id != s.Allocation.ID || hash != s.Digest() || amount != s.Allocation.Amount {
		d.Close()
		return nil, ErrInvalid
	}
	// A prior process may have sent bytes immediately before dying, even if it
	// did not record dispatch. Never replay or release any outstanding request.
	if _, err = d.Exec(`UPDATE requests SET state='unknown' WHERE state IN ('reserved','dispatched')`); err != nil {
		d.Close()
		return nil, err
	}
	return &Run{d, s}, nil
}
func (r *Run) Close() error { return r.DB.Close() }
func (r *Run) Reserve(ctx context.Context, v Reservation) (Reservation, bool, error) {
	if !ValidID(v.ID) || !ValidID(v.Account) || v.Key == "" || len(v.Key) > 128 || len(v.Hash) != 64 || v.AuthVersion < 1 || v.Generation < 1 || v.Amount < 1 || v.Amount > r.Document.Allocation.Amount {
		return Reservation{}, false, ErrInvalid
	}
	if !time.Now().Before(r.Document.Allocation.ExpiresAt) {
		return Reservation{}, false, ErrInvalid
	}
	if v.Amount > r.Document.Allocation.RequestLimit {
		return Reservation{}, false, ErrExhausted
	}
	tx, err := r.DB.BeginTx(ctx, nil)
	if err != nil {
		return Reservation{}, false, err
	}
	defer tx.Rollback()
	existing, err := scanRequest(tx.QueryRowContext(ctx, `SELECT id,account_id,idempotency_key,request_hash,auth_version,workspace_generation,reserved,state,provider_id,cost FROM requests WHERE account_id=? AND idempotency_key=?`, v.Account, v.Key))
	if err == nil {
		if existing.Hash != v.Hash {
			return existing, true, ErrConflict
		}
		return existing, true, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return Reservation{}, false, err
	}
	var exposure int64
	var state string
	if err = tx.QueryRowContext(ctx, `SELECT COALESCE(SUM(CASE WHEN state='settled' THEN cost ELSE reserved END),0) FROM requests`).Scan(&exposure); err != nil {
		return Reservation{}, false, err
	}
	if err = tx.QueryRowContext(ctx, `SELECT state FROM run_allocations WHERE id=?`, r.Document.Allocation.ID).Scan(&state); err != nil {
		return Reservation{}, false, err
	}
	if state != "active" || v.Amount > r.Document.Allocation.Amount-exposure {
		return Reservation{}, false, ErrExhausted
	}
	var daily int64
	day := time.Now().UTC().Format("2006-01-02") + "T00:00:00"
	if err = tx.QueryRowContext(ctx, `SELECT COALESCE(SUM(CASE WHEN state='settled' THEN cost ELSE reserved END),0) FROM requests WHERE account_id=? AND created_at>=?`, v.Account, day).Scan(&daily); err != nil {
		return Reservation{}, false, err
	}
	if v.Amount > r.Document.Allocation.UserDailyLimit-daily {
		return Reservation{}, false, ErrExhausted
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO requests(id,account_id,idempotency_key,request_hash,auth_version,workspace_generation,reserved,state,created_at) VALUES(?,?,?,?,?,?,?,'reserved',?)`, v.ID, v.Account, v.Key, v.Hash, v.AuthVersion, v.Generation, v.Amount, time.Now().UTC().Format(time.RFC3339Nano))
	if err != nil {
		return Reservation{}, false, err
	}
	if err = tx.Commit(); err != nil {
		return Reservation{}, false, err
	}
	v.State = "reserved"
	return v, false, nil
}
func scanRequest(row *sql.Row) (Reservation, error) {
	var v Reservation
	err := row.Scan(&v.ID, &v.Account, &v.Key, &v.Hash, &v.AuthVersion, &v.Generation, &v.Amount, &v.State, &v.ProviderID, &v.Cost)
	return v, err
}
func (r *Run) Lookup(ctx context.Context, account, id string) (Reservation, error) {
	return scanRequest(r.DB.QueryRowContext(ctx, `SELECT id,account_id,idempotency_key,request_hash,auth_version,workspace_generation,reserved,state,provider_id,cost FROM requests WHERE account_id=? AND id=?`, account, id))
}
func (r *Run) Dispatch(ctx context.Context, id string) error {
	res, err := r.DB.ExecContext(ctx, `UPDATE requests SET state='dispatched' WHERE id=? AND state='reserved'`, id)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return ErrConflict
	}
	return nil
}
func (r *Run) Unknown(id, providerID string) error {
	_, err := r.DB.Exec(`UPDATE requests SET state='unknown',provider_id=? WHERE id=? AND state IN ('reserved','dispatched','unknown')`, providerID, id)
	return err
}
func (r *Run) Settle(id, providerID string, cost int64) error {
	if cost < 0 || len(providerID) > 256 {
		return ErrInvalid
	}
	tx, err := r.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var reserved int64
	var state string
	var prior sql.NullInt64
	if err = tx.QueryRow(`SELECT reserved,state,cost FROM requests WHERE id=?`, id).Scan(&reserved, &state, &prior); err != nil {
		return err
	}
	if state == "settled" {
		if prior.Valid && prior.Int64 == cost {
			return nil
		}
		return ErrConflict
	}
	if cost > reserved {
		if _, err = tx.Exec(`UPDATE run_allocations SET state='paused'`); err != nil {
			return err
		}
		if err = tx.Commit(); err != nil {
			return err
		}
		return errors.New("reconciliation_required: provider charge exceeded reservation")
	}
	_, err = tx.Exec(`UPDATE requests SET state='settled',provider_id=?,cost=?,reconciled_at=? WHERE id=?`, providerID, cost, time.Now().UTC().Format(time.RFC3339Nano), id)
	if err != nil {
		return err
	}
	return tx.Commit()
}
func (r *Run) Usage(ctx context.Context, account string) (Usage, error) {
	u := Usage{Amount: r.Document.Allocation.Amount}
	query := `SELECT COALESCE(SUM(CASE WHEN state='settled' THEN cost ELSE 0 END),0),COALESCE(SUM(CASE WHEN state IN ('reserved','dispatched') THEN reserved ELSE 0 END),0),COALESCE(SUM(CASE WHEN state='unknown' THEN reserved ELSE 0 END),0) FROM requests`
	args := []any{}
	if account != "" {
		query += " WHERE account_id=?"
		args = append(args, account)
	}
	if err := r.DB.QueryRowContext(ctx, query, args...).Scan(&u.Settled, &u.Reserved, &u.Unknown); err != nil {
		return u, err
	}
	var state string
	if err := r.DB.QueryRowContext(ctx, `SELECT state FROM run_allocations`).Scan(&state); err != nil {
		return u, err
	}
	u.Paused = state != "active"
	if account != "" {
		u.DailyLimit = r.Document.Allocation.UserDailyLimit
		day := time.Now().UTC().Format("2006-01-02") + "T00:00:00"
		if err := r.DB.QueryRowContext(ctx, `SELECT COALESCE(SUM(CASE WHEN state='settled' THEN cost ELSE reserved END),0) FROM requests WHERE account_id=? AND created_at>=?`, account, day).Scan(&u.DailyExposure); err != nil {
			return u, err
		}
	}
	used := u.Settled + u.Reserved + u.Unknown
	if used*100 >= u.Amount*90 {
		u.Alert = 90
	} else if used*100 >= u.Amount*75 {
		u.Alert = 75
	}
	return u, nil
}
