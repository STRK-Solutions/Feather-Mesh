package collector

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
)

// ArchiveReceipt binds fresh object reads and a complete bounded research-prefix
// listing. Preserve the richer ArchiveLedger for metadata-only recreation.
type ArchiveReceipt struct {
	Protocol   string          `json:"protocol"`
	Status     string          `json:"status"`
	Batches    int             `json:"batches"`
	Exports    int             `json:"exports"`
	Deletions  int             `json:"deletions"`
	Absent     int             `json:"absent"`
	Bytes      int64           `json:"bytes"`
	SHA256     string          `json:"sha256"`
	VerifiedAt string          `json:"verified_at"`
	Inventory  []ArchiveObject `json:"inventory,omitempty"`
}

type ArchiveObject struct {
	Kind   string `json:"kind"`
	Key    string `json:"object_key"`
	Hash   string `json:"sha256"`
	Status string `json:"state"`
}

// VerifyArchive never trusts an earlier upload receipt. An immediate SQLite
// transaction prevents a second collector process changing the ledger while
// every retained object is fetched and every deleted object is checked absent.
// Run after draining capture; the caller must provide a bounded deadline.
func (s *Store) VerifyArchive(ctx context.Context, a Archive) (ArchiveReceipt, error) {
	return s.VerifyArchiveWithInventory(ctx, a, false)
}

func (s *Store) VerifyArchiveWithInventory(ctx context.Context, a Archive, includeInventory bool) (ArchiveReceipt, error) {
	l, e := s.ExportArchiveLedger(ctx, a)
	if e != nil {
		return ArchiveReceipt{}, e
	}
	items := ledgerInventory(l)
	r := ArchiveReceipt{Protocol: "feam.archive-verification.v1", Status: "verified", VerifiedAt: l.ExportedAt}
	if includeInventory {
		b, _ := json.Marshal(items)
		if len(b) > 768<<10 {
			return ArchiveReceipt{}, errors.New("archive inventory exceeds operator receipt bound; retain host")
		}
		r.Inventory = items
	}
	h := sha256.New()
	for _, v := range items {
		if v.Status == "deleted" {
			r.Absent++
		} else {
			switch v.Kind {
			case "batch":
				r.Batches++
			case "export":
				r.Exports++
			case "deletion":
				r.Deletions++
			}
		}
		b, _ := json.Marshal(v)
		h.Write(b)
		h.Write([]byte("\n"))
	}
	for _, v := range l.Objects {
		if v.State == "verified" {
			r.Bytes += v.Bytes
		}
	}
	for _, v := range l.Deletions {
		if v.ArchiveState == "verified" {
			_, b := deletionObject(v)
			r.Bytes += int64(len(b))
		}
	}
	r.SHA256 = hex.EncodeToString(h.Sum(nil))
	return r, nil
}
