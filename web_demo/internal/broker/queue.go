package broker

import (
	"context"
	"errors"
	"sync"
)

var ErrCapacity = errors.New("capacity")

type waiter struct {
	account string
	ready   chan struct{}
	granted bool
}

// Queue selects the oldest eligible account, skips accounts already in flight,
// and caps each account's waiting entries, preventing one user filling the host.
type Queue struct {
	mu      sync.Mutex
	active  map[string]bool
	waiting []*waiter
}

func NewQueue() *Queue { return &Queue{active: map[string]bool{}} }
func (q *Queue) schedule() {
	for len(q.active) < 3 {
		at := -1
		for i, w := range q.waiting {
			if !q.active[w.account] {
				at = i
				break
			}
		}
		if at < 0 {
			return
		}
		w := q.waiting[at]
		q.waiting = append(q.waiting[:at], q.waiting[at+1:]...)
		q.active[w.account] = true
		w.granted = true
		close(w.ready)
	}
}
func (q *Queue) Acquire(ctx context.Context, account string) (func(), error) {
	q.mu.Lock()
	count := 0
	for _, w := range q.waiting {
		if w.account == account {
			count++
		}
	}
	if count >= 2 || len(q.waiting) >= 64 {
		q.mu.Unlock()
		return nil, ErrCapacity
	}
	w := &waiter{account: account, ready: make(chan struct{})}
	q.waiting = append(q.waiting, w)
	q.schedule()
	q.mu.Unlock()
	select {
	case <-ctx.Done():
		q.mu.Lock()
		if w.granted {
			delete(q.active, account)
		} else {
			for i, v := range q.waiting {
				if v == w {
					q.waiting = append(q.waiting[:i], q.waiting[i+1:]...)
					break
				}
			}
		}
		q.schedule()
		q.mu.Unlock()
		return nil, ctx.Err()
	case <-w.ready:
		var once sync.Once
		return func() { once.Do(func() { q.mu.Lock(); delete(q.active, account); q.schedule(); q.mu.Unlock() }) }, nil
	}
}
func (q *Queue) State() (int, int) {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.active), len(q.waiting)
}
