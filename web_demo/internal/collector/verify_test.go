package collector

import (
	"context"
	"os"
	"testing"
	"time"
)

func TestFreshArchiveVerificationRejectsMutationAndDeletedResurrection(t *testing.T) {
	s, a := fixture(t), privateArchive(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	e := event()
	if _, err := s.Append(ctx, e); err != nil {
		t.Fatal(err)
	}
	if _, err := s.VerifyArchive(ctx, a); err == nil {
		t.Fatal("unflushed record verified")
	}
	if err := s.Flush(ctx, a); err != nil {
		t.Fatal(err)
	}
	r, err := s.VerifyArchiveWithInventory(ctx, a, true)
	if err != nil || r.Batches != 1 || len(r.Inventory) != 1 {
		t.Fatal(r, err)
	}
	key := r.Inventory[0].Key
	p, _ := a.path(key)
	original, _ := os.ReadFile(p)
	if err = os.WriteFile(p, []byte("tampered"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = s.VerifyArchive(ctx, a); err == nil {
		t.Fatal("stale upload receipt hid archive corruption")
	}
	if err = os.WriteFile(p, original, 0600); err != nil {
		t.Fatal(err)
	}
	if err = s.Withdraw(ctx, a, e.ParticipantID, "withdrawal"); err != nil {
		t.Fatal(err)
	}
	r, err = s.VerifyArchiveWithInventory(ctx, a, true)
	if err != nil || r.Deletions != 1 || r.Absent != 1 || r.Batches != 0 {
		t.Fatal(r, err)
	}
	if err = os.WriteFile(p, original, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = s.VerifyArchive(ctx, a); err == nil {
		t.Fatal("deleted object resurrection accepted")
	}
	os.Remove(p)
	for _, v := range r.Inventory {
		if v.Kind == "deletion" {
			if err = a.Delete(ctx, v.Key); err != nil {
				t.Fatal(err)
			}
		}
	}
	if _, err = s.VerifyArchive(ctx, a); err == nil {
		t.Fatal("missing deletion tombstone accepted")
	}
	if _, err = s.VerifyArchive(context.Background(), a); err == nil {
		t.Fatal("unbounded archive traversal accepted")
	}
}
