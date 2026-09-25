package collector

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"github.com/STRK-Solutions/Feather-Mesh/web_demo/internal/db"
	"sort"
	"sync"
	"time"
)

var Migrations = []db.Migration{{Version: 1, SQL: `
CREATE TABLE events(position INTEGER PRIMARY KEY AUTOINCREMENT,id TEXT NOT NULL UNIQUE,participant TEXT NOT NULL,stream TEXT NOT NULL,sequence INTEGER NOT NULL,request TEXT NOT NULL,kind TEXT NOT NULL,trust TEXT NOT NULL,body TEXT NOT NULL,sha256 TEXT NOT NULL,received_at TEXT NOT NULL,batch_id TEXT,UNIQUE(stream,sequence));
CREATE INDEX events_participant ON events(participant,position);
CREATE TABLE streams(id TEXT PRIMARY KEY,participant TEXT NOT NULL,last_sequence INTEGER NOT NULL,gaps INTEGER NOT NULL);
CREATE TABLE quotas(participant TEXT PRIMARY KEY,window_start INTEGER NOT NULL,event_count INTEGER NOT NULL,byte_count INTEGER NOT NULL);
CREATE TABLE batches(id TEXT PRIMARY KEY,participant TEXT NOT NULL,object_key TEXT NOT NULL UNIQUE,body TEXT NOT NULL,sha256 TEXT NOT NULL,status TEXT NOT NULL,verified_at TEXT);
CREATE TABLE splits(dimension TEXT NOT NULL,value TEXT NOT NULL,split TEXT NOT NULL,PRIMARY KEY(dimension,value));
CREATE TABLE exports(id TEXT PRIMARY KEY,reviewer TEXT NOT NULL,participant TEXT NOT NULL,object_key TEXT NOT NULL,body TEXT NOT NULL,sha256 TEXT NOT NULL,status TEXT NOT NULL,created_at TEXT NOT NULL,expires_at TEXT NOT NULL);
CREATE TABLE deletions(participant TEXT PRIMARY KEY,reason TEXT NOT NULL,status TEXT NOT NULL,created_at TEXT NOT NULL);
CREATE TRIGGER events_immutable BEFORE UPDATE OF body,sha256,id,participant,stream,sequence ON events BEGIN SELECT RAISE(ABORT,'event records are immutable'); END;
`}, {Version: 2, SQL: `
ALTER TABLE batches ADD COLUMN archive_bytes INTEGER NOT NULL DEFAULT 0;
ALTER TABLE batches ADD COLUMN expires_at TEXT NOT NULL DEFAULT '';
UPDATE batches SET archive_bytes=length(CAST(body AS BLOB)),expires_at=strftime('%Y-%m-%dT%H:%M:%fZ',COALESCE((SELECT MAX(received_at) FROM events WHERE batch_id=batches.id),verified_at,'1970-01-01T00:00:00Z'),'+30 days');
ALTER TABLE exports ADD COLUMN archive_bytes INTEGER NOT NULL DEFAULT 0;
UPDATE exports SET archive_bytes=length(CAST(body AS BLOB));
ALTER TABLE deletions ADD COLUMN archive_bytes INTEGER NOT NULL DEFAULT 0;
UPDATE deletions SET archive_bytes=length(CAST(json_object('created_at',created_at,'participant_id',participant,'reason',reason) AS BLOB));
CREATE TABLE archive_imports(id INTEGER PRIMARY KEY CHECK(id=1),sha256 TEXT NOT NULL,imported_at TEXT NOT NULL);
`}}

const archiveUsageSQL = `SELECT (SELECT COALESCE(SUM(archive_bytes),0) FROM batches WHERE status!='deleted')+(SELECT COALESCE(SUM(archive_bytes),0) FROM exports WHERE status!='deleted')+(SELECT COALESCE(SUM(archive_bytes),0) FROM deletions)`

type Store struct {
	DB              *sql.DB
	MaxBytes        int64
	MaxArchiveBytes int64
	Now             func() time.Time
	mu              sync.Mutex
}

func New(d *sql.DB, maxBytes int64) *Store {
	return &Store{DB: d, MaxBytes: maxBytes, MaxArchiveBytes: 1 << 30, Now: time.Now}
}

type Ack struct {
	EventID  string `json:"event_id"`
	Sequence int64  `json:"sequence"`
	Status   string `json:"status"`
	Gap      bool   `json:"gap"`
}

func (s *Store) Append(ctx context.Context, e Event) (Ack, error) {
	ack := Ack{EventID: e.EventID, Sequence: e.Sequence, Status: "durable"}
	if e.Validate() != nil || !digestRE.MatchString(e.PayloadSHA256) {
		return ack, ErrRejected
	}
	payload, _ := json.Marshal(e.Payload)
	if digest(payload) != e.PayloadSHA256 {
		return ack, ErrRejected
	}
	b, err := json.Marshal(e)
	if err != nil {
		return ack, ErrRejected
	}
	hash := digest(b)
	s.mu.Lock()
	defer s.mu.Unlock()
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return ack, ErrBackpressure
	}
	defer tx.Rollback()
	var deleted int
	if err = tx.QueryRow(`SELECT count(*) FROM deletions WHERE participant=?`, e.ParticipantID).Scan(&deleted); err != nil || deleted != 0 {
		return ack, ErrRejected
	}
	var prior string
	err = tx.QueryRow(`SELECT sha256 FROM events WHERE id=? OR (stream=? AND sequence=?)`, e.EventID, e.StreamID, e.Sequence).Scan(&prior)
	if err == nil {
		if prior != hash {
			return ack, ErrRejected
		}
		var gaps int64
		if err = tx.QueryRow(`SELECT gaps FROM streams WHERE id=?`, e.StreamID).Scan(&gaps); err != nil {
			return ack, ErrBackpressure
		}
		ack.Gap = gaps != 0
		ack.Status = "duplicate"
		return ack, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return ack, ErrBackpressure
	}
	var total int64
	if err = tx.QueryRow(`SELECT (SELECT COALESCE(SUM(length(body)),0) FROM events)+(SELECT COALESCE(SUM(length(body)),0) FROM batches)+(SELECT COALESCE(SUM(length(body)),0) FROM exports)`).Scan(&total); err != nil || s.MaxBytes <= 0 || total+4*int64(len(b)) > s.MaxBytes {
		return ack, ErrBackpressure
	}
	var count, bytes, window int64
	err = tx.QueryRow(`SELECT window_start,event_count,byte_count FROM quotas WHERE participant=?`, e.ParticipantID).Scan(&window, &count, &bytes)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return ack, ErrBackpressure
	}
	now := s.Now().UTC()
	if now.Unix()-window >= 60 {
		window = now.Unix()
		count = 0
		bytes = 0
	}
	if count >= 120 || bytes+int64(len(b)) > 1<<20 {
		return ack, ErrBackpressure
	}
	var last int64
	var owner string
	err = tx.QueryRow(`SELECT participant,last_sequence FROM streams WHERE id=?`, e.StreamID).Scan(&owner, &last)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return ack, ErrBackpressure
	}
	if owner != "" && owner != e.ParticipantID || e.Sequence <= last {
		return ack, ErrRejected
	}
	ack.Gap = e.Sequence != last+1
	gap := 0
	if ack.Gap {
		gap = 1
	}
	if _, err = tx.Exec(`INSERT INTO events(id,participant,stream,sequence,request,kind,trust,body,sha256,received_at) VALUES(?,?,?,?,?,?,?,?,?,?)`, e.EventID, e.ParticipantID, e.StreamID, e.Sequence, e.RequestID, e.Kind, e.Trust, string(b), hash, now.Format(time.RFC3339Nano)); err != nil {
		return ack, ErrBackpressure
	}
	if _, err = tx.Exec(`INSERT INTO streams VALUES(?,?,?,?) ON CONFLICT(id) DO UPDATE SET last_sequence=excluded.last_sequence,gaps=streams.gaps+excluded.gaps`, e.StreamID, e.ParticipantID, e.Sequence, gap); err != nil {
		return ack, ErrBackpressure
	}
	if _, err = tx.Exec(`INSERT INTO quotas VALUES(?,?,?,?) ON CONFLICT(participant) DO UPDATE SET window_start=excluded.window_start,event_count=excluded.event_count,byte_count=excluded.byte_count`, e.ParticipantID, window, count+1, bytes+int64(len(b))); err != nil {
		return ack, ErrBackpressure
	}
	if err = tx.Commit(); err != nil {
		return ack, ErrBackpressure
	}
	return ack, nil
}

// Recover rebuilds only derived ordering/index state from immutable event rows.
// It never removes deletion floors, replenishes quotas or grants consent.
func (s *Store) Recover(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	tx, e := s.DB.BeginTx(ctx, nil)
	if e != nil {
		return e
	}
	defer tx.Rollback()
	rows, e := tx.Query(`SELECT body,sha256 FROM events ORDER BY position`)
	if e != nil {
		return e
	}
	type state struct {
		participant string
		last, gaps  int64
	}
	streams := map[string]state{}
	for rows.Next() {
		var b, h string
		if e = rows.Scan(&b, &h); e != nil {
			rows.Close()
			return e
		}
		var event Event
		if digest([]byte(b)) != h || json.Unmarshal([]byte(b), &event) != nil || event.Validate() != nil {
			rows.Close()
			return errors.New("event store integrity failure")
		}
		st := streams[event.StreamID]
		if st.participant != "" && st.participant != event.ParticipantID || event.Sequence <= st.last {
			rows.Close()
			return ErrRejected
		}
		if event.Sequence != st.last+1 {
			st.gaps++
		}
		st.last = event.Sequence
		st.participant = event.ParticipantID
		streams[event.StreamID] = st
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return e
	}
	if _, e = tx.Exec(`DELETE FROM streams`); e != nil {
		return e
	}
	keys := make([]string, 0, len(streams))
	for k := range streams {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		st := streams[k]
		if _, e = tx.Exec(`INSERT INTO streams VALUES(?,?,?,?)`, k, st.participant, st.last, st.gaps); e != nil {
			return e
		}
	}
	if _, e = tx.Exec(`REINDEX events_participant`); e != nil {
		return e
	}
	return tx.Commit()
}
func (s *Store) TeardownReady(ctx context.Context) error {
	var n int
	e := s.DB.QueryRowContext(ctx, `SELECT (SELECT count(*) FROM events WHERE batch_id IS NULL)+(SELECT count(*) FROM batches WHERE status NOT IN ('verified','deleted'))+(SELECT count(*) FROM exports WHERE status NOT IN ('verified','deleted'))+(SELECT count(*) FROM deletions WHERE status!='verified')`).Scan(&n)
	if e != nil || n != 0 {
		return errors.New("retained research has not been verified in independent archive")
	}
	return nil
}
