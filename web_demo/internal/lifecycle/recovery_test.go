package lifecycle

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestControllerProcessLockRejectsSecondWriterAndSymlink(t *testing.T) {
	path := filepath.Join(t.TempDir(), "controller.db")
	first, err := ProcessLock(path)
	if err != nil {
		t.Fatal(err)
	}
	if second, err := ProcessLock(path); err == nil {
		second.Close()
		t.Fatal("concurrent operator writer accepted")
	}
	first.Close()
	second, err := ProcessLock(path)
	if err != nil {
		t.Fatal(err)
	}
	second.Close()
	link := filepath.Join(t.TempDir(), "other.db")
	if err = os.Symlink(path+".controller.lock", link+".controller.lock"); err != nil {
		t.Fatal(err)
	}
	if file, err := ProcessLock(link); err == nil {
		file.Close()
		t.Fatal("symlink lock accepted")
	}
}
func TestUnknownResetAbortBindsHashStopsBothAndPreservesSlots(t *testing.T) {
	s, w, r := fixture(t)
	r = reset(s, r)
	out, err := s.Admit(r)
	if err != nil {
		t.Fatal(err)
	}
	j, _ := s.Job(out.JobID)
	_ = s.Mark(j.ID, "unknown", "lost")
	target := w
	target.Generation, target.SlotID = j.TargetGeneration, j.TargetSlot
	runtime := &fakeRuntime{running: map[string]bool{name(w): true, name(target): true}}
	caps := &fakeCapabilities{}
	c := Controller{Store: s, Runtime: runtime, Capabilities: caps, Authority: &fakeAuthority{}}
	review, err := s.ReviewRecovery(j.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err = c.AbortJob(context.Background(), j.ID, strings.Repeat("0", 64)); err == nil {
		t.Fatal("unreviewed job aborted")
	}
	if !runtime.running[name(w)] {
		t.Fatal("unconfirmed abort touched runtime")
	}
	if err = c.AbortJob(context.Background(), j.ID, review.RequestSHA256); err != nil {
		t.Fatal(err)
	}
	current, _ := s.Workspace(w.ID)
	job, _ := s.Job(j.ID)
	if current.Generation != 3 || current.SlotID != w.SlotID || current.State != "stopped" || job.State != "failed" || runtime.starts != 0 || runtime.running[name(w)] || runtime.running[name(target)] {
		t.Fatal("unsafe recovery", current, job)
	}
	var state string
	_ = s.DB.QueryRow(`SELECT state FROM slots WHERE id=?`, target.SlotID).Scan(&state)
	if state != "retained" {
		t.Fatal("uncertain bytes released")
	}
	if err = c.AbortJob(context.Background(), j.ID, review.RequestSHA256); err != nil {
		t.Fatal("exact recovery retry failed", err)
	}
	current, _ = s.Workspace(w.ID)
	if current.Generation != 3 {
		t.Fatal("abort replay advanced generation")
	}
	r.Method = "workspace.start"
	r.Parameters = Parameters{}
	r.RequestID, r.IdempotencyKey = ID(), ID()
	r.ExpectedGeneration = current.Generation
	if _, err = s.Admit(r); err != nil {
		t.Fatal("explicit later action remained blocked", err)
	}
}
func TestAbortRevocationFailureKeepsJobUnresolvedButStopsRuntime(t *testing.T) {
	s, w, r := fixture(t)
	out, _ := s.Admit(r)
	_ = s.Mark(out.JobID, "unknown", "lost")
	runtime := &fakeRuntime{running: map[string]bool{name(w): true}}
	caps := &fakeCapabilities{failRevoke: true}
	c := Controller{Store: s, Runtime: runtime, Capabilities: caps}
	review, _ := s.ReviewRecovery(out.JobID)
	if c.AbortJob(context.Background(), out.JobID, review.RequestSHA256) == nil {
		t.Fatal("failed revocation acknowledged")
	}
	j, _ := s.Job(out.JobID)
	if j.State != "unknown" || runtime.running[name(w)] {
		t.Fatal("unsafe revocation failure")
	}
	// A restarted daemon must preserve the explicit abort intent even if the
	// last observed runtime state changes before the operator retries.
	runtime.running[name(w)] = true
	if c.Work(context.Background(), out.JobID) == nil {
		t.Fatal("daemon completed an abort-pending operation")
	}
	j, _ = s.Job(out.JobID)
	if j.Outcome != "operator_abort_pending" || j.State != "unknown" {
		t.Fatal("daemon erased durable operator resolution")
	}
	caps.failRevoke = false
	if err := c.AbortJob(context.Background(), out.JobID, review.RequestSHA256); err != nil {
		t.Fatal(err)
	}
}
func TestInactiveRetentionRequiresFreshAdvanceWarning(t *testing.T) {
	s, w, _ := fixture(t)
	now := s.Now().Add(60 * 24 * time.Hour)
	s.Now = func() time.Time { return now }
	notices, err := s.WarnRetention()
	if err != nil || len(notices) != 1 {
		t.Fatal(notices, err)
	}
	due, err := time.Parse(time.RFC3339Nano, notices[0].DeleteAfter)
	if err != nil || due.Sub(now) != 3*24*time.Hour {
		t.Fatal("offline period erased advance warning")
	}
	if _, err = s.DB.Exec(`UPDATE workspaces SET last_activity=? WHERE id=?`, stamp(now), w.ID); err != nil {
		t.Fatal(err)
	}
	notices, err = s.WarnRetention()
	if err != nil || len(notices) != 0 {
		t.Fatal("fresh activity retained old deletion permission", err)
	}
}

type removingRuntime struct {
	fakeRuntime
	removed []Workspace
}

func (r *removingRuntime) Remove(_ context.Context, w Workspace) error {
	r.removed = append(r.removed, w)
	delete(r.running, name(w))
	return nil
}
func receiptFor(r ReclaimReview) ReclaimReceipt {
	return ReclaimReceipt{"feam.slot-reclaimed.v1", r.Plan.ID, r.SHA256, r.Plan.FilesystemUUID, true, stamp(time.Now())}
}
func TestRetainedSlotNotReusableUntilExactRootReceipt(t *testing.T) {
	s, w, r := fixture(t)
	out, err := s.Admit(reset(s, r))
	if err != nil {
		t.Fatal(err)
	}
	job, _ := s.Job(out.JobID)
	if err = s.Finish(job); err != nil {
		t.Fatal(err)
	}
	_ = s.Desired(false)
	_, _ = s.DB.Exec(`UPDATE workspaces SET state='stopped' WHERE id=?`, w.ID)
	runtime := &removingRuntime{fakeRuntime: fakeRuntime{running: map[string]bool{name(w): false}}}
	c := Controller{Store: s, Runtime: runtime, Capabilities: &fakeCapabilities{}}
	review, err := c.PrepareReclaim(context.Background(), Slot{ID: w.SlotID, Path: "/home/feam-service-data/" + w.SlotID, FilesystemUUID: ID()})
	if err != nil {
		t.Fatal(err)
	}
	var state string
	_ = s.DB.QueryRow(`SELECT state FROM slots WHERE id=?`, w.SlotID).Scan(&state)
	if state != "retained" {
		t.Fatal("prepare freed private bytes")
	}
	bad := receiptFor(review)
	bad.PlanSHA256 = strings.Repeat("0", 64)
	if c.completeReclaim(bad) == nil {
		t.Fatal("unbound cleanup acknowledged")
	}
	if err = c.completeReclaim(receiptFor(review)); err != nil {
		t.Fatal(err)
	}
	var kind string
	_ = s.DB.QueryRow(`SELECT state,kind FROM slots WHERE id=?`, w.SlotID).Scan(&state, &kind)
	if state != "free" || kind != "spare" {
		t.Fatal("verified old workspace not recycled as finite spare", state, kind)
	}
	if err = c.completeReclaim(receiptFor(review)); err != nil {
		t.Fatal("completion retry failed", err)
	}
}
func TestInactiveCleanupBlocksAdmissionAndAdvancesGenerationAfterReceipt(t *testing.T) {
	s, w, r := fixture(t)
	now := s.Now().Add(27 * 24 * time.Hour)
	s.Now = func() time.Time { return now }
	_, err := s.WarnRetention()
	if err != nil {
		t.Fatal(err)
	}
	_ = s.Desired(false)
	runtime := &removingRuntime{fakeRuntime: fakeRuntime{running: map[string]bool{name(w): false}}}
	c := Controller{Store: s, Runtime: runtime, Capabilities: &fakeCapabilities{}}
	slot := Slot{ID: w.SlotID, Path: "/home/feam-service-data/" + w.SlotID, FilesystemUUID: ID()}
	if _, err = c.PrepareReclaim(context.Background(), slot); err == nil {
		t.Fatal("premature retention deletion")
	}
	now = now.Add(3 * 24 * time.Hour)
	review, err := c.PrepareReclaim(context.Background(), slot)
	if err != nil {
		t.Fatal(err)
	}
	_ = s.Desired(true)
	if _, err = s.Admit(r); err == nil {
		t.Fatal("cleanup race admitted a fresh runtime")
	}
	_ = s.Desired(false)
	if err = c.completeReclaim(receiptFor(review)); err != nil {
		t.Fatal(err)
	}
	current, _ := s.Workspace(w.ID)
	if current.Generation != 2 || current.State != "stopped" || current.SlotID != w.SlotID {
		t.Fatal("cleanup lost assignment or reused generation", current)
	}
}
