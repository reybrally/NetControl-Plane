package outbound

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type Job struct {
	ID        uuid.UUID
	Kind      JobKind
	Status    string
	Payload   map[string]any
	Error     *string
	LeasedBy  *string
	LeasedAt  *time.Time
	CreatedAt time.Time
	UpdatedAt time.Time
}

type JobRepo interface {
	GetByID(ctx context.Context, id uuid.UUID) (*Job, error)
}
