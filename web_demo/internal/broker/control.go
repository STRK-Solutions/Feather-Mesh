package broker

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/STRK-Solutions/Feather-Mesh/web_demo/internal/capability"
	"github.com/STRK-Solutions/Feather-Mesh/web_demo/internal/ipc"
)

func (s *Server) Control(controllerUID uint32, gatewayUIDs ...uint32) http.Handler {
	mutations := ipc.RequireUID([]uint32{controllerUID}, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			reject(w, 405, "invalid_request")
			return
		}
		switch r.URL.Path {
		case "/v1/run/drain":
			var empty struct{}
			if ipc.Decode(w, r, &empty) != nil {
				reject(w, 400, "invalid_request")
				return
			}
			ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
			defer cancel()
			receipt, err := s.Drain(ctx)
			if err != nil {
				reject(w, 503, "drain_incomplete")
				return
			}
			jsonReply(w, 200, receipt)
		case "/v1/capabilities/mint":
			var c capability.Claims
			if ipc.Decode(w, r, &c) != nil || !c.Valid(time.Now()) || c.DeploymentID != s.Run.Document.Allocation.DeploymentID || c.ActivationGeneration != s.Run.Document.Allocation.ActivationGeneration {
				reject(w, 400, "invalid_request")
				return
			}
			if s.Authority.Check(r.Context(), c) != nil {
				reject(w, 403, "forbidden")
				return
			}
			token, err := s.Capabilities.Mint(r.Context(), c)
			if err != nil {
				reject(w, 503, "unavailable")
				return
			}
			jsonReply(w, 200, map[string]string{"capability": token})
		case "/v1/capabilities/revoke":
			var v struct {
				WorkspaceID string `json:"workspace_id"`
			}
			if ipc.Decode(w, r, &v) != nil {
				reject(w, 400, "invalid_request")
				return
			}
			if err := s.Capabilities.Revoke(r.Context(), v.WorkspaceID); err != nil {
				reject(w, 503, "unavailable")
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]string{"status": "revoked"})
		default:
			reject(w, 404, "not_found")
		}
	}))
	status := ipc.RequireUID(append([]uint32{controllerUID}, gatewayUIDs...), http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" || r.URL.RawQuery != "" {
			reject(w, 400, "invalid_request")
			return
		}
		usage, err := s.Run.Usage(r.Context(), "")
		if err != nil {
			reject(w, 503, "unavailable")
			return
		}
		active, queued := s.Queue.State()
		jsonReply(w, 200, map[string]any{"usage": usage, "active": active, "queued": queued,
			"global_concurrency": 3, "per_user_concurrency": 1, "recording": s.Events != nil})
	}))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/status" {
			status.ServeHTTP(w, r)
			return
		}
		mutations.ServeHTTP(w, r)
	})
}
