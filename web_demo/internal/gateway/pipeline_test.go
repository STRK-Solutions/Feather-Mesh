package gateway

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/STRK-Solutions/Feather-Mesh/web_demo/internal/pipeline"
)

func gatewayJob() pipeline.Job {
	return pipeline.Job{Protocol: pipeline.Protocol, ID: "00000000-0000-4000-8000-000000000001", Bundle: "demo-climate", Namespace: "demo-climate", ImporterVersion: "1", Sources: []pipeline.Source{{ID: "eccc", Importer: "eccc-csv", URL: "https://climate.weather.gc.ca/climate_data/bulk_data_e.html?format=csv&stationID=49568&Year=2023&timeframe=2", SHA256: strings.Repeat("a", 64), MaxBytes: 1024, UpstreamVersion: "synthetic", RetrievedAt: "2026-09-25T00:00:00Z", License: "https://open.canada.ca/en/open-government-licence-canada", Attribution: "Synthetic ECCC fixture", Station: "6106001", StartDate: "2023-01-01", EndDate: "2023-01-02", ScientificTime: pipeline.ScientificTime{Kind: "dates-in-source", Evidence: "synthetic date field"}, BBox: []float64{}, Resampling: "none", Variable: "temperature", Unit: "degree_Celsius"}}, Products: []pipeline.Product{{SourceID: "eccc", ID: "daily", Version: "v1", Name: "Synthetic", Description: "Synthetic data", IntendedUse: "tests", Limitations: "synthetic", Producer: "Tests", Contact: "test@example.invalid", AssetID: "observations"}}, Limits: pipeline.Limits{SourceBytes: 1024, OutputBytes: 1024, CandidateBytes: 2048, ElapsedSeconds: 30}, Policy: pipeline.Policy{Audience: "invited-users", ModelMetadata: true, ResearchEligible: true}}
}
func TestPipelineReviewBindsExactJobActorAndCandidate(t *testing.T) {
	f := setup(t, "/run/absent.sock")
	job := gatewayJob()
	raw, _ := json.Marshal(job)
	var enqueues atomic.Int64
	digest := strings.Repeat("b", 64)
	f.g.Config.PipelineSocket = unixBackend(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/jobs":
			w.Write([]byte(`[]`))
		case "/v1/jobs/plan":
			json.NewEncoder(w).Encode(pipelinePlan{Job: job, Hash: job.Hash()})
		case "/v1/jobs/enqueue":
			enqueues.Add(1)
			json.NewEncoder(w).Encode(pipeline.Status{ID: job.ID, State: "queued"})
		case "/v1/jobs/" + job.ID + "/review":
			json.NewEncoder(w).Encode(pipeline.Review{Job: job, Status: pipeline.Status{ID: job.ID, State: "prepared", CandidateHash: digest}, CandidateBytes: 7, Inventory: []pipeline.InventoryEntry{{Path: "provider/serving/manifest.json", Bytes: 7, SHA256: digest}}})
		default:
			t.Error("unexpected pipeline call", r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	response := postForm(t, f, 0, f.g.Config.AdminHost, "admin", "/pipeline/plan", url.Values{"job": {string(raw)}})
	if response.Code != 200 || !strings.Contains(response.Body.String(), job.Hash()) {
		t.Fatal(response.Code, response.Body.String())
	}
	match := regexp.MustCompile(`name="review" value="([^"]+)"`).FindStringSubmatch(response.Body.String())
	if len(match) != 2 {
		t.Fatal("missing exact job review")
	}
	form := url.Values{"job": {string(raw)}, "review": {match[1]}}
	response = postForm(t, f, 1, f.g.Config.AdminHost, "admin", "/pipeline/enqueue", form)
	if response.Code != 403 {
		t.Fatal("another admin reused review")
	}
	changed := job
	changed.Products = append([]pipeline.Product{}, job.Products...)
	changed.Products[0].Description = "edited after review"
	changedJSON, _ := json.Marshal(changed)
	form.Set("job", string(changedJSON))
	response = postForm(t, f, 0, f.g.Config.AdminHost, "admin", "/pipeline/enqueue", form)
	if response.Code != 409 || enqueues.Load() != 0 {
		t.Fatal("changed job queued")
	}
	form.Set("job", string(raw))
	response = postForm(t, f, 0, f.g.Config.AdminHost, "admin", "/pipeline/enqueue", form)
	if response.Code != 303 || enqueues.Load() != 1 {
		t.Fatal("reviewed job not queued", response.Code, response.Body.String())
	}
	request := request(f, "GET", f.g.Config.AdminHost, "/pipeline/review/"+job.ID, f.accounts[0].Email, "admin", "")
	response = httptest.NewRecorder()
	f.g.ServeHTTP(response, request)
	if response.Code != 200 || !strings.Contains(response.Body.String(), "provider/serving/manifest.json") || !strings.Contains(response.Body.String(), digest) || !strings.Contains(response.Body.String(), "Approve this exact complete candidate") {
		t.Fatal("incomplete candidate approval view", response.Code, response.Body.String())
	}
	response = postForm(t, f, 0, f.g.Config.PortalHost, "portal", "/pipeline/plan", url.Values{"job": {string(raw)}})
	if response.Code != 403 {
		t.Fatal("portal pipeline mutation accepted")
	}
}
