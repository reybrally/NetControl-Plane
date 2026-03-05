package usecases

import (
	"context"

	"github.com/google/uuid"
	"github.com/reybrally/NetControl-Plane/ncp/internal/ncp/ports/inbound"
	"github.com/reybrally/NetControl-Plane/ncp/internal/ncp/ports/outbound"
)

func (s *IntentService) ApplyPlan(ctx context.Context, actor inbound.Actor, planID uuid.UUID) error {
	return s.Tx.WithinTx(ctx, func(ctx context.Context) error {
		p, err := s.Plans.GetPlan(ctx, planID)
		if err != nil {
			return err
		}

		if p.Status == "applied" {
			return nil
		}

		if p.ApplyJobID != nil {
			return nil
		}

		if err := s.Plans.UpdatePlanStatus(ctx, planID, "applying"); err != nil {
			return err
		}

		jobID, err := s.Jobs.Enqueue(ctx, outbound.JobApplyPlan, map[string]any{
			"planId": planID.String(),
			"actor":  actor.ID,
		})
		if err != nil {
			return err
		}

		ok, err := s.Plans.SetApplyJobOnce(ctx, planID, jobID)
		if err != nil {
			return err
		}
		if !ok {
			return nil
		}

		return s.Audit.Append(ctx, actor.ID, "apply_queued", "plan", planID.String(), map[string]any{
			"jobId": jobID.String(),
		})
	})
}
