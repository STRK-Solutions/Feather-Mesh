package pipeline

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/STRK-Solutions/Feather-Mesh/web_demo/internal/ipc"
)

type ReleaseView struct {
	Bundle           string `json:"bundle"`
	Digest           string `json:"digest"`
	ServingRoot      string `json:"serving_root"`
	Namespace        string `json:"namespace"`
	Revision         int64  `json:"revision"`
	ModelMetadata    bool   `json:"model_metadata"`
	ResearchEligible bool   `json:"research_eligible"`
}

// Handler exposes only the approved services' read-only release capability. It cannot
// grant a dataset, select a filesystem path, initialize state or approve a job.
func (m *Manager) Handler(serviceUIDs ...uint32) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1/releases/resolve", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		r.Body = http.MaxBytesReader(w, r.Body, 64*1024)
		raw, err := io.ReadAll(r.Body)
		var request struct {
			Bundle string `json:"bundle"`
			Digest string `json:"digest"`
		}
		if err != nil || uniqueJSON(raw) != nil {
			http.Error(w, `{"error":"invalid_request"}`, 400)
			return
		}
		var closed map[string]any
		if json.Unmarshal(raw, &closed) != nil || len(closed) != 2 || json.Unmarshal(raw, &request) != nil || !slug.MatchString(request.Bundle) || !digestPattern.MatchString(request.Digest) {
			http.Error(w, `{"error":"invalid_request"}`, 400)
			return
		}
		root, err := m.Assignable(request.Bundle, request.Digest)
		if err != nil {
			http.Error(w, `{"error":"release_unavailable"}`, 409)
			return
		}
		var spec []byte
		var revision int64
		err = m.DB.QueryRow(`SELECT j.spec,r.revision FROM releases r JOIN dataset_jobs j ON j.id=r.job_id WHERE r.bundle=? AND r.digest=? AND r.status='current' AND r.revision>=coalesce((SELECT revision FROM bundle_floors WHERE bundle=r.bundle),0)`, request.Bundle, request.Digest).Scan(&spec, &revision)
		if err != nil {
			http.Error(w, `{"error":"release_unavailable"}`, 409)
			return
		}
		job, err := DecodeJob(spec)
		if err != nil {
			http.Error(w, `{"error":"release_unavailable"}`, 503)
			return
		}
		json.NewEncoder(w).Encode(ReleaseView{Bundle: request.Bundle, Digest: request.Digest, ServingRoot: root, Namespace: job.Namespace, Revision: revision, ModelMetadata: job.Policy.ModelMetadata, ResearchEligible: job.Policy.ResearchEligible})
	})
	return ipc.RequireUID(serviceUIDs, mux)
}
