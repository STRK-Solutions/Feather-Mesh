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
	"github.com/STRK-Solutions/Feather-Mesh/web_demo/internal/pipeline"
)

func postForm(t *testing.T, f fixture, actor int, host, audience, path string, form url.Values) *httptest.ResponseRecorder {
	t.Helper()
	get := request(f, "GET", host, "/", f.accounts[actor].Email, audience, "")
	response := httptest.NewRecorder()
	f.g.ServeHTTP(response, get)
	cookies := response.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("missing csrf: %d %s", response.Code, response.Body.String())
	}
	form.Set("csrf", cookies[0].Value)
	req := request(f, "POST", host, path, f.accounts[actor].Email, audience, form.Encode())
	req.Header.Set("Origin", "https://"+host)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(cookies[0])
	response = httptest.NewRecorder()
	f.g.ServeHTTP(response, req)
	return response
}

type trackedCloser struct{ closed atomic.Bool }

func (c *trackedCloser) Close() error { c.closed.Store(true); return nil }

func TestOperatorGrantValidatesReleaseRolesAndVersion(t *testing.T) {
	for _, scenario := range []string{"valid", "unavailable", "different_release", "user_actor", "admin_target", "stale", "invalid_digest"} {
		t.Run(scenario, func(t *testing.T) {
			f := setup(t, "/run/absent.sock")
			ctx := context.Background()
			digest := strings.Repeat("a", 64)
			actor, target, version := f.accounts[0].ID, f.accounts[4].ID, int64(1)
			socket := unixBackend(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var grant control.Grant
				if r.Method != "POST" || r.URL.Path != "/v1/releases/resolve" || json.NewDecoder(r.Body).Decode(&grant) != nil || grant.Bundle != "climate" || grant.Digest != digest {
					t.Error("operator must resolve the exact reviewed release")
				}
				if scenario == "unavailable" {
					w.WriteHeader(503)
					return
				}
				if scenario == "different_release" {
					grant.Digest = strings.Repeat("b", 64)
				}
				json.NewEncoder(w).Encode(pipeline.ReleaseView{Bundle: grant.Bundle, Digest: grant.Digest, Namespace: grant.Bundle, Revision: 1, ServingRoot: "/datasets/releases/climate/provider/serving"})
			}))
			switch scenario {
			case "user_actor":
				actor = f.accounts[4].ID
			case "admin_target":
				target = f.accounts[0].ID
			case "stale":
				version = 2
			case "invalid_digest":
				digest = ""
			}
			err := AssignReviewedRelease(ctx, f.g.Store, socket, actor, target, "climate", digest, version)
			updates, _ := f.g.Store.PendingGrantUpdates(ctx)
			workspace, _ := f.g.Store.Workspace(ctx, f.workspaces[0].ID)
			if scenario != "valid" {
				if err == nil || len(updates) != 0 || workspace.GrantVersion != 1 || !workspace.Ready {
					t.Fatal("rejected operator grant changed state", err, workspace)
				}
				return
			}
			if err != nil || len(updates) != 1 || workspace.GrantVersion != 2 || workspace.Ready {
				t.Fatal("grant did not queue conservative reconfiguration", err, workspace)
			}
			var audits int
			if err := f.g.Store.DB.QueryRow(`SELECT count(*) FROM audit WHERE actor_id=? AND target_id=? AND action='dataset_grant_changed'`, actor, target).Scan(&audits); err != nil || audits != 1 {
				t.Fatal("grant audit missing", err, audits)
			}
			if AssignReviewedRelease(ctx, f.g.Store, socket, actor, target, "climate", digest, version) == nil {
				t.Fatal("stale operator intent replayed")
			}
		})
	}
}

func TestGrantExactReleaseAndDurableReconfigure(t *testing.T) {
	f := setup(t, "/run/absent.sock")
	ctx := context.Background()
	digest := strings.Repeat("a", 64)
	var rejectPipeline, acceptController atomic.Bool
	var resolveCalls, reconfigureCalls atomic.Int64
	f.g.Config.PipelineSocket = unixBackend(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" && r.URL.Path == "/v1/jobs" {
			w.Write([]byte(`[]`))
			return
		}
		resolveCalls.Add(1)
		var in control.Grant
		if r.Method != "POST" || r.URL.Path != "/v1/releases/resolve" || json.NewDecoder(r.Body).Decode(&in) != nil || in.Bundle != "climate" || in.Digest != digest {
			t.Error("non-exact release request")
		}
		if rejectPipeline.Load() {
			w.WriteHeader(409)
			return
		}
		json.NewEncoder(w).Encode(pipeline.ReleaseView{Bundle: in.Bundle, Digest: in.Digest, Namespace: in.Bundle, Revision: 1, ServingRoot: "/datasets/releases/climate/provider/serving"})
	}))
	f.g.Config.ControllerSocket = unixBackend(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reconfigureCalls.Add(1)
		var in control.GrantUpdate
		if r.URL.Path != "/v1/workspaces/reconfigure" || json.NewDecoder(r.Body).Decode(&in) != nil || in.WorkspaceID != f.workspaces[0].ID || in.GrantVersion < 2 {
			t.Error("bad reconfigure identity")
		}
		if !acceptController.Load() {
			w.WriteHeader(503)
			return
		}
		w.Write([]byte(`{"status":"completed"}`))
	}))
	form := url.Values{"account_id": {f.accounts[4].ID}, "bundle": {"climate"}, "digest": {digest}, "grant_version": {"1"}, "action": {"assign"}}
	rejectPipeline.Store(true)
	response := postForm(t, f, 0, f.g.Config.AdminHost, "admin", "/datasets/grant", form)
	if response.Code != 409 {
		t.Fatal(response.Code, response.Body.String())
	}
	ws, _ := f.g.Store.Workspace(ctx, f.workspaces[0].ID)
	if ws.GrantVersion != 1 || !ws.Ready {
		t.Fatal("failed resolve changed policy")
	}
	rejectPipeline.Store(false)
	stream := &trackedCloser{}
	untrack := f.g.Streams.Track(f.accounts[4].ID, stream, time.Now().Add(time.Hour), func(context.Context) bool { return true })
	defer untrack()
	response = postForm(t, f, 0, f.g.Config.AdminHost, "admin", "/datasets/grant", form)
	if response.Code != 303 || !stream.closed.Load() {
		t.Fatal("grant did not close stream", response.Code, response.Body.String())
	}
	ws, _ = f.g.Store.Workspace(ctx, ws.ID)
	if ws.Ready || ws.GrantVersion != 2 {
		t.Fatal("old mount admission remained")
	}
	pending, _ := f.g.Store.PendingGrantUpdates(ctx)
	if len(pending) != 1 {
		t.Fatal("failed dispatch not durable")
	}
	response = postForm(t, f, 0, f.g.Config.AdminHost, "admin", "/datasets/grant", form)
	if response.Code != 409 {
		t.Fatal("stale form accepted")
	}
	acceptController.Store(true)
	// A new gateway instance resumes the same pending policy after restart.
	restarted, e := New(f.g.Config, f.g.Store, testValidator{})
	if e != nil {
		t.Fatal(e)
	}
	restarted.reconfigureGrants(ctx)
	pending, _ = f.g.Store.PendingGrantUpdates(ctx)
	if len(pending) != 0 {
		t.Fatal("successful dispatch not acknowledged")
	}
	form.Set("grant_version", "2")
	form.Set("action", "revoke")
	form.Del("digest")
	before := resolveCalls.Load()
	response = postForm(t, f, 0, f.g.Config.AdminHost, "admin", "/datasets/grant", form)
	if response.Code != 303 {
		t.Fatal(response.Code, response.Body.String())
	}
	if resolveCalls.Load() != before {
		t.Fatal("revocation depended on release availability")
	}
	assign, _ := f.g.Store.Assignments(ctx, ws.ID)
	if len(assign.Grants) != 0 || assign.Workspace.GrantVersion != 3 {
		t.Fatal("revoke failed")
	}
	if reconfigureCalls.Load() < 3 {
		t.Fatal("missing durable reconfigure attempts")
	}
}

func TestGrantRoleAndAdminLifecycleBoundaries(t *testing.T) {
	f := setup(t, "/run/absent.sock")
	f.g.Config.ControllerSocket = unixBackend(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/confirmations" {
			w.Write([]byte(`{"status":"completed"}`))
			return
		}
		w.Write([]byte(`{"protocol":"feam.web.v1","status":"completed"}`))
	}))
	grant := url.Values{"account_id": {f.accounts[4].ID}, "bundle": {"climate"}, "grant_version": {"1"}, "action": {"revoke"}}
	for _, actor := range []int{0, 4} {
		r := postForm(t, f, actor, f.g.Config.PortalHost, "portal", "/datasets/grant", grant)
		if r.Code != 403 {
			t.Fatalf("portal grant actor %d: %d", actor, r.Code)
		}
	}
	for _, tc := range []struct {
		actor             int
		host, aud, action string
		status            int
	}{
		{0, f.g.Config.AdminHost, "admin", "workspace.stop", 200},
		{0, f.g.Config.AdminHost, "admin", "workspace.start", 403},
		{0, f.g.Config.AdminHost, "admin", "workspace.reset", 200},
		{0, f.g.Config.AdminHost, "admin", "workspace.delete", 200},
		{0, f.g.Config.PortalHost, "portal", "workspace.stop", 403},
		{5, f.g.Config.PortalHost, "portal", "workspace.stop", 403},
	} {
		form := url.Values{"workspace_id": {f.workspaces[0].ID}, "generation": {"1"}, "action": {tc.action}}
		r := postForm(t, f, tc.actor, tc.host, tc.aud, "/workspace/action", form)
		if r.Code != tc.status {
			t.Fatalf("%s %s: got %d: %s", tc.host, tc.action, r.Code, r.Body.String())
		}
		if tc.status == 200 && (tc.action == "workspace.reset" || tc.action == "workspace.delete") && !strings.Contains(r.Body.String(), "Confirm this change") {
			t.Fatal("destructive admin action skipped review")
		}
	}
}
