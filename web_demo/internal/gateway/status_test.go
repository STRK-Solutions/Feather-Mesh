package gateway

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestParticipantStatusWarningAndStaleGeneration(t *testing.T) {
	f := setup(t, "/run/absent.sock")
	var stale atomic.Bool
	f.g.Config.ControllerSocket = unixBackend(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		workspace := f.workspaces[0]
		if r.Method != "GET" || r.URL.Path != "/v1/workspaces/"+workspace.ID {
			t.Error("queried non-owned workspace status")
		}
		generation := workspace.Generation
		if stale.Load() {
			generation++
		}
		json.NewEncoder(w).Encode(map[string]any{"id": workspace.ID, "owner_id": workspace.OwnerID, "deployment_id": workspace.DeploymentID, "generation": generation, "assignment_version": workspace.GrantVersion, "state": "running", "warning_at": "2026-09-25T10:00:00Z", "retention_notified": "2026-09-25T10:00:00Z", "retention_delete_after": "2026-09-26T10:00:00Z"})
	}))
	for _, changed := range []bool{false, true} {
		stale.Store(changed)
		r := request(f, "GET", f.g.Config.PortalHost, "/", f.accounts[4].Email, "portal", "")
		response := httptest.NewRecorder()
		f.g.ServeHTTP(response, r)
		if response.Code != 200 {
			t.Fatal(response.Code)
		}
		body := response.Body.String()
		if !strings.Contains(body, "inactive workspace data is kept up to 30 days") {
			t.Fatal("retention policy missing")
		}
		if strings.Contains(body, "Retention notice:") == changed {
			t.Fatal("stale or missing retention warning")
		}
		if strings.Contains(body, "Idle warning:") == changed {
			t.Fatal("stale or missing idle warning")
		}
		if changed && !strings.Contains(body, "Updating workspace") {
			t.Fatal("stale controller generation displayed as current")
		}
	}
}
