package worker

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/reybrally/NetControl-Plane/ncp/internal/ncp/ports/outbound"
)

type ApplyWorker struct {
	Tx      outbound.Tx
	Plans   outbound.PlanRepo
	Audit   outbound.AuditRepo
	Intents outbound.IntentRepo

	Mode   string
	DryRun bool

	K8s outbound.K8sApplier
}

func (w ApplyWorker) Handle(ctx context.Context, payload map[string]any) error {
	if w.Tx == nil || w.Plans == nil || w.Audit == nil {
		return errors.New("apply worker not configured (tx/plans/audit)")
	}

	planIDStr, _ := payload["planId"].(string)
	actor, _ := payload["actor"].(string)
	if actor == "" {
		actor = "system"
	}

	planID, err := uuid.Parse(planIDStr)
	if err != nil {
		return errors.New("bad planId")
	}

	plan, err := w.Plans.GetPlan(ctx, planID)
	if err != nil {
		return err
	}

	markFailed := func(cause error) error {
		_ = w.Tx.WithinTx(ctx, func(txCtx context.Context) error {
			_ = w.Plans.UpdatePlanStatus(txCtx, planID, "apply_failed")
			_ = w.Audit.Append(txCtx, actor, "apply_failed", "plan", planID.String(), map[string]any{
				"mode":   w.Mode,
				"dryRun": w.DryRun,
				"error":  cause.Error(),
			})
			return nil
		})
		return cause
	}

	var (
		auditFields map[string]any
		appliedName string
		appliedNS   string
	)

	switch w.Mode {
	case "noop":
		auditFields = map[string]any{"mode": "noop", "dryRun": w.DryRun}

	case "k8s":
		if w.K8s == nil {
			return markFailed(errors.New("k8s applier is nil"))
		}
		k8sObj, ok := plan.Artifacts["k8s"].(map[string]any)
		if !ok {
			return markFailed(errors.New("plan.artifacts.k8s missing"))
		}

		name, err := w.K8s.ApplyNetworkPolicy(ctx, k8sObj, w.DryRun)
		if err != nil {
			return markFailed(err)
		}
		appliedName = name

		if ns, ok := k8sObj["namespace"].(string); ok && ns != "" {
			appliedNS = ns
		} else {
			appliedNS = "default"
		}

		auditFields = map[string]any{
			"mode":          "k8s",
			"dryRun":        w.DryRun,
			"networkPolicy": appliedName,
			"namespace":     appliedNS,
		}

	default:
		return markFailed(errors.New("unknown apply mode: " + w.Mode))
	}

	if err := w.Tx.WithinTx(ctx, func(txCtx context.Context) error {
		if err := w.Plans.UpdatePlanStatus(txCtx, planID, "applied"); err != nil {
			return err
		}

		if w.Mode == "k8s" && appliedName != "" {
			if err := w.Plans.SetAppliedK8sRef(txCtx, planID, appliedNS, appliedName); err != nil {
				return err
			}
		}

		if err := w.Audit.Append(txCtx, actor, "apply_done", "plan", planID.String(), auditFields); err != nil {
			return err
		}
		return nil
	}); err != nil {
		return markFailed(err)
	}

	if w.Intents != nil && plan.RevisionID != 0 {
		_ = w.Intents.SetNotAfterOnFirstApply(ctx, plan.RevisionID, time.Now().UTC())
	}

	return nil
}
