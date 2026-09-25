package collector

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"github.com/STRK-Solutions/Feather-Mesh/web_demo/internal/control"
	"time"
)

func (s *Store) Withdraw(ctx context.Context, a Archive, participant, reason string) error {
	if !control.ValidID(participant) || (reason != "withdrawal" && reason != "retention") {
		return ErrRejected
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	tx, e := s.DB.BeginTx(ctx, nil)
	if e != nil {
		return e
	}
	defer tx.Rollback()
	stamp := s.Now().UTC().Format(time.RFC3339Nano)
	_, body := deletionObject(LedgerDeletion{ParticipantID: participant, Reason: reason, CreatedAt: stamp})
	if _, e = tx.Exec(`INSERT INTO deletions(participant,reason,status,created_at,archive_bytes) VALUES(?,?,'pending',?,?) ON CONFLICT(participant) DO NOTHING`, participant, reason, stamp, len(body)); e != nil {
		return e
	}
	if _, e = tx.Exec(`UPDATE batches SET status='delete_pending',body='' WHERE participant=? AND status!='deleted'`, participant); e != nil {
		return e
	}
	if _, e = tx.Exec(`UPDATE exports SET status='delete_pending',body='' WHERE participant=? AND status!='deleted'`, participant); e != nil {
		return e
	}
	if _, e = tx.Exec(`DELETE FROM events WHERE participant=?`, participant); e != nil {
		return e
	}
	if _, e = tx.Exec(`DELETE FROM streams WHERE participant=?`, participant); e != nil {
		return e
	}
	if e = tx.Commit(); e != nil {
		return e
	}
	return s.finishDeletion(ctx, a, participant)
}
func (s *Store) finishDeletion(ctx context.Context, a Archive, participant string) error {
	var reason, created string
	if e := s.DB.QueryRowContext(ctx, `SELECT reason,created_at FROM deletions WHERE participant=?`, participant).Scan(&reason, &created); e != nil {
		return e
	}
	tombstone, _ := json.Marshal(map[string]string{"participant_id": participant, "reason": reason, "created_at": created})
	key := "research/deletions/" + participant + "/" + digest(tombstone) + ".json"
	stamp, err := time.Parse(time.RFC3339Nano, created)
	if err != nil {
		return err
	}
	for _, table := range []string{"batches", "exports"} {
		rows, e := s.DB.QueryContext(ctx, `SELECT id,object_key FROM `+table+` WHERE participant=? AND status='delete_pending'`, participant)
		if e != nil {
			return e
		}
		type item struct{ id, key string }
		var items []item
		for rows.Next() {
			var v item
			if e = rows.Scan(&v.id, &v.key); e != nil {
				rows.Close()
				return e
			}
			items = append(items, v)
		}
		e = rows.Err()
		rows.Close()
		if e != nil {
			return e
		}
		for _, v := range items {
			if e = verifiedDelete(ctx, a, v.key); e != nil {
				return e
			}
			if _, e = s.DB.ExecContext(ctx, `UPDATE `+table+` SET status='deleted',body='' WHERE id=?`, v.id); e != nil {
				return e
			}
		}
	}
	// The local durable deletion floor precedes remote erasure. Delete the old
	// participant objects before allocating the small independent tombstone.
	archiveBytes := int64(len(tombstone))
	if !s.Now().Before(stamp.Add(30 * 24 * time.Hour)) {
		if e := verifiedDelete(ctx, a, key); e != nil {
			return e
		}
		archiveBytes = 0
	} else {
		var total int64
		if e := s.DB.QueryRowContext(ctx, archiveUsageSQL).Scan(&total); e != nil || s.MaxArchiveBytes <= 0 || total > s.MaxArchiveBytes {
			return ErrBackpressure
		}
		if e := verifiedPut(ctx, a, key, tombstone); e != nil {
			return e
		}
	}
	_, e := s.DB.ExecContext(ctx, `UPDATE deletions SET status='verified',archive_bytes=? WHERE participant=?`, archiveBytes, participant)
	return e
}
func (s *Store) ReconcileDeletions(ctx context.Context, a Archive) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	rows, e := s.DB.QueryContext(ctx, `SELECT participant FROM deletions WHERE status!='verified'`)
	if e != nil {
		return e
	}
	var ids []string
	for rows.Next() {
		var id string
		if e = rows.Scan(&id); e != nil {
			rows.Close()
			return e
		}
		ids = append(ids, id)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return e
	}
	for _, id := range ids {
		if e = s.finishDeletion(ctx, a, id); e != nil {
			return e
		}
	}
	return nil
}

type Review struct {
	EventID     string `json:"event_id"`
	EventSHA256 string `json:"event_sha256"`
}
type Export struct {
	Protocol  string    `json:"protocol"`
	ID        string    `json:"id"`
	Reviewer  string    `json:"reviewer"`
	Purpose   string    `json:"purpose"`
	CreatedAt string    `json:"created_at"`
	ExpiresAt string    `json:"expires_at"`
	Split     string    `json:"split"`
	Synthetic bool      `json:"synthetic"`
	Examples  []Example `json:"examples"`
}
type Example struct {
	Event       Event  `json:"event"`
	EventSHA256 string `json:"event_sha256"`
	Label       string `json:"label"`
}

func (s *Store) Export(ctx context.Context, a Archive, reviewer, participant, split string, reviews []Review) (Export, error) {
	var out Export
	if reviewer == "" || !control.ValidID(participant) || (split != "train" && split != "held_out") || len(reviews) == 0 || len(reviews) > 100 {
		return out, ErrRejected
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	tx, e := s.DB.BeginTx(ctx, nil)
	if e != nil {
		return out, e
	}
	defer tx.Rollback()
	var withdrawn int
	if e = tx.QueryRow(`SELECT count(*) FROM deletions WHERE participant=?`, participant).Scan(&withdrawn); e != nil || withdrawn != 0 {
		return out, ErrRejected
	}
	created := s.Now()
	out = Export{Protocol: "feam.web.export.v1", ID: control.ID(), Reviewer: reviewer, Purpose: "reviewed-feam-workflows", CreatedAt: created.UTC().Format(time.RFC3339Nano), ExpiresAt: created.Add(30 * 24 * time.Hour).UTC().Format(time.RFC3339Nano), Split: split, Synthetic: true}
	for _, r := range reviews {
		var body, hash string
		if e = tx.QueryRow(`SELECT body,sha256 FROM events WHERE id=? AND participant=?`, r.EventID, participant).Scan(&body, &hash); e != nil || hash != r.EventSHA256 {
			return Export{}, ErrRejected
		}
		var event Event
		if json.Unmarshal([]byte(body), &event) != nil {
			return Export{}, ErrRejected
		}
		var gaps int
		if e = tx.QueryRow(`SELECT gaps FROM streams WHERE id=?`, event.StreamID).Scan(&gaps); e != nil || gaps != 0 {
			return Export{}, errors.New("incomplete stream excluded from export")
		}
		label := ""
		if event.Kind == "outcome" && event.Trust == "independently_verified" && event.Payload.Outcome == "committed" && digestRE.MatchString(event.Payload.ReceiptSHA256) {
			label = "verified_success"
		} else if event.Payload.Outcome == "denied" || event.Payload.Outcome == "failed" || event.Payload.Decision == "denied" || event.Payload.Decision == "cancelled" {
			label = "negative_" + event.Trust
		} else {
			return Export{}, errors.New("no verified or negative outcome label")
		}
		if event.Payload.TaskTemplate == "" || event.Payload.DatasetFamily == "" {
			return Export{}, errors.New("task/dataset split labels required")
		}
		for dim, value := range map[string]string{"participant": participant, "dataset": event.Payload.DatasetFamily, "task": event.Payload.TaskTemplate} {
			var prior string
			e = tx.QueryRow(`SELECT split FROM splits WHERE dimension=? AND value=?`, dim, value).Scan(&prior)
			if e == nil && prior != split {
				return Export{}, errors.New("held-out split leakage")
			}
			if e != nil && !errors.Is(e, sql.ErrNoRows) {
				return Export{}, e
			}
			if _, e = tx.Exec(`INSERT INTO splits VALUES(?,?,?) ON CONFLICT(dimension,value) DO NOTHING`, dim, value, split); e != nil {
				return Export{}, e
			}
		}
		out.Synthetic = out.Synthetic && event.Synthetic
		out.Examples = append(out.Examples, Example{event, hash, label})
	}
	b, _ := json.MarshalIndent(out, "", "  ")
	if len(b) > MaxBatch {
		return Export{}, ErrRejected
	}
	var archiveBytes int64
	if e = tx.QueryRow(archiveUsageSQL).Scan(&archiveBytes); e != nil || s.MaxArchiveBytes <= 0 || archiveBytes+int64(len(b)) > s.MaxArchiveBytes {
		return Export{}, ErrBackpressure
	}
	key := "research/exports/" + participant + "/" + digest(b) + ".json"
	if _, e = tx.Exec(`INSERT INTO exports(id,reviewer,participant,object_key,body,sha256,status,created_at,expires_at,archive_bytes) VALUES(?,?,?,?,?,?,'pending',?,?,?)`, out.ID, reviewer, participant, key, string(b), digest(b), out.CreatedAt, out.ExpiresAt, len(b)); e != nil {
		return Export{}, e
	}
	if e = tx.Commit(); e != nil {
		return Export{}, e
	}
	if e = verifiedPut(ctx, a, key, b); e != nil {
		return Export{}, e
	}
	if _, e = s.DB.ExecContext(ctx, `UPDATE exports SET status='verified' WHERE id=?`, out.ID); e != nil {
		return Export{}, e
	}
	return out, nil
}

// Expire removes only archive-verified data. Failure retains the sole copy and
// blocks teardown. Remote lifecycle is an additional, independently checked cap.
func (s *Store) Expire(ctx context.Context, a Archive) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	rows, e := s.DB.QueryContext(ctx, `SELECT id,object_key FROM batches WHERE status='verified' AND julianday(expires_at)<=julianday(?)`, s.Now().UTC().Format(time.RFC3339Nano))
	if e != nil {
		return e
	}
	type item struct{ id, key string }
	var items []item
	for rows.Next() {
		var v item
		if e = rows.Scan(&v.id, &v.key); e != nil {
			rows.Close()
			return e
		}
		items = append(items, v)
	}
	rows.Close()
	for _, v := range items {
		if e = verifiedDelete(ctx, a, v.key); e != nil {
			return e
		}
		tx, e := s.DB.BeginTx(ctx, nil)
		if e != nil {
			return e
		}
		if _, e = tx.Exec(`DELETE FROM events WHERE batch_id=?`, v.id); e == nil {
			_, e = tx.Exec(`UPDATE batches SET status='deleted',body='' WHERE id=?`, v.id)
		}
		if e != nil {
			tx.Rollback()
			return e
		}
		if e = tx.Commit(); e != nil {
			return e
		}
	}
	rows, e = s.DB.QueryContext(ctx, `SELECT id,object_key FROM exports WHERE status='verified' AND julianday(expires_at)<=julianday(?)`, s.Now().UTC().Format(time.RFC3339Nano))
	if e != nil {
		return e
	}
	items = nil
	for rows.Next() {
		var v item
		if e = rows.Scan(&v.id, &v.key); e != nil {
			rows.Close()
			return e
		}
		items = append(items, v)
	}
	rows.Close()
	for _, v := range items {
		if e = verifiedDelete(ctx, a, v.key); e != nil {
			return e
		}
		if _, e = s.DB.ExecContext(ctx, `UPDATE exports SET status='deleted',body='' WHERE id=?`, v.id); e != nil {
			return e
		}
	}
	rows, e = s.DB.QueryContext(ctx, `SELECT participant,reason,created_at FROM deletions WHERE status='verified' AND archive_bytes>0 AND julianday(created_at)<=julianday(?,'-30 days')`, s.Now().UTC().Format(time.RFC3339Nano))
	if e != nil {
		return e
	}
	var tombstones []item
	for rows.Next() {
		var participant, reason, created string
		if e = rows.Scan(&participant, &reason, &created); e != nil {
			break
		}
		b, _ := json.Marshal(map[string]string{"participant_id": participant, "reason": reason, "created_at": created})
		tombstones = append(tombstones, item{participant, "research/deletions/" + participant + "/" + digest(b) + ".json"})
	}
	if e == nil {
		e = rows.Err()
	}
	rows.Close()
	if e != nil {
		return e
	}
	for _, v := range tombstones {
		if e = verifiedDelete(ctx, a, v.key); e != nil {
			return e
		}
		if _, e = s.DB.ExecContext(ctx, `UPDATE deletions SET archive_bytes=0 WHERE participant=?`, v.id); e != nil {
			return e
		}
	}
	return nil
}
