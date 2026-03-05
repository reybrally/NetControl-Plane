package outbound

import (
	"context"

	"github.com/google/uuid"
)

type Plan struct {
	ID            uuid.UUID
	IntentID      uuid.UUID
	RevisionID    int64
	Status        string
	Diff          map[string]any
	Blast         map[string]any
	Artifacts     map[string]any
	CreatedBy     string
	CreatedAtUnix int64
	ApplyJobID    *uuid.UUID `json:"applyJobId,omitempty"`
}

type ExpirablePlan struct {
	Plan
	TTLSeconds   *int
	NotAfterUnix *int64
}

type PlanRepo interface {
	CreatePlan(ctx context.Context, p Plan) error
	GetPlan(ctx context.Context, id uuid.UUID) (Plan, error)
	UpdatePlanStatus(ctx context.Context, id uuid.UUID, status string) error
	SetApplyJobOnce(ctx context.Context, planID uuid.UUID, jobID uuid.UUID) (ok bool, err error)
	ListLatestAppliedPlans(ctx context.Context, limit int) ([]Plan, error)
	ListLatestAppliedPlansPerIntentWithTTL(ctx context.Context, limit int) ([]ExpirablePlan, error)
	ListAppliedPlansByIntent(ctx context.Context, intentID uuid.UUID, limit int) ([]Plan, error)
	ListTTLExpiredCandidates(ctx context.Context, limit int) ([]uuid.UUID, error)
	MarkPlanExpiredOnce(ctx context.Context, planID uuid.UUID) (bool, error)
	SetAppliedK8sRef(ctx context.Context, planID uuid.UUID, namespace, name string) error
}
