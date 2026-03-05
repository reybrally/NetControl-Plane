package worker

import (
	"context"
	"log"
	"time"

	"github.com/reybrally/NetControl-Plane/ncp/internal/ncp/ports/outbound"
)

type Runner struct {
	Queue    outbound.JobQueue
	WorkerID string

	ApplyPlan      func(ctx context.Context, payload map[string]any) error
	ReconcileDrift func(ctx context.Context, payload map[string]any) error
	ExpireTTL      func(ctx context.Context, payload map[string]any) error
}

func (r Runner) Run(ctx context.Context) {
	if r.Queue == nil {
		log.Printf("runner: queue is nil")
		return
	}
	workerID := r.WorkerID
	if workerID == "" {
		workerID = "worker-1"
	}

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			r.step(ctx, workerID)
		}
	}
}

func (r Runner) step(ctx context.Context, workerID string) {
	jobID, kind, payload, ok, err := r.Queue.LeaseNext(ctx, workerID)
	if err != nil {

		log.Printf("runner: lease failed: %v", err)
		return
	}
	if !ok {
		return
	}

	jobCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	var handleErr error
	switch kind {
	case outbound.JobApplyPlan:
		if r.ApplyPlan == nil {
			handleErr = runnerErr("no handler for apply_plan")
		} else {
			handleErr = r.ApplyPlan(jobCtx, payload)
		}

	case outbound.JobReconcileDrift:
		if r.ReconcileDrift == nil {
			handleErr = runnerErr("no handler for reconcile_drift")
		} else {
			handleErr = r.ReconcileDrift(jobCtx, payload)
		}

	case outbound.JobExpireTTL:
		if r.ExpireTTL == nil {
			handleErr = runnerErr("no handler for expire_ttl")
		} else {
			handleErr = r.ExpireTTL(jobCtx, payload)
		}

	default:
		handleErr = runnerErr("unknown job kind: " + string(kind))
	}

	markCtx, markCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer markCancel()

	if handleErr != nil {
		if err := r.Queue.MarkFailed(markCtx, jobID, handleErr.Error()); err != nil {
			log.Printf("runner: mark failed error: job=%s kind=%s err=%v (original=%v)", jobID.String(), kind, err, handleErr)
			return
		}
		log.Printf("job failed: kind=%s id=%s err=%v", kind, jobID.String(), handleErr)
		return
	}

	if err := r.Queue.MarkDone(markCtx, jobID); err != nil {
		log.Printf("runner: mark done error: job=%s kind=%s err=%v", jobID.String(), kind, err)
		return
	}
}

type runnerErr string

func (e runnerErr) Error() string { return string(e) }
