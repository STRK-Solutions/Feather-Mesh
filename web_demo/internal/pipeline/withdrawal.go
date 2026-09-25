package pipeline

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"time"
)

// A withdrawal clones the entire current release and changes only FEAM-owned
// tombstones. Policy/source/product descriptors are inherited exactly.
func (m *Manager) validateWithdrawalParent(j Job) error {
	var id string
	if m.DB.QueryRow(`SELECT job_id FROM releases WHERE bundle=? AND digest=? AND status='current'`, j.Bundle, j.ParentDigest).Scan(&id) != nil {
		return errors.New("withdrawal requires current parent")
	}
	parent, err := m.job(id)
	if err != nil {
		return err
	}
	parent.ID = j.ID
	parent.ParentDigest = j.ParentDigest
	parent.Withdrawal = j.Withdrawal
	if parent.Hash() != j.Hash() {
		return errors.New("withdrawal may not alter inherited source or policy")
	}
	return nil
}
func (m *Manager) WithdrawalPlan(id, bundle, digest string, withdrawal Withdrawal) (Job, error) {
	if !uuidPattern.MatchString(id) || !slug.MatchString(bundle) || !digestPattern.MatchString(digest) {
		return Job{}, errors.New("invalid withdrawal identity")
	}
	if _, err := m.Assignable(bundle, digest); err != nil {
		return Job{}, err
	}
	var parentID string
	if err := m.DB.QueryRow(`SELECT job_id FROM releases WHERE bundle=? AND digest=? AND status='current'`, bundle, digest).Scan(&parentID); err != nil {
		return Job{}, err
	}
	j, err := m.job(parentID)
	if err != nil {
		return Job{}, err
	}
	j.ID = id
	j.ParentDigest = digest
	j.Withdrawal = &withdrawal
	return j, j.Validate()
}
func (m *Manager) prepareWithdrawal(ctx context.Context, j Job) error {
	provider := filepath.Join(m.payload(j.ID), "provider")
	for _, target := range j.Withdrawal.Targets {
		if _, err := m.DB.Exec(`UPDATE dataset_jobs SET state='publishing',manifest_outcome='unknown' WHERE id=?`, j.ID); err != nil {
			return err
		}
		b, err := m.feam(ctx, provider, "withdraw", "product://"+j.Namespace+"/"+target.ID, "--version", target.Version, "--reason", j.Withdrawal.Reason)
		if err != nil {
			return ErrReconcile
		}
		var out struct {
			Protocol string `json:"protocol"`
			Status   string `json:"status"`
		}
		if json.Unmarshal(b, &out) != nil || out.Protocol != "feam.peer.v1" || out.Status != "withdrawn" {
			return ErrReconcile
		}
	}
	return m.finishCandidate(ctx, j)
}
func (m *Manager) finishWithdrawal(j Job) error {
	hash, size, err := TreeHash(m.payload(j.ID))
	if err != nil {
		return err
	}
	if size > j.Limits.CandidateBytes {
		return errors.New("withdrawal complete candidate exceeds limit")
	}
	_, err = m.DB.Exec(`UPDATE dataset_jobs SET state='prepared',candidate_hash=?,manifest_outcome='committed',updated_at=? WHERE id=? AND state IN ('publishing','unknown')`, hash, time.Now().UTC().Format(time.RFC3339Nano), j.ID)
	return err
}
