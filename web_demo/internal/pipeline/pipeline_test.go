package pipeline

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"net/netip"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func fixtureJob() Job {
	return Job{Protocol: Protocol, ID: "00000000-0000-4000-8000-000000000001", Bundle: "demo-climate", Namespace: "demo-climate", ImporterVersion: "1",
		Sources:  []Source{{ID: "eccc", Importer: "eccc-csv", URL: "https://climate.weather.gc.ca/climate_data/bulk_data_e.html?format=csv&stationID=49568&Year=2023&timeframe=2", SHA256: strings.Repeat("a", 64), MaxBytes: MiB, UpstreamVersion: "synthetic", RetrievedAt: "2026-09-25T00:00:00Z", License: "https://open.canada.ca/en/open-government-licence-canada", Attribution: "Synthetic ECCC-shaped test data", Station: "6106001", StartDate: "2023-01-01", EndDate: "2023-01-02", ScientificTime: ScientificTime{Kind: "dates-in-source", Evidence: "synthetic Date/Time field"}, BBox: []float64{}, Resampling: "none", Variable: "daily temperature and precipitation", Unit: "source columns"}},
		Products: []Product{{SourceID: "eccc", ID: "daily", Version: "v1", Name: "Synthetic daily observations", Description: "Synthetic data", IntendedUse: "tests", Limitations: "synthetic", Producer: "Feather Mesh tests", Contact: "test@example.test", AssetID: "observations"}},
		Limits:   Limits{SourceBytes: MiB, OutputBytes: MiB, CandidateBytes: 4 * MiB, ElapsedSeconds: 30, Redirects: 1}, Policy: Policy{Audience: "invited-users", ModelMetadata: true, ResearchEligible: true}}
}

func managerFixture(t *testing.T) *Manager {
	t.Helper()
	root := filepath.Join(t.TempDir(), "datasets")
	c := DefaultConfig(root, "/usr/bin/false", "/usr/bin/false", "/opt/converter.py")
	c.MinimumFree = 0
	m, err := Initialize(c)
	if err != nil {
		t.Fatal(err)
	}
	m.headroomPercent = 0 // Tiny local fixtures do not assert host storage acceptance.
	t.Cleanup(func() { m.DB.Close() })
	return m
}

func TestClosedJobAndExactApproval(t *testing.T) {
	j := fixtureJob()
	data, _ := json.Marshal(j)
	if _, err := DecodeJob(data); err != nil {
		t.Fatal(err)
	}
	for name, modify := range map[string]func(map[string]any){
		"command": func(v map[string]any) { v["command"] = "sh" }, "missing explicit policy": func(v map[string]any) { delete(v["policy"].(map[string]any), "model_payload") },
		"payload disclosure": func(v map[string]any) { v["policy"].(map[string]any)["model_payload"] = true }, "archives": func(v map[string]any) { v["limits"].(map[string]any)["archive_entries"] = 1 },
		"namespace": func(v map[string]any) { v["namespace"] = "practice" }, "source traversal": func(v map[string]any) { v["sources"].([]any)[0].(map[string]any)["id"] = "../raw" },
		"unbounded bytes": func(v map[string]any) { v["limits"].(map[string]any)["source_bytes"] = 1e12 }, "unknown source": func(v map[string]any) {
			v["sources"].([]any)[0].(map[string]any)["url"] = "https://169.254.169.254/latest/meta-data/"
		},
	} {
		t.Run(name, func(t *testing.T) {
			var v map[string]any
			json.Unmarshal(data, &v)
			modify(v)
			changed, _ := json.Marshal(v)
			if _, err := DecodeJob(changed); err == nil {
				t.Fatal("accepted invalid job")
			}
		})
	}
	if _, err := DecodeJob(append(data[:len(data)-1], []byte(`,"protocol":"feam.pipeline.v1"}`)...)); err == nil {
		t.Fatal("duplicate key accepted")
	}
	m := managerFixture(t)
	if err := m.reserve(context.Background(), j, strings.Repeat("0", 64)); err == nil {
		t.Fatal("unapproved job reserved")
	}
	if err := m.reserve(context.Background(), j, j.Hash()); err != nil {
		t.Fatal(err)
	}
	if err := m.reserve(context.Background(), j, j.Hash()); !errors.Is(err, ErrReconcile) {
		t.Fatalf("replay must reconcile: %v", err)
	}
}

func TestFetchDestinationAndRedirectPolicy(t *testing.T) {
	for _, raw := range []string{"127.0.0.1", "::1", "10.0.0.1", "172.16.0.1", "192.168.1.1", "169.254.169.254", "100.100.100.200", "::ffff:127.0.0.1", "0.0.0.1", "198.18.0.1", "192.0.2.1", "2001:db8::1", "64:ff9b::a00:1", "ff02::1"} {
		if PublicAddress(netip.MustParseAddr(raw)) {
			t.Errorf("accepted %s", raw)
		}
	}
	if !PublicAddress(netip.MustParseAddr("1.1.1.1")) {
		t.Fatal("public address rejected")
	}
	s := fixtureJob().Sources[0]
	c := sourceClient(s, fixtureJob().Limits)
	for _, raw := range []string{"http://climate.weather.gc.ca/climate_data/bulk_data_e.html", "https://climate.weather.gc.ca.evil.test/climate_data/bulk_data_e.html", "https://climate.weather.gc.ca:8443/climate_data/bulk_data_e.html", "https://user:secret@climate.weather.gc.ca/climate_data/bulk_data_e.html", "https://climate.weather.gc.ca/climate_data/../bulk_data_e.html", "https://climate.weather.gc.ca/climate_data/%2e%2e/bulk_data_e.html", "https://localhost/"} {
		u, _ := url.Parse(raw)
		if c.CheckRedirect(&http.Request{URL: u}, []*http.Request{{}}) == nil {
			t.Errorf("accepted %s", raw)
		}
	}
	u, _ := url.Parse(s.URL)
	if c.CheckRedirect(&http.Request{URL: u}, []*http.Request{{}, {}}) == nil {
		t.Fatal("redirect count not enforced")
	}
	if c.Transport.(*http.Transport).Proxy != nil {
		t.Fatal("environment proxy can bypass source policy")
	}
}

func TestDownloadNeverLeavesPartialOrOverwrites(t *testing.T) {
	data := []byte("known bytes")
	h := sha256.Sum256(data)
	expected := hex.EncodeToString(h[:])
	dest := filepath.Join(t.TempDir(), "raw")
	for _, test := range []struct {
		data  []byte
		limit int64
	}{{data[:3], 100}, {data, 3}, {[]byte("wrong"), 100}} {
		if err := writePinned(bytes.NewReader(test.data), dest, expected, test.limit); err == nil {
			t.Fatal("invalid source accepted")
		}
		if _, err := os.Stat(dest); !os.IsNotExist(err) {
			t.Fatal("partial artifact exposed")
		}
	}
	if err := writePinned(bytes.NewReader(data), dest, expected, 100); err != nil {
		t.Fatal(err)
	}
	if err := writePinned(bytes.NewReader(data), dest, expected, 100); err == nil {
		t.Fatal("pinned source overwritten")
	}
	files, _ := os.ReadDir(filepath.Dir(dest))
	if len(files) != 1 {
		t.Fatal("download scratch leaked")
	}
}

func TestBundleSerializationAndFullReservation(t *testing.T) {
	m := managerFixture(t)
	j := fixtureJob()
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			candidate := j
			candidate.ID = []string{"00000000-0000-4000-8000-000000000001", "00000000-0000-4000-8000-000000000002"}[i]
			results <- m.reserve(context.Background(), candidate, candidate.Hash())
		}(i)
	}
	wg.Wait()
	close(results)
	success := 0
	for err := range results {
		if err == nil {
			success++
		}
	}
	if success != 1 {
		t.Fatalf("concurrent bundle writers admitted: %d", success)
	}
	var reserved int64
	m.DB.QueryRow(`SELECT reserved_bytes FROM dataset_jobs`).Scan(&reserved)
	if reserved != j.Limits.CandidateBytes+j.Limits.SourceBytes+MiB {
		t.Fatal("incomplete reservation")
	}
	m2 := managerFixture(t)
	m2.Config.StagingBudget = j.Limits.CandidateBytes
	if err := m2.reserve(context.Background(), j, j.Hash()); err == nil {
		t.Fatal("raw bytes not reserved")
	}
}

func TestTreeIntegrityCopiesAndSymlinks(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source")
	os.Mkdir(source, 0700)
	os.WriteFile(filepath.Join(source, "asset"), []byte("old"), 0600)
	before, _, err := TreeHash(source)
	if err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(root, "copy")
	if err = copyTree(source, dest, 100); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(dest, "asset"), []byte("new"), 0600)
	after, _, _ := TreeHash(source)
	if before != after {
		t.Fatal("candidate shares released inode")
	}
	os.Symlink(filepath.Join(source, "asset"), filepath.Join(source, "alias"))
	if _, _, err = TreeHash(source); err == nil {
		t.Fatal("release link accepted")
	}
	if err = copyTree(source, filepath.Join(root, "badcopy"), 100); err == nil {
		t.Fatal("parent link copied")
	}
}

func TestUnknownNeverBlindlyRetriesOrPromotes(t *testing.T) {
	m := managerFixture(t)
	j := fixtureJob()
	status, err := m.Prepare(context.Background(), j, j.Hash(), t.TempDir())
	if err == nil || status.State != "unknown" {
		t.Fatalf("failure not held for reconciliation: %+v %v", status, err)
	}
	if _, err = m.Prepare(context.Background(), j, j.Hash(), t.TempDir()); !errors.Is(err, ErrReconcile) {
		t.Fatal("unknown job replay accepted")
	}
	if _, err = m.Reconcile(context.Background(), j.ID); !errors.Is(err, ErrReconcile) {
		t.Fatal("missing commit declared successful")
	}
	if _, err = m.Promote(j.ID); err == nil {
		t.Fatal("unapproved unknown promoted")
	}
	var n int
	m.DB.QueryRow(`SELECT count(*) FROM releases`).Scan(&n)
	if n != 0 {
		t.Fatal("failed candidate exposed")
	}
}

func TestStartupDoesNotInitialize(t *testing.T) {
	c := DefaultConfig(filepath.Join(t.TempDir(), "missing"), "/bin/false", "/bin/false", "/opt/converter")
	if _, err := Open(c); err == nil {
		t.Fatal("missing state initialized")
	}
	if _, err := os.Stat(c.Root); !os.IsNotExist(err) {
		t.Fatal("startup wrote state")
	}
}
