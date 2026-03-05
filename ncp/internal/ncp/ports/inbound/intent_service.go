package inbound

import (
	"context"

	"github.com/google/uuid"
	"github.com/reybrally/NetControl-Plane/ncp/internal/ncp/domain/intent"
)

type Actor struct {
	ID    string
	Roles []string
	Team  string
}

type IntentService interface {
	CreateIntent(ctx context.Context, actor Actor, key, title, ownerTeam string, labels map[string]string) (uuid.UUID, error)
	CreateRevision(ctx context.Context, actor Actor, intentID uuid.UUID, spec intent.Spec, ticketRef, justification string, ttlSeconds *int) (int, error)
	PlanIntent(ctx context.Context, actor Actor, intentID uuid.UUID, revision int) (uuid.UUID, error)
	ApplyPlan(ctx context.Context, actor Actor, planID uuid.UUID) error
	GetPlan(ctx context.Context, actor Actor, planID uuid.UUID) (map[string]any, error)

	GetIntent(ctx context.Context, actor Actor, intentID uuid.UUID) (map[string]any, error)
	ListRevisions(ctx context.Context, actor Actor, intentID uuid.UUID) ([]map[string]any, error)

	ListAudit(ctx context.Context, actor Actor, entityType, entityID string, limit int) ([]map[string]any, error)
}
