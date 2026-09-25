package control

import (
	"encoding/json"
	"github.com/STRK-Solutions/Feather-Mesh/web_demo/internal/ipc"
	"net/http"
	"strings"
)

// ServiceHandler is never mounted on the browser listener. Separate read,
// controller-update and reconciler UIDs are mandatory for service authority.
func (s *Store) ServiceHandler(readUIDs []uint32, controllerUID, reconcilerUID uint32, pipelineUIDs ...uint32) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("GET /v1/releases/pins", ipc.RequireUID(readUIDs, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		v, e := s.ReleasePins(r.Context())
		if e != nil {
			http.Error(w, "pins unavailable", 503)
			return
		}
		writeJSON(w, v)
	})))
	if len(pipelineUIDs) == 1 && pipelineUIDs[0] != 0 {
		mux.Handle("POST /v1/grants/revoke-bundle", ipc.RequireUID(pipelineUIDs, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var in struct {
				ActorID string `json:"actor_id"`
				Bundle  string `json:"bundle"`
			}
			if ipc.Decode(w, r, &in) != nil {
				http.Error(w, "invalid withdrawal", 400)
				return
			}
			if s.RevokeBundleGrants(r.Context(), in.ActorID, in.Bundle) != nil {
				http.Error(w, "withdrawal denied", 403)
				return
			}
			writeJSON(w, map[string]string{"status": "completed"})
		})))
	}
	mux.Handle("GET /v1/assignments/{id}", ipc.RequireUID(readUIDs, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		v, e := s.Assignments(r.Context(), r.PathValue("id"))
		if e != nil {
			http.Error(w, "assignment denied", 403)
			return
		}
		writeJSON(w, v)
	})))
	mux.Handle("POST /v1/authorize", ipc.RequireUID(readUIDs, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var c Check
		if ipc.Decode(w, r, &c) != nil {
			http.Error(w, "invalid request", 400)
			return
		}
		v, e := s.Authorize(r.Context(), c)
		if e != nil {
			http.Error(w, "authorization denied", 403)
			return
		}
		writeJSON(w, v)
	})))
	mux.Handle("GET /v1/accounts/", ipc.RequireUID(readUIDs, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := strings.TrimPrefix(r.URL.Path, "/v1/accounts/")
		a, e := s.Account(r.Context(), id)
		if e != nil || a.Status != "active" {
			http.Error(w, "authorization denied", 403)
			return
		}
		writeJSON(w, a)
	})))
	mux.Handle("POST /v1/workspaces/snapshot", ipc.RequireUID([]uint32{controllerUID}, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var in Workspace
		if ipc.Decode(w, r, &in) != nil {
			http.Error(w, "invalid snapshot", 400)
			return
		}
		if s.Snapshot(r.Context(), in) != nil {
			http.Error(w, "snapshot rejected", 409)
			return
		}
		writeJSON(w, map[string]string{"status": "completed"})
	})))
	mux.Handle("GET /v1/policy", ipc.RequireUID([]uint32{reconcilerUID}, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p, e := s.Policy(r.Context())
		if e != nil {
			http.Error(w, "policy unavailable", 503)
			return
		}
		writeJSON(w, p)
	})))
	mux.Handle("POST /v1/policy/result", ipc.RequireUID([]uint32{reconcilerUID}, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			Revision int64 `json:"revision"`
			Success  bool  `json:"success"`
		}
		if ipc.Decode(w, r, &in) != nil {
			http.Error(w, "invalid result", 400)
			return
		}
		if s.SyncResult(r.Context(), in.Revision, in.Success) != nil {
			http.Error(w, "policy changed", 409)
			return
		}
		writeJSON(w, map[string]string{"status": "completed"})
	})))
	return mux
}
func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(v)
}
