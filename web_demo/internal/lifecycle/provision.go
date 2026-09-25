package lifecycle

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/STRK-Solutions/Feather-Mesh/web_demo/internal/broker"
	"github.com/STRK-Solutions/Feather-Mesh/web_demo/internal/capability"
	"github.com/STRK-Solutions/Feather-Mesh/web_demo/internal/ipc"
	"github.com/STRK-Solutions/Feather-Mesh/web_demo/internal/pipeline"
)

type Endpoint struct {
	TerminalDirectory string `json:"terminal_directory"`
	ModelDirectory    string `json:"model_directory"`
	EventDirectory    string `json:"event_directory"`
	ConfigDirectory   string `json:"config_directory"`
	ParticipantID     string `json:"participant_id"`
	SoftwareSHA256    string `json:"software_sha256"`
	ProfileSHA256     string `json:"profile_sha256"`
	DatasetSHA256     string `json:"dataset_sha256"`
	Synthetic         bool   `json:"synthetic"`
}

type Provisioner struct {
	Authority                                                  HTTPAuthority
	BrokerSocket, CollectorSocket, PipelineSocket, ReleaseRoot string
	Profile                                                    []byte
	Endpoints                                                  map[string]Endpoint
	// ACL must grant only the mapped container identity read access. Tests may
	// replace this host-specific operation; the production binary cannot.
	ACL     func(string, bool) error
	expires map[string]time.Time
}

func (p *Provisioner) endpoint(w Workspace) (Endpoint, error) {
	e, ok := p.Endpoints[w.ID]
	if !ok || !uuidRE.MatchString(e.ParticipantID) {
		return e, errors.New("workspace endpoint not provisioned")
	}
	for _, path := range []string{e.TerminalDirectory, e.ModelDirectory, e.EventDirectory, e.ConfigDirectory} {
		if !filepath.IsAbs(path) || filepath.Clean(path) != path || !strings.HasPrefix(path, "/run/feam/") {
			return e, errors.New("endpoint outside private runtime root")
		}
		canonical, err := filepath.EvalSymlinks(path)
		if err != nil || canonical != path {
			return e, errors.New("missing or symlinked endpoint")
		}
	}
	return e, nil
}

func (p *Provisioner) Releases(ctx context.Context, w Workspace) ([]pipeline.ReleaseView, error) {
	a, err := p.Authority.Assignments(ctx, w.ID)
	if err != nil || a.Account.ID != w.OwnerID || a.Workspace.GrantVersion != w.AssignmentVersion || a.Workspace.DeploymentID != w.DeploymentID {
		return nil, ErrAuthority
	}
	out := []pipeline.ReleaseView{}
	service := HTTPAuthority{Client: ipc.Client(p.PipelineSocket), URL: "http://pipeline"}
	for _, g := range a.Grants {
		var release pipeline.ReleaseView
		if err := service.exchange(ctx, "POST", "/v1/releases/resolve", g, &release); err != nil {
			return nil, err
		}
		want := filepath.Join(p.ReleaseRoot, g.Bundle, g.Digest, "provider", "serving")
		if release.Bundle != g.Bundle || release.Digest != g.Digest || release.ServingRoot != want || !regexp.MustCompile(`^[a-z][a-z0-9_-]{0,62}$`).MatchString(release.Namespace) {
			return nil, errors.New("release identity mismatch")
		}
		actual, err := filepath.EvalSymlinks(want)
		if err != nil || actual != want {
			return nil, errors.New("unsafe release root")
		}
		out = append(out, release)
	}
	return out, nil
}

func (p *Provisioner) grantACL(path string, directory bool) error {
	if p.ACL != nil {
		return p.ACL(path, directory)
	}
	permission := "r--"
	if directory {
		permission = "r-x"
	}
	// No shared group receives permission. The mask merely enables this exact
	// mapped identity's named ACL; the runner requires only path traversal.
	return exec.Command("/usr/bin/setfacl", "-m", "g::---,o::---,u:200999:"+permission, path).Run()
}
func (p *Provisioner) write(dir, name string, b []byte) error {
	if name != filepath.Base(name) {
		return errors.New("invalid config name")
	}
	f, err := os.CreateTemp(dir, ".pending-")
	if err != nil {
		return err
	}
	path := f.Name()
	defer os.Remove(path)
	if _, err = f.Write(b); err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	if err = p.grantACL(path, false); err != nil {
		return err
	}
	if err = os.Rename(path, filepath.Join(dir, name)); err != nil {
		return err
	}
	d, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer d.Close()
	return d.Sync()
}

func (p *Provisioner) Prepare(ctx context.Context, w Workspace) error {
	endpoint, err := p.endpoint(w)
	if err != nil {
		return err
	}
	if len(p.Profile) == 0 {
		return errors.New("fixed profile unavailable")
	}
	releases, err := p.Releases(ctx, w)
	if err != nil {
		return err
	}
	temporary, err := os.MkdirTemp(endpoint.ConfigDirectory, ".trust-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(temporary)
	if err = broker.InitializeLoopbackTrust(temporary); err != nil {
		return err
	}
	for _, name := range []string{"ca.pem", "cert.pem", "key.pem"} {
		b, err := os.ReadFile(filepath.Join(temporary, name))
		if err != nil {
			return err
		}
		if err = p.write(endpoint.ConfigDirectory, name, b); err != nil {
			return err
		}
	}
	profileDir := filepath.Join(endpoint.ConfigDirectory, "feam")
	if err = os.MkdirAll(profileDir, 0700); err != nil {
		return err
	}
	if err = p.grantACL(profileDir, true); err != nil {
		return err
	}
	if err = p.write(profileDir, "agent.toml", p.Profile); err != nil {
		return err
	}
	for _, name := range []string{"capability", "event-capability"} {
		if err = p.write(endpoint.ConfigDirectory, name, []byte("pending-generation")); err != nil {
			return err
		}
	}
	event := map[string]any{"socket": "/run/feam-events/events.sock", "capability_file": "/run/feam-config/event-capability", "deployment_id": w.DeploymentID, "workspace_id": w.ID, "generation": w.Generation, "participant_id": endpoint.ParticipantID, "software_sha256": endpoint.SoftwareSHA256, "profile_sha256": endpoint.ProfileSHA256, "dataset_sha256": endpoint.DatasetSHA256, "synthetic": endpoint.Synthetic}
	raw, err := json.Marshal(event)
	if err != nil {
		return err
	}
	if err = p.write(endpoint.ConfigDirectory, "event.json", raw); err != nil {
		return err
	}
	// Only logical identities and container paths are disclosed to the sandbox.
	grants := []map[string]any{}
	for _, r := range releases {
		grants = append(grants, map[string]any{"bundle": r.Bundle, "digest": r.Digest, "namespace": r.Namespace, "serving_root": "/datasets/" + r.Bundle + "/serving"})
	}
	raw, err = json.Marshal(grants)
	if err != nil {
		return err
	}
	if err = p.write(endpoint.ConfigDirectory, "releases.json", raw); err != nil {
		return err
	}
	if err = p.grantACL(endpoint.ConfigDirectory, true); err != nil {
		return err
	}
	delete(p.expires, w.ID)
	return nil
}

func (p *Provisioner) Activate(ctx context.Context, w Workspace) error {
	if until := p.expires[w.ID]; time.Until(until) > 15*time.Minute {
		return nil
	}
	endpoint, err := p.endpoint(w)
	if err != nil {
		return err
	}
	a, err := p.Authority.Assignments(ctx, w.ID)
	if err != nil || a.Workspace.Generation != w.Generation || a.Workspace.GrantVersion != w.AssignmentVersion {
		return ErrAuthority
	}
	releases, err := p.Releases(ctx, w)
	if err != nil {
		return err
	}
	for _, r := range releases {
		if !r.ModelMetadata || !r.ResearchEligible {
			return errors.New("release policy denies assisted capture")
		}
	}
	until := time.Now().Add(45 * time.Minute)
	claims := capability.Claims{AccountID: w.OwnerID, WorkspaceID: w.ID, DeploymentID: w.DeploymentID, AuthVersion: a.Account.AuthVersion, WorkspaceGeneration: w.Generation, GrantVersion: w.AssignmentVersion, ActivationGeneration: a.Workspace.ActivationGeneration, ExpiresAt: until}
	for _, service := range []struct{ socket, operation, file string }{{p.CollectorSocket, "event", "event-capability"}, {p.BrokerSocket, "model", "capability"}} {
		claims.Operation = service.operation
		client := HTTPAuthority{Client: ipc.Client(service.socket), URL: "http://capabilities"}
		var out struct {
			Capability string `json:"capability"`
		}
		if err = client.exchange(ctx, "POST", "/v1/capabilities/mint", claims, &out); err != nil {
			return err
		}
		if !regexp.MustCompile(`^[A-Za-z0-9_-]{43}$`).MatchString(out.Capability) {
			return errors.New("invalid capability response")
		}
		if err = p.write(endpoint.ConfigDirectory, service.file, []byte(out.Capability)); err != nil {
			return err
		}
	}
	if p.expires == nil {
		p.expires = map[string]time.Time{}
	}
	p.expires[w.ID] = until
	return nil
}
func (p *Provisioner) Revoke(ctx context.Context, w Workspace) error {
	delete(p.expires, w.ID)
	var failures []error
	for _, socket := range []string{p.BrokerSocket, p.CollectorSocket} {
		client := HTTPAuthority{Client: ipc.Client(socket), URL: "http://capabilities"}
		if err := client.post(ctx, "/v1/capabilities/revoke", map[string]string{"workspace_id": w.ID}); err != nil {
			failures = append(failures, err)
		}
	}
	return errors.Join(failures...)
}
