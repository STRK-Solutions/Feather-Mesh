package lifecycle

import (
	"context"
	"errors"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/STRK-Solutions/Feather-Mesh/web_demo/internal/db"
)

func fixture(t *testing.T) (*Store, Workspace, Request) {
	t.Helper()
	d, e := db.Initialize(filepath.Join(t.TempDir(), "controller.db"), Migrations)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { d.Close() })
	s := New(d)
	dep := ID()
	if e = s.InitializeDeployment(dep, 1, []string{"slot-01", "slot-02", "slot-03", "slot-04"}); e != nil {
		t.Fatal(e)
	}
	if e = s.Desired(true); e != nil {
		t.Fatal(e)
	}
	w := Workspace{ID: ID(), OwnerID: ID(), Generation: 1, ImageID: "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", AssignmentVersion: 1}
	w.Hostname = "u-" + w.ID + ".613202690.xyz"
	if e = s.Assign(w); e != nil {
		t.Fatal(e)
	}
	w, e = s.Workspace(w.ID)
	if e != nil {
		t.Fatal(e)
	}
	r := Request{Protocol: "feam.web.v1", RequestID: ID(), IdempotencyKey: ID(), DeploymentID: dep, ActivationGeneration: 1, WorkspaceID: w.ID, ExpectedGeneration: 1, ActorID: w.OwnerID, AuthorizationVersion: 1, Method: "workspace.start"}
	return s, w, r
}
func reset(s *Store, r Request) Request {
	r.RequestID = ID()
	r.IdempotencyKey = ID()
	r.Method = "workspace.reset"
	r.Parameters.ConfirmationHash = Hash(r)
	_ = s.Confirm(r)
	return r
}
func TestConcurrentAdmissionAndIdempotence(t *testing.T) {
	s, _, r := fixture(t)
	var wg sync.WaitGroup
	results := make(chan Result, 20)
	for range 20 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			out, e := s.Admit(r)
			if e != nil {
				t.Error(e)
				return
			}
			results <- out
		}()
	}
	wg.Wait()
	close(results)
	job := ""
	for out := range results {
		if job == "" {
			job = out.JobID
		}
		if out.JobID != job {
			t.Fatal("duplicate runtime jobs")
		}
	}
	r.Method = "workspace.stop"
	if _, e := s.Admit(r); !errors.Is(e, ErrConflict) {
		t.Fatal("changed idempotency payload accepted", e)
	}
	r.IdempotencyKey = ID()
	if _, e := s.Admit(r); e == nil {
		t.Fatal("concurrent mutation accepted")
	}
}
func TestResetUsesFiniteSparesAndRetainsPrior(t *testing.T) {
	s, _, r := fixture(t)
	for i := range 2 {
		r = reset(s, r)
		out, e := s.Admit(r)
		if e != nil {
			t.Fatal(e)
		}
		j, e := s.Job(out.JobID)
		if e != nil {
			t.Fatal(e)
		}
		if e = s.Finish(j); e != nil {
			t.Fatal(e)
		}
		w, e := s.Workspace(r.WorkspaceID)
		if e != nil {
			t.Fatal(e)
		}
		if w.Generation != int64(i+2) {
			t.Fatal("generation not switched")
		}
		r.ExpectedGeneration = w.Generation
	}
	r = reset(s, r)
	if _, e := s.Admit(r); !errors.Is(e, ErrCapacity) {
		t.Fatal("allocated beyond fixed spare pool", e)
	}
	var retained int
	if e := s.DB.QueryRow(`SELECT count(*) FROM slots WHERE state='retained'`).Scan(&retained); e != nil || retained != 2 {
		t.Fatal("old generations not preserved", retained, e)
	}
}
func TestConfirmationOwnershipExpiryAndStopIntent(t *testing.T) {
	s, _, r := fixture(t)
	r = reset(s, r)
	r.ActorID = ID()
	if _, e := s.Admit(r); e == nil {
		t.Fatal("foreign owner accepted")
	}
}
func TestExpiredConfirmationAndStoppedDeployment(t *testing.T) {
	s, _, r := fixture(t)
	r = reset(s, r)
	now := s.Now()
	s.Now = func() time.Time { return now.Add(6 * time.Minute) }
	if _, e := s.Admit(r); !errors.Is(e, ErrAuthority) {
		t.Fatal("expired confirmation accepted", e)
	}
	r.Method = "workspace.start"
	r.Parameters = Parameters{}
	if e := s.Desired(false); e != nil {
		t.Fatal(e)
	}
	if _, e := s.Admit(r); !errors.Is(e, ErrAuthority) {
		t.Fatal("intentional stop accepted start", e)
	}
}

type fakeAuthority struct {
	denied    bool
	snapshots int
}

func (a *fakeAuthority) Check(context.Context, Request, Workspace) error {
	if a.denied {
		return ErrAuthority
	}
	return nil
}
func (a *fakeAuthority) Snapshot(context.Context, Workspace, int64) error { a.snapshots++; return nil }

type fakeRuntime struct {
	running map[string]bool
	starts  int
	lost    bool
	deny    bool
}

func (r *fakeRuntime) Inspect(_ context.Context, w Workspace) (bool, bool, error) {
	if r.deny {
		return false, false, errors.New("daemon unavailable")
	}
	v, ok := r.running[name(w)]
	return ok, v, nil
}
func (r *fakeRuntime) Start(_ context.Context, w Workspace) error {
	r.starts++
	r.running[name(w)] = true
	if r.lost {
		return errors.New("lost response")
	}
	return nil
}
func (r *fakeRuntime) Stop(_ context.Context, w Workspace) error {
	r.running[name(w)] = false
	return nil
}
func TestLostStartResponseReconcilesWithoutReplay(t *testing.T) {
	s, _, r := fixture(t)
	a := &fakeAuthority{}
	runtime := &fakeRuntime{running: map[string]bool{}, lost: true}
	c := Controller{Store: s, Authority: a, Runtime: runtime}
	out, e := c.Admit(context.Background(), r)
	if e != nil {
		t.Fatal(e)
	}
	if e = c.Work(context.Background(), out.JobID); e == nil {
		t.Fatal("lost response hidden")
	}
	if e = c.Work(context.Background(), out.JobID); e != nil {
		t.Fatal(e)
	}
	j, e := s.Job(out.JobID)
	if e != nil || j.State != "completed" || runtime.starts != 1 {
		t.Fatal("uncertain dispatch replayed", j, runtime.starts, e)
	}
}
func TestRevokedQueuedStartNeverRuns(t *testing.T) {
	s, _, r := fixture(t)
	a := &fakeAuthority{}
	runtime := &fakeRuntime{running: map[string]bool{}}
	c := Controller{Store: s, Authority: a, Runtime: runtime}
	out, e := c.Admit(context.Background(), r)
	if e != nil {
		t.Fatal(e)
	}
	a.denied = true
	if e = c.Work(context.Background(), out.JobID); !errors.Is(e, ErrAuthority) {
		t.Fatal(e)
	}
	if runtime.starts != 0 {
		t.Fatal("revoked start executed")
	}
}
func TestUnknownDoesNotReinitializeOrReplay(t *testing.T) {
	s, _, r := fixture(t)
	out, e := s.Admit(r)
	if e != nil {
		t.Fatal(e)
	}
	if e = s.Mark(out.JobID, "unknown", "lost"); e != nil {
		t.Fatal(e)
	}
	runtime := &fakeRuntime{running: map[string]bool{}}
	c := Controller{Store: s, Authority: &fakeAuthority{}, Runtime: runtime}
	if e = c.Work(context.Background(), out.JobID); e == nil {
		t.Fatal("missing runtime treated as success")
	}
	if runtime.starts != 0 {
		t.Fatal("uncertain start replayed")
	}
}
func TestHeartbeatNeverExtendsIdle(t *testing.T) {
	s, w, _ := fixture(t)
	now := time.Now()
	s.Now = func() time.Time { return now }
	_, e := s.DB.Exec(`UPDATE workspaces SET state='running',last_activity=? WHERE id=?`, stamp(now.Add(-31*time.Minute)), w.ID)
	if e != nil {
		t.Fatal(e)
	}
	if e = s.Activity(w.ID, "heartbeat"); e == nil {
		t.Fatal("heartbeat extended retention")
	}
	due, e := s.Due()
	if e != nil || len(due) != 1 {
		t.Fatal(due, e)
	}
	if e = s.Activity(w.ID, "terminal_input"); e != nil {
		t.Fatal(e)
	}
	due, e = s.Due()
	if e != nil || len(due) != 0 {
		t.Fatal(due, e)
	}
}
