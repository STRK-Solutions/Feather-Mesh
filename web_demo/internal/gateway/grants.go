package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"path/filepath"
	"regexp"
	"strconv"

	"github.com/STRK-Solutions/Feather-Mesh/web_demo/internal/control"
	"github.com/STRK-Solutions/Feather-Mesh/web_demo/internal/ipc"
	"github.com/STRK-Solutions/Feather-Mesh/web_demo/internal/pipeline"
)

var bundleName = regexp.MustCompile(`^[a-z][a-z0-9-]{0,62}$`)
var releaseDigest = regexp.MustCompile(`^[0-9a-f]{64}$`)

// Assignment changes accept identities only. The pipeline resolves the exact
// current immutable release; the controller independently validates it at mount.
func (g *Gateway) grant(w http.ResponseWriter, r *http.Request, a control.Account) {
	if r.Host != g.Config.AdminHost || a.Role != "admin" {
		http.Error(w, "admin authorization required", 403)
		return
	}
	for key, values := range r.PostForm {
		if len(values) != 1 || (key != "csrf" && key != "account_id" && key != "bundle" && key != "digest" && key != "grant_version" && key != "action") {
			http.Error(w, "invalid grant request", 400)
			return
		}
	}
	target, bundle, digest := r.PostForm.Get("account_id"), r.PostForm.Get("bundle"), r.PostForm.Get("digest")
	version, e := strconv.ParseInt(r.PostForm.Get("grant_version"), 10, 64)
	if e != nil || version < 1 || !bundleName.MatchString(bundle) {
		http.Error(w, "invalid grant request", 400)
		return
	}
	switch r.PostForm.Get("action") {
	case "assign":
		if !releaseDigest.MatchString(digest) {
			http.Error(w, "exact approved release digest required", 400)
			return
		}
		if !g.resolveRelease(r.Context(), bundle, digest) {
			http.Error(w, "release unavailable; grant unchanged", 409)
			return
		}
	case "revoke":
		if digest != "" {
			http.Error(w, "revocation must omit digest", 400)
			return
		}
	default:
		http.Error(w, "invalid grant action", 400)
		return
	}
	if e = g.Store.SetGrant(r.Context(), a.ID, target, bundle, digest, version); e != nil {
		http.Error(w, "grant changed or account unavailable; reload", 409)
		return
	}
	g.Streams.Revoke(target)
	// The transaction persists the pending dispatch before this best-effort send.
	// The background loop repeats it after failures or a gateway restart.
	g.reconfigureGrants(r.Context())
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (g *Gateway) resolveRelease(ctx context.Context, bundle, digest string) bool {
	if g.Config.PipelineSocket == "" {
		return false
	}
	b, _ := json.Marshal(control.Grant{Bundle: bundle, Digest: digest})
	req, e := http.NewRequestWithContext(ctx, "POST", "http://pipeline/v1/releases/resolve", bytes.NewReader(b))
	if e != nil {
		return false
	}
	req.Header.Set("Content-Type", "application/json")
	resp, e := ipc.Client(g.Config.PipelineSocket).Do(req)
	if e != nil {
		return false
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return false
	}
	var release pipeline.ReleaseView
	d := json.NewDecoder(io.LimitReader(resp.Body, 64<<10))
	d.DisallowUnknownFields()
	if d.Decode(&release) != nil || d.Decode(new(any)) != io.EOF {
		return false
	}
	return release.Bundle == bundle && release.Digest == digest && release.Namespace == bundle && release.Revision > 0 && filepath.IsAbs(release.ServingRoot) && filepath.Clean(release.ServingRoot) == release.ServingRoot && filepath.Base(release.ServingRoot) == "serving"
}

func (g *Gateway) reconfigureGrants(ctx context.Context) {
	updates, e := g.Store.PendingGrantUpdates(ctx)
	if e != nil {
		return
	}
	for _, update := range updates {
		g.Streams.Revoke(update.OwnerID)
		if g.Config.ControllerSocket == "" {
			continue
		}
		b, _ := json.Marshal(update)
		req, e := http.NewRequestWithContext(ctx, "POST", "http://controller/v1/workspaces/reconfigure", bytes.NewReader(b))
		if e != nil {
			continue
		}
		req.Header.Set("Content-Type", "application/json")
		resp, e := ipc.Client(g.Config.ControllerSocket).Do(req)
		if e != nil {
			continue
		}
		// A queued job is insufficient: the endpoint acknowledges only once the
		// old generation is stopped and the exact policy version is installed.
		var result struct {
			Status string `json:"status"`
		}
		d := json.NewDecoder(io.LimitReader(resp.Body, 64<<10))
		d.DisallowUnknownFields()
		valid := resp.StatusCode == http.StatusOK && d.Decode(&result) == nil && d.Decode(new(any)) == io.EOF && result.Status == "completed"
		resp.Body.Close()
		if valid {
			_ = g.Store.GrantUpdateComplete(ctx, update.WorkspaceID, update.GrantVersion)
		}
	}
}

type workspacePage struct {
	control.Workspace
	OwnerEmail string
	Grants     []control.Grant
}

func (g *Gateway) workspacePages(ctx context.Context, accounts []control.Account) ([]workspacePage, error) {
	workspaces, e := g.Store.WorkspaceList(ctx)
	if e != nil {
		return nil, e
	}
	byID := map[string]control.Account{}
	for _, a := range accounts {
		byID[a.ID] = a
	}
	var result []workspacePage
	for _, w := range workspaces {
		a := byID[w.OwnerID]
		if a.Status != "active" {
			continue
		}
		assignments, e := g.Store.Assignments(ctx, w.ID)
		if e != nil {
			return nil, e
		}
		result = append(result, workspacePage{Workspace: assignments.Workspace, OwnerEmail: a.Email, Grants: assignments.Grants})
	}
	return result, nil
}
