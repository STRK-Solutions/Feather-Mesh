package pipeline

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/STRK-Solutions/Feather-Mesh/web_demo/internal/db"
)

var Migrations = []db.Migration{{Version: 1, SQL: `
CREATE TABLE dataset_jobs (
 id TEXT PRIMARY KEY, bundle TEXT NOT NULL, spec BLOB NOT NULL, job_hash TEXT NOT NULL,
 parent_digest TEXT NOT NULL, state TEXT NOT NULL, reserved_bytes INTEGER NOT NULL CHECK(reserved_bytes>0),
 candidate_hash TEXT NOT NULL DEFAULT '', approval_hash TEXT NOT NULL DEFAULT '', approved_by TEXT NOT NULL DEFAULT '',
 manifest_outcome TEXT NOT NULL DEFAULT 'not_attempted', promotion_outcome TEXT NOT NULL DEFAULT 'not_attempted',
 created_at TEXT NOT NULL, updated_at TEXT NOT NULL
);
CREATE UNIQUE INDEX one_active_bundle_job ON dataset_jobs(bundle) WHERE state NOT IN ('released','aborted');
CREATE TABLE releases (digest TEXT PRIMARY KEY, bundle TEXT NOT NULL, job_id TEXT UNIQUE NOT NULL REFERENCES dataset_jobs(id),
 bytes INTEGER NOT NULL CHECK(bytes>0), revision INTEGER NOT NULL, status TEXT NOT NULL CHECK(status IN ('current','retained','revoked')),
 created_at TEXT NOT NULL);
CREATE UNIQUE INDEX one_current_bundle_release ON releases(bundle) WHERE status='current';
CREATE TABLE bundle_floors (bundle TEXT PRIMARY KEY, revision INTEGER NOT NULL CHECK(revision>=0));
CREATE TABLE pipeline_audit (id INTEGER PRIMARY KEY, job_id TEXT NOT NULL, action TEXT NOT NULL, actor TEXT NOT NULL, hash TEXT NOT NULL, at TEXT NOT NULL);
`}, {Version: 2, SQL: `
DROP INDEX one_active_bundle_job;
CREATE UNIQUE INDEX one_active_bundle_job ON dataset_jobs(bundle) WHERE state NOT IN ('released','aborted','pruned');
`}}

// Runtime paths are operator configuration, never fields accepted from a web job.
type Config struct {
	Root           string
	FEAM           string
	Python         string
	Converter      string
	StagingBudget  int64
	RetainedBudget int64
	MinimumFree    int64
	WebDemoACL     bool
	ControlSocket  string
}

func DefaultConfig(root, feam, python, converter string) Config {
	return Config{Root: root, FEAM: feam, Python: python, Converter: converter, StagingBudget: 50 * 1024 * MiB, RetainedBudget: 100 * 1024 * MiB, MinimumFree: 20 * 1024 * MiB}
}

type Manager struct {
	DB              *sql.DB
	Config          Config
	headroomPercent int64
}
type Status struct {
	ID               string `json:"id"`
	Bundle           string `json:"bundle"`
	State            string `json:"state"`
	JobHash          string `json:"job_hash"`
	CandidateHash    string `json:"candidate_hash"`
	ApprovalHash     string `json:"approval_hash"`
	ManifestOutcome  string `json:"manifest_outcome"`
	PromotionOutcome string `json:"promotion_outcome"`
}

func Initialize(c Config) (*Manager, error) {
	if err := validateConfig(c); err != nil {
		return nil, err
	}
	if err := os.Mkdir(c.Root, 0700); err != nil {
		return nil, err
	}
	for _, name := range []string{"staging", "releases", "locks"} {
		if err := os.Mkdir(filepath.Join(c.Root, name), 0700); err != nil {
			return nil, err
		}
	}
	d, err := db.Initialize(filepath.Join(c.Root, "pipeline.db"), Migrations)
	if err != nil {
		return nil, err
	}
	return &Manager{DB: d, Config: c, headroomPercent: 15}, nil
}

func Open(c Config) (*Manager, error) {
	return open(c, false)
}
func Migrate(c Config) (*Manager, error) { return open(c, true) }
func open(c Config, migrate bool) (*Manager, error) {
	if err := validateConfig(c); err != nil {
		return nil, err
	}
	for _, dir := range []string{c.Root, filepath.Join(c.Root, "staging"), filepath.Join(c.Root, "releases"), filepath.Join(c.Root, "locks")} {
		info, err := os.Lstat(dir)
		if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 || info.Mode().Perm()&0007 != 0 {
			return nil, errors.New("pipeline directories must be existing private directories")
		}
	}
	var d *sql.DB
	var err error
	if migrate {
		d, err = db.Migrate(filepath.Join(c.Root, "pipeline.db"), Migrations)
	} else {
		d, err = db.Open(filepath.Join(c.Root, "pipeline.db"), Migrations)
	}
	if err != nil {
		return nil, err
	}
	return &Manager{DB: d, Config: c, headroomPercent: 15}, nil
}

func validateConfig(c Config) error {
	if c.ControlSocket != "" && (!filepath.IsAbs(c.ControlSocket) || filepath.Clean(c.ControlSocket) != c.ControlSocket) {
		return errors.New("clean absolute authority socket required")
	}
	for _, p := range []string{c.Root, c.FEAM, c.Python, c.Converter} {
		if !filepath.IsAbs(p) || filepath.Clean(p) != p {
			return errors.New("pipeline config requires clean absolute paths")
		}
	}
	if c.StagingBudget < 1 || c.RetainedBudget < 1 || c.MinimumFree < 0 {
		return errors.New("invalid storage policy")
	}
	return nil
}

func (m *Manager) Status(id string) (Status, error) {
	var s Status
	err := m.DB.QueryRow(`SELECT id,bundle,state,job_hash,candidate_hash,approval_hash,manifest_outcome,promotion_outcome FROM dataset_jobs WHERE id=?`, id).Scan(&s.ID, &s.Bundle, &s.State, &s.JobHash, &s.CandidateHash, &s.ApprovalHash, &s.ManifestOutcome, &s.PromotionOutcome)
	return s, err
}

func (m *Manager) job(id string) (Job, error) {
	var raw []byte
	if err := m.DB.QueryRow(`SELECT spec FROM dataset_jobs WHERE id=?`, id).Scan(&raw); err != nil {
		return Job{}, err
	}
	return DecodeJob(raw)
}

func (m *Manager) stage(id string) string   { return filepath.Join(m.Config.Root, "staging", id) }
func (m *Manager) payload(id string) string { return filepath.Join(m.stage(id), "payload") }
func (m *Manager) release(bundle, hash string) string {
	return filepath.Join(m.Config.Root, "releases", bundle, hash)
}

func (m *Manager) reserve(ctx context.Context, j Job, approved string, actors ...string) error {
	if err := j.Validate(); err != nil {
		return err
	}
	if j.Hash() != approved {
		return errors.New("job approval hash mismatch")
	}
	if j.Withdrawal != nil {
		if err := m.validateWithdrawalParent(j); err != nil {
			return err
		}
	}
	// A complete candidate plus every raw input and metadata overhead is reserved.
	reservation := j.Limits.CandidateBytes + MiB
	for _, source := range j.Sources {
		reservation += source.MaxBytes
	}
	tx, err := m.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var existingHash string
	if err = tx.QueryRow(`SELECT job_hash FROM dataset_jobs WHERE id=?`, j.ID).Scan(&existingHash); err == nil {
		if existingHash != approved {
			return errors.New("job ID conflicts with approved hash")
		}
		return ErrReconcile
	} else if !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	var staging, retained int64
	if err = tx.QueryRow(`SELECT coalesce(sum(reserved_bytes),0) FROM dataset_jobs WHERE state NOT IN ('released','aborted','pruned')`).Scan(&staging); err != nil {
		return err
	}
	if err = tx.QueryRow(`SELECT coalesce(sum(bytes),0) FROM releases`).Scan(&retained); err != nil {
		return err
	}
	if staging+reservation > m.Config.StagingBudget || retained+staging+j.Limits.CandidateBytes > m.Config.RetainedBudget {
		return errors.New("complete candidate exceeds storage budget")
	}
	var stat syscall.Statfs_t
	if err = syscall.Statfs(m.Config.Root, &stat); err != nil {
		return err
	}
	free := int64(stat.Bavail) * int64(stat.Bsize)
	headroom := int64(stat.Blocks) * int64(stat.Bsize) * m.headroomPercent / 100
	if headroom < m.Config.MinimumFree {
		headroom = m.Config.MinimumFree
	}
	if free-reservation-staging < headroom {
		return errors.New("dataset filesystem headroom")
	}
	var parent string
	err = tx.QueryRow(`SELECT digest FROM releases WHERE bundle=? AND status='current'`, j.Bundle).Scan(&parent)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	if parent != j.ParentDigest {
		return errors.New("stale parent release")
	}
	var floor int64
	err = tx.QueryRow(`SELECT revision FROM bundle_floors WHERE bundle=?`, j.Bundle).Scan(&floor)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	if floor > 0 && parent == "" {
		return errors.New("withdrawal floor requires current successor")
	}
	raw, _ := json.Marshal(j)
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err = tx.Exec(`INSERT INTO dataset_jobs(id,bundle,spec,job_hash,parent_digest,state,reserved_bytes,created_at,updated_at) VALUES(?,?,?,?,?,'queued',?,?,?)`, j.ID, j.Bundle, raw, approved, j.ParentDigest, reservation, now, now)
	if err != nil {
		return errors.New("bundle is busy or job already exists")
	}
	if len(actors) > 0 {
		if _, err = tx.Exec(`INSERT INTO pipeline_audit(job_id,action,actor,hash,at) VALUES(?,'enqueue',?,?,?)`, j.ID, actors[0], approved, now); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// Prepare is an operator-only synchronous worker entrypoint. The durable job is
// recorded before any effects. IPC enqueue returns this ID immediately.
// inbox contains exactly source-ID files; their bytes are rehashed into private staging.
func (m *Manager) Prepare(ctx context.Context, j Job, approved, inbox string) (Status, error) {
	unlock, err := m.lock(j.ID)
	if err != nil {
		return Status{}, err
	}
	defer unlock()
	if err := m.reserve(ctx, j, approved); err != nil {
		return Status{}, err
	}
	return m.executeQueued(ctx, j, inbox)
}

func (m *Manager) executeQueued(ctx context.Context, j Job, inbox string) (Status, error) {
	r, err := m.DB.ExecContext(ctx, `UPDATE dataset_jobs SET state='preparing' WHERE id=? AND state='queued'`, j.ID)
	if err != nil {
		return Status{}, err
	}
	n, _ := r.RowsAffected()
	if n != 1 {
		return Status{}, ErrReconcile
	}
	if err := m.prepare(ctx, j, inbox); err != nil {
		// Unknown is deliberately sticky. Repeated Prepare never invokes serve again.
		_, _ = m.DB.Exec(`UPDATE dataset_jobs SET state='unknown',updated_at=? WHERE id=?`, time.Now().UTC().Format(time.RFC3339Nano), j.ID)
		s, _ := m.Status(j.ID)
		return s, err
	}
	return m.Status(j.ID)
}

func (m *Manager) prepare(ctx context.Context, j Job, inbox string) error {
	ctx, cancel := context.WithTimeout(ctx, time.Duration(j.Limits.ElapsedSeconds)*time.Second)
	defer cancel()
	stage := m.stage(j.ID)
	payload := m.payload(j.ID)
	provider := filepath.Join(payload, "provider")
	reports := filepath.Join(stage, "reports")
	raw := filepath.Join(stage, "raw")
	for _, dir := range []string{stage, payload, reports, raw} {
		if err := os.Mkdir(dir, 0700); err != nil {
			return err
		}
	}
	if j.ParentDigest != "" {
		parent := m.release(j.Bundle, j.ParentDigest)
		if hash, _, err := TreeHash(parent); err != nil || hash != j.ParentDigest {
			return errors.New("parent integrity mismatch")
		}
		if err := copyTree(parent, payload, j.Limits.CandidateBytes); err != nil {
			return err
		}
	} else {
		if err := os.Mkdir(provider, 0700); err != nil {
			return err
		}
		if _, err := m.feam(ctx, provider, "init", "--namespace", j.Namespace, "--serving-dir", "serving", "--owner-team", "Demo"); err != nil {
			return err
		}
	}
	if j.Withdrawal != nil {
		return m.prepareWithdrawal(ctx, j)
	}
	for _, source := range j.Sources {
		if inbox == "" {
			if err := Fetch(ctx, source, j.Limits, filepath.Join(raw, source.ID)); err != nil {
				return err
			}
			hash, n, err := fileHash(filepath.Join(raw, source.ID))
			if err != nil {
				return err
			}
			receipt, _ := json.Marshal(map[string]any{"source_id": source.ID, "source_url": source.URL, "sha256": hash, "bytes": n, "downloaded_at": time.Now().UTC().Format(time.RFC3339Nano)})
			if err = writeFile(filepath.Join(reports, source.ID+".download.json"), receipt); err != nil {
				return err
			}
			continue
		}
		p := filepath.Join(inbox, source.ID)
		info, err := os.Lstat(p)
		if err != nil || !info.Mode().IsRegular() {
			return errors.New("source must be a regular private file")
		}
		f, err := os.Open(p)
		if err != nil {
			return err
		}
		err = writePinned(f, filepath.Join(raw, source.ID), source.SHA256, source.MaxBytes)
		f.Close()
		if err != nil {
			return err
		}
	}
	jobBytes, _ := json.Marshal(j)
	jobPath := filepath.Join(stage, "job.json")
	if err := writeFile(jobPath, jobBytes); err != nil {
		return err
	}
	if _, err := run(ctx, m.Config.Python, "-I", m.Config.Converter, "--job", jobPath, "--input-dir", raw, "--output-dir", filepath.Join(provider, "serving"), "--reports-dir", reports); err != nil {
		return errors.New("bounded offline conversion failed; inspect private provenance")
	}
	if _, n, err := TreeHash(payload); err != nil || n > j.Limits.CandidateBytes {
		return errors.New("candidate byte limit")
	}
	// Validate the complete incoming batch before publishing any of its products.
	for _, product := range j.Products {
		if _, err := m.feam(ctx, provider, "validate-metadata", filepath.Join(reports, product.ID+".metadata.json")); err != nil {
			return errors.New("FEAM candidate validation failed")
		}
	}
	for _, product := range j.Products {
		if _, err := m.DB.Exec(`UPDATE dataset_jobs SET state='publishing',manifest_outcome='unknown' WHERE id=?`, j.ID); err != nil {
			return err
		}
		output, err := m.feam(ctx, provider, "serve", filepath.Join(provider, "serving"), "--metadata", filepath.Join(reports, product.ID+".metadata.json"))
		if err != nil {
			return ErrReconcile
		}
		var result struct {
			Protocol string `json:"protocol"`
			Status   string `json:"status"`
		}
		if json.Unmarshal(output, &result) != nil || result.Protocol != "feam.peer.v1" || result.Status != "published" {
			return ErrReconcile
		}
	}
	return m.finishCandidate(ctx, j)
}

func (m *Manager) feam(ctx context.Context, project string, args ...string) ([]byte, error) {
	return run(ctx, m.Config.FEAM, append([]string{"--project", project, "--format", "json"}, args...)...)
}

type boundedOutput struct {
	data []byte
	max  int
}

func (b *boundedOutput) Write(p []byte) (int, error) {
	n := len(p)
	remaining := b.max - len(b.data)
	if remaining > 0 {
		if len(p) > remaining {
			p = p[:remaining]
		}
		b.data = append(b.data, p...)
	}
	return n, nil
}
func run(ctx context.Context, executable string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, executable, args...)
	// No cloud keys, proxy variables, GDAL network configuration or user Python path.
	cmd.Env = []string{"PATH=/usr/local/bin:/usr/bin:/bin", "LANG=C.UTF-8", "TZ=UTC", "GDAL_DISABLE_READDIR_ON_OPEN=EMPTY_DIR", "GDAL_PAM_ENABLED=NO", "PROJ_NETWORK=OFF"}
	cmd.Dir = os.TempDir()
	cmd.WaitDelay = 2 * time.Second
	out := &boundedOutput{max: 1024 * 1024}
	errout := &boundedOutput{max: 8192}
	cmd.Stdout = out
	cmd.Stderr = errout
	err := cmd.Run()
	if err != nil {
		return out.data, errors.New("pipeline child operation failed")
	}
	return out.data, nil
}

type manifest struct {
	Namespace string `json:"namespace"`
	Revision  int64  `json:"revision"`
	Products  []struct {
		ID       string `json:"id"`
		Versions []struct {
			Version   string `json:"version"`
			Lifecycle string `json:"lifecycle"`
			Assets    []struct {
				ID     string `json:"id"`
				Path   string `json:"path"`
				SHA256 string `json:"sha256"`
				Size   int64  `json:"size"`
			} `json:"assets"`
		} `json:"versions"`
	} `json:"products"`
}

func (m *Manager) verifyCandidate(j Job) (manifest, error) {
	var result manifest
	provider := filepath.Join(m.payload(j.ID), "provider")
	serving := filepath.Join(provider, "serving")
	data, err := readBounded(filepath.Join(serving, "manifest.json"), 16*MiB)
	if err != nil {
		return result, err
	}
	if json.Unmarshal(data, &result) != nil || result.Namespace != j.Namespace || result.Revision < 1 {
		return result, errors.New("invalid FEAM manifest")
	}
	allowed := map[string]bool{"manifest.json": true}
	found := map[string]bool{}
	for _, product := range result.Products {
		for _, version := range product.Versions {
			for _, asset := range version.Assets {
				if asset.Path == "" || filepath.IsAbs(asset.Path) || filepath.ToSlash(filepath.Clean(asset.Path)) != asset.Path || strings.HasPrefix(asset.Path, "../") || strings.Contains(asset.Path, "\\") {
					return result, errors.New("unsafe registered path")
				}
				allowed[asset.Path] = true
				if !digestPattern.MatchString(asset.SHA256) {
					return result, errors.New("release requires every registered asset digest")
				}
				hash, size, e := fileHash(filepath.Join(serving, filepath.FromSlash(asset.Path)))
				if e != nil || hash != asset.SHA256 || size != asset.Size {
					return result, errors.New("registered asset integrity mismatch")
				}
			}
			if j.Withdrawal != nil {
				for _, target := range j.Withdrawal.Targets {
					if target.ID == product.ID && target.Version == version.Version {
						if version.Lifecycle != "withdrawn" {
							return result, ErrReconcile
						}
						found[target.ID+"/"+target.Version] = true
					}
				}
				continue
			}
			for _, wanted := range j.Products {
				if wanted.ID == product.ID && wanted.Version == version.Version {
					if version.Lifecycle != "active" || len(version.Assets) != 1 || version.Assets[0].ID != wanted.AssetID {
						return result, errors.New("candidate version differs from job")
					}
					var report struct {
						Path   string `json:"output_path"`
						SHA256 string `json:"output_sha256"`
					}
					data, e := readBounded(filepath.Join(m.stage(j.ID), "reports", wanted.ID+".provenance.json"), 64*1024)
					if e != nil || json.Unmarshal(data, &report) != nil || report.Path != version.Assets[0].Path || report.SHA256 != version.Assets[0].SHA256 {
						return result, errors.New("manifest commit does not match prepared bytes")
					}
					found[wanted.ID] = true
				}
			}
		}
	}
	expected := len(j.Products)
	if j.Withdrawal != nil {
		expected = len(j.Withdrawal.Targets)
	}
	if len(found) != expected {
		return result, ErrReconcile
	}
	err = filepath.WalkDir(serving, func(p string, d os.DirEntry, e error) error {
		if e != nil {
			return e
		}
		if d.IsDir() {
			return nil
		}
		rel, e := filepath.Rel(serving, p)
		if e != nil {
			return e
		}
		if !allowed[filepath.ToSlash(rel)] || !d.Type().IsRegular() {
			return errors.New("unregistered file or link in serving tree")
		}
		return nil
	})
	return result, err
}

func (m *Manager) finishCandidate(ctx context.Context, j Job) error {
	if _, err := m.verifyCandidate(j); err != nil {
		return err
	}
	if j.Withdrawal != nil {
		return m.finishWithdrawal(j)
	}
	provider := filepath.Join(m.payload(j.ID), "provider")
	// Real resolver verifies the same exact inventory through FEAM; the importer
	// already round-trips native Polars/Rasterio outputs. Sandbox SDK is a later gate.
	for _, product := range j.Products {
		if _, err := m.feam(ctx, provider, "resolve", "product://"+j.Namespace+"/"+product.ID, "--version", product.Version, "--verify-integrity"); err != nil {
			return errors.New("FEAM consumer integrity check failed")
		}
	}
	provenance := filepath.Join(m.payload(j.ID), "provenance")
	if err := os.MkdirAll(provenance, 0700); err != nil {
		return err
	}
	for _, product := range j.Products {
		b, err := readBounded(filepath.Join(m.stage(j.ID), "reports", product.ID+".provenance.json"), 64*1024)
		if err != nil {
			return err
		}
		dest := filepath.Join(provenance, j.ID+"-"+product.ID+".json")
		if _, err = os.Stat(dest); errors.Is(err, os.ErrNotExist) {
			if err = writeFile(dest, b); err != nil {
				return err
			}
		}
	}
	for _, source := range j.Sources {
		b, err := readBounded(filepath.Join(m.stage(j.ID), "reports", source.ID+".download.json"), 64*1024)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return err
		}
		dest := filepath.Join(provenance, j.ID+"-"+source.ID+"-download.json")
		if _, err = os.Stat(dest); errors.Is(err, os.ErrNotExist) {
			if err = writeFile(dest, b); err != nil {
				return err
			}
		}
	}
	hash, size, err := TreeHash(m.payload(j.ID))
	if err != nil {
		return err
	}
	if size > j.Limits.CandidateBytes {
		return errors.New("complete prepared release exceeds reservation")
	}
	_, err = m.DB.Exec(`UPDATE dataset_jobs SET state='prepared',candidate_hash=?,manifest_outcome='committed',updated_at=? WHERE id=? AND state IN ('publishing','unknown')`, hash, time.Now().UTC().Format(time.RFC3339Nano), j.ID)
	return err
}

func (m *Manager) Approve(id, hash, actor string) error {
	unlock, err := m.lock(id)
	if err != nil {
		return err
	}
	defer unlock()
	if !uuidPattern.MatchString(actor) || !digestPattern.MatchString(hash) {
		return errors.New("invalid approval")
	}
	s, err := m.Status(id)
	if err != nil {
		return err
	}
	if s.State != "prepared" || s.CandidateHash != hash {
		return errors.New("fresh exact candidate approval required")
	}
	actual, _, err := TreeHash(m.payload(id))
	if err != nil || actual != hash {
		return errors.New("candidate changed after review")
	}
	tx, err := m.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	r, err := tx.Exec(`UPDATE dataset_jobs SET state='approved',approval_hash=?,approved_by=? WHERE id=? AND state='prepared' AND candidate_hash=?`, hash, actor, id, hash)
	if err != nil {
		return err
	}
	n, _ := r.RowsAffected()
	if n != 1 {
		return errors.New("approval race")
	}
	_, err = tx.Exec(`INSERT INTO pipeline_audit(job_id,action,actor,hash,at) VALUES(?,'approve',?,?,?)`, id, actor, hash, time.Now().UTC().Format(time.RFC3339Nano))
	if err != nil {
		return err
	}
	return tx.Commit()
}

func (m *Manager) Promote(id string) (Status, error) {
	unlock, err := m.lock(id)
	if err != nil {
		return Status{}, err
	}
	defer unlock()
	j, err := m.job(id)
	if err != nil {
		return Status{}, err
	}
	s, err := m.Status(id)
	if err != nil {
		return s, err
	}
	if s.State != "approved" || s.ApprovalHash != s.CandidateHash {
		return s, errors.New("release is not approved")
	}
	if err = m.checkParent(j); err != nil {
		return s, err
	}
	hash, _, err := TreeHash(m.payload(id))
	if err != nil || hash != s.ApprovalHash {
		return s, errors.New("approved payload changed")
	}
	r, err := m.DB.Exec(`UPDATE dataset_jobs SET state='promoting',promotion_outcome='unknown' WHERE id=? AND state='approved'`, id)
	if err != nil {
		return s, err
	}
	n, _ := r.RowsAffected()
	if n != 1 {
		return s, ErrReconcile
	}
	parent := filepath.Join(m.Config.Root, "releases", j.Bundle)
	if err = os.MkdirAll(parent, 0700); err != nil {
		return s, err
	}
	dest := m.release(j.Bundle, hash)
	if _, err = os.Lstat(dest); !errors.Is(err, os.ErrNotExist) {
		return s, ErrReconcile
	}
	if err = sealTree(m.payload(id)); err != nil {
		return s, err
	}
	// EXDEV is a hard error: never degrade atomic promotion to a copy.
	if err = os.Rename(m.payload(id), dest); err != nil {
		return s, err
	}
	if err = os.Chmod(dest, 0500); err != nil {
		return s, ErrReconcile
	}
	if err = syncDir(parent); err != nil {
		return s, ErrReconcile
	}
	if err = syncDir(m.stage(id)); err != nil {
		return s, ErrReconcile
	}
	if err = m.recordRelease(j, hash); err != nil {
		return s, ErrReconcile
	}
	return m.Status(id)
}

func (m *Manager) checkParent(j Job) error {
	var current string
	err := m.DB.QueryRow(`SELECT digest FROM releases WHERE bundle=? AND status='current'`, j.Bundle).Scan(&current)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	if current != j.ParentDigest {
		return errors.New("parent changed; candidate requires new review")
	}
	return nil
}

func (m *Manager) recordRelease(j Job, hash string) error {
	root := m.release(j.Bundle, hash)
	actual, size, err := TreeHash(root)
	if err != nil || actual != hash {
		return errors.New("promoted release integrity mismatch")
	}
	if err = os.Chmod(root, 0500); err != nil {
		return err
	}
	if err = syncDir(root); err != nil {
		return err
	}
	var manifest manifest
	b, err := readBounded(filepath.Join(root, "provider", "serving", "manifest.json"), 16*MiB)
	if err != nil || json.Unmarshal(b, &manifest) != nil {
		return errors.New("missing committed manifest")
	}
	tx, err := m.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var current string
	err = tx.QueryRow(`SELECT digest FROM releases WHERE bundle=? AND status='current'`, j.Bundle).Scan(&current)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	if current != j.ParentDigest {
		return errors.New("stale parent during promotion")
	}
	var floor int64
	err = tx.QueryRow(`SELECT revision FROM bundle_floors WHERE bundle=?`, j.Bundle).Scan(&floor)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	if manifest.Revision < floor {
		return errors.New("release predates withdrawal floor")
	}
	if j.Withdrawal != nil {
		if _, err = tx.Exec(`INSERT INTO bundle_floors VALUES(?,?) ON CONFLICT(bundle) DO UPDATE SET revision=max(revision,excluded.revision)`, j.Bundle, manifest.Revision); err != nil {
			return err
		}
		var actor string
		if err = tx.QueryRow(`SELECT approved_by FROM dataset_jobs WHERE id=?`, j.ID).Scan(&actor); err != nil {
			return err
		}
		if err = m.revokeBundleGrants(j.Bundle, actor); err != nil {
			return err
		}
	}
	if m.Config.WebDemoACL {
		if err = m.shareServing(j.Bundle, hash); err != nil {
			return err
		}
	}
	if _, err = tx.Exec(`UPDATE releases SET status='retained' WHERE bundle=? AND status='current'`, j.Bundle); err != nil {
		return err
	}
	if _, err = tx.Exec(`INSERT INTO releases VALUES(?,?,?,?,?,'current',?)`, hash, j.Bundle, j.ID, size, manifest.Revision, time.Now().UTC().Format(time.RFC3339Nano)); err != nil {
		return err
	}
	if _, err = tx.Exec(`UPDATE dataset_jobs SET state='released',promotion_outcome='committed' WHERE id=? AND state='promoting'`, j.ID); err != nil {
		return err
	}
	return tx.Commit()
}

// Reconcile examines artifacts; it never invokes serve or retries a rename.
func (m *Manager) Reconcile(ctx context.Context, id string) (Status, error) {
	unlock, err := m.lock(id)
	if err != nil {
		return Status{}, err
	}
	defer unlock()
	j, err := m.job(id)
	if err != nil {
		return Status{}, err
	}
	s, err := m.Status(id)
	if err != nil {
		return s, err
	}
	switch s.State {
	case "preparing":
		_, err = m.DB.Exec(`UPDATE dataset_jobs SET state='unknown' WHERE id=? AND state='preparing'`, id)
		if err == nil {
			err = m.finishCandidate(ctx, j)
		}
	case "unknown", "publishing":
		err = m.finishCandidate(ctx, j)
	case "promoting":
		if s.ApprovalHash == "" || s.ApprovalHash != s.CandidateHash {
			return s, ErrReconcile
		}
		if _, e := os.Lstat(m.payload(id)); !errors.Is(e, os.ErrNotExist) {
			return s, ErrReconcile
		}
		err = m.recordRelease(j, s.ApprovalHash)
	case "released", "prepared", "approved":
		return s, nil
	default:
		return s, ErrReconcile
	}
	if err != nil {
		return s, ErrReconcile
	}
	return m.Status(id)
}

// Kernel locks serialize live workers and are released by the OS on crash.
// Database states still determine recovery; an unlocked operation is not proof
// that publication or promotion did not commit.
func (m *Manager) lock(id string) (func(), error) {
	if !uuidPattern.MatchString(id) {
		return nil, errors.New("invalid job UUID")
	}
	fd, err := syscall.Open(filepath.Join(m.Config.Root, "locks", id), syscall.O_CREAT|syscall.O_RDWR|syscall.O_NOFOLLOW, 0600)
	if err != nil {
		return nil, err
	}
	if err = syscall.Flock(fd, syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		syscall.Close(fd)
		return nil, errors.New("job worker is active")
	}
	return func() { syscall.Flock(fd, syscall.LOCK_UN); syscall.Close(fd) }, nil
}

// Assignable does not grant access. The gateway/controller must additionally
// verify current account grant/version and mount only this exact serving root.
func (m *Manager) Assignable(bundle, hash string) (string, error) {
	if !slug.MatchString(bundle) || !digestPattern.MatchString(hash) {
		return "", errors.New("invalid release identity")
	}
	var status string
	var revision, floor int64
	err := m.DB.QueryRow(`SELECT status,revision FROM releases WHERE bundle=? AND digest=?`, bundle, hash).Scan(&status, &revision)
	if err != nil || status != "current" {
		return "", errors.New("release is not current")
	}
	err = m.DB.QueryRow(`SELECT revision FROM bundle_floors WHERE bundle=?`, bundle).Scan(&floor)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return "", err
	}
	if revision < floor {
		return "", errors.New("release predates withdrawal")
	}
	root := m.release(bundle, hash)
	actual, _, err := TreeHash(root)
	if err != nil || actual != hash {
		return "", errors.New("release integrity mismatch")
	}
	return filepath.Join(root, "provider", "serving"), nil
}

// RevokeBundle is conservative: block every attachment and rollback immediately.
// A separately FEAM-withdrawn successor and a new grant are required to restore it.
func (m *Manager) RevokeBundle(bundle string, minimumRevision int64) error {
	if !slug.MatchString(bundle) || minimumRevision < 1 {
		return errors.New("invalid withdrawal floor")
	}
	tx, err := m.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	_, err = tx.Exec(`INSERT INTO bundle_floors VALUES(?,?) ON CONFLICT(bundle) DO UPDATE SET revision=max(revision,excluded.revision)`, bundle, minimumRevision)
	if err != nil {
		return err
	}
	_, err = tx.Exec(`UPDATE releases SET status='revoked' WHERE bundle=?`, bundle)
	if err != nil {
		return err
	}
	return tx.Commit()
}

func readBounded(path string, limit int64) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	b, err := io.ReadAll(io.LimitReader(f, limit+1))
	if int64(len(b)) > limit {
		return nil, errors.New("file exceeds bound")
	}
	return b, err
}
func fileHash(path string) (string, int64, error) {
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() {
		return "", 0, errors.New("payload includes nonregular file")
	}
	f, err := os.Open(path)
	if err != nil {
		return "", 0, err
	}
	defer f.Close()
	h := sha256.New()
	n, err := io.Copy(h, f)
	return hex.EncodeToString(h.Sum(nil)), n, err
}

// TreeHash includes exact names, byte counts and hashes, never inode/mtime values.
// It rejects symlinks, devices, sockets and empty trees. Call only on private trees.
func TreeHash(root string) (string, int64, error) {
	var entries []string
	var total int64
	err := filepath.WalkDir(root, func(p string, d os.DirEntry, e error) error {
		if e != nil {
			return e
		}
		if d.IsDir() {
			return nil
		}
		if !d.Type().IsRegular() {
			return errors.New("release contains nonregular file")
		}
		rel, e := filepath.Rel(root, p)
		if e != nil {
			return e
		}
		hash, n, e := fileHash(p)
		if e != nil {
			return e
		}
		total += n
		entries = append(entries, fmt.Sprintf("%s\x00%d\x00%s\n", filepath.ToSlash(rel), n, hash))
		return nil
	})
	if err != nil {
		return "", 0, err
	}
	if len(entries) == 0 {
		return "", 0, errors.New("empty payload")
	}
	sort.Strings(entries)
	h := sha256.Sum256([]byte(strings.Join(entries, "")))
	return hex.EncodeToString(h[:]), total, nil
}

func copyTree(source, dest string, limit int64) error {
	var total int64
	return filepath.WalkDir(source, func(p string, d os.DirEntry, e error) error {
		if e != nil {
			return e
		}
		rel, e := filepath.Rel(source, p)
		if e != nil {
			return e
		}
		target := filepath.Join(dest, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0700)
		}
		if !d.Type().IsRegular() {
			return errors.New("parent release contains a link or special file")
		}
		in, e := os.Open(p)
		if e != nil {
			return e
		}
		defer in.Close()
		out, e := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if e != nil {
			return e
		}
		n, e := io.Copy(out, io.LimitReader(in, limit-total+1))
		total += n
		if e == nil {
			e = out.Sync()
		}
		closeErr := out.Close()
		if e != nil {
			return e
		}
		if closeErr != nil {
			return closeErr
		}
		if total > limit {
			return errors.New("complete parent exceeds candidate reservation")
		}
		return nil
	})
}
func writeFile(path string, b []byte) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	if _, err = f.Write(b); err == nil {
		err = f.Sync()
	}
	e := f.Close()
	if err != nil {
		return err
	}
	if e != nil {
		return e
	}
	return syncDir(filepath.Dir(path))
}
func syncDir(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return f.Sync()
}
func sealTree(root string) error {
	return filepath.WalkDir(root, func(p string, d os.DirEntry, e error) error {
		if e != nil {
			return e
		}
		if d.IsDir() {
			if err := syncDir(p); err != nil {
				return err
			}
			// macOS requires the moved directory writable for rename. The payload
			// root is never mounted; seal it immediately after the atomic rename.
			if p == root {
				return nil
			}
			return os.Chmod(p, 0500)
		}
		if !d.Type().IsRegular() {
			return errors.New("cannot seal nonregular payload")
		}
		f, err := os.Open(p)
		if err != nil {
			return err
		}
		err = f.Sync()
		f.Close()
		if err != nil {
			return err
		}
		return os.Chmod(p, 0400)
	})
}
