package lifecycle

import (
	"context"
	"strings"
	"testing"
)

func TestAssignStoppedPreservesExistingWorkspaceAndQueuesNewSnapshot(t *testing.T) {
	s, w, _ := fixture(t)
	newWorkspace := w
	newWorkspace.ID, newWorkspace.OwnerID = ID(), ID()
	newWorkspace.Hostname = "u-" + newWorkspace.ID + ".613202690.xyz"
	if err := s.AssignStopped(newWorkspace); err == nil {
		t.Fatal("assigned while running")
	}
	if err := s.Desired(false); err != nil {
		t.Fatal(err)
	}
	newWorkspace.DeploymentID = ID()
	if err := s.AssignStopped(newWorkspace); err == nil {
		t.Fatal("assigned to wrong deployment")
	}
	newWorkspace.DeploymentID = w.DeploymentID
	if err := s.AssignStopped(newWorkspace); err != nil {
		t.Fatal(err)
	}
	got, _ := s.Workspace(w.ID)
	if got != w {
		t.Fatal("existing workspace changed", got)
	}
	assigned, _ := s.Workspace(newWorkspace.ID)
	if assigned.SlotID == w.SlotID || assigned.State != "stopped" {
		t.Fatal("unsafe new assignment", assigned)
	}
	if err := s.AssignStopped(newWorkspace); err == nil {
		t.Fatal("assignment replay accepted")
	}
}

func TestOfflineImageUpgradePreservesSlotAndRejectsReplayOrRunning(t *testing.T) {
	s, w, _ := fixture(t)
	runtime := &fakeRuntime{running: map[string]bool{name(w): true}}
	c := Controller{Store: s, Runtime: runtime}
	image := "sha256:" + strings.Repeat("b", 64)
	if err := c.UpgradeImage(context.Background(), w.ID, w.Generation, w.ImageID, image); err == nil {
		t.Fatal("running container upgraded")
	}
	runtime.running[name(w)] = false
	if err := c.UpgradeImage(context.Background(), w.ID, w.Generation, w.ImageID, image); err == nil {
		t.Fatal("active deployment upgraded")
	}
	if err := s.Desired(false); err != nil {
		t.Fatal(err)
	}
	if err := c.UpgradeImage(context.Background(), w.ID, w.Generation+1, w.ImageID, image); err == nil {
		t.Fatal("stale generation upgraded")
	}
	if err := c.UpgradeImage(context.Background(), w.ID, w.Generation, w.ImageID, image); err != nil {
		t.Fatal(err)
	}
	got, _ := s.Workspace(w.ID)
	if got.Generation != w.Generation+1 || got.ImageID != image || got.SlotID != w.SlotID || got.AssignmentVersion != w.AssignmentVersion || got.State != "stopped" || runtime.starts != 0 {
		t.Fatal("unsafe upgrade", got)
	}
	var snapshots int
	if err := s.DB.QueryRow(`SELECT count(*) FROM snapshot_outbox WHERE workspace_id=?`, w.ID).Scan(&snapshots); err != nil || snapshots != 1 {
		t.Fatal("missing gateway snapshot", snapshots, err)
	}
	if err := c.UpgradeImage(context.Background(), w.ID, w.Generation, w.ImageID, image); err == nil {
		t.Fatal("upgrade replayed")
	}
}

func TestOfflineRestoreRejectsPendingJobsAndKeepsOtherRevocations(t *testing.T) {
	s, w, r := fixture(t)
	if _, err := s.Admit(r); err != nil {
		t.Fatal(err)
	}
	other := ID()
	if _, err := s.DB.Exec(`INSERT INTO revoked_accounts VALUES(?),(?)`, w.OwnerID, other); err != nil {
		t.Fatal(err)
	}
	c := Controller{Store: s, Runtime: &fakeRuntime{running: map[string]bool{}}}
	if err := c.RestoreAccount(context.Background(), w.ID, w.Generation); err == nil {
		t.Fatal("active deployment restored")
	}
	// A previously admitted job still makes offline restoration unsafe.
	if err := s.Desired(false); err != nil {
		t.Fatal(err)
	}
	if err := c.RestoreAccount(context.Background(), w.ID, w.Generation); err == nil {
		t.Fatal("pending job ignored")
	}
	if _, err := s.DB.Exec(`UPDATE lifecycle_jobs SET state='failed'`); err != nil {
		t.Fatal(err)
	}
	if err := c.RestoreAccount(context.Background(), w.ID, w.Generation); err == nil {
		t.Fatal("unfinished workspace stop ignored")
	}
	if _, err := s.DB.Exec(`UPDATE workspaces SET state='stopped' WHERE id=?`, w.ID); err != nil {
		t.Fatal(err)
	}
	if err := c.RestoreAccount(context.Background(), w.ID, w.Generation); err != nil {
		t.Fatal(err)
	}
	var remaining string
	if err := s.DB.QueryRow(`SELECT account_id FROM revoked_accounts`).Scan(&remaining); err != nil || remaining != other {
		t.Fatal("unrelated revocation changed", err)
	}
	if err := c.RestoreAccount(context.Background(), w.ID, w.Generation); err == nil {
		t.Fatal("restore replay accepted")
	}
}
