package usecases

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/reybrally/NetControl-Plane/ncp/internal/ncp/app/services"
	"github.com/reybrally/NetControl-Plane/ncp/internal/ncp/ports/inbound"
	"github.com/reybrally/NetControl-Plane/ncp/internal/ncp/ports/outbound"
)

func (s *IntentService) PlanIntent(ctx context.Context, actor inbound.Actor, intentID uuid.UUID, revision int) (uuid.UUID, error) {
	var planID uuid.UUID

	err := s.Tx.WithinTx(ctx, func(ctx context.Context) error {
		rev, err := s.Intents.GetRevision(ctx, intentID, revision)
		if err != nil {
			return err
		}

		cr, err := services.Compile(rev.Spec)
		if err != nil {
			return err
		}

		planID = uuid.New()
		p := outbound.Plan{
			ID:            planID,
			IntentID:      intentID,
			RevisionID:    rev.ID,
			Status:        "planned",
			Diff:          cr.Diff,
			Blast:         cr.Blast,
			Artifacts:     cr.Artifacts,
			CreatedBy:     actor.ID,
			CreatedAtUnix: time.Now().Unix(),
		}
		if err := s.Plans.CreatePlan(ctx, p); err != nil {
			return err
		}
		return s.Audit.Append(ctx, actor.ID, "plan_created", "plan", planID.String(), map[string]any{"intentId": intentID.String(), "revision": revision})
	})

	return planID, err
}
