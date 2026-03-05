package outbound

import (
	"context"

	"github.com/reybrally/NetControl-Plane/ncp/internal/ncp/domain/drift"
)

type DriftRepo interface {
	InsertSnapshot(ctx context.Context, s drift.Snapshot) error
	ListSnapshots(ctx context.Context, scope string, limit int) ([]drift.Snapshot, error)
}
