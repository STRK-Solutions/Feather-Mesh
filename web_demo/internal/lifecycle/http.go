package lifecycle

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/STRK-Solutions/Feather-Mesh/web_demo/internal/ipc"
)

func (c *Controller) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1/workspaces/operation", func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			WorkspaceID string `json:"workspace_id"`
			RequestID   string `json:"request_id"`
			Active      bool   `json:"active"`
		}
		if ipc.Decode(w, r, &in) != nil || c.Store.Operation(in.WorkspaceID, in.RequestID, in.Active) != nil {
			http.Error(w, "operation activity denied", 403)
			return
		}
		writeJSON(w, map[string]string{"status": "completed"})
	})
	mux.HandleFunc("POST /v1/workspaces/reconfigure", func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			WorkspaceID  string `json:"workspace_id"`
			GrantVersion int64  `json:"grant_version"`
		}
		if ipc.Decode(w, r, &in) != nil || !uuidRE.MatchString(in.WorkspaceID) || in.GrantVersion < 1 {
			http.Error(w, "invalid grant update", 400)
			return
		}
		if c.Reconfigure(r.Context(), in.WorkspaceID, in.GrantVersion) != nil {
			http.Error(w, "grant reconciliation pending", 503)
			return
		}
		writeJSON(w, map[string]string{"status": "completed"})
	})
	mux.HandleFunc("POST /v1/workspaces/action", func(w http.ResponseWriter, r *http.Request) {
		var in Request
		if e := ipc.Decode(w, r, &in); e != nil {
			http.Error(w, "invalid request", 400)
			return
		}
		out, e := c.Admit(r.Context(), in)
		if e != nil {
			code := 409
			if e == ErrAuthority {
				code = 403
			}
			if e == ErrCapacity {
				code = 429
			}
			http.Error(w, "lifecycle admission denied", code)
			return
		}
		writeJSON(w, out)
	})
	mux.HandleFunc("POST /v1/workspaces/confirm", func(w http.ResponseWriter, r *http.Request) {
		var in Request
		if e := ipc.Decode(w, r, &in); e != nil {
			http.Error(w, "invalid confirmation", 400)
			return
		}
		workspace, e := c.Store.Workspace(in.WorkspaceID)
		if e != nil || c.Authority.Check(r.Context(), in, workspace) != nil {
			http.Error(w, "forbidden", 403)
			return
		}
		if e = c.Store.Confirm(in); e != nil {
			http.Error(w, "confirmation conflict", 409)
			return
		}
		writeJSON(w, map[string]string{"confirmation_hash": Hash(in)})
	})
	mux.HandleFunc("GET /v1/workspaces/{id}", func(w http.ResponseWriter, r *http.Request) {
		v, e := c.Store.Workspace(r.PathValue("id"))
		if e != nil {
			http.Error(w, "workspace unavailable", 404)
			return
		}
		writeJSON(w, v)
	})
	mux.HandleFunc("GET /v1/jobs/{id}", func(w http.ResponseWriter, r *http.Request) {
		v, e := c.Store.Job(r.PathValue("id"))
		if e != nil {
			http.Error(w, "job unavailable", 404)
			return
		}
		writeJSON(w, Result{Protocol: "feam.web.v1", RequestID: v.Request.RequestID, Status: v.State, JobID: v.ID, Error: v.Outcome})
	})
	mux.HandleFunc("POST /v1/accounts/revoke", func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			AccountID   string `json:"account_id"`
			AuthVersion int64  `json:"auth_version"`
		}
		if e := ipc.Decode(w, r, &in); e != nil || !uuidRE.MatchString(in.AccountID) || in.AuthVersion < 1 {
			http.Error(w, "invalid revocation", 400)
			return
		}
		if e := c.Revoke(r.Context(), in.AccountID); e != nil {
			http.Error(w, "revocation requires reconciliation", 503)
			return
		}
		writeJSON(w, map[string]string{"status": "completed"})
	})
	mux.HandleFunc("POST /v1/workspaces/activity", func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			WorkspaceID string `json:"workspace_id"`
			Kind        string `json:"kind"`
		}
		if e := ipc.Decode(w, r, &in); e != nil || !uuidRE.MatchString(in.WorkspaceID) {
			http.Error(w, "invalid activity", 400)
			return
		}
		if e := c.Store.Activity(in.WorkspaceID, in.Kind); e != nil {
			http.Error(w, "activity denied", 400)
			return
		}
		writeJSON(w, map[string]string{"status": "completed"})
	})
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.RawPath, "%") || r.URL.RawQuery != "" {
			http.Error(w, "unsupported parameters", 400)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		mux.ServeHTTP(w, r)
	})
}
func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}
