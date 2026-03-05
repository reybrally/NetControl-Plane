package outbound

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/reybrally/NetControl-Plane/ncp/internal/ncp/domain/intent"
)

type Intent struct {
	ID              uuid.UUID
	Key             string
	Title           string
	OwnerTeam       string
	Status          string
	CurrentRevision *int64
	Labels          map[string]string
	CreatedBy       string
	CreatedAtUnix   int64
	UpdatedAtUnix   int64
}

type Revision struct {
	ID            int64
	IntentID      uuid.UUID
	Revision      int
	Spec          intent.Spec
	SpecHash      string
	State         string
	TicketRef     string
	Justification string
	TTLSeconds    *int
	NotAfterUnix  *int64
	CreatedBy     string
	CreatedAtUnix int64
}

type IntentRepo interface {
	CreateIntent(ctx context.Context, it Intent) error
	GetIntent(ctx context.Context, id uuid.UUID) (Intent, error)

	ListIntents(ctx context.Context, limit int) ([]Intent, error)

	ListRevisions(ctx context.Context, intentID uuid.UUID, limit int) ([]Revision, error)

	CreateRevision(ctx context.Context, rev Revision) (int, error)
	GetRevision(ctx context.Context, intentID uuid.UUID, revision int) (Revision, error)
	RefreshNotAfterOnApply(ctx context.Context, revisionID int64) error
	SetNotAfterOnFirstApply(ctx context.Context, revisionID int64, appliedAt time.Time) error
}
