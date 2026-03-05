package usecases

import (
	"context"

	"github.com/reybrally/NetControl-Plane/ncp/internal/ncp/domain/drift"
	ncperr "github.com/reybrally/NetControl-Plane/ncp/internal/ncp/domain/errors"
	"github.com/reybrally/NetControl-Plane/ncp/internal/ncp/ports/inbound"
	"github.com/reybrally/NetControl-Plane/ncp/internal/ncp/ports/outbound"
)

type ListDrift struct {
	Repo outbound.DriftRepo
}

func (uc ListDrift) Handle(ctx context.Context, actor inbound.Actor, scope string, limit int) ([]drift.Snapshot, error) {
	_ = actor
	if limit <= 0 || limit > 1000 {
		return nil, ncperr.InvalidArgument("bad limit", map[string]any{"limit": limit}, nil)
	}
	return uc.Repo.ListSnapshots(ctx, scope, limit)
}
