package worker

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/reybrally/NetControl-Plane/ncp/internal/ncp/ports/outbound"
)

type ExpireTTLWorker struct {
	Tx    outbound.Tx
	Plans outbound.PlanRepo
	Audit outbound.AuditRepo

	Mode   string
	DryRun bool
	K8s    outbound.K8sApplier
}

func (w ExpireTTLWorker) Handle(ctx context.Context, payload map[string]any) error {
	if w.Tx == nil || w.Plans == nil || w.Audit == nil {
		return errors.New("expire ttl worker not configured (tx/plans/audit)")
	}

	planIDStr, _ := payload["planId"].(string)
	actor, _ := payload["actor"].(string)
	if actor == "" {
		actor = "system:ttl"
	}

	planID, err := uuid.Parse(planIDStr)
	if err != nil {
		return errors.New("bad planId")
	}

	plan, err := w.Plans.GetPlan(ctx, planID)
	if err != nil {
		return err
	}

	expiredNow, err := w.Plans.MarkPlanExpiredOnce(ctx, planID)
	if err != nil {
		return err
	}
	if !expiredNow {
		return nil
	}

	_ = w.Tx.WithinTx(ctx, func(txCtx context.Context) error {
		return w.Audit.Append(txCtx, actor, "ttl_expired", "plan", planID.String(), map[string]any{
			"intentId": plan.IntentID.String(),
			"dryRun":   w.DryRun,
			"mode":     w.Mode,
		})
	})

	applied, err := w.Plans.ListAppliedPlansByIntent(ctx, plan.IntentID, 50)
	if err != nil {
		return err
	}

	var rollback *outbound.Plan
	for i := range applied {
		if applied[i].ID != planID {
			rollback = &applied[i]
			break
		}
	}

	if rollback != nil {
		switch w.Mode {
		case "noop":
		case "k8s":
			if w.K8s == nil {
				return errors.New("k8s applier is nil")
			}
			k8sObj, ok := rollback.Artifacts["k8s"].(map[string]any)
			if !ok {
				return fmt.Errorf("rollback plan %s missing artifacts.k8s", rollback.ID.String())
			}
			if _, err := w.K8s.ApplyNetworkPolicy(ctx, k8sObj, w.DryRun); err != nil {
				return err
			}
		default:
			return errors.New("unknown apply mode: " + w.Mode)
		}

		_ = w.Tx.WithinTx(ctx, func(txCtx context.Context) error {
			return w.Audit.Append(txCtx, actor, "rollback_done", "intent", plan.IntentID.String(), map[string]any{
				"fromPlanId": planID.String(),
				"toPlanId":   rollback.ID.String(),
				"dryRun":     w.DryRun,
				"mode":       w.Mode,
			})
		})

		return nil
	}

	switch w.Mode {
	case "noop":

	case "k8s":
		if w.K8s == nil {
			return errors.New("k8s applier is nil")
		}

		k8sObj, ok := plan.Artifacts["k8s"].(map[string]any)
		if !ok {
			return errors.New("expired plan missing artifacts.k8s")
		}

		ns := ""
		if v, ok := k8sObj["namespace"].(string); ok {
			ns = v
		}
		if ns == "" {
			ns = "default"
		}

		name := ""
		if appliedRef, ok := k8sObj["applied"].(map[string]any); ok {
			if v, ok := appliedRef["name"].(string); ok {
				name = v
			}
			if v, ok := appliedRef["namespace"].(string); ok && v != "" {
				ns = v
			}
		}

		if name == "" {
			if v, ok := k8sObj["name"].(string); ok {
				name = v
			}
		}

		if name == "" {
			if v, ok := k8sObj["policyName"].(string); ok {
				name = v
			}
		}
		if name == "" {
			if sel, ok := k8sObj["podSelector"].(map[string]any); ok {
				if app, ok := sel["app"].(string); ok && app != "" {
					name = "ncp-" + app
				}
			}
		}

		if name == "" {
			return errors.New("artifacts.k8s missing policy name (expected k8s.applied.name OR k8s.name OR k8s.policyName)")
		}

		if err := w.K8s.DeleteNetworkPolicy(ctx, ns, name, w.DryRun); err != nil {
			return err
		}

	default:
		return errors.New("unknown apply mode: " + w.Mode)
	}

	_ = w.Tx.WithinTx(ctx, func(txCtx context.Context) error {
		return w.Audit.Append(txCtx, actor, "ttl_deleted", "plan", planID.String(), map[string]any{
			"intentId": plan.IntentID.String(),
			"dryRun":   w.DryRun,
			"mode":     w.Mode,
		})
	})

	return nil
}
