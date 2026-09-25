package collector

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"github.com/STRK-Solutions/Feather-Mesh/web_demo/internal/capability"
	"github.com/STRK-Solutions/Feather-Mesh/web_demo/internal/control"
	"github.com/STRK-Solutions/Feather-Mesh/web_demo/internal/db"
	"github.com/STRK-Solutions/Feather-Mesh/web_demo/internal/ipc"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func fixture(t *testing.T) *Store {
	t.Helper()
	d, e := db.Initialize(filepath.Join(t.TempDir(), "events.db"), Migrations)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { d.Close() })
	return New(d, 8<<20)
}
func privateArchive(t *testing.T) DirectoryArchive {
	t.Helper()
	root := t.TempDir()
	if err := os.Chmod(root, 0700); err != nil {
		t.Fatal(err)
	}
	return DirectoryArchive{root}
}
func event() Event {
	e := Event{Protocol: Protocol, EventID: control.ID(), StreamID: control.ID(), Sequence: 1, DeploymentID: control.ID(), WorkspaceID: control.ID(), Generation: 1, ParticipantID: control.ID(), ConversationID: control.ID(), RequestID: control.ID(), OccurredAt: time.Now().UTC().Format(time.RFC3339Nano), Kind: "request", Trust: "client_reported", SoftwareSHA256: strings.Repeat("a", 64), ProfileSHA256: strings.Repeat("b", 64), DatasetSHA256: strings.Repeat("c", 64), Synthetic: true}
	e.Sanitize(false)
	return e
}
func TestDurabilityDedupGapsRecoveryAndBounds(t *testing.T) {
	s := fixture(t)
	ctx := context.Background()
	e := event()
	ack, err := s.Append(ctx, e)
	if err != nil || ack.Gap {
		t.Fatal(err)
	}
	ack, err = s.Append(ctx, e)
	if err != nil || ack.Status != "duplicate" {
		t.Fatal("retry duplicated", err)
	}
	changed := e
	changed.Payload.Text = "changed"
	changed.Sanitize(false)
	if _, err = s.Append(ctx, changed); err == nil {
		t.Fatal("changed replay accepted")
	}
	e.EventID = control.ID()
	e.Sequence = 3
	ack, err = s.Append(ctx, e)
	if err != nil || !ack.Gap {
		t.Fatal("missing span hidden")
	}
	ack, err = s.Append(ctx, e)
	if err != nil || !ack.Gap || ack.Status != "duplicate" {
		t.Fatal("lost gap acknowledgment hidden by retry", err)
	}
	s.DB.Exec(`DELETE FROM streams`)
	if err = s.Recover(ctx); err != nil {
		t.Fatal(err)
	}
	var gaps, last int64
	if err = s.DB.QueryRow(`SELECT last_sequence,gaps FROM streams WHERE id=?`, e.StreamID).Scan(&last, &gaps); err != nil || last != 3 || gaps != 1 {
		t.Fatal("index recovery lost ordering")
	}
	s.MaxBytes = 1
	e.EventID = control.ID()
	e.Sequence = 4
	if _, err = s.Append(ctx, e); !errors.Is(err, ErrBackpressure) {
		t.Fatal("storage exhaustion acknowledged", err)
	}
}
func TestSQLiteDiskFullNoDurableAck(t *testing.T) {
	s := fixture(t)
	ctx := context.Background()
	var pages int
	s.DB.QueryRow(`PRAGMA page_count`).Scan(&pages)
	s.DB.Exec(`PRAGMA max_page_count=` + strconv.Itoa(pages))
	e := event()
	e.Payload.Text = strings.Repeat("structured narrative ", 200)
	e.Sanitize(true)
	_, err := s.Append(ctx, e)
	if !errors.Is(err, ErrBackpressure) {
		t.Fatal("disk-full did not fail closed", err)
	}
	var count int
	s.DB.QueryRow(`SELECT count(*) FROM events`).Scan(&count)
	if count != 0 {
		t.Fatal("partially acknowledged disk-full event")
	}
}
func TestRedactionBeforePersistence(t *testing.T) {
	s := fixture(t)
	e := event()
	e.Payload.Text = "Email user@example.invalid path /Users/person/private.csv Bearer SECRET123 token=ABC123 sk-abcdefghijk"
	e.Payload.ProviderID = "user@example.invalid"
	e.Sanitize(true)
	if _, err := s.Append(context.Background(), e); err != nil {
		t.Fatal(err)
	}
	var b string
	s.DB.QueryRow(`SELECT body FROM events`).Scan(&b)
	for _, secret := range []string{"user@example.invalid", "/Users/person", "SECRET123", "ABC123", "sk-abcdefghijk"} {
		if strings.Contains(b, secret) {
			t.Fatal("secret persisted", secret)
		}
	}
	e.Payload.Text = "raw dataset contents"
	e.Sanitize(false)
	if strings.Contains(e.Payload.Text, "raw dataset") {
		t.Fatal("default text capture persisted payload")
	}
}

type brokenArchive struct {
	base          Archive
	fail, corrupt bool
}

func (a *brokenArchive) Put(ctx context.Context, k string, b []byte) error {
	if a.fail {
		return errors.New("offline")
	}
	return a.base.Put(ctx, k, b)
}
func (a *brokenArchive) Get(ctx context.Context, k string) ([]byte, error) {
	b, e := a.base.Get(ctx, k)
	if a.corrupt && e == nil {
		return []byte("changed"), nil
	}
	return b, e
}
func (a *brokenArchive) Delete(ctx context.Context, k string) error {
	if a.fail {
		return errors.New("offline")
	}
	return a.base.Delete(ctx, k)
}
func TestArchiveReadbackWatermarkAndIndependentRetrieval(t *testing.T) {
	s := fixture(t)
	ctx := context.Background()
	e := event()
	s.Append(ctx, e)
	archive := privateArchive(t)
	broken := &brokenArchive{base: archive, corrupt: true}
	if err := s.Flush(ctx, broken); err == nil {
		t.Fatal("corrupt upload verified")
	}
	if s.TeardownReady(ctx) == nil {
		t.Fatal("teardown discarded unverified copy")
	}
	broken.corrupt = false
	if err := s.Flush(ctx, broken); err != nil {
		t.Fatal(err)
	}
	if err := s.TeardownReady(ctx); err != nil {
		t.Fatal(err)
	}
	var key, hash string
	s.DB.QueryRow(`SELECT object_key,sha256 FROM batches`).Scan(&key, &hash)
	s.DB.Close()
	b, err := archive.Get(ctx, key)
	if err != nil || digest(b) != hash {
		t.Fatal("independent retrieval failed", err)
	}
}
func reviews(s *Store, e Event) []Review {
	b, _ := json.Marshal(e)
	return []Review{{e.EventID, digest(b)}}
}
func TestFailedExportUploadResumesExactArtifact(t *testing.T) {
	s := fixture(t)
	ctx := context.Background()
	e := event()
	e.Kind = "review"
	e.Payload = Payload{Decision: "denied", TaskTemplate: "local-review", DatasetFamily: "synthetic"}
	e.Sanitize(false)
	if _, err := s.Append(ctx, e); err != nil {
		t.Fatal(err)
	}
	a := &brokenArchive{base: privateArchive(t), fail: true}
	if _, err := s.Export(ctx, a, "synthetic-reviewer", e.ParticipantID, "train", reviews(s, e)); err == nil {
		t.Fatal("failed upload acknowledged")
	}
	var key, hash, state string
	if err := s.DB.QueryRow(`SELECT object_key,sha256,status FROM exports`).Scan(&key, &hash, &state); err != nil || state != "pending" {
		t.Fatal("export intent not durable", err)
	}
	if s.TeardownReady(ctx) == nil {
		t.Fatal("pending export allowed teardown")
	}
	a.fail = false
	if err := s.Flush(ctx, a); err != nil {
		t.Fatal(err)
	}
	if err := s.DB.QueryRow(`SELECT status FROM exports`).Scan(&state); err != nil || state != "verified" {
		t.Fatal("export recovery not verified", err)
	}
	got, err := a.Get(ctx, key)
	if err != nil || digest(got) != hash {
		t.Fatal("recovered export changed", err)
	}
	if err := s.TeardownReady(ctx); err != nil {
		t.Fatal(err)
	}
}
func TestReviewedExportSplitWithdrawalAndNoResurrection(t *testing.T) {
	s := fixture(t)
	ctx := context.Background()
	a := privateArchive(t)
	e := event()
	e.Kind = "outcome"
	e.Trust = "independently_verified"
	e.Payload = Payload{Outcome: "committed", ReceiptSHA256: strings.Repeat("d", 64), TaskTemplate: "resolve-window", DatasetFamily: "climate"}
	e.Sanitize(false)
	if _, err := s.Append(ctx, e); err != nil {
		t.Fatal(err)
	}
	out, err := s.Export(ctx, a, "Saif", e.ParticipantID, "held_out", reviews(s, e))
	if err != nil || !out.Synthetic || out.Examples[0].Label != "verified_success" {
		t.Fatal("reviewed synthetic export failed", err)
	}
	if _, err = s.Export(ctx, a, "Saif", e.ParticipantID, "train", reviews(s, e)); err == nil {
		t.Fatal("held-out leakage")
	}
	fake := e
	fake.EventID = control.ID()
	fake.Sequence = 2
	fake.Trust = "client_reported"
	fake.Sanitize(false)
	s.Append(ctx, fake)
	if _, err = s.Export(ctx, a, "Saif", e.ParticipantID, "held_out", reviews(s, fake)); err == nil {
		t.Fatal("client claimed success exported as verified")
	}
	if err = s.Flush(ctx, a); err != nil {
		t.Fatal(err)
	}
	var oldKey string
	s.DB.QueryRow(`SELECT object_key FROM exports LIMIT 1`).Scan(&oldKey)
	broken := &brokenArchive{base: a, fail: true}
	if err = s.Withdraw(ctx, broken, e.ParticipantID, "withdrawal"); err == nil {
		t.Fatal("archive failure hidden")
	}
	if _, err = s.Append(ctx, e); err == nil {
		t.Fatal("withdrawn participant collected")
	}
	if s.TeardownReady(ctx) == nil {
		t.Fatal("pending deletion allowed teardown")
	}
	broken.fail = false
	if err = s.ReconcileDeletions(ctx, broken); err != nil {
		t.Fatal(err)
	}
	if _, err = a.Get(ctx, oldKey); !errors.Is(err, ErrNotFound) {
		t.Fatal("derived export survived withdrawal")
	}
	if err = s.Recover(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err = s.Append(ctx, e); err == nil {
		t.Fatal("index rebuild restored deleted consent")
	}
	if err = s.TeardownReady(ctx); err != nil {
		t.Fatal(err)
	}
}

type authority struct{ deny bool }

func (a *authority) Check(context.Context, capability.Claims) error {
	if a.deny {
		return ErrRejected
	}
	return nil
}
func TestWorkspaceAuthProvenanceAndNoQueries(t *testing.T) {
	s := fixture(t)
	caps, e := capability.Initialize(filepath.Join(t.TempDir(), "caps.db"))
	if e != nil {
		t.Fatal(e)
	}
	defer caps.DB.Close()
	ev := event()
	account := control.ID()
	claims := capability.Claims{AccountID: account, WorkspaceID: ev.WorkspaceID, DeploymentID: ev.DeploymentID, AuthVersion: 1, WorkspaceGeneration: 1, GrantVersion: 1, ActivationGeneration: 1, Operation: "event", ExpiresAt: time.Now().Add(time.Minute)}
	token, e := caps.Mint(context.Background(), claims)
	if e != nil {
		t.Fatal(e)
	}
	auth := &authority{}
	srv := &Server{Store: s, Capabilities: caps, Authority: auth, Participants: map[string]Participant{account: {AccountID: account, ParticipantID: ev.ParticipantID, Eligible: true, ConsentReference: "owner-confirmed-existing-consent"}}}
	h := srv.Workspace(ev.WorkspaceID)
	send := func(event Event, method string) int {
		b, _ := json.Marshal(event)
		r := httptest.NewRequest(method, "http://collector/v1/events", bytes.NewReader(b))
		r.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w.Code
	}
	ev.Trust = "independently_verified"
	if send(ev, "POST") != 400 {
		t.Fatal("sandbox forged provenance")
	}
	ev.Trust = "client_reported"
	if send(ev, "GET") != 404 {
		t.Fatal("sandbox can query events")
	}
	if send(ev, "POST") != 200 {
		t.Fatal("eligible event denied")
	}
	auth.deny = true
	ev.Sequence = 2
	ev.EventID = control.ID()
	if send(ev, "POST") != 403 {
		t.Fatal("revoked account can append")
	}
}

type roundtrip func(*http.Request) (*http.Response, error)

func (f roundtrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestR2SignedBoundedFixedDestination(t *testing.T) {
	calls := 0
	a := R2{AccountID: strings.Repeat("a", 32), Bucket: "feam-research-web-demo", AccessKeyID: "synthetic-access", SecretAccessKey: "synthetic-secret", Client: &http.Client{Transport: roundtrip(func(r *http.Request) (*http.Response, error) {
		calls++
		if !strings.HasPrefix(r.Header.Get("Authorization"), "AWS4-HMAC-SHA256 ") || !strings.Contains(r.URL.Host, ".r2.cloudflarestorage.com") || r.Header.Get("x-amz-content-sha256") == "" {
			t.Fatal("unsigned or wrong archive request")
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader("{}"))}, nil
	})}}
	key := "research/events/" + control.ID() + "/" + strings.Repeat("b", 64) + ".json"
	if e := a.Put(context.Background(), key, []byte("{}")); e != nil {
		t.Fatal(e)
	}
	if e := a.Put(context.Background(), "../secrets", nil); e == nil {
		t.Fatal("arbitrary object key")
	}
	if calls != 1 {
		t.Fatal("invalid request reached provider")
	}
}
func TestBrokerRecorderPrivatePeerAndRetry(t *testing.T) {
	s := fixture(t)
	ev := event()
	account := control.ID()
	auth := &authority{}
	server := &Server{Store: s, Authority: auth, Participants: map[string]Participant{account: {AccountID: account, ParticipantID: ev.ParticipantID, Eligible: true, ConsentReference: "owner-confirmed"}}, BrokerUID: uint32(os.Getuid())}
	dir, e := os.MkdirTemp("/tmp", "fc-")
	if e != nil {
		t.Fatal(e)
	}
	defer os.RemoveAll(dir)
	socket := filepath.Join(dir, "collector.sock")
	l, e := net.Listen("unix", socket)
	if e != nil {
		t.Fatal(e)
	}
	httpServer := ipc.Server(server.Control())
	go httpServer.Serve(l)
	defer httpServer.Close()
	r, e := NewBrokerRecorder(socket, ev.SoftwareSHA256, ev.ProfileSHA256, ev.DatasetSHA256)
	if e != nil {
		t.Fatal(e)
	}
	claims := capability.Claims{AccountID: account, WorkspaceID: ev.WorkspaceID, DeploymentID: ev.DeploymentID, AuthVersion: 1, WorkspaceGeneration: 1, GrantVersion: 1, ActivationGeneration: 1, Operation: "model", ExpiresAt: time.Now().Add(time.Minute)}
	payload := map[string]any{"request_sha256": strings.Repeat("d", 64), "model": "test/model", "provider": "test/provider", "profile": "test"}
	for i := 0; i < 2; i++ {
		if e = r.Record(context.Background(), claims, ev.RequestID, "model.request", payload); e != nil {
			t.Fatal("broker capture failed", e)
		}
	}
	var count int
	s.DB.QueryRow(`SELECT count(*) FROM events`).Scan(&count)
	if count != 1 {
		t.Fatal("broker retry duplicated")
	}
}
