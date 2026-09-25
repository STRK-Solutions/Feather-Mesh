//go:build pipeline_integration

package pipeline

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/STRK-Solutions/Feather-Mesh/web_demo/internal/ipc"
)

func realManager(t *testing.T) *Manager {
	t.Helper()
	for _, key := range []string{"FEAM_PIPELINE_FEAM", "FEAM_PIPELINE_PYTHON", "FEAM_PIPELINE_CONVERTER"} {
		if os.Getenv(key) == "" {
			t.Fatalf("required integration dependency %s missing", key)
		}
	}
	c := DefaultConfig(filepath.Join(t.TempDir(), "dataset root"), os.Getenv("FEAM_PIPELINE_FEAM"), os.Getenv("FEAM_PIPELINE_PYTHON"), os.Getenv("FEAM_PIPELINE_CONVERTER"))
	c.MinimumFree = 0
	m, err := Initialize(c)
	if err != nil {
		t.Fatal(err)
	}
	m.headroomPercent = 0 // This temporary directory is not the 200 GiB dataset filesystem.
	t.Cleanup(func() {
		m.DB.Close()
		filepath.WalkDir(c.Root, func(path string, d os.DirEntry, err error) error {
			if err == nil && d.IsDir() {
				return os.Chmod(path, 0700)
			}
			return err
		})
	})
	return m
}

func realInput(t *testing.T, j *Job) string {
	t.Helper()
	dir := t.TempDir()
	raw := []byte("Climate ID,Date/Time,Max Temp (°C),Max Temp Flag,Min Temp (°C),Min Temp Flag,Mean Temp (°C),Mean Temp Flag,Total Precip (mm),Total Precip Flag\n6106001,2023-01-01,2.5,E,-1,,0.5,,,M\n6106001,2023-01-02,3,,-2,,0,,0,T\n")
	p := filepath.Join(dir, "eccc")
	if err := os.WriteFile(p, raw, 0600); err != nil {
		t.Fatal(err)
	}
	hash, _, err := fileHash(p)
	if err != nil {
		t.Fatal(err)
	}
	j.Sources[0].SHA256 = hash
	return dir
}

func TestRealFEAMPipelineReleaseAndSuccessor(t *testing.T) {
	m := realManager(t)
	j := fixtureJob()
	input := realInput(t, &j)
	s, err := m.Prepare(context.Background(), j, j.Hash(), input)
	if err != nil {
		t.Fatalf("prepare: %+v: %v", s, err)
	}
	if s.State != "prepared" || s.ManifestOutcome != "committed" {
		t.Fatalf("wrong outcomes: %+v", s)
	}
	if _, err = m.Promote(j.ID); err == nil {
		t.Fatal("promotion without approval")
	}
	if err = m.Approve(j.ID, strings.Repeat("a", 64), "00000000-0000-4000-8000-000000000099"); err == nil {
		t.Fatal("different candidate hash approved")
	}
	if err = m.Approve(j.ID, s.CandidateHash, "00000000-0000-4000-8000-000000000099"); err != nil {
		t.Fatal(err)
	}
	s, err = m.Promote(j.ID)
	if err != nil {
		t.Fatal(err)
	}
	first := s.CandidateHash
	socket := releaseTestServer(t, m)
	view, code := releaseTestResolve(t, socket, j.Bundle, first)
	if code != 200 || view.Namespace != j.Namespace || view.ModelMetadata != j.Policy.ModelMetadata || view.ResearchEligible != j.Policy.ResearchEligible || view.Revision < 1 {
		t.Fatalf("exact release IPC/policy: %+v status %d", view, code)
	}
	serving, err := m.Assignable(j.Bundle, first)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"raw", "job.json", "daily.metadata.json", "private.csv"} {
		if _, err = os.Stat(filepath.Join(serving, name)); !errors.Is(err, os.ErrNotExist) {
			t.Fatal("private input exposed")
		}
	}
	asset := filepath.Join(serving, "datasets/daily/v1/observations.parquet")
	info, err := os.Stat(asset)
	if err != nil || info.Mode().Perm()&0222 != 0 {
		t.Fatal("released bytes remain writable")
	}
	before, _, err := TreeHash(m.release(j.Bundle, first))
	if err != nil {
		t.Fatal(err)
	}
	next := j
	next.ID = "00000000-0000-4000-8000-000000000002"
	next.ParentDigest = first
	next.Products = append([]Product{}, j.Products...)
	next.Products[0].Version = "v2"
	s, err = m.Prepare(context.Background(), next, next.Hash(), input)
	if err != nil {
		t.Fatalf("successor: %+v %v", s, err)
	}
	manifest, err := m.verifyCandidate(next)
	if err != nil || len(manifest.Products) != 1 || len(manifest.Products[0].Versions) != 2 {
		t.Fatalf("prior records lost: %+v %v", manifest, err)
	}
	if err = m.Approve(next.ID, s.CandidateHash, "00000000-0000-4000-8000-000000000099"); err != nil {
		t.Fatal(err)
	}
	s, err = m.Promote(next.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = m.Assignable(j.Bundle, first); err == nil {
		t.Fatal("old release assigned after successor")
	}
	if _, code = releaseTestResolve(t, socket, j.Bundle, first); code != 409 {
		t.Fatal("stale release resolved over IPC")
	}
	after, _, _ := TreeHash(m.release(j.Bundle, first))
	if before != after {
		t.Fatal("successor changed retained bytes")
	}
	if err = m.RevokeBundle(j.Bundle, manifest.Revision+1); err != nil {
		t.Fatal(err)
	}
	if _, err = m.Assignable(j.Bundle, s.CandidateHash); err == nil {
		t.Fatal("revoked release assignable")
	}
	if _, code = releaseTestResolve(t, socket, j.Bundle, s.CandidateHash); code != 409 {
		t.Fatal("withdrawn bundle resolved over IPC")
	}
}

func releaseTestServer(t *testing.T, m *Manager) string {
	t.Helper()
	dir, err := os.MkdirTemp("/tmp", "fpreal-")
	if err != nil {
		t.Fatal(err)
	}
	socket := filepath.Join(dir, "p.sock")
	listener, err := ipc.Listen(socket)
	if err != nil {
		t.Fatal(err)
	}
	server := ipc.Server(m.Handler(uint32(os.Getuid())))
	go server.Serve(listener)
	t.Cleanup(func() { server.Close(); os.RemoveAll(dir) })
	return socket
}

func releaseTestResolve(t *testing.T, socket, bundle, digest string) (ReleaseView, int) {
	t.Helper()
	b, _ := json.Marshal(map[string]string{"bundle": bundle, "digest": digest})
	response, err := ipc.Client(socket).Post("http://pipeline/v1/releases/resolve", "application/json", strings.NewReader(string(b)))
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	var view ReleaseView
	if response.StatusCode == http.StatusOK && json.NewDecoder(response.Body).Decode(&view) != nil {
		t.Fatal("invalid release response")
	}
	return view, response.StatusCode
}

func TestRealUnknownManifestAndOrphanReconciliation(t *testing.T) {
	m := realManager(t)
	j := fixtureJob()
	input := realInput(t, &j)
	s, err := m.Prepare(context.Background(), j, j.Hash(), input)
	if err != nil {
		t.Fatal(err)
	}
	// Model a lost response after the manifest commit; reconciliation reads the
	// existing record and does not call serve, which would conflict on this version.
	m.DB.Exec(`UPDATE dataset_jobs SET state='unknown',manifest_outcome='unknown',candidate_hash='' WHERE id=?`, j.ID)
	s, err = m.Reconcile(context.Background(), j.ID)
	if err != nil || s.State != "prepared" {
		t.Fatalf("commit reconciliation: %+v %v", s, err)
	}
	if err = m.Approve(j.ID, s.CandidateHash, "00000000-0000-4000-8000-000000000099"); err != nil {
		t.Fatal(err)
	}
	m.DB.Exec(`UPDATE dataset_jobs SET state='promoting',promotion_outcome='unknown' WHERE id=?`, j.ID)
	dest := m.release(j.Bundle, s.CandidateHash)
	os.MkdirAll(filepath.Dir(dest), 0700)
	if err = sealTree(m.payload(j.ID)); err != nil {
		t.Fatal(err)
	}
	if err = os.Rename(m.payload(j.ID), dest); err != nil {
		t.Fatal(err)
	}
	if _, err = m.Assignable(j.Bundle, s.CandidateHash); err == nil {
		t.Fatal("orphan was assignable")
	}
	s, err = m.Reconcile(context.Background(), j.ID)
	if err != nil || s.State != "released" {
		t.Fatalf("promotion reconciliation: %+v %v", s, err)
	}
	if _, err = m.Assignable(j.Bundle, s.CandidateHash); err != nil {
		t.Fatal(err)
	}
}

func TestRealUnregisteredScratchAndPostApprovalChange(t *testing.T) {
	m := realManager(t)
	j := fixtureJob()
	input := realInput(t, &j)
	s, err := m.Prepare(context.Background(), j, j.Hash(), input)
	if err != nil {
		t.Fatal(err)
	}
	serving := filepath.Join(m.payload(j.ID), "provider", "serving")
	scratch := filepath.Join(serving, "private.csv")
	os.WriteFile(scratch, []byte("private"), 0600)
	if _, err = m.verifyCandidate(j); err == nil {
		t.Fatal("private scratch accepted")
	}
	os.Remove(scratch)
	if err = m.Approve(j.ID, s.CandidateHash, "00000000-0000-4000-8000-000000000099"); err != nil {
		t.Fatal(err)
	}
	var record map[string]any
	b, _ := os.ReadFile(filepath.Join(serving, "manifest.json"))
	json.Unmarshal(b, &record)
	record["namespace"] = "changed"
	b, _ = json.Marshal(record)
	os.WriteFile(filepath.Join(serving, "manifest.json"), b, 0600)
	if _, err = m.Promote(j.ID); err == nil {
		t.Fatal("changed approved payload promoted")
	}
}

func TestRealWithdrawalSuccessorPreservesTombstones(t *testing.T) {
	m := realManager(t)
	ctx := context.Background()
	j := fixtureJob()
	input := realInput(t, &j)
	publish := func(job Job) Status {
		t.Helper()
		s, e := m.Prepare(ctx, job, job.Hash(), input)
		if e != nil {
			t.Fatalf("prepare %+v: %v", s, e)
		}
		if e = m.Approve(job.ID, s.CandidateHash, "00000000-0000-4000-8000-000000000099"); e != nil {
			t.Fatal(e)
		}
		s, e = m.Promote(job.ID)
		if e != nil {
			t.Fatal(e)
		}
		return s
	}
	first := publish(j)
	next := j
	next.ID = "00000000-0000-4000-8000-000000000002"
	next.ParentDigest = first.CandidateHash
	next.Products = append([]Product{}, j.Products...)
	next.Products[0].Version = "v2"
	second := publish(next)
	withdraw, e := m.WithdrawalPlan("00000000-0000-4000-8000-000000000003", j.Bundle, second.CandidateHash, Withdrawal{Targets: []WithdrawalTarget{{ID: "daily", Version: "v1"}}, Reason: "synthetic withdrawal acceptance"})
	if e != nil {
		t.Fatal(e)
	}
	changed := withdraw
	changed.Policy.ModelMetadata = !changed.Policy.ModelMetadata
	if _, e = m.Prepare(ctx, changed, changed.Hash(), input); e == nil {
		t.Fatal("withdrawal changed inherited policy")
	}
	third := publish(withdraw)
	var floor int64
	m.DB.QueryRow(`SELECT revision FROM bundle_floors WHERE bundle=?`, j.Bundle).Scan(&floor)
	if floor != 3 {
		t.Fatal("withdrawal floor missing", floor)
	}
	root, e := m.Assignable(j.Bundle, third.CandidateHash)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = m.feam(ctx, filepath.Dir(root), "resolve", "product://"+j.Namespace+"/daily", "--version", "v1"); e == nil {
		t.Fatal("withdrawn version resolved")
	}
	if _, e = m.feam(ctx, filepath.Dir(root), "resolve", "product://"+j.Namespace+"/daily", "--version", "v2", "--verify-integrity"); e != nil {
		t.Fatal("unwithdrawn data lost", e)
	}
	view, e := m.Review(withdraw.ID)
	if e != nil || view.CandidateBytes < 1 || len(view.Inventory) < 3 || view.Job.Withdrawal == nil {
		t.Fatal("complete withdrawal review missing", e)
	}
	fourth := next
	fourth.ID = "00000000-0000-4000-8000-000000000004"
	fourth.ParentDigest = third.CandidateHash
	fourth.Products = append([]Product{}, next.Products...)
	fourth.Products[0].Version = "v3"
	successor := publish(fourth)
	root, e = m.Assignable(j.Bundle, successor.CandidateHash)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = m.feam(ctx, filepath.Dir(root), "resolve", "product://"+j.Namespace+"/daily", "--version", "v1"); e == nil {
		t.Fatal("successor resurrected withdrawal")
	}
	duplicate := fourth
	duplicate.ID = "00000000-0000-4000-8000-000000000005"
	duplicate.ParentDigest = successor.CandidateHash
	duplicate.Products = append([]Product{}, fourth.Products...)
	duplicate.Products[0].Version = "v1"
	if s, e := m.Prepare(ctx, duplicate, duplicate.Hash(), input); e == nil || s.State != "unknown" {
		t.Fatal("duplicate withdrawn version republished")
	}
	if _, e = m.Reconcile(ctx, duplicate.ID); e != ErrReconcile {
		t.Fatal("duplicate invalid publication reconciled as success")
	}
}

func TestRealRetentionRequiresAuthoritativeUnpinnedRelease(t *testing.T) {
	m := realManager(t)
	ctx := context.Background()
	j := fixtureJob()
	input := realInput(t, &j)
	actor := "00000000-0000-4000-8000-000000000099"
	first, e := m.Prepare(ctx, j, j.Hash(), input)
	if e != nil {
		t.Fatal(e)
	}
	if e = m.Approve(j.ID, first.CandidateHash, actor); e != nil {
		t.Fatal(e)
	}
	if _, e = m.Promote(j.ID); e != nil {
		t.Fatal(e)
	}
	if e = m.Prune(ctx, j.Bundle, first.CandidateHash, actor); e == nil {
		t.Fatal("current release pruned")
	}
	next := j
	next.ID = "00000000-0000-4000-8000-000000000002"
	next.ParentDigest = first.CandidateHash
	next.Products = append([]Product{}, j.Products...)
	next.Products[0].Version = "v2"
	second, e := m.Prepare(ctx, next, next.Hash(), input)
	if e != nil {
		t.Fatal(e)
	}
	m.Approve(next.ID, second.CandidateHash, actor)
	if _, e = m.Promote(next.ID); e != nil {
		t.Fatal(e)
	}
	if e = m.Prune(ctx, j.Bundle, first.CandidateHash, actor); e == nil {
		t.Fatal("missing authority treated as no grants")
	}
	dir, e := os.MkdirTemp("/tmp", "fpins-")
	if e != nil {
		t.Fatal(e)
	}
	defer os.RemoveAll(dir)
	socket := filepath.Join(dir, "pins.sock")
	listener, e := net.Listen("unix", socket)
	if e != nil {
		t.Fatal(e)
	}
	var pinned atomic.Bool
	pinned.Store(true)
	server := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/releases/pins" {
			t.Error("wrong authority route")
		}
		if pinned.Load() {
			json.NewEncoder(w).Encode([]map[string]string{{"bundle": j.Bundle, "digest": first.CandidateHash}})
		} else {
			w.Write([]byte(`[]`))
		}
	})}
	go server.Serve(listener)
	defer server.Close()
	m.Config.ControlSocket = socket
	if e = m.Prune(ctx, j.Bundle, first.CandidateHash, actor); e == nil {
		t.Fatal("assigned retained release pruned")
	}
	pinned.Store(false)
	if e = m.Prune(ctx, j.Bundle, first.CandidateHash, actor); e != nil {
		t.Fatal(e)
	}
	if _, e = os.Stat(m.release(j.Bundle, first.CandidateHash)); !os.IsNotExist(e) {
		t.Fatal("pruned bytes retained", e)
	}
	s, e := m.Status(j.ID)
	if e != nil || s.State != "pruned" {
		t.Fatal("prune outcome not durable")
	}
	if _, e = m.Assignable(j.Bundle, second.CandidateHash); e != nil {
		t.Fatal("prune harmed current release")
	}
}

func TestRealValidationAndProjectionFailuresDoNotChangeManifest(t *testing.T) {
	m := realManager(t)
	ctx := context.Background()
	j := fixtureJob()
	input := realInput(t, &j)
	if _, e := m.Prepare(ctx, j, j.Hash(), input); e != nil {
		t.Fatal(e)
	}
	provider := filepath.Join(m.payload(j.ID), "provider")
	serving := filepath.Join(provider, "serving")
	manifestPath := filepath.Join(serving, "manifest.json")
	before, _, _ := fileHash(manifestPath)
	metadataPath := filepath.Join(m.stage(j.ID), "reports", "daily.metadata.json")
	metadataBytes, e := os.ReadFile(metadataPath)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = m.feam(ctx, provider, "validate-metadata", filepath.Join(m.stage(j.ID), "missing.json")); e == nil {
		t.Fatal("missing metadata accepted")
	}
	var metadata map[string]any
	json.Unmarshal(metadataBytes, &metadata)
	metadata["namespace"] = "foreign"
	b, _ := json.Marshal(metadata)
	os.WriteFile(metadataPath, b, 0600)
	if _, e = m.feam(ctx, provider, "validate-metadata", metadataPath); e == nil {
		t.Fatal("namespace conflict accepted")
	}
	os.WriteFile(metadataPath, metadataBytes, 0600)
	asset := filepath.Join(serving, "datasets/daily/v1/observations.parquet")
	original, e := os.ReadFile(asset)
	if e != nil {
		t.Fatal(e)
	}
	os.WriteFile(asset, []byte("CSV renamed as Parquet"), 0600)
	if _, e = m.feam(ctx, provider, "validate-metadata", metadataPath); e == nil {
		t.Fatal("renamed corrupt parquet accepted")
	}
	os.WriteFile(asset, original, 0600)
	other := filepath.Join(serving, "datasets/daily/v1/incompatible.parquet")
	if _, e = run(ctx, m.Config.Python, "-I", "-c", "import polars as pl,sys; pl.DataFrame({'unrelated':[1]}).write_parquet(sys.argv[1])", other); e != nil {
		t.Fatal(e)
	}
	json.Unmarshal(metadataBytes, &metadata)
	metadata["assets"] = append(metadata["assets"].([]any), map[string]any{"id": "other", "path": "datasets/daily/v1/incompatible.parquet", "role": "data", "media_type": "application/vnd.apache.parquet"})
	b, _ = json.Marshal(metadata)
	os.WriteFile(metadataPath, b, 0600)
	if _, e = m.feam(ctx, provider, "validate-metadata", metadataPath); e == nil {
		t.Fatal("incompatible parquet shards accepted")
	}
	os.Remove(other)
	os.WriteFile(metadataPath, metadataBytes, 0600)
	// Inspectors are compiled into core. Missing Python native dependencies must
	// also fail explicitly rather than bypassing the conversion validation.
	if _, e = run(ctx, m.Config.Python, "-I", "-S", m.Config.Converter, "--job", filepath.Join(m.stage(j.ID), "job.json"), "--input-dir", filepath.Join(m.stage(j.ID), "raw"), "--output-dir", filepath.Join(m.stage(j.ID), "missing-inspector-output"), "--reports-dir", filepath.Join(m.stage(j.ID), "missing-inspector-reports")); e == nil {
		t.Fatal("missing native reader packages silently accepted")
	}
	blocked := filepath.Join(m.stage(j.ID), "blocked-cache")
	os.WriteFile(blocked, []byte("not a directory"), 0600)
	command := exec.CommandContext(ctx, m.Config.FEAM, "--project", provider, "--format", "json", "refresh")
	command.Env = []string{"PATH=/usr/bin:/bin", "FEAM_CACHE_DIR=" + blocked}
	if e = command.Run(); e == nil {
		t.Fatal("failed cache projection reported success")
	}
	after, _, _ := fileHash(manifestPath)
	if before != after {
		t.Fatal("validation/projection error changed authoritative manifest")
	}
	if _, e = m.feam(ctx, provider, "resolve", "product://"+j.Namespace+"/daily", "--version", "v1", "--verify-integrity"); e != nil {
		t.Fatal("projection failure hid committed direct inventory", e)
	}
}
