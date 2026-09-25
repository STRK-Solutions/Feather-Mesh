package collector

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/STRK-Solutions/Feather-Mesh/web_demo/internal/control"
	"github.com/STRK-Solutions/Feather-Mesh/web_demo/internal/db"
)

func ledgerFixture(t *testing.T) (*Store, DirectoryArchive, ArchiveLedger, Event, Event) {
	t.Helper()
	s, a := fixture(t), privateArchive(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	e := event()
	e.Kind = "review"
	e.Payload = Payload{Decision: "denied", TaskTemplate: "carryover-review", DatasetFamily: "carryover-data"}
	e.Sanitize(false)
	if _, err := s.Append(ctx, e); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Export(ctx, a, "synthetic-reviewer", e.ParticipantID, "held_out", reviews(s, e)); err != nil {
		t.Fatal(err)
	}
	withdrawn := event()
	if _, err := s.Append(ctx, withdrawn); err != nil {
		t.Fatal(err)
	}
	if err := s.Flush(ctx, a); err != nil {
		t.Fatal(err)
	}
	if err := s.Withdraw(ctx, a, withdrawn.ParticipantID, "withdrawal"); err != nil {
		t.Fatal(err)
	}
	l, err := s.ExportArchiveLedger(ctx, a)
	if err != nil {
		t.Fatal(err)
	}
	return s, a, l, e, withdrawn
}

func TestArchiveLedgerCarryoverPreservesCapacitySplitsAndWithdrawal(t *testing.T) {
	_, a, l, old, withdrawn := ledgerFixture(t)
	s := fixture(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := s.ImportArchiveLedger(ctx, a, l); err != nil {
		t.Fatal(err)
	}
	if err := s.ImportArchiveLedger(ctx, a, l); err == nil {
		t.Fatal("baseline imported twice")
	}
	var bytes, local int64
	if err := s.DB.QueryRow(archiveUsageSQL).Scan(&bytes); err != nil {
		t.Fatal(err)
	}
	if err := s.DB.QueryRow(`SELECT (SELECT SUM(length(body)) FROM batches)+(SELECT SUM(length(body)) FROM exports)`).Scan(&local); err != nil || local != 0 {
		t.Fatal("event bodies restored", local, err)
	}
	var want int64
	for _, v := range l.Objects {
		if v.State == "verified" {
			want += v.Bytes
		}
	}
	for _, d := range l.Deletions {
		if d.ArchiveState == "verified" {
			_, b := deletionObject(d)
			want += int64(len(b))
		}
	}
	if bytes != want || bytes == 0 {
		t.Fatal("old archive capacity vanished", bytes, want)
	}
	withdrawn.EventID = control.ID()
	withdrawn.StreamID = control.ID()
	if _, err := s.Append(ctx, withdrawn); err == nil {
		t.Fatal("import resurrected withdrawn participant")
	}
	next := old
	next.EventID = control.ID()
	next.StreamID = control.ID()
	next.ParticipantID = control.ID()
	if _, err := s.Append(ctx, next); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Export(ctx, a, "synthetic-reviewer", next.ParticipantID, "train", reviews(s, next)); err == nil || !strings.Contains(err.Error(), "split leakage") {
		t.Fatal("recreation lost held-out split", err)
	}
	s.MaxArchiveBytes = bytes
	if err := s.Flush(ctx, a); !errors.Is(err, ErrBackpressure) {
		t.Fatal("recreation replenished archive cap", err)
	}
	if err := s.Withdraw(ctx, a, old.ParticipantID, "withdrawal"); err != nil {
		t.Fatal(err)
	}
	for _, v := range l.Objects {
		if v.ParticipantID == old.ParticipantID {
			if _, err := a.Get(ctx, v.Key); !errors.Is(err, ErrNotFound) {
				t.Fatal("imported artifact survived withdrawal", err)
			}
		}
	}
}

func TestArchiveLedgerImportRequiresFreshRemoteProofAndUntouchedDatabase(t *testing.T) {
	_, a, l, _, _ := ledgerFixture(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	s := fixture(t)
	if _, err := s.Append(ctx, event()); err != nil {
		t.Fatal(err)
	}
	if err := s.ImportArchiveLedger(ctx, a, l); err == nil {
		t.Fatal("nonempty store overwritten")
	}
	var key string
	for _, v := range l.Objects {
		if v.State == "verified" {
			key = v.Key
			break
		}
	}
	if err := a.Delete(ctx, key); err != nil {
		t.Fatal(err)
	}
	fresh := fixture(t)
	if err := fresh.ImportArchiveLedger(ctx, a, l); err == nil {
		t.Fatal("missing unexpired artifact accepted")
	}
	var count int
	if err := fresh.DB.QueryRow(`SELECT (SELECT count(*) FROM batches)+(SELECT count(*) FROM exports)+(SELECT count(*) FROM deletions)+(SELECT count(*) FROM archive_imports)`).Scan(&count); err != nil || count != 0 {
		t.Fatal("failed import left partial baseline", count, err)
	}
}

func TestArchiveLedgerExpiredRemoteDataRetainsDeletionFloors(t *testing.T) {
	old, a, l, _, withdrawn := ledgerFixture(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	now := time.Now().Add(31 * 24 * time.Hour)
	old.Now = func() time.Time { return now }
	if err := old.Expire(ctx, a); err != nil {
		t.Fatal(err)
	}
	fresh := fixture(t)
	fresh.Now = old.Now
	if err := fresh.ImportArchiveLedger(ctx, a, l); err != nil {
		t.Fatal("legitimate independent archive expiry blocked carryover", err)
	}
	var bytes int64
	if err := fresh.DB.QueryRow(archiveUsageSQL).Scan(&bytes); err != nil || bytes != 0 {
		t.Fatal("verified expired bytes still reserved", bytes, err)
	}
	withdrawn.EventID = control.ID()
	withdrawn.StreamID = control.ID()
	if _, err := fresh.Append(ctx, withdrawn); err == nil {
		t.Fatal("expired tombstone removed withdrawal floor")
	}
	r, err := fresh.VerifyArchive(ctx, a)
	if err != nil || r.Bytes != 0 || r.Absent != len(l.Objects)+len(l.Deletions) {
		t.Fatal(r, err)
	}
	if err := fresh.Withdraw(ctx, a, withdrawn.ParticipantID, "withdrawal"); err != nil {
		t.Fatal(err)
	}
	for _, d := range l.Deletions {
		key, _ := deletionObject(d)
		if _, err := a.Get(ctx, key); !errors.Is(err, ErrNotFound) {
			t.Fatal("old tombstone recreated after retention")
		}
	}
}

func TestArchiveLedgerImportChecksRemoteLengthAndCanonicalHash(t *testing.T) {
	_, a, l, _, _ := ledgerFixture(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	for i := range l.Objects {
		if l.Objects[i].State == "verified" {
			l.Objects[i].Bytes++
			break
		}
	}
	if err := fixture(t).ImportArchiveLedger(ctx, a, l); err == nil {
		t.Fatal("changed ledger accepted unchanged hash")
	}
	l.SHA256 = ledgerHash(l)
	if err := fixture(t).ImportArchiveLedger(ctx, a, l); err == nil {
		t.Fatal("false imported byte reservation accepted")
	}
}

func TestArchiveBaselineAndImportRejectUnknownPrefixObjects(t *testing.T) {
	_, a, l, _, _ := ledgerFixture(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	fresh := fixture(t)
	if fresh.ArchiveBaselineReady(ctx) == nil {
		t.Fatal("new database implicitly reset archive baseline")
	}
	if _, err := fresh.InitializeArchiveBaseline(ctx, a); err == nil {
		t.Fatal("unknown existing archive treated as empty")
	}
	extra := []byte("unrecorded content")
	key := "research/events/" + control.ID() + "/" + digest(extra) + ".json"
	if err := a.Put(ctx, key, extra); err != nil {
		t.Fatal(err)
	}
	if err := fresh.ImportArchiveLedger(ctx, a, l); err == nil {
		t.Fatal("omitted archive bytes hidden by imported ledger")
	}
	if err := a.Delete(ctx, key); err != nil {
		t.Fatal(err)
	}
	if err := fresh.ImportArchiveLedger(ctx, a, l); err != nil {
		t.Fatal(err)
	}
	if err := fresh.ArchiveBaselineReady(ctx); err != nil {
		t.Fatal(err)
	}
	empty := fixture(t)
	if _, err := empty.InitializeArchiveBaseline(ctx, privateArchive(t)); err != nil {
		t.Fatal(err)
	}
	if err := empty.ArchiveBaselineReady(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestR2ListingIsSignedBoundedAndChecksContinuation(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	key := "research/events/" + control.ID() + "/" + strings.Repeat("b", 64) + ".json"
	pages := 0
	a := R2{AccountID: strings.Repeat("a", 32), Bucket: "synthetic-archive", AccessKeyID: "synthetic", SecretAccessKey: "synthetic", Client: &http.Client{Transport: roundtrip(func(r *http.Request) (*http.Response, error) {
		pages++
		if r.URL.Path != "/synthetic-archive" || r.URL.Query().Get("prefix") != "research/" || r.URL.Query().Get("list-type") != "2" || !strings.HasPrefix(r.Header.Get("Authorization"), "AWS4-HMAC-SHA256 ") {
			t.Fatal("listing escaped signed fixed archive")
		}
		content := "<Name>synthetic-archive</Name><Prefix>research/</Prefix>"
		if pages == 1 {
			content += "<IsTruncated>true</IsTruncated><NextContinuationToken>opaque-token</NextContinuationToken><Contents><Key>" + key + "</Key><Size>2</Size></Contents>"
		} else {
			if r.URL.Query().Get("continuation-token") != "opaque-token" {
				t.Fatal("continuation not bound")
			}
			content += "<IsTruncated>false</IsTruncated>"
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader("<ListBucketResult>" + content + "</ListBucketResult>"))}, nil
	})}}
	objects, err := a.List(ctx)
	if err != nil || pages != 2 || len(objects) != 1 || objects[0].Key != key {
		t.Fatal(objects, pages, err)
	}
	a.Client = &http.Client{Transport: roundtrip(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader("<ListBucketResult><Name>synthetic-archive</Name><Prefix>research/</Prefix><IsTruncated>true</IsTruncated><NextContinuationToken>same</NextContinuationToken></ListBucketResult>"))}, nil
	})}
	if _, err = a.List(ctx); err == nil {
		t.Fatal("endless continuation accepted")
	}
}

type cancellingArchive struct {
	DirectoryArchive
	cancel context.CancelFunc
}

func (a cancellingArchive) Get(ctx context.Context, key string) ([]byte, error) {
	b, e := a.DirectoryArchive.Get(ctx, key)
	a.cancel()
	return b, e
}

func TestArchiveImportCancellationLeavesNoBaseline(t *testing.T) {
	_, a, l, _, _ := ledgerFixture(t)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	fresh := fixture(t)
	if err := fresh.ImportArchiveLedger(ctx, cancellingArchive{a, cancel}, l); err == nil {
		t.Fatal("cancelled import succeeded")
	}
	var count int
	if err := fresh.DB.QueryRow(`SELECT (SELECT count(*) FROM batches)+(SELECT count(*) FROM exports)+(SELECT count(*) FROM deletions)+(SELECT count(*) FROM archive_imports)`).Scan(&count); err != nil || count != 0 {
		t.Fatal("cancelled import left metadata", count, err)
	}
}

func TestArchiveMigrationRetainsVersionOneReservationsAndDeadlines(t *testing.T) {
	path := filepath.Join(t.TempDir(), "old.db")
	d, err := db.Initialize(path, Migrations[:1])
	if err != nil {
		t.Fatal(err)
	}
	s := New(d, 8<<20)
	a := privateArchive(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	e := event()
	if _, err = s.Append(ctx, e); err != nil {
		t.Fatal(err)
	}
	b, _ := json.Marshal([]Event{e})
	batchID := control.ID()
	key := "research/events/" + e.ParticipantID + "/" + digest(b) + ".json"
	now := time.Now().UTC()
	if _, err = d.Exec(`INSERT INTO batches VALUES(?,?,?,?,?,'verified',?)`, batchID, e.ParticipantID, key, string(b), digest(b), now.Format(time.RFC3339Nano)); err != nil {
		t.Fatal(err)
	}
	if _, err = d.Exec(`UPDATE events SET batch_id=? WHERE id=?`, batchID, e.EventID); err != nil {
		t.Fatal(err)
	}
	if err = a.Put(ctx, key, b); err != nil {
		t.Fatal(err)
	}
	out := Export{Protocol: "feam.web.export.v1", ID: control.ID(), Reviewer: "synthetic-reviewer", Purpose: "reviewed-feam-workflows", CreatedAt: now.Format(time.RFC3339Nano), ExpiresAt: now.Add(30*24*time.Hour + 30*time.Microsecond).Format(time.RFC3339Nano), Split: "train", Synthetic: true}
	exported, _ := json.Marshal(out)
	exportKey := "research/exports/" + e.ParticipantID + "/" + digest(exported) + ".json"
	if _, err = d.Exec(`INSERT INTO exports VALUES(?,?,?,?,?,?,'verified',?,?)`, out.ID, out.Reviewer, e.ParticipantID, exportKey, string(exported), digest(exported), out.CreatedAt, out.ExpiresAt); err != nil {
		t.Fatal(err)
	}
	if err = a.Put(ctx, exportKey, exported); err != nil {
		t.Fatal(err)
	}
	d.Close()
	if wrong, err := db.Open(path, Migrations); err == nil {
		wrong.Close()
		t.Fatal("new schema silently migrated at startup")
	}
	d, err = db.Migrate(path, Migrations)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	s = New(d, 8<<20)
	l, err := s.ExportArchiveLedger(ctx, a)
	if err != nil || len(l.Objects) != 2 {
		t.Fatal(l, err)
	}
	var usage int64
	if err = d.QueryRow(archiveUsageSQL).Scan(&usage); err != nil || usage != int64(len(b)+len(exported)) {
		t.Fatal("migration replenished reserve", usage, err)
	}
}
