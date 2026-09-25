package lifecycle

import (
	"context"
	"errors"
	"github.com/STRK-Solutions/Feather-Mesh/web_demo/internal/control"
	"testing"
	"time"
)

type assignmentAuthority struct {
	fakeAuthority
	assignment control.Assignments
}

func TestGrantChangeWaitsForPendingSlotReclamation(t *testing.T) {
	s, w, _ := fixture(t)
	now := s.Now().Add(27 * 24 * time.Hour)
	s.Now = func() time.Time { return now }
	if _, e := s.WarnRetention(); e != nil {
		t.Fatal(e)
	}
	now = now.Add(3 * 24 * time.Hour)
	if e := s.Desired(false); e != nil {
		t.Fatal(e)
	}
	runtime := &removingRuntime{fakeRuntime: fakeRuntime{running: map[string]bool{name(w): false}}}
	authority := &assignmentAuthority{assignment: control.Assignments{Account: control.Account{ID: w.OwnerID}, Workspace: control.Workspace{ID: w.ID, Generation: 1, GrantVersion: 2, DeploymentID: w.DeploymentID, ActivationGeneration: 1}}}
	c := Controller{Store: s, Runtime: runtime, Authority: authority, Capabilities: &fakeCapabilities{}}
	review, e := c.PrepareReclaim(context.Background(), Slot{ID: w.SlotID, Path: "/home/feam-service-data/" + w.SlotID, FilesystemUUID: ID()})
	if e != nil {
		t.Fatal(e)
	}
	if e = c.Reconfigure(context.Background(), w.ID, 2); !errors.Is(e, ErrConflict) {
		t.Fatal("grant changed during cleanup", e)
	}
	current, _ := s.Workspace(w.ID)
	if current.Generation != w.Generation || current.AssignmentVersion != w.AssignmentVersion {
		t.Fatal("cleanup scope changed", current)
	}
	if e = c.completeReclaim(receiptFor(review)); e != nil {
		t.Fatal(e)
	}
	if e = c.Reconfigure(context.Background(), w.ID, 2); e != nil {
		t.Fatal("grant retry after cleanup failed", e)
	}
}

func (a *assignmentAuthority) Assignments(context.Context, string) (control.Assignments, error) {
	return a.assignment, nil
}

type fakeCapabilities struct {
	revoked, prepared, activated int
	failRevoke                   bool
}

func (f *fakeCapabilities) Prepare(context.Context, Workspace) error  { f.prepared++; return nil }
func (f *fakeCapabilities) Activate(context.Context, Workspace) error { f.activated++; return nil }
func (f *fakeCapabilities) Revoke(context.Context, Workspace) error {
	f.revoked++
	if f.failRevoke {
		return errors.New("broker unavailable")
	}
	return nil
}
func TestGrantChangeStopsUncertainResetPreservesBytesAndAdvancesPastTarget(t *testing.T) {
	s, w, r := fixture(t)
	r = reset(s, r)
	out, e := s.Admit(r)
	if e != nil {
		t.Fatal(e)
	}
	j, _ := s.Job(out.JobID)
	_ = s.Mark(j.ID, "unknown", "lost")
	target := w
	target.Generation = j.TargetGeneration
	target.SlotID = j.TargetSlot
	runtime := &fakeRuntime{running: map[string]bool{name(w): true, name(target): true}}
	authority := &assignmentAuthority{assignment: control.Assignments{Account: control.Account{ID: w.OwnerID}, Workspace: control.Workspace{ID: w.ID, Generation: 1, GrantVersion: 2, DeploymentID: w.DeploymentID, ActivationGeneration: 1}}}
	caps := &fakeCapabilities{}
	c := Controller{Store: s, Runtime: runtime, Authority: authority, Capabilities: caps}
	if e = c.Reconfigure(context.Background(), w.ID, 2); e != nil {
		t.Fatal(e)
	}
	current, _ := s.Workspace(w.ID)
	if current.Generation != 3 || current.State != "stopped" || current.SlotID != w.SlotID || current.AssignmentVersion != 2 || runtime.running[name(w)] || runtime.running[name(target)] || runtime.starts != 0 || caps.revoked != 1 {
		t.Fatalf("unsafe grant transition: %#v %#v %#v", current, runtime, caps)
	}
	j, _ = s.Job(j.ID)
	if j.State != "failed" {
		t.Fatal("uncertain job remained replayable", j)
	}
	var state string
	_ = s.DB.QueryRow(`SELECT state FROM slots WHERE id=?`, target.SlotID).Scan(&state)
	if state != "retained" {
		t.Fatal("uncertain reset bytes not retained")
	}
	if e = c.Reconfigure(context.Background(), w.ID, 2); e != nil {
		t.Fatal(e)
	}
	current, _ = s.Workspace(w.ID)
	if current.Generation != 3 {
		t.Fatal("repeated grant delivery changed generation")
	}
	if e = c.Reconfigure(context.Background(), w.ID, 1); e == nil {
		t.Fatal("stale grant version accepted")
	}
}
func TestDisableStillStopsWhenCapabilityServiceUnavailable(t *testing.T) {
	s, w, _ := fixture(t)
	runtime := &fakeRuntime{running: map[string]bool{name(w): true}}
	c := Controller{Store: s, Runtime: runtime, Authority: &fakeAuthority{}, Capabilities: &fakeCapabilities{failRevoke: true}}
	if e := c.Revoke(context.Background(), w.OwnerID); e == nil {
		t.Fatal("failed revocation reported complete")
	}
	if runtime.running[name(w)] {
		t.Fatal("capability failure prevented workspace stop")
	}
}
func TestIntentionalStopReconcilesUnknownStartWithoutResuming(t *testing.T) {
	s, w, r := fixture(t)
	out, e := s.Admit(r)
	if e != nil {
		t.Fatal(e)
	}
	_ = s.Mark(out.JobID, "unknown", "lost")
	_ = s.Desired(false)
	runtime := &fakeRuntime{running: map[string]bool{name(w): true}}
	c := Controller{Store: s, Runtime: runtime, Authority: &fakeAuthority{}}
	if e = c.Work(context.Background(), out.JobID); !errors.Is(e, ErrAuthority) {
		t.Fatal(e)
	}
	if runtime.running[name(w)] || runtime.starts != 0 {
		t.Fatal("intentional off state was ignored")
	}
}

func TestRunningStopAndDeleteStopBeforeVerification(t *testing.T) {
	for _, method := range []string{"workspace.stop", "workspace.delete"} {
		t.Run(method, func(t *testing.T) {
			s, w, r := fixture(t)
			r.Method = method
			if method == "workspace.delete" {
				r.Parameters.ConfirmationHash = Hash(r)
				if e := s.Confirm(r); e != nil {
					t.Fatal(e)
				}
			}
			runtime := &fakeRuntime{running: map[string]bool{name(w): true}}
			caps := &fakeCapabilities{}
			c := Controller{Store: s, Runtime: runtime, Authority: &fakeAuthority{}, Capabilities: caps}
			out, e := c.Admit(context.Background(), r)
			if e != nil {
				t.Fatal(e)
			}
			if e = c.Work(context.Background(), out.JobID); e != nil {
				t.Fatal(e)
			}
			if runtime.running[name(w)] || runtime.starts != 0 || caps.prepared != 0 || caps.revoked < 1 {
				t.Fatal("running workspace not safely stopped")
			}
			j, _ := s.Job(out.JobID)
			if j.State != "completed" {
				t.Fatal(j)
			}
		})
	}
}
func TestAlreadyRunningStartDoesNotRotateMountedCredentials(t *testing.T) {
	s, w, r := fixture(t)
	runtime := &fakeRuntime{running: map[string]bool{name(w): true}}
	caps := &fakeCapabilities{}
	c := Controller{Store: s, Runtime: runtime, Authority: &fakeAuthority{}, Capabilities: caps}
	out, e := c.Admit(context.Background(), r)
	if e != nil {
		t.Fatal(e)
	}
	if e = c.Work(context.Background(), out.JobID); e != nil {
		t.Fatal(e)
	}
	if caps.prepared != 0 || runtime.starts != 0 {
		t.Fatal("active credentials or process replaced")
	}
}
