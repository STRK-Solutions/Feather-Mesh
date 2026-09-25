package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"html/template"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/STRK-Solutions/Feather-Mesh/web_demo/internal/control"
	"github.com/STRK-Solutions/Feather-Mesh/web_demo/internal/ipc"
	"github.com/STRK-Solutions/Feather-Mesh/web_demo/internal/pipeline"
)

func (g *Gateway) pipelineCall(ctx context.Context, method, path string, input, output any) error {
	if g.Config.PipelineSocket == "" {
		return errors.New("pipeline unavailable")
	}
	b, e := json.Marshal(input)
	if e != nil {
		return e
	}
	req, e := http.NewRequestWithContext(ctx, method, "http://pipeline"+path, bytes.NewReader(b))
	if e != nil {
		return e
	}
	req.Header.Set("Content-Type", "application/json")
	resp, e := ipc.Client(g.Config.PipelineSocket).Do(req)
	if e != nil {
		return e
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return errors.New("pipeline operation rejected; inspect status before retry")
	}
	d := json.NewDecoder(io.LimitReader(resp.Body, 4<<20))
	d.DisallowUnknownFields()
	if e = d.Decode(output); e != nil {
		return e
	}
	if d.Decode(new(any)) != io.EOF {
		return errors.New("invalid pipeline response")
	}
	return nil
}
func (g *Gateway) pipelineJobs(ctx context.Context) ([]pipeline.Status, bool) {
	var jobs []pipeline.Status
	e := g.pipelineCall(ctx, "GET", "/v1/jobs", nil, &jobs)
	return jobs, e == nil
}

type pipelineToken struct {
	Actor            string
	Version          int64
	Action, ID, Hash string
	Expires          int64
}

func (g *Gateway) pipelineToken(a control.Account, action, id, hash string) string {
	return g.sign(pipelineToken{Actor: a.ID, Version: a.AuthVersion, Action: action, ID: id, Hash: hash, Expires: time.Now().Add(5 * time.Minute).Unix()})
}
func (g *Gateway) pipelineReview(w http.ResponseWriter, r *http.Request, a control.Account, csrf string) {
	if a.Role != "admin" || r.Host != g.Config.AdminHost {
		http.Error(w, "admin required", 403)
		return
	}
	id := strings.TrimPrefix(r.URL.Path, "/pipeline/review/")
	if !control.ValidID(id) {
		http.NotFound(w, r)
		return
	}
	var view pipeline.Review
	if g.pipelineCall(r.Context(), "GET", "/v1/jobs/"+id+"/review", nil, &view) != nil {
		http.Error(w, "candidate review unavailable; inspect recorded job", 503)
		return
	}
	if view.Status.ID != id || view.Job.ID != id {
		http.Error(w, "candidate identity mismatch", 503)
		return
	}
	jobJSON, _ := json.MarshalIndent(view.Job, "", "  ")
	data := pipelinePage{CSRF: csrf, Review: &view, JobJSON: string(jobJSON)}
	switch view.Status.State {
	case "prepared":
		data.Action = "approve"
	case "approved":
		data.Action = "promote"
	case "unknown", "publishing", "promoting", "preparing":
		data.Action = "reconcile"
	case "released":
		if view.ReleaseStatus == "retained" {
			data.Action = "prune"
		}
	}
	if data.Action != "" {
		data.Token = g.pipelineToken(a, data.Action, id, view.Status.CandidateHash)
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = pipelineTemplate.Execute(w, data)
}

type pipelinePlan struct {
	Job  pipeline.Job `json:"job"`
	Hash string       `json:"hash"`
}

func (g *Gateway) showPipelinePlan(w http.ResponseWriter, r *http.Request, a control.Account, plan pipelinePlan) {
	if plan.Hash != plan.Job.Hash() || plan.Job.Validate() != nil {
		http.Error(w, "invalid pipeline plan", 503)
		return
	}
	jobJSON, _ := json.MarshalIndent(plan.Job, "", "  ")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = pipelineTemplate.Execute(w, pipelinePage{CSRF: r.PostForm.Get("csrf"), Plan: &plan, JobJSON: string(jobJSON), Action: "enqueue", Token: g.pipelineToken(a, "enqueue", plan.Job.ID, plan.Hash)})
}
func (g *Gateway) pipelineAction(w http.ResponseWriter, r *http.Request, a control.Account) {
	if a.Role != "admin" || r.Host != g.Config.AdminHost {
		http.Error(w, "admin required", 403)
		return
	}
	for _, v := range r.PostForm {
		if len(v) != 1 {
			http.Error(w, "duplicate form field", 400)
			return
		}
	}
	path := strings.TrimPrefix(r.URL.Path, "/pipeline/")
	if path == "plan" {
		job, e := pipeline.DecodeJob([]byte(r.PostForm.Get("job")))
		if e != nil {
			http.Error(w, "invalid closed source job", 400)
			return
		}
		var plan pipelinePlan
		if g.pipelineCall(r.Context(), "POST", "/v1/jobs/plan", map[string]any{"job": job}, &plan) != nil {
			http.Error(w, "pipeline plan unavailable", 503)
			return
		}
		g.showPipelinePlan(w, r, a, plan)
		return
	}
	if path == "withdrawal-plan" {
		var plan pipelinePlan
		input := map[string]any{"id": control.ID(), "bundle": r.PostForm.Get("bundle"), "digest": r.PostForm.Get("digest"), "withdrawal": pipeline.Withdrawal{Targets: []pipeline.WithdrawalTarget{{ID: r.PostForm.Get("product"), Version: r.PostForm.Get("version")}}, Reason: r.PostForm.Get("reason")}}
		if g.pipelineCall(r.Context(), "POST", "/v1/withdrawals/plan", input, &plan) != nil {
			http.Error(w, "withdrawal plan unavailable", 409)
			return
		}
		g.showPipelinePlan(w, r, a, plan)
		return
	}
	var reviewed pipelineToken
	if !g.unsign(r.PostForm.Get("review"), &reviewed) || reviewed.Actor != a.ID || reviewed.Version != a.AuthVersion || reviewed.Action != path || reviewed.Expires < time.Now().Unix() || !control.ValidID(reviewed.ID) {
		http.Error(w, "fresh exact review required", 403)
		return
	}
	var output any
	var e error
	switch path {
	case "enqueue":
		job, err := pipeline.DecodeJob([]byte(r.PostForm.Get("job")))
		if err != nil || job.ID != reviewed.ID || job.Hash() != reviewed.Hash {
			http.Error(w, "reviewed source job changed", 409)
			return
		}
		e = g.pipelineCall(r.Context(), "POST", "/v1/jobs/enqueue", map[string]any{"job": job, "hash": reviewed.Hash, "actor": a.ID}, &output)
	case "approve", "promote", "reconcile":
		e = g.pipelineCall(r.Context(), "POST", "/v1/jobs/"+reviewed.ID+"/"+path, map[string]string{"hash": reviewed.Hash, "actor": a.ID}, &output)
	case "prune":
		var view pipeline.Review
		e = g.pipelineCall(r.Context(), "GET", "/v1/jobs/"+reviewed.ID+"/review", nil, &view)
		if e == nil && (view.Status.CandidateHash != reviewed.Hash || view.ReleaseStatus != "retained") {
			e = errors.New("release changed")
		}
		if e == nil {
			e = g.pipelineCall(r.Context(), "POST", "/v1/releases/prune", map[string]string{"bundle": view.Job.Bundle, "digest": reviewed.Hash, "actor": a.ID}, &output)
		}
	default:
		http.NotFound(w, r)
		return
	}
	if e != nil {
		http.Error(w, "pipeline outcome requires inspection; open job review before retry", 409)
		return
	}
	g.reconfigureGrants(r.Context())
	http.Redirect(w, r, "/pipeline/review/"+reviewed.ID, 303)
}

type pipelinePage struct {
	CSRF, JobJSON, Action, Token string
	Plan                         *pipelinePlan
	Review                       *pipeline.Review
}

var pipelineTemplate = template.Must(template.New("pipeline").Parse(`<!doctype html><html lang="en"><meta charset="utf-8"><meta name="viewport" content="width=device-width"><title>Dataset review</title><style>body{font:16px system-ui;max-width:75rem;margin:2rem auto;padding:1rem}pre{white-space:pre-wrap;overflow-wrap:anywhere}td{padding:.5rem;overflow-wrap:anywhere}table{width:100%;table-layout:fixed}button{padding:.7rem}</style><h1>Dataset review</h1>{{with .Plan}}<h2>Bounded source job</h2><p>Review the exact source objects, hashes, limits, audience and disclosure policy. Confirming this step queues private preparation; release promotion requires a separate complete-candidate review.</p><p>Job hash: <code>{{.Hash}}</code></p>{{end}}{{with .Review}}<p>Job {{.Status.ID}} · {{.Status.State}} · {{.ReleaseStatus}}</p><p>Manifest outcome: {{.Status.ManifestOutcome}} · Promotion outcome: {{.Status.PromotionOutcome}}</p><h2>Complete candidate</h2><p>{{.CandidateBytes}} bytes · SHA-256 <code>{{.Status.CandidateHash}}</code></p><table><tr><th>Relative path</th><th>Bytes</th><th>SHA-256</th></tr>{{range .Inventory}}<tr><td>{{.Path}}</td><td>{{.Bytes}}</td><td>{{.SHA256}}</td></tr>{{end}}</table>{{end}}<h2>Source and policy record</h2><pre>{{.JobJSON}}</pre>{{if .Action}}<form method="post" action="/pipeline/{{.Action}}"><input type="hidden" name="csrf" value="{{.CSRF}}"><input type="hidden" name="review" value="{{.Token}}">{{if .Plan}}<textarea name="job" hidden>{{.JobJSON}}</textarea>{{end}}{{if eq .Action "enqueue"}}<button>Confirm exact job and queue preparation</button>{{else if eq .Action "approve"}}<button>Approve this exact complete candidate</button>{{else if eq .Action "promote"}}<p>Promotion makes this approved release eligible for grants. A withdrawal release also revokes existing grants and stops affected workspaces.</p><button>Promote approved release</button>{{else if eq .Action "prune"}}<p>Delete this exact retained release only if no active or pending grant, candidate or withdrawal policy protects it. This deletion cannot be undone.</p><button>Delete unpinned retained release</button>{{else}}<button>Reconcile recorded outcome without replay</button>{{end}}</form>{{end}}<p><a href="/">Administrator page</a></p></html>`))
