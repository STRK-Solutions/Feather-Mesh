package pipeline

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/STRK-Solutions/Feather-Mesh/web_demo/internal/ipc"
)

// ServiceHandler grants mutation capability only to the gateway kernel UID.
// The controller can resolve immutable releases but cannot enqueue or approve.
func (m *Manager) ServiceHandler(controllerUID, gatewayUID uint32) http.Handler {
	if gatewayUID == 0 {
		return m.Handler(controllerUID)
	}
	root := http.NewServeMux()
	root.Handle("POST /v1/releases/resolve", m.Handler(controllerUID, gatewayUID))
	admin := http.NewServeMux()
	admin.HandleFunc("GET /v1/jobs", func(w http.ResponseWriter, r *http.Request) { v, e := m.Jobs(); pipelineResult(w, v, e) })
	admin.HandleFunc("GET /v1/jobs/{id}/review", func(w http.ResponseWriter, r *http.Request) {
		v, e := m.Review(r.PathValue("id"))
		pipelineResult(w, v, e)
	})
	admin.HandleFunc("POST /v1/jobs/plan", func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			Job json.RawMessage `json:"job"`
		}
		if closedRequest(w, r, &in) != nil {
			http.Error(w, "invalid request", 400)
			return
		}
		j, e := DecodeJob(in.Job)
		pipelineResult(w, map[string]any{"job": j, "hash": j.Hash()}, e)
	})
	admin.HandleFunc("POST /v1/jobs/enqueue", func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			Job   json.RawMessage `json:"job"`
			Hash  string          `json:"hash"`
			Actor string          `json:"actor"`
		}
		if closedRequest(w, r, &in) != nil {
			http.Error(w, "invalid request", 400)
			return
		}
		j, e := DecodeJob(in.Job)
		if e != nil {
			pipelineResult(w, nil, e)
			return
		}
		v, e := m.Enqueue(r.Context(), j, in.Hash, in.Actor)
		pipelineResult(w, v, e)
	})
	for _, action := range []string{"approve", "promote", "reconcile"} {
		admin.HandleFunc("POST /v1/jobs/{id}/"+action, func(w http.ResponseWriter, r *http.Request) {
			var in struct {
				Hash  string `json:"hash"`
				Actor string `json:"actor"`
			}
			if closedRequest(w, r, &in) != nil || !uuidPattern.MatchString(in.Actor) {
				http.Error(w, "invalid request", 400)
				return
			}
			id := r.PathValue("id")
			var v Status
			var e error
			s, e := m.Status(id)
			if e == nil && action != "reconcile" && s.CandidateHash != in.Hash {
				e = errors.New("candidate changed")
			}
			if e == nil {
				switch action {
				case "approve":
					e = m.Approve(id, in.Hash, in.Actor)
					v, _ = m.Status(id)
				case "promote":
					v, e = m.Promote(id)
				case "reconcile":
					v, e = m.Reconcile(r.Context(), id)
				}
			}
			pipelineResult(w, v, e)
		})
	}
	admin.HandleFunc("POST /v1/withdrawals/plan", func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			ID         string     `json:"id"`
			Bundle     string     `json:"bundle"`
			Digest     string     `json:"digest"`
			Withdrawal Withdrawal `json:"withdrawal"`
		}
		if closedRequest(w, r, &in) != nil {
			http.Error(w, "invalid withdrawal", 400)
			return
		}
		j, e := m.WithdrawalPlan(in.ID, in.Bundle, in.Digest, in.Withdrawal)
		pipelineResult(w, map[string]any{"job": j, "hash": j.Hash()}, e)
	})
	admin.HandleFunc("POST /v1/releases/prune", func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			Bundle string `json:"bundle"`
			Digest string `json:"digest"`
			Actor  string `json:"actor"`
		}
		if closedRequest(w, r, &in) != nil {
			http.Error(w, "invalid request", 400)
			return
		}
		e := m.Prune(r.Context(), in.Bundle, in.Digest, in.Actor)
		pipelineResult(w, map[string]string{"status": "completed"}, e)
	})
	root.Handle("/", ipc.RequireUID([]uint32{gatewayUID}, admin))
	return root
}

func closedRequest(w http.ResponseWriter, r *http.Request, value any) error {
	r.Body = http.MaxBytesReader(w, r.Body, 128<<10)
	b, e := io.ReadAll(r.Body)
	if e != nil {
		return e
	}
	if e = uniqueJSON(b); e != nil {
		return e
	}
	d := json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	return d.Decode(value)
}
func pipelineResult(w http.ResponseWriter, v any, e error) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	if e != nil {
		http.Error(w, `{"error":"operation_rejected; inspect status before retry"}`, 409)
		return
	}
	json.NewEncoder(w).Encode(v)
}
