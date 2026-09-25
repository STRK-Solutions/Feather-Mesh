package budget

import (
	"context"
	"time"
)

// DrainReceipt is broker-observed accounting, not independently verified provider
// billing. Unknown requests continue consuming their full reservations.
type DrainReceipt struct {
	Protocol          string    `json:"protocol"`
	Provenance        string    `json:"provenance"`
	AllocationID      string    `json:"allocation_id"`
	AllocationSHA256  string    `json:"allocation_sha256"`
	DeploymentID      string    `json:"deployment_id"`
	Generation        int64     `json:"activation_generation"`
	CreatedAt         time.Time `json:"created_at"`
	Usage             Usage     `json:"usage"`
	RequestCount      int64     `json:"request_count"`
	UnknownRequests   int64     `json:"unknown_requests"`
	RequiresReconcile bool      `json:"provider_reconciliation_required"`
}

// Pause is durable and never implicitly undone by reopening the run.
func (r *Run) Pause(ctx context.Context) error {
	_, err := r.DB.ExecContext(ctx, `UPDATE run_allocations SET state='paused'`)
	return err
}

// Drain must be called after the server has stopped admission and joined all
// request handlers. A receipt is emitted only after a successful WAL checkpoint.
func (r *Run) Drain(ctx context.Context) (DrainReceipt, error) {
	return r.checkpoint(ctx, true)
}

// Checkpoint preserves the prior admission state for an ordinary service/OS
// shutdown. It cannot unpause a deliberately drained or over-limit run.
func (r *Run) Checkpoint(ctx context.Context) (DrainReceipt, error) {
	return r.checkpoint(ctx, false)
}

func (r *Run) checkpoint(ctx context.Context, pause bool) (DrainReceipt, error) {
	v := DrainReceipt{}
	tx, err := r.DB.BeginTx(ctx, nil)
	if err != nil {
		return v, err
	}
	defer tx.Rollback()
	if pause {
		if _, err = tx.ExecContext(ctx, `UPDATE run_allocations SET state='paused'`); err != nil {
			return v, err
		}
	}
	if _, err = tx.ExecContext(ctx, `UPDATE requests SET state='unknown' WHERE state IN ('reserved','dispatched')`); err != nil {
		return v, err
	}
	if err = tx.Commit(); err != nil {
		return v, err
	}
	var busy, frames, checkpointed int
	if err = r.DB.QueryRowContext(ctx, `PRAGMA wal_checkpoint(TRUNCATE)`).Scan(&busy, &frames, &checkpointed); err != nil {
		return v, err
	}
	if busy != 0 || frames != checkpointed {
		return v, ErrConflict
	}
	a := r.Document.Allocation
	v = DrainReceipt{Protocol: "feam.broker-drain.v1", Provenance: "broker_observed", AllocationID: a.ID,
		AllocationSHA256: r.Document.Digest(), DeploymentID: a.DeploymentID, Generation: a.ActivationGeneration,
		CreatedAt: time.Now().UTC()}
	if v.Usage, err = r.Usage(ctx, ""); err != nil {
		return DrainReceipt{}, err
	}
	if err = r.DB.QueryRowContext(ctx, `SELECT COUNT(*),COALESCE(SUM(CASE WHEN state='unknown' THEN 1 ELSE 0 END),0) FROM requests`).Scan(&v.RequestCount, &v.UnknownRequests); err != nil {
		return DrainReceipt{}, err
	}
	v.RequiresReconcile = v.UnknownRequests != 0
	return v, nil
}
