package pipeline

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/STRK-Solutions/Feather-Mesh/web_demo/internal/ipc"
)

func (m *Manager) authority(ctx context.Context, method, path string, input, output any) error {
	if m.Config.ControlSocket == "" {
		return errors.New("live grant authority required")
	}
	b, err := json.Marshal(input)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, method, "http://control"+path, bytes.NewReader(b))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := ipc.Client(m.Config.ControlSocket).Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return errors.New("grant authority rejected operation")
	}
	d := json.NewDecoder(io.LimitReader(resp.Body, 1<<20))
	d.DisallowUnknownFields()
	if err = d.Decode(output); err != nil {
		return err
	}
	if d.Decode(new(any)) != io.EOF {
		return errors.New("invalid grant authority response")
	}
	return nil
}

func (m *Manager) revokeBundleGrants(bundle, actor string) error {
	// Standalone private operator projects have no mounted participant grants.
	if m.Config.ControlSocket == "" && !m.Config.WebDemoACL {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	var out struct {
		Status string `json:"status"`
	}
	err := m.authority(ctx, "POST", "/v1/grants/revoke-bundle", map[string]string{"bundle": bundle, "actor_id": actor}, &out)
	if err != nil {
		return err
	}
	if out.Status != "completed" {
		return errors.New("bundle grant revocation pending")
	}
	return nil
}

// Prune is an explicit administrator action for an exact retained release.
// Current/revoked/withdrawal-protected releases and uncertain parent jobs never
// qualify. Retained releases cannot acquire new grants, making the authoritative
// pin check safe from a new-assignment race. Partial deletions keep their budget.
func (m *Manager) Prune(ctx context.Context, bundle, digest, actor string) error {
	if !slug.MatchString(bundle) || !digestPattern.MatchString(digest) || !uuidPattern.MatchString(actor) {
		return errors.New("invalid prune identity")
	}
	var id, status string
	if err := m.DB.QueryRowContext(ctx, `SELECT job_id,status FROM releases WHERE bundle=? AND digest=?`, bundle, digest).Scan(&id, &status); err != nil {
		return err
	}
	unlock, err := m.lock(id)
	if err != nil {
		return err
	}
	defer unlock()
	if status != "retained" {
		return errors.New("only retained releases may be pruned")
	}
	var protected int
	if err = m.DB.QueryRowContext(ctx, `SELECT (SELECT count(*) FROM bundle_floors WHERE bundle=? AND revision>0)+(SELECT count(*) FROM dataset_jobs WHERE parent_digest=? AND state NOT IN ('released','aborted','pruned'))`, bundle, digest).Scan(&protected); err != nil {
		return err
	}
	if protected != 0 {
		return errors.New("release protected by withdrawal or uncertain candidate")
	}
	var pins []struct {
		Bundle string `json:"bundle"`
		Digest string `json:"digest"`
	}
	if err = m.authority(ctx, "GET", "/v1/releases/pins", nil, &pins); err != nil {
		return err
	}
	for _, pin := range pins {
		if !slug.MatchString(pin.Bundle) || !digestPattern.MatchString(pin.Digest) {
			return errors.New("invalid authoritative release pin")
		}
		if pin.Bundle == bundle && pin.Digest == digest {
			return errors.New("release remains assigned or revocation pending")
		}
	}
	root := m.release(bundle, digest)
	actual, _, err := TreeHash(root)
	if err != nil || actual != digest {
		return errors.New("retained release integrity mismatch")
	}
	if _, err = m.DB.ExecContext(ctx, `INSERT INTO pipeline_audit(job_id,action,actor,hash,at) VALUES(?,'prune_started',?,?,?)`, id, actor, digest, time.Now().UTC().Format(time.RFC3339Nano)); err != nil {
		return err
	}
	quarantine := filepath.Join(m.Config.Root, "staging", "prune-"+digest)
	if _, err = os.Lstat(quarantine); !os.IsNotExist(err) {
		return ErrReconcile
	}
	if err = os.Chmod(root, 0700); err != nil {
		return err
	}
	if err = os.Rename(root, quarantine); err != nil {
		return ErrReconcile
	}
	if err = filepath.WalkDir(quarantine, func(path string, entry os.DirEntry, e error) error {
		if e != nil {
			return e
		}
		if entry.IsDir() {
			return os.Chmod(path, 0700)
		}
		return nil
	}); err != nil {
		return ErrReconcile
	}
	if err = os.RemoveAll(quarantine); err != nil {
		return ErrReconcile
	}
	if err = syncDir(filepath.Dir(root)); err != nil {
		return ErrReconcile
	}
	if err = syncDir(filepath.Dir(quarantine)); err != nil {
		return ErrReconcile
	}
	tx, err := m.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	result, err := tx.Exec(`DELETE FROM releases WHERE bundle=? AND digest=? AND status='retained'`, bundle, digest)
	if err != nil {
		return err
	}
	n, _ := result.RowsAffected()
	if n != 1 {
		return ErrReconcile
	}
	if _, err = tx.Exec(`UPDATE dataset_jobs SET state='pruned' WHERE id=? AND state='released'`, id); err != nil {
		return err
	}
	if _, err = tx.Exec(`INSERT INTO pipeline_audit(job_id,action,actor,hash,at) VALUES(?,'prune_completed',?,?,?)`, id, actor, digest, time.Now().UTC().Format(time.RFC3339Nano)); err != nil {
		return err
	}
	return tx.Commit()
}
