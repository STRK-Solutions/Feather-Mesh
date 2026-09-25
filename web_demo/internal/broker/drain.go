package broker

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"

	"github.com/STRK-Solutions/Feather-Mesh/web_demo/internal/budget"
)

var errOperation = errors.New("workspace operation unavailable")
var errDraining = errors.New("broker run is draining")

func (s *Server) admit(cancel context.CancelFunc) (func(), error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.draining {
		return nil, errDraining
	}
	s.next++
	id := s.next
	s.pending[id] = cancel
	s.requests.Add(1)
	return func() {
		s.mu.Lock()
		delete(s.pending, id)
		s.mu.Unlock()
		s.requests.Done()
	}, nil
}

// Drain cancels all queued/active requests and joins their accounting defers.
// Durable pause happens before waiting; timeout is a failed drain, not a receipt.
func (s *Server) Drain(ctx context.Context) (budget.DrainReceipt, error) {
	return s.stop(ctx, true)
}

// Shutdown stops this process but preserves the existing run admission state.
// The next process still rechecks current capabilities, grants and activation.
func (s *Server) Shutdown(ctx context.Context) (budget.DrainReceipt, error) {
	return s.stop(ctx, false)
}

func (s *Server) stop(ctx context.Context, pause bool) (budget.DrainReceipt, error) {
	s.drainMu.Lock()
	defer s.drainMu.Unlock()
	s.mu.Lock()
	s.draining = true
	for _, cancel := range s.pending {
		cancel()
	}
	s.mu.Unlock()
	if pause {
		if err := s.Run.Pause(ctx); err != nil {
			return budget.DrainReceipt{}, err
		}
	}
	done := make(chan struct{})
	go func() { s.requests.Wait(); close(done) }()
	select {
	case <-ctx.Done():
		return budget.DrainReceipt{}, ctx.Err()
	case <-done:
	}
	var receipt budget.DrainReceipt
	var err error
	if pause {
		receipt, err = s.Run.Drain(ctx)
	} else {
		receipt, err = s.Run.Checkpoint(ctx)
	}
	if err == nil && s.ReceiptFile != "" {
		err = WriteDrainReceipt(s.ReceiptFile, receipt)
	}
	return receipt, err
}

func WriteDrainReceipt(path string, receipt budget.DrainReceipt) error {
	if !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return budget.ErrInvalid
	}
	parent := filepath.Dir(path)
	info, err := os.Lstat(parent)
	if err != nil || !info.IsDir() || info.Mode().Perm()&0077 != 0 {
		return budget.ErrInvalid
	}
	if info, err = os.Lstat(path); err == nil && (!info.Mode().IsRegular() || info.Mode().Perm()&0077 != 0) {
		return budget.ErrInvalid
	} else if err != nil && !os.IsNotExist(err) {
		return err
	}
	b, err := json.MarshalIndent(receipt, "", "  ")
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(parent, ".drain-")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err = f.Write(append(b, '\n')); err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err == nil {
		err = closeErr
	}
	if err == nil {
		err = os.Rename(f.Name(), path)
	}
	if err == nil {
		d, e := os.Open(parent)
		if e != nil {
			return e
		}
		err = d.Sync()
		_ = d.Close()
	}
	return err
}
