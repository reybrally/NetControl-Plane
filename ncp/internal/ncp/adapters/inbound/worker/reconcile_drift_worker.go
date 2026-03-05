package worker

import (
	"context"
	"fmt"

	"github.com/reybrally/NetControl-Plane/ncp/internal/ncp/adapters/outbound/k8s"
	"github.com/reybrally/NetControl-Plane/ncp/internal/ncp/ports/outbound"
	"k8s.io/client-go/kubernetes"
)

type ReconcileDriftWorker struct {
	Plans outbound.PlanRepo
	Audit outbound.AuditRepo

	K8sClient kubernetes.Interface
	Applier   *k8s.Applier
}

func (w ReconcileDriftWorker) Handle(ctx context.Context, payload map[string]any) error {
	if w.Plans == nil || w.Audit == nil {
		return fmt.Errorf("reconcile worker deps are nil")
	}
	if w.K8sClient == nil || w.Applier == nil {
		return fmt.Errorf("k8s client/applier is nil")
	}

	scope, _ := payload["scope"].(string)
	if scope == "" {
		scope = "k8s:unknown"
	}
	namespace, _ := payload["namespace"].(string)
	if namespace == "" {
		namespace = "default"
	}

	dryRun := false
	if v, ok := payload["dryRun"].(bool); ok {
		dryRun = v
	}

	actor, _ := payload["actor"].(string)
	if actor == "" {
		actor = "system"
	}

	observed, err := k8s.DiscoverManagedNetworkPolicies(ctx, w.K8sClient, namespace)
	if err != nil {
		_ = w.Audit.Append(ctx, actor, "drift_reconcile_failed", "drift", scope, map[string]any{
			"phase":     "discover_observed",
			"namespace": namespace,
			"error":     err.Error(),
		})
		return err
	}

	_, obsByKey, _, err := k8s.IndexAndHashPolicies(observed)
	if err != nil {
		_ = w.Audit.Append(ctx, actor, "drift_reconcile_failed", "drift", scope, map[string]any{
			"phase":     "hash_observed",
			"namespace": namespace,
			"error":     err.Error(),
		})
		return err
	}

	plans, err := w.Plans.ListLatestAppliedPlans(ctx, 500)
	if err != nil {
		_ = w.Audit.Append(ctx, actor, "drift_reconcile_failed", "drift", scope, map[string]any{
			"phase": "list_applied_plans",
			"error": err.Error(),
		})
		return err
	}

	type item struct {
		obj map[string]any
		at  int64
		ns  string
		nm  string
	}

	desiredByKeyObj := map[string]item{}

	for _, p := range plans {
		k8sAny, ok := p.Artifacts["k8s"]
		if !ok {
			continue
		}
		obj, ok := k8sAny.(map[string]any)
		if !ok {
			continue
		}

		np, name, err := k8s.RenderNetworkPolicy(obj)
		if err != nil {
			_ = w.Audit.Append(ctx, actor, "drift_reconcile_failed", "drift", scope, map[string]any{
				"phase":  "render_desired",
				"planId": p.ID.String(),
				"error":  err.Error(),
			})
			return err
		}

		if namespace != "" && namespace != "*" && np.Namespace != namespace {
			continue
		}

		key := np.Namespace + "/" + name
		cand := item{obj: obj, at: p.CreatedAtUnix, ns: np.Namespace, nm: name}

		prev, exists := desiredByKeyObj[key]
		if !exists || cand.at > prev.at {
			desiredByKeyObj[key] = cand
		}
	}

	applied := 0
	for _, it := range desiredByKeyObj {
		if _, err := w.Applier.ApplyNetworkPolicy(ctx, it.obj, dryRun); err != nil {
			_ = w.Audit.Append(ctx, actor, "drift_reconcile_failed", "drift", scope, map[string]any{
				"phase":     "apply_desired",
				"namespace": it.ns,
				"name":      it.nm,
				"error":     err.Error(),
				"dryRun":    dryRun,
			})
			return err
		}
		applied++
	}

	deleted := 0
	for key := range obsByKey {
		if _, ok := desiredByKeyObj[key]; ok {
			continue
		}

		var ns, nm string
		for i := 0; i < len(key); i++ {
			if key[i] == '/' {
				ns = key[:i]
				nm = key[i+1:]
				break
			}
		}
		if ns == "" || nm == "" {
			continue
		}

		if err := w.Applier.DeleteNetworkPolicy(ctx, ns, nm, dryRun); err != nil {
			_ = w.Audit.Append(ctx, actor, "drift_reconcile_failed", "drift", scope, map[string]any{
				"phase":     "delete_extra",
				"namespace": ns,
				"name":      nm,
				"error":     err.Error(),
				"dryRun":    dryRun,
			})
			return err
		}
		deleted++
	}

	_ = w.Audit.Append(ctx, actor, "drift_reconciled", "drift", scope, map[string]any{
		"namespace":     namespace,
		"dryRun":        dryRun,
		"desiredCount":  len(desiredByKeyObj),
		"observedCount": len(obsByKey),
		"applied":       applied,
		"deleted":       deleted,
	})

	return nil
}
