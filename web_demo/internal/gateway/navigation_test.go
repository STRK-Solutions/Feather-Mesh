package gateway

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/STRK-Solutions/Feather-Mesh/web_demo/internal/control"
)

func TestWorkspaceActionBrowserNavigationAndAPIResponse(t *testing.T) {
	for _, tc := range []struct {
		name, method, accept string
		admin, confirmed     bool
		backendStatus, want  int
		malformed            bool
	}{
		{name: "browser_start", method: "workspace.start", accept: "text/html,application/xhtml+xml,*/*;q=0.8", backendStatus: 202, want: 303},
		{name: "browser_stop", method: "workspace.stop", accept: "text/html", backendStatus: 202, want: 303},
		{name: "admin_stop", method: "workspace.stop", accept: "text/html", admin: true, backendStatus: 202, want: 303},
		{name: "confirmed_reset", method: "workspace.reset", accept: "text/html", confirmed: true, backendStatus: 202, want: 303},
		{name: "admin_confirmed_delete", method: "workspace.delete", accept: "text/html", admin: true, confirmed: true, backendStatus: 202, want: 303},
		{name: "json_api", method: "workspace.start", accept: "application/json", backendStatus: 202, want: 202},
		{name: "existing_api", method: "workspace.start", backendStatus: 202, want: 202},
		{name: "rejected_admission", method: "workspace.start", accept: "text/html", backendStatus: 409, want: 409},
		{name: "unknown_response", method: "workspace.start", accept: "text/html", backendStatus: 202, want: 503, malformed: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := setup(t, "/run/absent.sock")
			var calls atomic.Int32
			f.g.Config.ControllerSocket = unixBackend(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method == "GET" {
					http.Error(w, "status unavailable", 503)
					return
				}
				var action Action
				if r.Method != "POST" || r.URL.Path != "/v1/workspaces/action" || json.NewDecoder(r.Body).Decode(&action) != nil || action.Method != tc.method {
					t.Error("unexpected lifecycle request")
					http.Error(w, "bad request", 400)
					return
				}
				calls.Add(1)
				w.WriteHeader(tc.backendStatus)
				if tc.malformed {
					_, _ = w.Write([]byte("incomplete"))
					return
				}
				_ = json.NewEncoder(w).Encode(map[string]string{"protocol": "feam.web.v1", "request_id": action.RequestID, "status": "accepted", "job_id": control.ID()})
			}))
			actor, host, aud := 4, f.g.Config.PortalHost, "portal"
			if tc.admin {
				actor, host, aud = 0, f.g.Config.AdminHost, "admin"
			}
			ws := f.workspaces[0]
			form := url.Values{"workspace_id": {ws.ID}, "generation": {"1"}, "action": {tc.method}}
			if tc.confirmed {
				action := Action{Protocol: "feam.web.v1", RequestID: control.ID(), IdempotencyKey: control.ID(), DeploymentID: ws.DeploymentID, ActivationGeneration: 1, WorkspaceID: ws.ID, ExpectedGeneration: 1, ActorID: f.accounts[actor].ID, AuthorizationVersion: f.accounts[actor].AuthVersion, Method: tc.method, Parameters: map[string]string{}}
				form = url.Values{"confirmation": {f.g.sign(confirmation{action, time.Now().Add(time.Minute).Unix()})}}
			}
			resp := postFormAccept(t, f, actor, host, aud, "/workspace/action", form, tc.accept)
			if resp.Code != tc.want || calls.Load() != 1 {
				t.Fatalf("status=%d, calls=%d, body=%s", resp.Code, calls.Load(), resp.Body.String())
			}
			if tc.want == 303 {
				if resp.Header().Get("Location") != "/" || strings.Contains(resp.Body.String(), "job_id") {
					t.Fatal("browser was not returned to its dashboard")
				}
				// Follow the redirect and refresh again: both must be GET-only reads.
				for range 2 {
					page := httptest.NewRecorder()
					f.g.ServeHTTP(page, request(f, "GET", host, "/", f.accounts[actor].Email, aud, ""))
					if page.Code != 200 || !strings.Contains(page.Body.String(), "<title>Feather Mesh</title>") {
						t.Fatal("redirect did not reach the dashboard")
					}
				}
				if calls.Load() != 1 {
					t.Fatal("dashboard reload repeated the lifecycle action")
				}
			} else if resp.Header().Get("Location") != "" {
				t.Fatal("non-browser success or failure unexpectedly redirected")
			}
			if tc.want == 202 {
				var result map[string]string
				if resp.Header().Get("Content-Type") != "application/json" || json.Unmarshal(resp.Body.Bytes(), &result) != nil || result["status"] != "accepted" || result["job_id"] == "" {
					t.Fatal("API response changed")
				}
			}
		})
	}
}

func TestDashboardRefreshUntilWorkspaceReady(t *testing.T) {
	f := setup(t, "/run/absent.sock")
	var state atomic.Value
	state.Store("stopped")
	var stale atomic.Bool
	f.g.Config.ControllerSocket = unixBackend(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ws := f.workspaces[0]
		if r.Method != "GET" || r.URL.Path != "/v1/workspaces/"+ws.ID {
			t.Error("refresh must read only the owner's workspace")
		}
		generation := ws.Generation
		if stale.Load() {
			generation++
		}
		if state.Load().(string) == "unavailable" {
			http.Error(w, "unavailable", 503)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"id": ws.ID, "owner_id": ws.OwnerID, "deployment_id": ws.DeploymentID, "generation": generation, "assignment_version": ws.GrantVersion, "state": state.Load().(string)})
	}))
	for _, tc := range []struct {
		state                       string
		ready, stale, refresh, link bool
	}{
		{"stopped", false, false, false, false},
		{"starting", false, false, true, false},
		{"running", false, false, true, false},
		{"running", true, true, true, false},
		{"running", true, false, false, true},
		{"stopping", true, false, true, false},
		{"resetting", true, false, true, false},
		{"pending-delete", true, false, true, false},
		{"deleted", false, false, false, false},
		{"error", false, false, false, false},
		{"unavailable", false, false, false, false},
	} {
		state.Store(tc.state)
		stale.Store(tc.stale)
		if _, err := f.g.Store.DB.ExecContext(context.Background(), "UPDATE workspaces SET ready=? WHERE id=?", tc.ready, f.workspaces[0].ID); err != nil {
			t.Fatal(err)
		}
		resp := httptest.NewRecorder()
		f.g.ServeHTTP(resp, request(f, "GET", f.g.Config.PortalHost, "/", f.accounts[4].Email, "portal", ""))
		body := resp.Body.String()
		if resp.Code != 200 || strings.Contains(body, `<meta http-equiv="refresh" content="2">`) != tc.refresh || strings.Contains(body, `<fieldset disabled>`) != tc.refresh || strings.Contains(body, "Open assisted FEAM") != tc.link {
			t.Fatalf("incorrect dashboard for %+v: %s", tc, body)
		}
		if strings.Contains(body, "This page refreshes automatically.") != tc.refresh {
			t.Fatal("refresh explanation does not match behavior")
		}
	}
}
