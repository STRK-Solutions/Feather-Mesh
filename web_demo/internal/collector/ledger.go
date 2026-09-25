package collector

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"time"

	"github.com/STRK-Solutions/Feather-Mesh/web_demo/internal/control"
)

const MaxLedgerBytes = 1 << 20

type LedgerObject struct {
	ID            string `json:"id"`
	Kind          string `json:"kind"`
	ParticipantID string `json:"participant_id"`
	Key           string `json:"object_key"`
	Hash          string `json:"sha256"`
	State         string `json:"state"`
	Bytes         int64  `json:"bytes"`
	CreatedAt     string `json:"created_at"`
	ExpiresAt     string `json:"expires_at"`
	Reviewer      string `json:"reviewer"`
}
type LedgerDeletion struct {
	ParticipantID string `json:"participant_id"`
	Reason        string `json:"reason"`
	CreatedAt     string `json:"created_at"`
	ArchiveState  string `json:"archive_state"`
}
type LedgerSplit struct {
	Dimension string `json:"dimension"`
	Value     string `json:"value"`
	Split     string `json:"split"`
}
type ArchiveLedger struct {
	Protocol   string           `json:"protocol"`
	ExportedAt string           `json:"exported_at"`
	Objects    []LedgerObject   `json:"objects"`
	Deletions  []LedgerDeletion `json:"deletions"`
	Splits     []LedgerSplit    `json:"splits"`
	SHA256     string           `json:"sha256,omitempty"`
}

func ledgerHash(v ArchiveLedger) string { v.SHA256 = ""; b, _ := json.Marshal(v); return digest(b) }

func (s *Store) ledgerConnection(ctx context.Context) (*sql.Conn, error) {
	if _, ok := ctx.Deadline(); !ok {
		return nil, errors.New("archive ledger operation requires deadline")
	}
	c, e := s.DB.Conn(ctx)
	if e != nil {
		return nil, e
	}
	if _, e = c.ExecContext(ctx, `BEGIN IMMEDIATE`); e != nil {
		c.Close()
		return nil, e
	}
	return c, nil
}
func closeLedger(c *sql.Conn) { c.ExecContext(context.Background(), `ROLLBACK`); c.Close() }

func readLedger(ctx context.Context, c *sql.Conn, now time.Time) (ArchiveLedger, error) {
	l := ArchiveLedger{Protocol: "feam.archive-ledger.v1", ExportedAt: now.UTC().Format(time.RFC3339Nano), Objects: []LedgerObject{}, Deletions: []LedgerDeletion{}, Splits: []LedgerSplit{}}
	var pending int
	if e := c.QueryRowContext(ctx, `SELECT (SELECT count(*) FROM events WHERE batch_id IS NULL)+(SELECT count(*) FROM batches WHERE status NOT IN ('verified','deleted'))+(SELECT count(*) FROM exports WHERE status NOT IN ('verified','deleted'))+(SELECT count(*) FROM deletions WHERE status!='verified')`).Scan(&pending); e != nil || pending != 0 {
		return l, errors.New("archive ledger has pending research")
	}
	rows, e := c.QueryContext(ctx, `SELECT id,'batch',participant,object_key,sha256,status,archive_bytes,'',expires_at,'' FROM batches UNION ALL SELECT id,'export',participant,object_key,sha256,status,archive_bytes,created_at,expires_at,reviewer FROM exports ORDER BY 4`)
	if e != nil {
		return l, e
	}
	for rows.Next() {
		var v LedgerObject
		if e = rows.Scan(&v.ID, &v.Kind, &v.ParticipantID, &v.Key, &v.Hash, &v.State, &v.Bytes, &v.CreatedAt, &v.ExpiresAt, &v.Reviewer); e != nil {
			break
		}
		if v.Kind == "batch" {
			expiry, err := time.Parse(time.RFC3339Nano, v.ExpiresAt)
			if err != nil {
				e = err
				break
			}
			v.CreatedAt = expiry.Add(-30 * 24 * time.Hour).UTC().Format(time.RFC3339Nano)
		}
		l.Objects = append(l.Objects, v)
		if len(l.Objects) > 4000 {
			e = errors.New("archive ledger object bound exceeded")
			break
		}
	}
	if e == nil {
		e = rows.Err()
	}
	rows.Close()
	if e != nil {
		return l, e
	}
	rows, e = c.QueryContext(ctx, `SELECT participant,reason,created_at FROM deletions ORDER BY participant`)
	if e != nil {
		return l, e
	}
	for rows.Next() {
		var v LedgerDeletion
		if e = rows.Scan(&v.ParticipantID, &v.Reason, &v.CreatedAt); e != nil {
			break
		}
		v.ArchiveState = "verified"
		l.Deletions = append(l.Deletions, v)
		if len(l.Deletions) > 4000 {
			e = ErrRejected
			break
		}
	}
	if e == nil {
		e = rows.Err()
	}
	rows.Close()
	if e != nil {
		return l, e
	}
	rows, e = c.QueryContext(ctx, `SELECT dimension,value,split FROM splits ORDER BY dimension,value`)
	if e != nil {
		return l, e
	}
	for rows.Next() {
		var v LedgerSplit
		if e = rows.Scan(&v.Dimension, &v.Value, &v.Split); e != nil {
			break
		}
		l.Splits = append(l.Splits, v)
		if len(l.Splits) > 12000 {
			e = ErrRejected
			break
		}
	}
	if e == nil {
		e = rows.Err()
	}
	rows.Close()
	return l, e
}

func deletionObject(v LedgerDeletion) (string, []byte) {
	b, _ := json.Marshal(map[string]string{"participant_id": v.ParticipantID, "reason": v.Reason, "created_at": v.CreatedAt})
	return "research/deletions/" + v.ParticipantID + "/" + digest(b) + ".json", b
}

func validateLedger(l ArchiveLedger, now time.Time, checkHash bool) error {
	b, _ := json.Marshal(l)
	stamp, e := time.Parse(time.RFC3339Nano, l.ExportedAt)
	if len(b) > MaxLedgerBytes || l.Protocol != "feam.archive-ledger.v1" || e != nil || stamp.After(now.Add(5*time.Minute)) || len(l.Objects) > 4000 || len(l.Deletions) > 4000 || len(l.Splits) > 12000 {
		return ErrRejected
	}
	if checkHash && (!digestRE.MatchString(l.SHA256) || ledgerHash(l) != l.SHA256) {
		return errors.New("archive ledger hash mismatch")
	}
	keys, ids, deleted, splits := map[string]bool{}, map[string]bool{}, map[string]bool{}, map[string]bool{}
	for _, d := range l.Deletions {
		created, err := time.Parse(time.RFC3339Nano, d.CreatedAt)
		if !control.ValidID(d.ParticipantID) || deleted[d.ParticipantID] || (d.Reason != "withdrawal" && d.Reason != "retention") || err != nil || created.After(stamp) || (d.ArchiveState != "verified" && d.ArchiveState != "deleted") || (d.ArchiveState == "deleted" && now.Before(created.Add(30*24*time.Hour))) {
			return ErrRejected
		}
		deleted[d.ParticipantID] = true
	}
	for _, v := range l.Objects {
		created, err := time.Parse(time.RFC3339Nano, v.CreatedAt)
		expiry, expiryErr := time.Parse(time.RFC3339Nano, v.ExpiresAt)
		prefix := "events"
		if v.Kind == "export" {
			prefix = "exports"
		}
		// Version1 exports sampled the clock twice; preserve their original
		// deadline, allowing only the sub-second creation/expiry clock skew.
		retention := expiry.Sub(created)
		if !control.ValidID(v.ID) || !control.ValidID(v.ParticipantID) || ids[v.ID] || keys[v.Key] || (v.Kind != "batch" && v.Kind != "export") || (v.State != "verified" && v.State != "deleted") || !digestRE.MatchString(v.Hash) || v.Key != "research/"+prefix+"/"+v.ParticipantID+"/"+v.Hash+".json" || v.Bytes < 0 || v.Bytes > MaxBatch || (v.State == "verified" && v.Bytes == 0) || err != nil || expiryErr != nil || retention < 30*24*time.Hour-time.Second || retention > 30*24*time.Hour+time.Second || created.After(stamp) || len(v.Reviewer) > 256 || (v.Kind == "export" && v.Reviewer == "") || (v.Kind == "batch" && v.Reviewer != "") || (deleted[v.ParticipantID] && v.State != "deleted") {
			return ErrRejected
		}
		ids[v.ID] = true
		keys[v.Key] = true
	}
	for _, v := range l.Splits {
		key := v.Dimension + "\x00" + v.Value
		if (v.Dimension != "participant" && v.Dimension != "dataset" && v.Dimension != "task") || v.Value == "" || len(v.Value) > 1024 || strings.ContainsRune(v.Value, 0) || splits[key] || (v.Split != "train" && v.Split != "held_out") || (v.Dimension == "participant" && !control.ValidID(v.Value)) {
			return ErrRejected
		}
		splits[key] = true
	}
	return nil
}

// verifyLedger permits an already-expired recorded object to be absent. The
// returned metadata retains its deletion floor; it never replenishes consent.
func verifyLedger(ctx context.Context, a Archive, l *ArchiveLedger, now time.Time, maxBytes int64) error {
	lister, canList := a.(ArchiveLister)
	if a == nil || !canList {
		return ErrRejected
	}
	var total int64
	expected := map[string]int64{}
	for i := range l.Objects {
		v := &l.Objects[i]
		b, e := a.Get(ctx, v.Key)
		expiry, _ := time.Parse(time.RFC3339Nano, v.ExpiresAt)
		if v.State == "deleted" {
			if !errors.Is(e, ErrNotFound) {
				return errors.New("deleted archive object is not absent")
			}
		} else if errors.Is(e, ErrNotFound) && !now.Before(expiry) {
			v.State = "deleted"
		} else if e != nil || digest(b) != v.Hash || int64(len(b)) != v.Bytes {
			return errors.New("fresh archive ledger readback failed")
		} else {
			total += v.Bytes
			expected[v.Key] = v.Bytes
		}
		if maxBytes <= 0 || total > maxBytes {
			return errors.New("imported archive exceeds approved capacity")
		}
	}
	for i := range l.Deletions {
		v := &l.Deletions[i]
		key, want := deletionObject(*v)
		got, e := a.Get(ctx, key)
		created, _ := time.Parse(time.RFC3339Nano, v.CreatedAt)
		if v.ArchiveState == "deleted" {
			if !errors.Is(e, ErrNotFound) {
				return errors.New("expired deletion tombstone is not absent")
			}
		} else if errors.Is(e, ErrNotFound) && !now.Before(created.Add(30*24*time.Hour)) {
			v.ArchiveState = "deleted"
		} else if e != nil || digest(got) != digest(want) {
			return errors.New("deletion lineage readback failed")
		} else {
			expected[key] = int64(len(want))
			total += int64(len(want))
		}
	}
	if maxBytes <= 0 || total > maxBytes {
		return errors.New("archive including deletion lineage exceeds approved capacity")
	}
	listed, e := lister.List(ctx)
	if e != nil {
		return e
	}
	if len(listed) != len(expected) {
		return errors.New("archive inventory differs from complete operator ledger")
	}
	for _, v := range listed {
		want, ok := expected[v.Key]
		if !ok || want != v.Bytes {
			return errors.New("unknown or changed archive object; retain source ledger")
		}
		delete(expected, v.Key)
	}
	if len(expected) != 0 {
		return errors.New("archive listing omitted recorded objects")
	}
	return ctx.Err()
}

func (s *Store) ExportArchiveLedger(ctx context.Context, a Archive) (ArchiveLedger, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	c, e := s.ledgerConnection(ctx)
	if e != nil {
		return ArchiveLedger{}, e
	}
	defer closeLedger(c)
	l, e := readLedger(ctx, c, s.Now())
	if e != nil {
		return ArchiveLedger{}, e
	}
	if e = validateLedger(l, s.Now(), false); e != nil {
		return ArchiveLedger{}, e
	}
	if e = verifyLedger(ctx, a, &l, s.Now(), s.MaxArchiveBytes); e != nil {
		return ArchiveLedger{}, e
	}
	l.SHA256 = ledgerHash(l)
	b, _ := json.Marshal(l)
	if len(b)+1 > MaxLedgerBytes {
		return ArchiveLedger{}, ErrRejected
	}
	return l, nil
}

// InitializeArchiveBaseline explicitly verifies an existing local ledger (or
// proves the research prefix empty for a new deployment) before first serve.
func (s *Store) InitializeArchiveBaseline(ctx context.Context, a Archive) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	c, e := s.ledgerConnection(ctx)
	if e != nil {
		return "", e
	}
	defer closeLedger(c)
	var count int
	if e = c.QueryRowContext(ctx, `SELECT count(*) FROM archive_imports`).Scan(&count); e != nil || count != 0 {
		return "", errors.New("archive baseline already exists or is unavailable")
	}
	l, e := readLedger(ctx, c, s.Now())
	if e != nil {
		return "", e
	}
	if e = validateLedger(l, s.Now(), false); e != nil {
		return "", e
	}
	if e = verifyLedger(ctx, a, &l, s.Now(), s.MaxArchiveBytes); e != nil {
		return "", e
	}
	hash := ledgerHash(l)
	if _, e = c.ExecContext(ctx, `INSERT INTO archive_imports VALUES(1,?,?)`, hash, s.Now().UTC().Format(time.RFC3339Nano)); e != nil {
		return "", e
	}
	_, e = c.ExecContext(ctx, `COMMIT`)
	return hash, e
}

func (s *Store) ArchiveBaselineReady(ctx context.Context) error {
	var count int
	if e := s.DB.QueryRowContext(ctx, `SELECT count(*) FROM archive_imports WHERE id=1`).Scan(&count); e != nil || count != 1 {
		return errors.New("explicit archive baseline initialization or import required before capture")
	}
	return nil
}

// ImportArchiveLedger is an explicit offline bootstrap, only into a pristine
// initialized database. No events, consent mapping or reviews are recreated.
func (s *Store) ImportArchiveLedger(ctx context.Context, a Archive, l ArchiveLedger) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if e := validateLedger(l, s.Now(), true); e != nil {
		return e
	}
	inputHash := l.SHA256
	c, e := s.ledgerConnection(ctx)
	if e != nil {
		return e
	}
	defer closeLedger(c)
	var count int
	if e = c.QueryRowContext(ctx, `SELECT (SELECT count(*) FROM events)+(SELECT count(*) FROM batches)+(SELECT count(*) FROM exports)+(SELECT count(*) FROM deletions)+(SELECT count(*) FROM splits)+(SELECT count(*) FROM streams)+(SELECT count(*) FROM quotas)+(SELECT count(*) FROM archive_imports)`).Scan(&count); e != nil || count != 0 {
		return errors.New("archive import requires an untouched initialized database")
	}
	if e = verifyLedger(ctx, a, &l, s.Now(), s.MaxArchiveBytes); e != nil {
		return e
	}
	for _, v := range l.Objects {
		if v.Kind == "batch" {
			_, e = c.ExecContext(ctx, `INSERT INTO batches(id,participant,object_key,body,sha256,status,verified_at,archive_bytes,expires_at) VALUES(?,?,?,'',?,?,?,?,?)`, v.ID, v.ParticipantID, v.Key, v.Hash, v.State, l.ExportedAt, v.Bytes, v.ExpiresAt)
		} else {
			_, e = c.ExecContext(ctx, `INSERT INTO exports(id,reviewer,participant,object_key,body,sha256,status,created_at,expires_at,archive_bytes) VALUES(?,?,?,?,'',?,?,?,?,?)`, v.ID, v.Reviewer, v.ParticipantID, v.Key, v.Hash, v.State, v.CreatedAt, v.ExpiresAt, v.Bytes)
		}
		if e != nil {
			return e
		}
	}
	for _, v := range l.Deletions {
		_, b := deletionObject(v)
		size := len(b)
		if v.ArchiveState == "deleted" {
			size = 0
		}
		if _, e = c.ExecContext(ctx, `INSERT INTO deletions(participant,reason,status,created_at,archive_bytes) VALUES(?,?,'verified',?,?)`, v.ParticipantID, v.Reason, v.CreatedAt, size); e != nil {
			return e
		}
	}
	for _, v := range l.Splits {
		if _, e = c.ExecContext(ctx, `INSERT INTO splits VALUES(?,?,?)`, v.Dimension, v.Value, v.Split); e != nil {
			return e
		}
	}
	if _, e = c.ExecContext(ctx, `INSERT INTO archive_imports VALUES(1,?,?)`, inputHash, s.Now().UTC().Format(time.RFC3339Nano)); e != nil {
		return e
	}
	_, e = c.ExecContext(ctx, `COMMIT`)
	return e
}

func ledgerInventory(l ArchiveLedger) []ArchiveObject {
	items := []ArchiveObject{}
	for _, v := range l.Objects {
		items = append(items, ArchiveObject{v.Kind, v.Key, v.Hash, v.State})
	}
	for _, v := range l.Deletions {
		key, b := deletionObject(v)
		items = append(items, ArchiveObject{"deletion", key, digest(b), v.ArchiveState})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Key < items[j].Key })
	return items
}
