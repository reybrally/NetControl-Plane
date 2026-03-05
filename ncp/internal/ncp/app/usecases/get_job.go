package usecases

import (
	"context"

	"github.com/google/uuid"

	ncperr "github.com/reybrally/NetControl-Plane/ncp/internal/ncp/domain/errors"
	"github.com/reybrally/NetControl-Plane/ncp/internal/ncp/ports/inbound"
	"github.com/reybrally/NetControl-Plane/ncp/internal/ncp/ports/outbound"
)

type GetJob struct {
	Repo outbound.JobRepo
}

func (uc GetJob) Handle(ctx context.Context, actor inbound.Actor, id uuid.UUID) (*outbound.Job, error) {
	if actor.ID == "" {
		return nil, ncperr.Unauthenticated("missing actor", nil, nil)
	}

	j, err := uc.Repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if j == nil {
		return nil, ncperr.NotFound("job not found", map[string]any{"id": id.String()}, nil)
	}
	return j, nil
}
