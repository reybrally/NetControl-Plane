package inbound

import (
	"context"

	"github.com/google/uuid"
	"github.com/reybrally/NetControl-Plane/ncp/internal/ncp/domain/intent"
	"github.com/reybrally/NetControl-Plane/ncp/internal/ncp/domain/reachability"
)

type ReachabilityQuery struct {
	IntentID uuid.UUID
	Revision *int

	FromNamespace string
	FromSelector  map[string]string

	ToNamespace string
	ToService   string

	Port      int
	Protocol  intent.Protocol
	Direction intent.Direction
}

type ReachabilityService interface {
	CheckReachability(ctx context.Context, actor Actor, q ReachabilityQuery) (reachability.Result, error)
}
