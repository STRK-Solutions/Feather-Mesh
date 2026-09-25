package pipeline

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"time"
)

type InventoryEntry struct {
	Path   string `json:"path"`
	Bytes  int64  `json:"bytes"`
	SHA256 string `json:"sha256"`
}
type Review struct {
	Status         Status           `json:"status"`
	Job            Job              `json:"job"`
	CandidateBytes int64            `json:"candidate_bytes"`
	Inventory      []InventoryEntry `json:"inventory"`
	ReleaseStatus  string           `json:"release_status"`
}

func (m *Manager) Jobs() ([]Status, error) {
	rows, err := m.DB.Query(`SELECT id FROM dataset_jobs ORDER BY created_at DESC LIMIT 100`)
	if err != nil {
		return nil, err
	}
	var ids []string
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			return nil, err
		}
		ids = append(ids, id)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	result := []Status{}
	for _, id := range ids {
		s, e := m.Status(id)
		if e != nil {
			return nil, e
		}
		result = append(result, s)
	}
	return result, nil
}

func (m *Manager) Review(id string) (Review, error) {
	// Status remains readable while a live worker holds its mutation lock.
	status, statusErr := m.Status(id)
	if statusErr != nil {
		return Review{}, statusErr
	}
	if status.State != "prepared" && status.State != "approved" && status.State != "released" {
		job, e := m.job(id)
		return Review{Status: status, Job: job, Inventory: []InventoryEntry{}}, e
	}
	unlock, err := m.lock(id)
	if err != nil {
		return Review{}, err
	}
	defer unlock()
	s, err := m.Status(id)
	if err != nil {
		return Review{}, err
	}
	j, err := m.job(id)
	if err != nil {
		return Review{}, err
	}
	out := Review{Status: s, Job: j, Inventory: []InventoryEntry{}}
	if s.State == "released" {
		if err = m.DB.QueryRow(`SELECT status FROM releases WHERE digest=? AND bundle=?`, s.CandidateHash, j.Bundle).Scan(&out.ReleaseStatus); err != nil {
			return Review{}, err
		}
	}
	if s.State != "prepared" && s.State != "approved" && s.State != "released" {
		return out, nil
	}
	root := m.payload(id)
	if s.State == "released" {
		root = m.release(j.Bundle, s.CandidateHash)
	}
	hash, total, err := TreeHash(root)
	if err != nil || hash != s.CandidateHash {
		return Review{}, errors.New("candidate review integrity mismatch")
	}
	out.CandidateBytes = total
	err = filepath.WalkDir(root, func(path string, entry os.DirEntry, e error) error {
		if e != nil {
			return e
		}
		if entry.IsDir() {
			return nil
		}
		rel, e := filepath.Rel(root, path)
		if e != nil {
			return e
		}
		h, n, e := fileHash(path)
		if e != nil {
			return e
		}
		out.Inventory = append(out.Inventory, InventoryEntry{Path: filepath.ToSlash(rel), Bytes: n, SHA256: h})
		return nil
	})
	return out, err
}

func (m *Manager) Enqueue(ctx context.Context, j Job, hash, actor string) (Status, error) {
	if !uuidPattern.MatchString(actor) {
		return Status{}, errors.New("approving admin identity required")
	}
	if err := m.reserve(ctx, j, hash, actor); err != nil {
		return Status{}, err
	}
	return m.Status(j.ID)
}

// RunQueue executes only unstarted durable jobs. A preparing/publishing state
// left by a crashed worker is never replayed automatically.
func (m *Manager) RunQueue(ctx context.Context) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		if ctx.Err() != nil {
			return
		}
		m.runNext(ctx)
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
func (m *Manager) runNext(ctx context.Context) {
	var id string
	if m.DB.QueryRowContext(ctx, `SELECT id FROM dataset_jobs WHERE state='queued' ORDER BY created_at LIMIT 1`).Scan(&id) != nil {
		return
	}
	unlock, err := m.lock(id)
	if err != nil {
		return
	}
	defer unlock()
	j, err := m.job(id)
	if err != nil {
		return
	}
	_, _ = m.executeQueued(ctx, j, "")
}
