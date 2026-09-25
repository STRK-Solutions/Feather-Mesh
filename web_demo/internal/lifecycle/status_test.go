package lifecycle

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestOperatorStatusTracksIntentAndOutstandingWorkWithoutOwnerIdentity(t *testing.T) {
	s, w, request := fixture(t)
	status, err := s.LifecycleStatus(context.Background())
	if err != nil || status.Desired != "running" || status.DeploymentID != w.DeploymentID || status.ActivationGeneration != 1 || status.Workspaces[w.ID] != "stopped" || len(status.PendingJobs) != 0 {
		t.Fatal(status, err)
	}
	job, err := s.Admit(request)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.Desired(false); err != nil {
		t.Fatal(err)
	}
	status, err = s.LifecycleStatus(context.Background())
	if err != nil || status.Desired != "stopped" || len(status.PendingJobs) != 1 || status.PendingJobs[0] != job.JobID {
		t.Fatal(status, err)
	}
	b, _ := json.Marshal(status)
	if strings.Contains(string(b), w.OwnerID) || strings.Contains(string(b), w.Hostname) || strings.Contains(string(b), w.ImageID) {
		t.Fatal("operator status includes unrelated private owner details")
	}
}
