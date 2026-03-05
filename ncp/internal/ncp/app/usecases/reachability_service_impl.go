package usecases

import (
	"context"

	"github.com/reybrally/NetControl-Plane/ncp/internal/ncp/domain/reachability"
	"github.com/reybrally/NetControl-Plane/ncp/internal/ncp/ports/inbound"
)

type ReachabilityService struct {
	UC CheckReachability
}

func (s ReachabilityService) CheckReachability(ctx context.Context, actor inbound.Actor, q inbound.ReachabilityQuery) (reachability.Result, error) {

	return s.UC.Handle(ctx, actor, ReachabilityQuery{
		IntentID:      q.IntentID,
		Revision:      q.Revision,
		FromNamespace: q.FromNamespace,
		FromSelector:  q.FromSelector,
		ToNamespace:   q.ToNamespace,
		ToService:     q.ToService,
		Port:          q.Port,
		Protocol:      q.Protocol,
		Direction:     q.Direction,
	})
}
