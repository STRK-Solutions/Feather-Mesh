package auth

import (
	"context"
	"io"
	"sync"
	"time"
)

// Streams closes every tracked transport when current local policy or token
// expiry ceases to authorize it. Rechecks never exceed five seconds.
type Streams struct {
	mu    sync.Mutex
	next  uint64
	items map[uint64]stream
}
type stream struct {
	account string
	close   io.Closer
	check   func(context.Context) bool
	expiry  time.Time
}

func (s *Streams) Track(account string, c io.Closer, expiry time.Time, check func(context.Context) bool) func() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.items == nil {
		s.items = map[uint64]stream{}
	}
	s.next++
	id := s.next
	s.items[id] = stream{account, c, check, expiry}
	return func() { s.mu.Lock(); delete(s.items, id); s.mu.Unlock() }
}
func (s *Streams) Revoke(account string) {
	s.mu.Lock()
	var close []io.Closer
	for id, v := range s.items {
		if v.account == account {
			close = append(close, v.close)
			delete(s.items, id)
		}
	}
	s.mu.Unlock()
	for _, c := range close {
		_ = c.Close()
	}
}
func (s *Streams) Check(ctx context.Context) {
	s.mu.Lock()
	items := make(map[uint64]stream, len(s.items))
	for id, v := range s.items {
		items[id] = v
	}
	s.mu.Unlock()
	var checks sync.WaitGroup
	for id, v := range items {
		checks.Add(1)
		go func(id uint64, v stream) {
			defer checks.Done()
			checkCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
			valid := time.Now().Before(v.expiry) && v.check(checkCtx)
			cancel()
			if !valid {
				s.mu.Lock()
				_, exists := s.items[id]
				delete(s.items, id)
				s.mu.Unlock()
				if exists {
					_ = v.close.Close()
				}
			}
		}(id, v)
	}
	checks.Wait()

}
func (s *Streams) Run(ctx context.Context) {
	t := time.NewTicker(5 * time.Second)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			s.mu.Lock()
			var close []io.Closer
			for id, v := range s.items {
				close = append(close, v.close)
				delete(s.items, id)
			}
			s.mu.Unlock()
			for _, c := range close {
				_ = c.Close()
			}
			return
		case <-t.C:
			s.Check(ctx)
		}
	}
}
