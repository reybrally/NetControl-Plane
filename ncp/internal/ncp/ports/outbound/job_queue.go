package outbound

import (
	"context"

	"github.com/google/uuid"
)

type JobKind string

const (
	JobApplyPlan      JobKind = "apply_plan"
	JobReconcileDrift JobKind = "reconcile_drift"
	JobExpireTTL      JobKind = "expire_ttl"
)

type JobQueue interface {
	Enqueue(ctx context.Context, kind JobKind, payload map[string]any) (uuid.UUID, error)
	LeaseNext(ctx context.Context, workerID string) (jobID uuid.UUID, kind JobKind, payload map[string]any, ok bool, err error)
	MarkDone(ctx context.Context, jobID uuid.UUID) error
	MarkFailed(ctx context.Context, jobID uuid.UUID, errMsg string) error
	FindActiveReconcileDrift(ctx context.Context, scope, namespace string, dryRun bool) (jobID uuid.UUID, ok bool, err error)
	FindActiveExpireTTL(ctx context.Context, planID uuid.UUID) (jobID uuid.UUID, ok bool, err error)
}
