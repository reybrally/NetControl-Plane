package usecases

import (
	"context"

	"github.com/google/uuid"
	ncperr "github.com/reybrally/NetControl-Plane/ncp/internal/ncp/domain/errors"
	"github.com/reybrally/NetControl-Plane/ncp/internal/ncp/ports/inbound"
	"github.com/reybrally/NetControl-Plane/ncp/internal/ncp/ports/outbound"
)

type ReconcileDrift struct {
	Tx    outbound.Tx
	Jobs  outbound.JobQueue
	Audit outbound.AuditRepo
}

func (uc ReconcileDrift) Handle(ctx context.Context, actor inbound.Actor, scope, namespace string, dryRun bool) (uuid.UUID, error) {
	if actor.ID == "" {
		return uuid.Nil, ncperr.Unauthenticated("missing actor", nil, nil)
	}
	if scope == "" {
		scope = "k8s:unknown"
	}
	if namespace == "" {
		namespace = "default"
	}

	var jobID uuid.UUID

	err := uc.Tx.WithinTx(ctx, func(ctx context.Context) error {

		if existingID, ok, err := uc.Jobs.FindActiveReconcileDrift(ctx, scope, namespace, dryRun); err != nil {
			return err
		} else if ok {
			jobID = existingID

			_ = uc.Audit.Append(ctx, actor.ID, "drift_reconcile_deduped", "drift", scope, map[string]any{
				"jobId":     jobID.String(),
				"namespace": namespace,
				"dryRun":    dryRun,
			})

			return nil
		}

		var err error
		jobID, err = uc.Jobs.Enqueue(ctx, outbound.JobReconcileDrift, map[string]any{
			"scope":     scope,
			"namespace": namespace,
			"dryRun":    dryRun,
			"actor":     actor.ID,
		})
		if err != nil {
			return err
		}

		return uc.Audit.Append(ctx, actor.ID, "drift_reconcile_queued", "drift", scope, map[string]any{
			"jobId":     jobID.String(),
			"namespace": namespace,
			"dryRun":    dryRun,
		})
	})

	return jobID, err
}
