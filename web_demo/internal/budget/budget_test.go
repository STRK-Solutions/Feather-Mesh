package budget

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

const testID = "00000000-0000-4000-8000-000000000001"

func allocation() Allocation {
	return Allocation{Protocol: Protocol, ID: testID, ProjectID: testID, RunID: testID, DeploymentID: testID, ActivationGeneration: 1, LedgerRevision: 1, Amount: 1000, RequestLimit: 1000, UserDailyLimit: 1000, Model: Model, Provider: Provider, Profile: Profile, InputPrice: 100000, OutputPrice: 500000, FeeBasisPoints: 10000, MaxOutputTokens: 64, NotBefore: time.Now().UTC().Add(-time.Minute), ExpiresAt: time.Now().UTC().Add(time.Hour)}
}
func signed(t *testing.T) (SignedAllocation, ed25519.PublicKey, ed25519.PrivateKey) {
	t.Helper()
	pub, key, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	doc, err := Sign(allocation(), key)
	if err != nil {
		t.Fatal(err)
	}
	return doc, pub, key
}
func TestAllocationIdentityPriceAndSignature(t *testing.T) {
	doc, pub, _ := signed(t)
	if err := doc.Verify(pub, testID, 1, time.Now()); err != nil {
		t.Fatal(err)
	}
	for _, change := range []func(*SignedAllocation){func(s *SignedAllocation) { s.Allocation.Amount++ }, func(s *SignedAllocation) { s.Signature[0] ^= 1 }, func(s *SignedAllocation) { s.Allocation.Provider = "other" }} {
		copy := doc
		copy.Signature = append([]byte(nil), doc.Signature...)
		change(&copy)
		if copy.Verify(pub, testID, 1, time.Now()) == nil {
			t.Fatal("tampered allocation accepted")
		}
	}
	if doc.Verify(pub, testID, 2, time.Now()) == nil || doc.Verify(pub, testID, 1, time.Now().Add(2*time.Hour)) == nil {
		t.Fatal("stale activation/expiry accepted")
	}
	cost, err := doc.Allocation.ReserveCost(11)
	if err != nil || cost != 34 {
		t.Fatalf("round-up reserve: %d %v", cost, err)
	}
}

func TestDrainReceiptPreservesUnknownAndPauseAcrossRestart(t *testing.T) {
	doc, pub, _ := signed(t)
	path := filepath.Join(t.TempDir(), "run.sqlite")
	r, err := InitializeRun(path, doc, pub, testID, 1)
	if err != nil {
		t.Fatal(err)
	}
	v := Reservation{ID: testID, Account: testID, Key: "drained", Hash: strings.Repeat("a", 64), AuthVersion: 1, Generation: 1, Amount: 800}
	if _, _, err = r.Reserve(context.Background(), v); err != nil {
		t.Fatal(err)
	}
	receipt, err := r.Drain(context.Background())
	if err != nil || receipt.Usage.Unknown != 800 || !receipt.Usage.Paused || receipt.Usage.Reserved != 0 || receipt.UnknownRequests != 1 || receipt.AllocationSHA256 != doc.Digest() {
		t.Fatal(receipt, err)
	}
	r.Close()
	r, err = OpenRun(path, doc, pub, testID, 1)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	v.ID = "00000000-0000-4000-8000-000000000002"
	v.Key = "after-restart"
	v.Amount = 1
	if _, _, err = r.Reserve(context.Background(), v); !errors.Is(err, ErrExhausted) {
		t.Fatal("restart replenished paused run", err)
	}
	u, _ := r.Usage(context.Background(), "")
	if !u.Paused || u.Unknown != 800 {
		t.Fatal("restart lost drain", u)
	}
}

func TestOrdinaryCheckpointRetainsSpendAndNeverUnpauses(t *testing.T) {
	doc, pub, _ := signed(t)
	path := filepath.Join(t.TempDir(), "run.sqlite")
	r, err := InitializeRun(path, doc, pub, testID, 1)
	if err != nil {
		t.Fatal(err)
	}
	v := Reservation{ID: testID, Account: testID, Key: "before-stop", Hash: strings.Repeat("a", 64), AuthVersion: 1, Generation: 1, Amount: 800}
	if _, _, err = r.Reserve(context.Background(), v); err != nil {
		t.Fatal(err)
	}
	receipt, err := r.Checkpoint(context.Background())
	if err != nil || receipt.Usage.Paused || receipt.Usage.Unknown != 800 {
		t.Fatal(receipt, err)
	}
	r.Close()
	r, err = OpenRun(path, doc, pub, testID, 1)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	prior, exists, err := r.Reserve(context.Background(), v)
	if err != nil || !exists || prior.State != "unknown" {
		t.Fatal("restart replayed request", prior, exists, err)
	}
	v.ID = "00000000-0000-4000-8000-000000000002"
	v.Key = "after-restart"
	v.Amount = 201
	if _, _, err = r.Reserve(context.Background(), v); !errors.Is(err, ErrExhausted) {
		t.Fatal("restart replenished budget", err)
	}
	v.Amount = 200
	if _, _, err = r.Reserve(context.Background(), v); err != nil {
		t.Fatal("active run did not preserve remaining budget", err)
	}
	if err = r.Pause(context.Background()); err != nil {
		t.Fatal(err)
	}
	receipt, err = r.Checkpoint(context.Background())
	if err != nil || !receipt.Usage.Paused || receipt.Usage.Unknown != 1000 {
		t.Fatal("checkpoint unpaused run", receipt, err)
	}
}
func TestRunCrashDuplicateUnknownAndNearLimit(t *testing.T) {
	ctx := context.Background()
	doc, pub, _ := signed(t)
	path := filepath.Join(t.TempDir(), "run.sqlite")
	r, err := InitializeRun(path, doc, pub, testID, 1)
	if err != nil {
		t.Fatal(err)
	}
	v := Reservation{ID: testID, Account: testID, Key: "request", Hash: strings.Repeat("a", 64), AuthVersion: 1, Generation: 1, Amount: 800}
	_, dup, err := r.Reserve(ctx, v)
	if err != nil || dup {
		t.Fatal(err)
	}
	_, dup, err = r.Reserve(ctx, v)
	if err != nil || !dup {
		t.Fatal("duplicate lost")
	}
	v.Hash = strings.Repeat("b", 64)
	if _, _, err = r.Reserve(ctx, v); !errors.Is(err, ErrConflict) {
		t.Fatal("changed idempotency accepted")
	}
	r.Close()
	r, err = OpenRun(path, doc, pub, testID, 1)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	got, err := r.Lookup(ctx, testID, testID)
	if err != nil || got.State != "unknown" {
		t.Fatal("crash reservation released", got, err)
	}
	v.ID = "00000000-0000-4000-8000-000000000002"
	v.Key = "second"
	v.Amount = 201
	if _, _, err = r.Reserve(ctx, v); !errors.Is(err, ErrExhausted) {
		t.Fatal("unknown reservation replenished", err)
	}
	if err = r.Settle(testID, "provider-id", 101); err != nil {
		t.Fatal(err)
	}
	if err = r.Settle(testID, "provider-id", 101); err != nil {
		t.Fatal(err)
	}
	u, err := r.Usage(ctx, "")
	if err != nil || u.Settled != 101 || u.Unknown != 0 {
		t.Fatal(u, err)
	}
	v.Amount = 899
	if _, _, err = r.Reserve(ctx, v); err != nil {
		t.Fatal(err)
	}
	if r.Dispatch(ctx, testID) == nil {
		t.Fatal("settled request replay")
	}
	u, _ = r.Usage(ctx, "")
	if u.Alert != 90 {
		t.Fatal("missing alert", u)
	}
}
func TestRunConcurrentReservationsAndOverchargePauses(t *testing.T) {
	doc, pub, _ := signed(t)
	r, err := InitializeRun(filepath.Join(t.TempDir(), "run.sqlite"), doc, pub, testID, 1)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	var wg sync.WaitGroup
	results := make(chan error, 20)
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			id := strings.Repeat("0", 7) + string(rune('a'+i%6)) + "-0000-4000-8000-" + strings.Repeat("0", 11) + string("0123456789abcdef"[i%16])
			_, _, err := r.Reserve(context.Background(), Reservation{ID: id, Account: testID, Key: id, Hash: strings.Repeat("f", 64), AuthVersion: 1, Generation: 1, Amount: 100})
			results <- err
		}(i)
	}
	wg.Wait()
	close(results)
	success := 0
	for err := range results {
		if err == nil {
			success++
		} else if !errors.Is(err, ErrExhausted) {
			t.Fatal(err)
		}
	}
	if success != 10 {
		t.Fatalf("admitted %d", success)
	}
	u, _ := r.Usage(context.Background(), "")
	if u.Reserved != 1000 {
		t.Fatal(u)
	}
	var id string
	if err = r.DB.QueryRow(`SELECT id FROM requests LIMIT 1`).Scan(&id); err != nil {
		t.Fatal(err)
	}
	if r.Settle(id, "provider", 101) == nil {
		t.Fatal("overcharge accepted")
	}
	u, _ = r.Usage(context.Background(), "")
	if !u.Paused {
		t.Fatal("overcharge did not pause")
	}
}
func TestMissingRunDoesNotInitialize(t *testing.T) {
	doc, pub, _ := signed(t)
	path := filepath.Join(t.TempDir(), "missing")
	if _, err := OpenRun(path, doc, pub, testID, 1); err == nil {
		t.Fatal("missing initialized")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("state created")
	}
}
func TestProjectPersistsUnknownAndExactlyOnceClosure(t *testing.T) {
	doc, _, key := signed(t)
	dir := t.TempDir()
	if err := os.Chmod(dir, 0700); err != nil {
		t.Fatal(err)
	}
	s := ProjectStore{filepath.Join(dir, "project.enc"), make([]byte, 32)}
	if err := s.Initialize(testID, "operator", 100000000, 17); err != nil {
		t.Fatal(err)
	}
	issued, err := s.Allocate("operator", doc.Allocation, key)
	if err != nil {
		t.Fatal(err)
	}
	if issued.Allocation.LedgerRevision != 2 {
		t.Fatal("revision unbound")
	}
	if err = s.MarkUnknown("operator", testID); err != nil {
		t.Fatal(err)
	}
	p, err := s.Read()
	if err != nil || p.Exposure() != 1017 {
		t.Fatal(p, err)
	}
	if err = s.Initialize(testID, "operator", 100000000, 0); err == nil {
		t.Fatal("reinitialized")
	}
	if err = s.SetCeiling("admin", 1000); !errors.Is(err, ErrExhausted) {
		t.Fatal("lowered committed allowance")
	}
	for i := 0; i < 2; i++ {
		if err = s.Reconcile("operator", testID, strings.Repeat("a", 64), 9); err != nil {
			t.Fatal(err)
		}
	}
	p, err = s.Read()
	if err != nil || p.Settled != 26 || p.Exposure() != 26 {
		t.Fatal(p, err)
	}
	if err = s.SetCeiling("admin", 200000000); err != nil {
		t.Fatal(err)
	}
	if _, err = s.Allocate("operator", doc.Allocation, key); !errors.Is(err, ErrConflict) {
		t.Fatal("allocation replay accepted")
	}
	encrypted, _ := os.ReadFile(s.Path)
	if strings.Contains(string(encrypted), "operator") || strings.Contains(string(encrypted), testID) {
		t.Fatal("plaintext ledger")
	}
	s.Key[0] = 1
	if _, err = s.Read(); err == nil {
		t.Fatal("wrong encryption key accepted")
	}
}
func TestProjectTornAuditFailsClosed(t *testing.T) {
	dir := t.TempDir()
	_ = os.Chmod(dir, 0700)
	s := ProjectStore{filepath.Join(dir, "project.enc"), make([]byte, 32)}
	if err := s.Initialize(testID, "operator", 1000, 0); err != nil {
		t.Fatal(err)
	}
	f, err := os.OpenFile(s.Path+".audit", os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = f.WriteString("torn\n")
	_ = f.Close()
	if _, err = s.Read(); err == nil {
		t.Fatal("interrupted state silently recovered")
	}
	if err = s.Initialize(testID, "operator", 1000, 0); err == nil {
		t.Fatal("interrupted state replenished")
	}
}

func TestPerRequestAndDailyGuardrailsNeverReplenishRun(t *testing.T) {
	pub, key, _ := ed25519.GenerateKey(rand.Reader)
	a := allocation()
	a.RequestLimit = 400
	a.UserDailyLimit = 600
	doc, err := Sign(a, key)
	if err != nil {
		t.Fatal(err)
	}
	r, err := InitializeRun(filepath.Join(t.TempDir(), "run"), doc, pub, testID, 1)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	ctx := context.Background()
	v := Reservation{ID: testID, Account: testID, Key: "first", Hash: strings.Repeat("e", 64), AuthVersion: 1, Generation: 1, Amount: 401}
	if _, _, err = r.Reserve(ctx, v); !errors.Is(err, ErrExhausted) {
		t.Fatal("request limit bypassed", err)
	}
	v.Amount = 400
	if _, _, err = r.Reserve(ctx, v); err != nil {
		t.Fatal(err)
	}
	v.ID = "00000000-0000-4000-8000-000000000002"
	v.Key = "next"
	v.Amount = 201
	if _, _, err = r.Reserve(ctx, v); !errors.Is(err, ErrExhausted) {
		t.Fatal("daily limit bypassed", err)
	}
	v.Account = "00000000-0000-4000-8000-000000000003"
	v.Amount = 400
	if _, _, err = r.Reserve(ctx, v); err != nil {
		t.Fatal(err)
	}
	if _, err = r.DB.Exec(`UPDATE requests SET created_at='2000-01-01T00:00:00Z' WHERE id=?`, testID); err != nil {
		t.Fatal(err)
	}
	v.Account = testID
	v.ID = "00000000-0000-4000-8000-000000000004"
	v.Key = "new-day"
	v.Amount = 200
	if _, _, err = r.Reserve(ctx, v); err != nil {
		t.Fatal(err)
	}
	v.ID = "00000000-0000-4000-8000-000000000005"
	v.Key = "cannot-replenish"
	v.Amount = 1
	if _, _, err = r.Reserve(ctx, v); !errors.Is(err, ErrExhausted) {
		t.Fatal("calendar replenished global reservation", err)
	}
	u, err := r.Usage(ctx, testID)
	if err != nil || u.DailyLimit != 600 || u.DailyExposure != 200 || u.Reserved != 600 {
		t.Fatal(u, err)
	}
}
