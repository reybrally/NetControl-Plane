package usecases

import (
	"context"

	"github.com/google/uuid"
	"github.com/reybrally/NetControl-Plane/ncp/internal/ncp/ports/inbound"
	"github.com/reybrally/NetControl-Plane/ncp/internal/ncp/ports/outbound"
)

func (s *IntentService) CreateIntent(ctx context.Context, actor inbound.Actor, key, title, ownerTeam string, labels map[string]string) (uuid.UUID, error) {
	id := uuid.New()

	it := outbound.Intent{
		ID:        id,
		Key:       key,
		Title:     title,
		OwnerTeam: ownerTeam,
		Status:    "draft",
		Labels:    labels,
		CreatedBy: actor.ID,
	}

	err := s.Tx.WithinTx(ctx, func(ctx context.Context) error {
		if err := s.Intents.CreateIntent(ctx, it); err != nil {
			return err
		}
		return s.Audit.Append(ctx, actor.ID, "create_intent", "intent", id.String(), map[string]any{"key": key})
	})

	return id, err
}
