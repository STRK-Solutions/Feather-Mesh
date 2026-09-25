// Package control is the gateway-owned authorization database. Other services
// query its authenticated Unix API and never open this SQLite file.
package control

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/STRK-Solutions/Feather-Mesh/web_demo/internal/db"
	"net/mail"
	"regexp"
	"strings"
	"time"
)

var ErrForbidden = errors.New("authorization denied")
var ErrConflict = errors.New("stale version or conflicting identity")
var uuidRE = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

func ValidID(s string) bool { return uuidRE.MatchString(s) }
func ID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err)
	}
	b[6] = (b[6] & 15) | 64
	b[8] = (b[8] & 63) | 128
	s := hex.EncodeToString(b[:])
	return s[:8] + "-" + s[8:12] + "-" + s[12:16] + "-" + s[16:20] + "-" + s[20:]
}
func Email(s string) (string, error) {
	if s != strings.TrimSpace(s) || s != strings.ToLower(s) {
		return "", errors.New("email must use canonical lowercase spelling")
	}
	a, e := mail.ParseAddress(s)
	if e != nil || a.Address != s || !strings.Contains(s, "@") {
		return "", errors.New("invalid exact email")
	}
	return s, nil
}
func now() string { return time.Now().UTC().Format(time.RFC3339Nano) }

var Migrations = []db.Migration{{Version: 1, SQL: `
CREATE TABLE metadata(key TEXT PRIMARY KEY,value TEXT NOT NULL);
INSERT INTO metadata VALUES('policy_revision','1');
CREATE TABLE accounts(id TEXT PRIMARY KEY,email TEXT NOT NULL UNIQUE,subject TEXT UNIQUE,role TEXT NOT NULL CHECK(role IN ('admin','user')),status TEXT NOT NULL CHECK(status IN ('pending','active','disabled')),auth_version INTEGER NOT NULL CHECK(auth_version>0),edge_ready INTEGER NOT NULL DEFAULT 0,created_at TEXT NOT NULL,updated_at TEXT NOT NULL);
CREATE TRIGGER immutable_account_id BEFORE UPDATE OF id ON accounts BEGIN SELECT RAISE(ABORT,'immutable account ID'); END;
CREATE TABLE workspaces(id TEXT PRIMARY KEY,owner_id TEXT NOT NULL UNIQUE REFERENCES accounts(id),hostname TEXT NOT NULL UNIQUE,socket TEXT NOT NULL,ready INTEGER NOT NULL DEFAULT 0,generation INTEGER NOT NULL CHECK(generation>0),grant_version INTEGER NOT NULL CHECK(grant_version>0),deployment_id TEXT NOT NULL,activation_generation INTEGER NOT NULL CHECK(activation_generation>0));
CREATE TABLE release_assignments(id TEXT PRIMARY KEY,account_id TEXT NOT NULL REFERENCES accounts(id),bundle TEXT NOT NULL,release_digest TEXT NOT NULL,grant_version INTEGER NOT NULL,status TEXT NOT NULL,actor_id TEXT NOT NULL,effective_at TEXT NOT NULL);
CREATE UNIQUE INDEX live_release_assignment ON release_assignments(account_id,bundle) WHERE status='active';
CREATE TABLE grants(id TEXT PRIMARY KEY,account_id TEXT NOT NULL REFERENCES accounts(id),bundle TEXT NOT NULL,release_digest TEXT NOT NULL,version INTEGER NOT NULL,status TEXT NOT NULL CHECK(status IN ('active','revoked')),actor_id TEXT NOT NULL,effective_at TEXT NOT NULL);
CREATE UNIQUE INDEX live_grant ON grants(account_id,bundle) WHERE status='active';
CREATE TABLE policy_jobs(id TEXT PRIMARY KEY,idempotency_key TEXT NOT NULL UNIQUE,request_hash TEXT NOT NULL,revision INTEGER NOT NULL,status TEXT NOT NULL,error TEXT NOT NULL DEFAULT '',created_at TEXT NOT NULL);
CREATE TABLE revocations(account_id TEXT PRIMARY KEY REFERENCES accounts(id),auth_version INTEGER NOT NULL,status TEXT NOT NULL DEFAULT 'pending',updated_at TEXT NOT NULL);
CREATE TABLE grant_updates(workspace_id TEXT PRIMARY KEY REFERENCES workspaces(id),grant_version INTEGER NOT NULL,status TEXT NOT NULL DEFAULT 'pending');
CREATE TABLE audit(id TEXT PRIMARY KEY,actor_id TEXT NOT NULL,target_id TEXT NOT NULL,action TEXT NOT NULL,outcome TEXT NOT NULL,created_at TEXT NOT NULL);
`}}

type Store struct{ DB *sql.DB }
type Account struct {
	ID          string `json:"id"`
	Email       string `json:"email"`
	Subject     string `json:"subject,omitempty"`
	Role        string `json:"role"`
	Status      string `json:"status"`
	AuthVersion int64  `json:"auth_version"`
	EdgeReady   bool   `json:"edge_ready"`
}
type Workspace struct {
	ID                   string `json:"id"`
	OwnerID              string `json:"owner_id"`
	Hostname             string `json:"hostname"`
	Socket               string `json:"-"`
	Ready                bool   `json:"ready"`
	Generation           int64  `json:"generation"`
	GrantVersion         int64  `json:"grant_version"`
	DeploymentID         string `json:"deployment_id"`
	ActivationGeneration int64  `json:"activation_generation"`
}
type Enrollment struct {
	Email   string `json:"email"`
	Role    string `json:"role"`
	Subject string `json:"subject,omitempty"`
}

const accountSelect = `SELECT id,email,COALESCE(subject,''),role,status,auth_version,edge_ready FROM accounts`

func scanAccount(row interface{ Scan(...any) error }) (Account, error) {
	var a Account
	err := row.Scan(&a.ID, &a.Email, &a.Subject, &a.Role, &a.Status, &a.AuthVersion, &a.EdgeReady)
	return a, err
}
func (s *Store) Account(ctx context.Context, id string) (Account, error) {
	return scanAccount(s.DB.QueryRowContext(ctx, accountSelect+` WHERE id=?`, id))
}
func (s *Store) Accounts(ctx context.Context) ([]Account, error) {
	rows, e := s.DB.QueryContext(ctx, accountSelect+` ORDER BY email`)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	var out []Account
	for rows.Next() {
		a, e := scanAccount(rows)
		if e != nil {
			return nil, e
		}
		out = append(out, a)
	}
	return out, rows.Err()
}
func validateEnrollment(e Enrollment) error {
	if _, err := Email(e.Email); err != nil {
		return err
	}
	if e.Role != "admin" && e.Role != "user" {
		return errors.New("invalid role")
	}
	return nil
}

// Bootstrap is a one-time import. A changed or repeated roster never modifies
// existing policy. Disabled/deleted entries cannot be resurrected by converge.
func (s *Store) Bootstrap(ctx context.Context, roster []Enrollment) error {
	admins, users := 0, 0
	for _, e := range roster {
		if err := validateEnrollment(e); err != nil {
			return err
		}
		if e.Role == "admin" {
			admins++
		} else {
			users++
		}
	}
	if admins < 1 || admins > 4 || users < 1 || users > 10 {
		return errors.New("bootstrap requires one to four admins and one to ten users")
	}
	tx, e := s.DB.BeginTx(ctx, nil)
	if e != nil {
		return e
	}
	defer tx.Rollback()
	var present int
	if e = tx.QueryRow(`SELECT count(*) FROM metadata WHERE key='bootstrap_completed'`).Scan(&present); e != nil {
		return e
	}
	if present == 1 {
		return nil
	}
	for _, a := range roster {
		var sub any
		if a.Subject != "" {
			sub = a.Subject
		}
		if _, e = tx.Exec(`INSERT INTO accounts VALUES(?,?,?,?,'pending',1,0,?,?)`, ID(), a.Email, sub, a.Role, now(), now()); e != nil {
			return e
		}
	}
	if _, e = tx.Exec(`INSERT INTO metadata VALUES('bootstrap_completed',?)`, now()); e != nil {
		return e
	}
	return tx.Commit()
}
func (s *Store) Invite(ctx context.Context, actor string, in Enrollment) (Account, error) {
	if e := validateEnrollment(in); e != nil {
		return Account{}, e
	}
	a, e := s.Account(ctx, actor)
	if e != nil || a.Role != "admin" || a.Status != "active" {
		return Account{}, ErrForbidden
	}
	id := ID()
	var sub any
	if in.Subject != "" {
		sub = in.Subject
	}
	tx, e := s.DB.BeginTx(ctx, nil)
	if e != nil {
		return Account{}, e
	}
	defer tx.Rollback()
	if _, e = tx.Exec(`INSERT INTO accounts VALUES(?,?,?,?,'pending',1,0,?,?)`, id, in.Email, sub, in.Role, now(), now()); e != nil {
		return Account{}, e
	}
	if e = bump(tx); e != nil {
		return Account{}, e
	}
	if e = audit(tx, actor, id, "invite", "pending"); e != nil {
		return Account{}, e
	}
	if e = tx.Commit(); e != nil {
		return Account{}, e
	}
	return s.Account(ctx, id)
}
func (s *Store) Authenticate(ctx context.Context, email, subject string) (Account, error) {
	if _, e := Email(email); e != nil || subject == "" || len(subject) > 256 {
		return Account{}, ErrForbidden
	}
	tx, e := s.DB.BeginTx(ctx, nil)
	if e != nil {
		return Account{}, e
	}
	defer tx.Rollback()
	a, e := scanAccount(tx.QueryRow(accountSelect+` WHERE email=?`, email))
	if e != nil || a.Status != "active" || a.Subject != "" && a.Subject != subject {
		return Account{}, ErrForbidden
	}
	if a.Subject == "" {
		if _, e = tx.Exec(`UPDATE accounts SET subject=?,updated_at=? WHERE id=? AND subject IS NULL`, subject, now(), a.ID); e != nil {
			return Account{}, ErrForbidden
		}
		a.Subject = subject
	}
	if e = tx.Commit(); e != nil {
		return Account{}, e
	}
	return a, nil
}
func (s *Store) Assign(ctx context.Context, w Workspace) error {
	if !ValidID(w.ID) || !ValidID(w.OwnerID) || !ValidID(w.DeploymentID) || w.Generation < 1 || w.GrantVersion < 1 || w.ActivationGeneration < 1 || !strings.HasPrefix(w.Socket, "/") || !regexp.MustCompile(`^u-[0-9a-f-]+\.[a-z0-9.-]+$`).MatchString(w.Hostname) {
		return errors.New("invalid workspace assignment")
	}
	tx, e := s.DB.BeginTx(ctx, nil)
	if e != nil {
		return e
	}
	defer tx.Rollback()
	var role, status string
	if e = tx.QueryRow(`SELECT role,status FROM accounts WHERE id=?`, w.OwnerID).Scan(&role, &status); e != nil {
		return e
	}
	if role != "user" || status == "disabled" {
		return ErrForbidden
	}
	if _, e = tx.Exec(`INSERT INTO workspaces VALUES(?,?,?,?,?,?,?,?,?)`, w.ID, w.OwnerID, w.Hostname, w.Socket, w.Ready, w.Generation, w.GrantVersion, w.DeploymentID, w.ActivationGeneration); e != nil {
		return e
	}
	if _, e = tx.Exec(`UPDATE accounts SET status='active' WHERE id=? AND edge_ready=1 AND status='pending' AND ?=1`, w.OwnerID, w.Ready); e != nil {
		return e
	}
	return tx.Commit()
}

const workspaceSelect = `SELECT id,owner_id,hostname,socket,ready,generation,grant_version,deployment_id,activation_generation FROM workspaces`

func scanWorkspace(row interface{ Scan(...any) error }) (Workspace, error) {
	var w Workspace
	e := row.Scan(&w.ID, &w.OwnerID, &w.Hostname, &w.Socket, &w.Ready, &w.Generation, &w.GrantVersion, &w.DeploymentID, &w.ActivationGeneration)
	return w, e
}
func (s *Store) Workspace(ctx context.Context, id string) (Workspace, error) {
	return scanWorkspace(s.DB.QueryRowContext(ctx, workspaceSelect+` WHERE id=?`, id))
}
func (s *Store) WorkspaceForOwner(ctx context.Context, id string) (Workspace, error) {
	return scanWorkspace(s.DB.QueryRowContext(ctx, workspaceSelect+` WHERE owner_id=?`, id))
}
func (s *Store) WorkspaceForHost(ctx context.Context, host string) (Workspace, error) {
	return scanWorkspace(s.DB.QueryRowContext(ctx, workspaceSelect+` WHERE hostname=?`, host))
}
func (s *Store) Snapshot(ctx context.Context, w Workspace) error {
	r, e := s.DB.ExecContext(ctx, `UPDATE workspaces SET ready=?,generation=?,activation_generation=? WHERE id=? AND owner_id=? AND deployment_id=? AND generation<=? AND grant_version=? AND activation_generation=?`, w.Ready, w.Generation, w.ActivationGeneration, w.ID, w.OwnerID, w.DeploymentID, w.Generation, w.GrantVersion, w.ActivationGeneration)
	if e != nil {
		return e
	}
	n, e := r.RowsAffected()
	if e == nil && n != 1 {
		return ErrConflict
	}
	return e
}

type Check struct {
	AccountID            string `json:"account_id"`
	AuthVersion          int64  `json:"auth_version"`
	WorkspaceID          string `json:"workspace_id,omitempty"`
	WorkspaceGeneration  int64  `json:"workspace_generation,omitempty"`
	GrantVersion         int64  `json:"grant_version,omitempty"`
	DeploymentID         string `json:"deployment_id,omitempty"`
	ActivationGeneration int64  `json:"activation_generation,omitempty"`
}
type Snapshot struct {
	Account   Account    `json:"account"`
	Workspace *Workspace `json:"workspace,omitempty"`
}

func (s *Store) Authorize(ctx context.Context, c Check) (Snapshot, error) {
	a, e := s.Account(ctx, c.AccountID)
	if e != nil || a.Status != "active" || c.AuthVersion < 1 || a.AuthVersion != c.AuthVersion {
		return Snapshot{}, ErrForbidden
	}
	out := Snapshot{Account: a}
	if c.WorkspaceID != "" {
		w, e := s.Workspace(ctx, c.WorkspaceID)
		if e != nil || w.OwnerID != a.ID || !w.Ready || c.WorkspaceGeneration != 0 && c.WorkspaceGeneration != w.Generation || c.GrantVersion != 0 && c.GrantVersion != w.GrantVersion || c.DeploymentID != "" && c.DeploymentID != w.DeploymentID || c.ActivationGeneration != 0 && c.ActivationGeneration != w.ActivationGeneration {
			return Snapshot{}, ErrForbidden
		}
		out.Workspace = &w
	}
	return out, nil
}
func bump(tx *sql.Tx) error {
	_, e := tx.Exec(`UPDATE metadata SET value=CAST(value AS INTEGER)+1 WHERE key='policy_revision'`)
	return e
}
func audit(tx *sql.Tx, actor, target, action, outcome string) error {
	_, e := tx.Exec(`INSERT INTO audit VALUES(?,?,?,?,?,?)`, ID(), actor, target, action, outcome, now())
	return e
}
func (s *Store) Disable(ctx context.Context, actor, target string, version int64) error {
	tx, e := s.DB.BeginTx(ctx, nil)
	if e != nil {
		return e
	}
	defer tx.Rollback()
	a, e := scanAccount(tx.QueryRow(accountSelect+` WHERE id=?`, actor))
	if e != nil || a.Role != "admin" || a.Status != "active" {
		return ErrForbidden
	}
	r, e := tx.Exec(`UPDATE accounts SET status='disabled',edge_ready=0,auth_version=auth_version+1,updated_at=? WHERE id=? AND auth_version=? AND status!='disabled'`, now(), target, version)
	if e != nil {
		return e
	}
	n, _ := r.RowsAffected()
	if n != 1 {
		return ErrConflict
	}
	if _, e = tx.Exec(`INSERT INTO revocations VALUES(?,?,'pending',?) ON CONFLICT(account_id) DO UPDATE SET auth_version=excluded.auth_version,status='pending',updated_at=excluded.updated_at`, target, version+1, now()); e != nil {
		return e
	}
	if e = bump(tx); e != nil {
		return e
	}
	if e = audit(tx, actor, target, "disable", "local_revocation_committed"); e != nil {
		return e
	}
	return tx.Commit()
}

type Policy struct {
	Revision int64    `json:"revision"`
	Users    []string `json:"users"`
	Admins   []string `json:"admins"`
}

func (s *Store) Policy(ctx context.Context) (Policy, error) {
	tx, e := s.DB.BeginTx(ctx, nil)
	if e != nil {
		return Policy{}, e
	}
	defer tx.Rollback()
	var p Policy
	if e = tx.QueryRow(`SELECT CAST(value AS INTEGER) FROM metadata WHERE key='policy_revision'`).Scan(&p.Revision); e != nil {
		return p, e
	}
	rows, e := tx.Query(`SELECT email,role FROM accounts WHERE status!='disabled' ORDER BY email`)
	if e != nil {
		return p, e
	}
	for rows.Next() {
		var email, role string
		if e = rows.Scan(&email, &role); e != nil {
			rows.Close()
			return p, e
		}
		if role == "admin" {
			p.Admins = append(p.Admins, email)
		} else {
			p.Users = append(p.Users, email)
		}
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return p, e
	}
	return p, tx.Commit()
}
func (s *Store) SyncResult(ctx context.Context, revision int64, success bool) error {
	tx, e := s.DB.BeginTx(ctx, nil)
	if e != nil {
		return e
	}
	defer tx.Rollback()
	var cur int64
	if e = tx.QueryRow(`SELECT CAST(value AS INTEGER) FROM metadata WHERE key='policy_revision'`).Scan(&cur); e != nil {
		return e
	}
	if cur != revision {
		return ErrConflict
	}
	status, detail := "completed", ""
	if !success {
		status, detail = "failed", "edge membership synchronization failed"
	}
	if _, e = tx.Exec(`INSERT INTO policy_jobs VALUES(?,?,?,?,?,?,?) ON CONFLICT(idempotency_key) DO UPDATE SET status=excluded.status,error=excluded.error`, ID(), fmt.Sprintf("edge-%d", revision), fmt.Sprintf("revision-%d", revision), revision, status, detail, now()); e != nil {
		return e
	}
	if success {
		if _, e = tx.Exec(`UPDATE accounts SET edge_ready=1,status=CASE WHEN role='admin' OR EXISTS(SELECT 1 FROM workspaces WHERE owner_id=accounts.id AND ready=1) THEN 'active' ELSE 'pending' END WHERE status!='disabled'`); e != nil {
			return e
		}
	}
	return tx.Commit()
}
func (s *Store) PendingRevocations(ctx context.Context) ([]Account, error) {
	rows, e := s.DB.QueryContext(ctx, accountSelect+` WHERE id IN (SELECT account_id FROM revocations WHERE status='pending')`)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	var out []Account
	for rows.Next() {
		a, e := scanAccount(rows)
		if e != nil {
			return nil, e
		}
		out = append(out, a)
	}
	return out, rows.Err()
}
func (s *Store) RevokeComplete(ctx context.Context, id string, version int64) error {
	_, e := s.DB.ExecContext(ctx, `UPDATE revocations SET status='completed',updated_at=? WHERE account_id=? AND auth_version=?`, now(), id, version)
	return e
}

type Job struct {
	ID     string `json:"id"`
	Status string `json:"status"`
	Error  string `json:"error"`
}

func (s *Store) Jobs(ctx context.Context) ([]Job, error) {
	rows, e := s.DB.QueryContext(ctx, `SELECT id,status,error FROM policy_jobs ORDER BY created_at DESC LIMIT 100`)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	var out []Job
	for rows.Next() {
		var j Job
		if e = rows.Scan(&j.ID, &j.Status, &j.Error); e != nil {
			return nil, e
		}
		out = append(out, j)
	}
	return out, rows.Err()
}
