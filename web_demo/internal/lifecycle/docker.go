package lifecycle

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"github.com/STRK-Solutions/Feather-Mesh/web_demo/internal/ipc"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type Slot struct {
	ID             string `json:"id"`
	Path           string `json:"path"`
	FilesystemUUID string `json:"filesystem_uuid"`
	TerminalPath   string `json:"terminal_path"`
}
type Docker struct {
	Client     *http.Client
	Deployment string
	Slots      map[string]Slot
	Image      string
	Verify     func(Slot) error
	Provision  *Provisioner
	Headroom   func() error
	Store      *Store
}

func name(w Workspace) string { return "feam-" + w.ID + "-g" + strconv.FormatInt(w.Generation, 10) }
func (d *Docker) labels(w Workspace) map[string]string {
	return map[string]string{"feam.deployment": d.Deployment, "feam.workspace": w.ID, "feam.generation": strconv.FormatInt(w.Generation, 10), "feam.slot": w.SlotID, "feam.image": d.Image, "feam.grant_version": strconv.FormatInt(w.AssignmentVersion, 10)}
}
func (d *Docker) request(ctx context.Context, method, path string, body any) (int, []byte, error) {
	var b []byte
	var e error
	if body != nil {
		b, e = json.Marshal(body)
		if e != nil {
			return 0, nil, e
		}
	}
	req, e := http.NewRequestWithContext(ctx, method, "http://docker/v1.47"+path, bytes.NewReader(b))
	if e != nil {
		return 0, nil, e
	}
	req.Header.Set("Content-Type", "application/json")
	res, e := d.Client.Do(req)
	if e != nil {
		return 0, nil, e
	}
	defer res.Body.Close()
	raw, e := io.ReadAll(io.LimitReader(res.Body, (1<<20)+1))
	if e != nil || len(raw) > 1<<20 {
		return 0, nil, errors.New("runtime response exceeded bounds")
	}
	return res.StatusCode, raw, nil
}
func (d *Docker) Inspect(ctx context.Context, w Workspace) (bool, bool, error) {
	status, b, e := d.request(ctx, "GET", "/containers/"+name(w)+"/json", nil)
	if e != nil {
		return false, false, e
	}
	if status == 404 {
		return false, false, nil
	}
	if status != 200 {
		return false, false, errors.New("runtime unavailable")
	}
	var v struct {
		Image  string
		Config struct {
			Labels map[string]string
			User   string
		}
		State      struct{ Running bool }
		HostConfig struct {
			NetworkMode    string
			Privileged     bool
			ReadonlyRootfs bool
			RestartPolicy  struct{ Name string }
			Memory         int64
			MemorySwap     int64
			NanoCpus       int64
			PidsLimit      int64
			CapDrop        []string
			SecurityOpt    []string
		}
	}
	if e = json.Unmarshal(b, &v); e != nil {
		return false, false, e
	}
	for k, value := range d.labels(w) {
		if v.Config.Labels[k] != value {
			return false, false, errors.New("unowned or mismatched runtime resource")
		}
	}
	if v.Image != d.Image || v.HostConfig.Privileged || !v.HostConfig.ReadonlyRootfs || v.HostConfig.NetworkMode != "none" || v.HostConfig.RestartPolicy.Name != "no" {
		return false, false, errors.New("runtime template drift")
	}
	if v.Config.User != "1000:1000" || v.HostConfig.Memory != 512<<20 || v.HostConfig.MemorySwap != 512<<20 || v.HostConfig.NanoCpus != 500000000 || v.HostConfig.PidsLimit != 128 || len(v.HostConfig.CapDrop) != 1 || v.HostConfig.CapDrop[0] != "ALL" || len(v.HostConfig.SecurityOpt) != 1 || v.HostConfig.SecurityOpt[0] != "no-new-privileges" && v.HostConfig.SecurityOpt[0] != "no-new-privileges=true" {
		return false, false, errors.New("runtime resource policy drift")
	}
	if d.Store == nil {
		return false, false, errors.New("runtime template store missing")
	}
	template, e := d.Store.RuntimeTemplate(w)
	if e != nil {
		return false, false, e
	}
	if e = template.verify(b); e != nil {
		return false, false, e
	}
	return true, v.State.Running, nil
}
func (d *Docker) Start(ctx context.Context, w Workspace) error {
	if w.DeploymentID != d.Deployment || w.ImageID != d.Image || !digestRE.MatchString(d.Image) || !uuidRE.MatchString(w.ID) {
		return errors.New("incompatible deployment image")
	}
	slot, ok := d.Slots[w.SlotID]
	if !ok {
		return errors.New("unknown finite slot")
	}
	verify := d.Verify
	if verify == nil {
		verify = VerifySlot
	}
	if e := verify(slot); e != nil {
		return e
	}
	exists, running, e := d.Inspect(ctx, w)
	if e != nil {
		return e
	}
	if running {
		return nil
	}
	headroom := d.Headroom
	if headroom == nil {
		headroom = VerifyHeadroom
	}
	if e = headroom(); e != nil {
		return e
	}
	if d.Provision == nil {
		return errors.New("hosted workspace configuration required")
	}
	endpoint, e := d.Provision.endpoint(w)
	if e != nil {
		return e
	}
	releases, e := d.Provision.Releases(ctx, w)
	if e != nil {
		return e
	}
	if !exists {
		mounts := []map[string]any{
			{"Type": "bind", "Source": slot.Path, "Target": "/workspace"},
			{"Type": "bind", "Source": endpoint.TerminalDirectory, "Target": "/run/feam-terminal"},
			{"Type": "bind", "Source": endpoint.ModelDirectory, "Target": "/run/feam-model", "ReadOnly": true},
			{"Type": "bind", "Source": endpoint.EventDirectory, "Target": "/run/feam-events", "ReadOnly": true},
			{"Type": "bind", "Source": endpoint.ConfigDirectory, "Target": "/run/feam-config", "ReadOnly": true},
		}
		for _, release := range releases {
			mounts = append(mounts, map[string]any{"Type": "bind", "Source": release.ServingRoot, "Target": "/datasets/" + release.Bundle + "/serving", "ReadOnly": true})
		}
		body := map[string]any{"Image": d.Image, "User": "1000:1000", "Labels": d.labels(w), "HostConfig": map[string]any{"NetworkMode": "none", "ReadonlyRootfs": true, "CapDrop": []string{"ALL"}, "SecurityOpt": []string{"no-new-privileges"}, "RestartPolicy": map[string]string{"Name": "no"}, "Init": true, "Memory": 512 << 20, "MemorySwap": 512 << 20, "NanoCpus": 500000000, "PidsLimit": 128, "ShmSize": 16 << 20, "Ulimits": []map[string]any{{"Name": "nofile", "Soft": 1024, "Hard": 1024}}, "Tmpfs": map[string]string{"/tmp": "rw,nosuid,nodev,noexec,size=64m,mode=1777"}, "Mounts": []map[string]any{{"Type": "bind", "Source": slot.Path, "Target": "/workspace"}, {"Type": "bind", "Source": slot.TerminalPath, "Target": "/run/feam-terminal"}}, "LogConfig": map[string]any{"Type": "local", "Config": map[string]string{"max-size": "1m", "max-file": "2"}}}}
		body["Env"] = []string{"FEAM_DEMO_MODE=hosted", "FEAM_EVENT_CONFIG=/run/feam-config/event.json"}
		body["HostConfig"].(map[string]any)["Mounts"] = mounts
		if d.Store == nil {
			return errors.New("runtime template store missing")
		}
		raw, e := json.Marshal(body)
		if e != nil {
			return e
		}
		if e = d.Store.RecordTemplate(w, raw); e != nil {
			return e
		}
		code, _, e := d.request(ctx, "POST", "/containers/create?name="+name(w), body)
		if e != nil {
			return e
		}
		if code != 201 {
			return errors.New("runtime create uncertain; reconcile by recorded labels")
		}
	}
	code, _, e := d.request(ctx, "POST", "/containers/"+name(w)+"/start", nil)
	if e != nil {
		return e
	}
	if code != 204 && code != 304 {
		return errors.New("runtime start uncertain")
	}
	return d.Ready(ctx, w)
}

// Readiness proves the fixed entrypoint completed fixture initialization and
// exposed ttyd. Container Running alone does not establish a usable workspace.
func (d *Docker) Ready(ctx context.Context, w Workspace) error {
	if d.Provision == nil {
		return errors.New("private endpoint missing")
	}
	endpoint, e := d.Provision.endpoint(w)
	if e != nil {
		return e
	}
	deadline := time.Now().Add(20 * time.Second)
	client := ipc.Client(filepath.Join(endpoint.TerminalDirectory, "ttyd.sock"))
	client.Timeout = time.Second
	for time.Now().Before(deadline) {
		req, e := http.NewRequestWithContext(ctx, "GET", "http://terminal/", nil)
		if e != nil {
			return e
		}
		response, e := client.Do(req)
		if e == nil {
			_ = response.Body.Close()
			if response.StatusCode == 200 {
				return nil
			}
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(100 * time.Millisecond):
		}
	}
	return errors.New("workspace initialization not ready; inspect recorded generation")
}
func (d *Docker) Stop(ctx context.Context, w Workspace) error {
	exists, running, e := d.Inspect(ctx, w)
	if e != nil {
		return e
	}
	if !exists || !running {
		return nil
	}
	code, _, e := d.request(ctx, "POST", "/containers/"+name(w)+"/stop?t=15", nil)
	if e != nil {
		return e
	}
	if code != 204 && code != 304 {
		return errors.New("runtime stop uncertain")
	}
	return nil
}

// Only operator-recorded exact mountpoints are usable. A missing mount cannot
// fall through to an ordinary host directory. This check runs unprivileged.
func VerifySlot(s Slot) error {
	if !strings.HasPrefix(s.Path, "/home/feam-service-data/") || s.FilesystemUUID == "" {
		return errors.New("unapproved slot scope")
	}
	for _, p := range []string{s.Path} {
		canonical, e := filepath.EvalSymlinks(p)
		if e != nil || canonical != p {
			return errors.New("symlink or missing slot path")
		}
		f, e := os.Stat(p)
		if e != nil || !f.IsDir() {
			return errors.New("missing slot directory")
		}
	}
	out, e := exec.Command("/usr/bin/findmnt", "-n", "-o", "UUID", "--mountpoint", s.Path).Output()
	if e != nil || strings.TrimSpace(string(out)) != s.FilesystemUUID {
		return errors.New("workspace filesystem identity mismatch")
	}
	return nil
}
