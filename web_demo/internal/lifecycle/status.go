package lifecycle

import "context"

// LifecycleStatus is an operator-only consistent snapshot. It contains no
// owner identity, hostname, prompt, credential or physical workspace path.
type LifecycleStatus struct {
	Protocol             string            `json:"protocol"`
	DeploymentID         string            `json:"deployment_id"`
	ActivationGeneration int64             `json:"activation_generation"`
	Desired              string            `json:"desired"`
	Workspaces           map[string]string `json:"workspaces"`
	PendingJobs          []string          `json:"pending_jobs"`
	PendingSnapshots     int64             `json:"pending_snapshots"`
	PendingReclaims      int64             `json:"pending_reclaims"`
}

func (s *Store) LifecycleStatus(ctx context.Context) (LifecycleStatus, error) {
	v := LifecycleStatus{Protocol: "feam.lifecycle-status.v1", Workspaces: map[string]string{}, PendingJobs: []string{}}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return v, err
	}
	defer tx.Rollback()
	if err = tx.QueryRowContext(ctx, `SELECT id,activation,desired FROM deployment`).Scan(&v.DeploymentID, &v.ActivationGeneration, &v.Desired); err != nil {
		return v, err
	}
	rows, err := tx.QueryContext(ctx, `SELECT id,state FROM workspaces ORDER BY id`)
	if err != nil {
		return v, err
	}
	for rows.Next() {
		var id, state string
		if err = rows.Scan(&id, &state); err != nil {
			rows.Close()
			return v, err
		}
		v.Workspaces[id] = state
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return v, err
	}
	rows, err = tx.QueryContext(ctx, `SELECT id FROM lifecycle_jobs WHERE state IN ('accepted','executing','unknown') ORDER BY id`)
	if err != nil {
		return v, err
	}
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			return v, err
		}
		v.PendingJobs = append(v.PendingJobs, id)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return v, err
	}
	if err = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM snapshot_outbox`).Scan(&v.PendingSnapshots); err != nil {
		return v, err
	}
	if err = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM slot_reclaims WHERE state='pending'`).Scan(&v.PendingReclaims); err != nil {
		return v, err
	}
	return v, tx.Commit()
}
