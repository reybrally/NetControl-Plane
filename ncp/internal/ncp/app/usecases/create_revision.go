package usecases

import (
	"context"

	"github.com/google/uuid"
	ncperr "github.com/reybrally/NetControl-Plane/ncp/internal/ncp/domain/errors"
	"github.com/reybrally/NetControl-Plane/ncp/internal/ncp/domain/intent"
	"github.com/reybrally/NetControl-Plane/ncp/internal/ncp/ports/inbound"
	"github.com/reybrally/NetControl-Plane/ncp/internal/ncp/ports/outbound"
)

func (s *IntentService) CreateRevision(ctx context.Context, actor inbound.Actor, intentID uuid.UUID, spec intent.Spec, ticketRef, justification string, ttlSeconds *int) (int, error) {
	if err := spec.ValidateBasic(); err != nil {
		return 0, ncperr.InvalidArgument(err.Error(), nil, err)
	}

	if err := spec.ValidateGuardrails(); err != nil {
		if ge, ok := err.(intent.GuardrailError); ok {
			_ = s.Audit.Append(ctx, actor.ID, "guardrail_denied", "intent", intentID.String(), map[string]any{
				"ticketRef":  ticketRef,
				"violations": ge.Violations,
			})
			return 0, ncperr.GuardrailDenied("guardrails denied", map[string]any{
				"violations": ge.Violations,
			}, err)
		}

		_ = s.Audit.Append(ctx, actor.ID, "guardrail_denied", "intent", intentID.String(), map[string]any{
			"ticketRef": ticketRef,
			"error":     err.Error(),
		})
		return 0, ncperr.GuardrailDenied("guardrails denied", nil, err)
	}

	var ttl *int
	if ttlSeconds != nil && *ttlSeconds > 0 {
		ttl = ttlSeconds
	}

	rev := outbound.Revision{
		IntentID:      intentID,
		Spec:          spec,
		State:         "draft",
		TicketRef:     ticketRef,
		Justification: justification,
		TTLSeconds:    ttl,
		CreatedBy:     actor.ID,
	}

	var n int
	err := s.Tx.WithinTx(ctx, func(ctx context.Context) error {
		var err error
		n, err = s.Intents.CreateRevision(ctx, rev)
		if err != nil {
			return err
		}
		return s.Audit.Append(ctx, actor.ID, "create_revision", "intent", intentID.String(), map[string]any{"revision": n})
	})

	return n, err
}
