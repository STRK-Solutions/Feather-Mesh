// Package lifecycle owns finite workspace allocation and durable runtime intent.
// Browser input can select only an owned workspace and a fixed action.
package lifecycle

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"time"

	"github.com/STRK-Solutions/Feather-Mesh/web_demo/internal/db"
)

var uuidRE = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
var digestRE = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)
var ErrConflict = errors.New("conflict or stale generation")
var ErrCapacity = errors.New("finite capacity exhausted")
var ErrAuthority = errors.New("current authorization unavailable or denied")

var Migrations = []db.Migration{{Version: 1, SQL: `
CREATE TABLE deployment(id TEXT PRIMARY KEY, activation INTEGER NOT NULL CHECK(activation>0), desired TEXT NOT NULL CHECK(desired IN ('running','stopped')));
CREATE TABLE workspaces(id TEXT PRIMARY KEY,owner_id TEXT NOT NULL UNIQUE,hostname TEXT NOT NULL UNIQUE,generation INTEGER NOT NULL CHECK(generation>0),state TEXT NOT NULL,slot_id TEXT UNIQUE,image_id TEXT NOT NULL,assignment_version INTEGER NOT NULL,last_activity TEXT NOT NULL,warning_at TEXT NOT NULL DEFAULT '', FOREIGN KEY(slot_id) REFERENCES slots(id));
CREATE TABLE slots(id TEXT PRIMARY KEY,kind TEXT NOT NULL CHECK(kind IN ('workspace','spare')),state TEXT NOT NULL CHECK(state IN ('free','reserved','assigned','retained')),workspace_id TEXT,generation INTEGER);
CREATE TABLE lifecycle_jobs(id TEXT PRIMARY KEY,idempotency_key TEXT NOT NULL,actor_id TEXT NOT NULL,request_hash TEXT NOT NULL,workspace_id TEXT NOT NULL,request_json TEXT NOT NULL,old_generation INTEGER NOT NULL,target_generation INTEGER NOT NULL,old_slot TEXT,target_slot TEXT,action TEXT NOT NULL,state TEXT NOT NULL,outcome TEXT NOT NULL DEFAULT '',created_at TEXT NOT NULL,UNIQUE(actor_id,idempotency_key));
CREATE UNIQUE INDEX one_mutation ON lifecycle_jobs(workspace_id) WHERE state IN ('accepted','executing','unknown');
CREATE TABLE confirmations(hash TEXT PRIMARY KEY,actor_id TEXT NOT NULL,expires_at TEXT NOT NULL,used INTEGER NOT NULL DEFAULT 0);
CREATE TABLE activity_leases(workspace_id TEXT PRIMARY KEY,request_id TEXT NOT NULL,expires_at TEXT NOT NULL);
CREATE TABLE snapshot_outbox(workspace_id TEXT PRIMARY KEY,activation INTEGER NOT NULL);
CREATE TABLE revoked_accounts(account_id TEXT PRIMARY KEY);
CREATE TABLE runtime_templates(workspace_id TEXT NOT NULL,generation INTEGER NOT NULL,payload TEXT NOT NULL,sha256 TEXT NOT NULL,PRIMARY KEY(workspace_id,generation));
`}, {Version: 2, SQL: `
ALTER TABLE slots ADD COLUMN retained_at TEXT NOT NULL DEFAULT '';
CREATE TABLE recovery_audit(job_id TEXT PRIMARY KEY,request_hash TEXT NOT NULL,workspace_id TEXT NOT NULL,generation INTEGER NOT NULL,created_at TEXT NOT NULL);
CREATE TABLE retention_notices(workspace_id TEXT PRIMARY KEY,last_activity TEXT NOT NULL,notified_at TEXT NOT NULL,delete_after TEXT NOT NULL);
CREATE TABLE slot_reclaims(id TEXT PRIMARY KEY,slot_id TEXT NOT NULL,workspace_id TEXT NOT NULL,generation INTEGER NOT NULL,mode TEXT NOT NULL,payload TEXT NOT NULL,sha256 TEXT NOT NULL,state TEXT NOT NULL,receipt_sha256 TEXT NOT NULL DEFAULT '');
CREATE UNIQUE INDEX one_slot_reclaim ON slot_reclaims(slot_id) WHERE state='pending';
CREATE TRIGGER reclaim_blocks_jobs BEFORE INSERT ON lifecycle_jobs WHEN EXISTS(SELECT 1 FROM slot_reclaims WHERE workspace_id=NEW.workspace_id AND state='pending') BEGIN SELECT RAISE(ABORT,'slot cleanup pending'); END;
`}}

type Request struct {
	Protocol             string     `json:"protocol"`
	RequestID            string     `json:"request_id"`
	IdempotencyKey       string     `json:"idempotency_key"`
	DeploymentID         string     `json:"deployment_id"`
	ActivationGeneration int64      `json:"activation_generation"`
	WorkspaceID          string     `json:"workspace_id"`
	ExpectedGeneration   int64      `json:"expected_generation"`
	ActorID              string     `json:"actor_id"`
	AuthorizationVersion int64      `json:"authorization_version"`
	Method               string     `json:"method"`
	Parameters           Parameters `json:"parameters"`
}
type Parameters struct {
	ConfirmationHash string `json:"confirmation_hash,omitempty"`
}
type Result struct {
	Protocol  string `json:"protocol"`
	RequestID string `json:"request_id"`
	Status    string `json:"status"`
	JobID     string `json:"job_id,omitempty"`
	Error     string `json:"error,omitempty"`
}
type Workspace struct {
	ID                   string `json:"id"`
	OwnerID              string `json:"owner_id"`
	Hostname             string `json:"hostname"`
	DeploymentID         string `json:"deployment_id"`
	Generation           int64  `json:"generation"`
	State                string `json:"state"`
	SlotID               string `json:"slot_id"`
	ImageID              string `json:"image_id"`
	AssignmentVersion    int64  `json:"assignment_version"`
	LastActivity         string `json:"last_activity"`
	WarningAt            string `json:"warning_at"`
	RetentionDeleteAfter string `json:"retention_delete_after,omitempty"`
	RetentionNotified    string `json:"retention_notified,omitempty"`
}
type Job struct {
	ID, State, OldSlot, TargetSlot, Outcome string
	Request                                 Request
	TargetGeneration                        int64
}
type Store struct {
	DB  *sql.DB
	Now func() time.Time
}

func New(d *sql.DB) *Store { return &Store{DB: d, Now: time.Now} }
func ID() string {
	var b [16]byte
	if _, e := rand.Read(b[:]); e != nil {
		panic(e)
	}
	b[6] = (b[6] & 15) | 64
	b[8] = (b[8] & 63) | 128
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[:4], b[4:6], b[6:8], b[8:10], b[10:])
}
func stamp(t time.Time) string { return t.UTC().Format(time.RFC3339Nano) }
func Hash(r Request) string {
	r.Parameters = Parameters{}
	b, _ := json.Marshal(r)
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}
func ConfirmationHash(r Request) string { return Hash(r) }
func (r Request) Validate() error {
	if r.Protocol != "feam.web.v1" || !uuidRE.MatchString(r.RequestID) || !uuidRE.MatchString(r.IdempotencyKey) || !uuidRE.MatchString(r.DeploymentID) || !uuidRE.MatchString(r.WorkspaceID) || !uuidRE.MatchString(r.ActorID) || r.ActivationGeneration < 1 || r.ExpectedGeneration < 1 || r.AuthorizationVersion < 1 {
		return errors.New("invalid lifecycle envelope")
	}
	switch r.Method {
	case "workspace.start", "workspace.stop":
		if r.Parameters.ConfirmationHash != "" {
			return errors.New("unexpected confirmation")
		}
	case "workspace.reset", "workspace.delete":
		if r.Parameters.ConfirmationHash != Hash(r) {
			return errors.New("confirmation must bind the exact action")
		}
	default:
		return errors.New("unknown action")
	}
	return nil
}
func (s *Store) InitializeDeployment(id string, activation int64, slots []string) error {
	if !uuidRE.MatchString(id) || activation < 1 || len(slots) < 3 || len(slots) > 12 {
		return errors.New("invalid finite deployment")
	}
	tx, e := s.DB.Begin()
	if e != nil {
		return e
	}
	defer tx.Rollback()
	if _, e = tx.Exec(`INSERT INTO deployment VALUES(?,?,'stopped')`, id, activation); e != nil {
		return e
	}
	for i, id := range slots {
		if !regexp.MustCompile(`^slot-[0-9]{2}$`).MatchString(id) {
			return errors.New("invalid slot")
		}
		kind := "workspace"
		if i >= len(slots)-2 {
			kind = "spare"
		}
		if _, e = tx.Exec(`INSERT INTO slots(id,kind,state) VALUES(?,?,'free')`, id, kind); e != nil {
			return e
		}
	}
	return tx.Commit()
}
func (s *Store) Desired(running bool) error {
	v := "stopped"
	if running {
		v = "running"
	}
	res, e := s.DB.Exec(`UPDATE deployment SET desired=?`, v)
	if e != nil {
		return e
	}
	n, e := res.RowsAffected()
	if e == nil && n != 1 {
		return errors.New("deployment missing")
	}
	return e
}
func (s *Store) Assign(w Workspace) error {
	tx, e := s.DB.Begin()
	if e != nil {
		return e
	}
	defer tx.Rollback()
	if e = s.assign(tx, w); e != nil {
		return e
	}
	return tx.Commit()
}

func (s *Store) assign(tx *sql.Tx, w Workspace) error {
	if !uuidRE.MatchString(w.ID) || !uuidRE.MatchString(w.OwnerID) || !digestRE.MatchString(w.ImageID) || w.Generation != 1 || w.AssignmentVersion < 1 || w.Hostname != "u-"+w.ID+".613202690.xyz" {
		return errors.New("invalid fixed workspace assignment")
	}
	var slot string
	if e := tx.QueryRow(`SELECT id FROM slots WHERE kind='workspace' AND state='free' ORDER BY id LIMIT 1`).Scan(&slot); e != nil {
		return ErrCapacity
	}
	if _, e := tx.Exec(`INSERT INTO workspaces(id,owner_id,hostname,generation,state,slot_id,image_id,assignment_version,last_activity) VALUES(?,?,?,1,'stopped',?,?,?,?)`, w.ID, w.OwnerID, w.Hostname, slot, w.ImageID, w.AssignmentVersion, stamp(s.Now())); e != nil {
		return e
	}
	if _, e := tx.Exec(`UPDATE slots SET state='assigned',workspace_id=?,generation=1 WHERE id=?`, w.ID, slot); e != nil {
		return e
	}
	return nil
}
func (s *Store) Workspace(id string) (Workspace, error) {
	var w Workspace
	var slot sql.NullString
	e := s.DB.QueryRow(`SELECT w.id,owner_id,hostname,d.id,generation,state,slot_id,image_id,assignment_version,last_activity,warning_at FROM workspaces w CROSS JOIN deployment d WHERE w.id=?`, id).Scan(&w.ID, &w.OwnerID, &w.Hostname, &w.DeploymentID, &w.Generation, &w.State, &slot, &w.ImageID, &w.AssignmentVersion, &w.LastActivity, &w.WarningAt)
	w.SlotID = slot.String
	if e == nil {
		err := s.DB.QueryRow(`SELECT notified_at,delete_after FROM retention_notices WHERE workspace_id=? AND last_activity=?`, id, w.LastActivity).Scan(&w.RetentionNotified, &w.RetentionDeleteAfter)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return w, err
		}
	}
	return w, e
}
func (s *Store) Confirm(r Request) error {
	if r.Method != "workspace.reset" && r.Method != "workspace.delete" {
		return errors.New("confirmation only for destruction")
	}
	r.Parameters.ConfirmationHash = Hash(r)
	if e := r.Validate(); e != nil {
		return e
	}
	_, e := s.DB.Exec(`INSERT INTO confirmations(hash,actor_id,expires_at) VALUES(?,?,?)`, Hash(r), r.ActorID, stamp(s.Now().Add(5*time.Minute)))
	return e
}
func (s *Store) Admit(r Request) (Result, error) {
	return s.admit(r, false)
}

// Management is only selected after the controller validates the current admin
// through the gateway owner; it is never a client JSON field.
func (s *Store) admit(r Request, management bool) (Result, error) {
	out := Result{Protocol: "feam.web.v1", RequestID: r.RequestID}
	if e := r.Validate(); e != nil {
		return out, e
	}
	raw, _ := json.Marshal(r)
	h := sha256.Sum256(raw)
	hash := hex.EncodeToString(h[:])
	tx, e := s.DB.BeginTx(context.Background(), nil)
	if e != nil {
		return out, e
	}
	defer tx.Rollback()
	var oldHash, state string
	e = tx.QueryRow(`SELECT id,request_hash,state FROM lifecycle_jobs WHERE actor_id=? AND idempotency_key=?`, r.ActorID, r.IdempotencyKey).Scan(&out.JobID, &oldHash, &state)
	if e == nil {
		if hash != oldHash {
			return out, ErrConflict
		}
		out.Status = state
		return out, nil
	}
	if !errors.Is(e, sql.ErrNoRows) {
		return out, e
	}
	var dep, desired, owner, slot, image, currentState string
	var activation, generation, assignment int64
	e = tx.QueryRow(`SELECT d.id,d.activation,d.desired,w.owner_id,w.generation,w.slot_id,w.image_id,w.state,w.assignment_version FROM deployment d CROSS JOIN workspaces w WHERE w.id=?`, r.WorkspaceID).Scan(&dep, &activation, &desired, &owner, &generation, &slot, &image, &currentState, &assignment)
	if e != nil {
		return out, e
	}
	if dep != r.DeploymentID || activation != r.ActivationGeneration || generation != r.ExpectedGeneration || owner != r.ActorID && !management || currentState == "deleted" {
		return out, ErrConflict
	}
	if management && r.Method == "workspace.start" {
		return out, ErrAuthority
	}
	if r.Method == "workspace.start" || r.Method == "workspace.reset" {
		var revoked int
		if e = tx.QueryRow(`SELECT count(*) FROM revoked_accounts WHERE account_id=?`, r.ActorID).Scan(&revoked); e != nil {
			return out, e
		}
		if revoked != 0 {
			return out, ErrAuthority
		}
	}
	if desired != "running" && r.Method != "workspace.stop" && r.Method != "workspace.delete" {
		return out, ErrAuthority
	}
	target := slot
	next := generation
	if r.Method == "workspace.reset" || r.Method == "workspace.delete" {
		var exp string
		var used int
		e = tx.QueryRow(`SELECT expires_at,used FROM confirmations WHERE hash=? AND actor_id=?`, Hash(r), r.ActorID).Scan(&exp, &used)
		expiry, parse := time.Parse(time.RFC3339Nano, exp)
		if e != nil || parse != nil || used != 0 || !s.Now().Before(expiry) {
			return out, ErrAuthority
		}
		if _, e = tx.Exec(`UPDATE confirmations SET used=1 WHERE hash=?`, Hash(r)); e != nil {
			return out, e
		}
	}
	if r.Method == "workspace.reset" {
		next++
		if e = tx.QueryRow(`SELECT id FROM slots WHERE kind='spare' AND state='free' ORDER BY id LIMIT 1`).Scan(&target); e != nil {
			return out, ErrCapacity
		}
		if _, e = tx.Exec(`UPDATE slots SET state='reserved',workspace_id=?,generation=? WHERE id=?`, r.WorkspaceID, next, target); e != nil {
			return out, e
		}
	}
	out.JobID = ID()
	out.Status = "accepted"
	if _, e = tx.Exec(`INSERT INTO lifecycle_jobs(id,idempotency_key,actor_id,request_hash,workspace_id,request_json,old_generation,target_generation,old_slot,target_slot,action,state,created_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,'accepted',?)`, out.JobID, r.IdempotencyKey, r.ActorID, hash, r.WorkspaceID, string(raw), generation, next, slot, target, r.Method, stamp(s.Now())); e != nil {
		return out, ErrConflict
	}
	nextState := map[string]string{"workspace.start": "starting", "workspace.stop": "stopping", "workspace.reset": "resetting", "workspace.delete": "pending-delete"}[r.Method]
	if _, e = tx.Exec(`UPDATE workspaces SET state=? WHERE id=?`, nextState, r.WorkspaceID); e != nil {
		return out, e
	}
	return out, tx.Commit()
}
func (s *Store) Job(id string) (Job, error) {
	var j Job
	var raw string
	e := s.DB.QueryRow(`SELECT id,state,request_json,old_slot,target_slot,target_generation,outcome FROM lifecycle_jobs WHERE id=?`, id).Scan(&j.ID, &j.State, &raw, &j.OldSlot, &j.TargetSlot, &j.TargetGeneration, &j.Outcome)
	if e == nil {
		e = json.Unmarshal([]byte(raw), &j.Request)
	}
	return j, e
}
func (s *Store) Pending() ([]string, error) {
	rows, e := s.DB.Query(`SELECT id FROM lifecycle_jobs WHERE state IN ('accepted','executing','unknown') ORDER BY created_at`)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if e = rows.Scan(&id); e != nil {
			return nil, e
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}
func (s *Store) Mark(id, state, outcome string) error {
	_, e := s.DB.Exec(`UPDATE lifecycle_jobs SET state=?,outcome=? WHERE id=?`, state, outcome, id)
	return e
}
func (s *Store) Finish(j Job) error {
	tx, e := s.DB.Begin()
	if e != nil {
		return e
	}
	defer tx.Rollback()
	state := "running"
	switch j.Request.Method {
	case "workspace.stop":
		state = "stopped"
	case "workspace.delete":
		state = "deleted"
	case "workspace.reset":
		if _, e = tx.Exec(`UPDATE slots SET state='retained' WHERE id=?`, j.OldSlot); e != nil {
			return e
		}
		if _, e = tx.Exec(`UPDATE slots SET state='assigned' WHERE id=?`, j.TargetSlot); e != nil {
			return e
		}
	}
	if j.Request.Method == "workspace.delete" {
		if _, e = tx.Exec(`UPDATE slots SET state='retained' WHERE id=?`, j.OldSlot); e != nil {
			return e
		}
	}
	if _, e = tx.Exec(`UPDATE workspaces SET generation=?,state=?,slot_id=?,last_activity=? WHERE id=? AND generation=?`, j.TargetGeneration, state, j.TargetSlot, stamp(s.Now()), j.Request.WorkspaceID, j.Request.ExpectedGeneration); e != nil {
		return e
	}
	if _, e = tx.Exec(`UPDATE lifecycle_jobs SET state='completed',outcome='verified' WHERE id=?`, j.ID); e != nil {
		return e
	}
	if _, e = tx.Exec(`INSERT INTO snapshot_outbox VALUES(?,?) ON CONFLICT(workspace_id) DO UPDATE SET activation=excluded.activation`, j.Request.WorkspaceID, j.Request.ActivationGeneration); e != nil {
		return e
	}
	return tx.Commit()
}

// Activity must originate from accepted terminal input or an actual operation;
// WebSocket keepalive and page polling deliberately have no activity endpoint.
func (s *Store) Activity(id, kind string) error {
	if kind != "terminal_input" && kind != "operation" {
		return errors.New("heartbeat is not activity")
	}
	_, e := s.DB.Exec(`UPDATE workspaces SET last_activity=?,warning_at='' WHERE id=? AND state='running'`, stamp(s.Now()), id)
	return e
}

// Due marks an advance warning; it never deletes files or silently extends a
// broker operation. The caller stops idle containers with the same durable job.
func (s *Store) Due() ([]Workspace, error) {
	rows, e := s.DB.Query(`SELECT id FROM workspaces WHERE state='running'`)
	if e != nil {
		return nil, e
	}
	var ids []string
	for rows.Next() {
		var id string
		if e = rows.Scan(&id); e != nil {
			rows.Close()
			return nil, e
		}
		ids = append(ids, id)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return nil, e
	}
	var due []Workspace
	for _, id := range ids {
		w, e := s.Workspace(id)
		if e != nil {
			return nil, e
		}
		t, e := time.Parse(time.RFC3339Nano, w.LastActivity)
		if e != nil {
			return nil, e
		}
		idle := s.Now().Sub(t)
		if idle >= 25*time.Minute && w.WarningAt == "" {
			if _, e = s.DB.Exec(`UPDATE workspaces SET warning_at=? WHERE id=?`, stamp(s.Now()), id); e != nil {
				return nil, e
			}
		}
		if idle >= 30*time.Minute {
			var active int
			if e = s.DB.QueryRow(`SELECT count(*) FROM activity_leases WHERE workspace_id=? AND expires_at>?`, id, stamp(s.Now())).Scan(&active); e != nil {
				return nil, e
			}
			if active != 0 {
				continue
			}
			due = append(due, w)
		}
	}
	return due, nil
}
