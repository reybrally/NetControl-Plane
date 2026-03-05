package worker

import (
	"context"
	"errors"
	"sort"
	"time"

	"github.com/google/uuid"
	k8sout "github.com/reybrally/NetControl-Plane/ncp/internal/ncp/adapters/outbound/k8s"
	"github.com/reybrally/NetControl-Plane/ncp/internal/ncp/domain/drift"
	"github.com/reybrally/NetControl-Plane/ncp/internal/ncp/ports/outbound"
	"k8s.io/client-go/kubernetes"
)

type DriftWorker struct {
	Repo      outbound.DriftRepo
	Plans     outbound.PlanRepo
	K8s       kubernetes.Interface
	Namespace string

	Scope    string
	Interval time.Duration
	ClockNow func() time.Time
}

func (w DriftWorker) Run(ctx context.Context) error {
	interval := w.Interval
	if interval <= 0 {
		interval = 30 * time.Second
	}
	nowFn := w.ClockNow
	if nowFn == nil {
		nowFn = time.Now
	}
	scope := w.Scope
	if scope == "" {
		scope = "k8s:unknown"
	}

	_ = w.tick(ctx, scope, nowFn)

	t := time.NewTicker(interval)
	defer t.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-t.C:

			_ = w.tick(ctx, scope, nowFn)
		}
	}
}

func (w DriftWorker) tick(ctx context.Context, scope string, nowFn func() time.Time) error {
	ts := nowFn().UTC().Unix()

	s := drift.Snapshot{
		ID:     uuid.New(),
		AtUnix: ts,
		Scope:  scope,
	}

	if w.K8s == nil {
		s.Status = drift.StatusUnknown
		s.DesiredHash = "unknown"
		s.ObservedHash = "unknown"
		s.Details = map[string]any{"note": "no k8s client configured"}
		return w.Repo.InsertSnapshot(ctx, s)
	}

	observedPolicies, err := k8sout.DiscoverManagedNetworkPolicies(ctx, w.K8s, w.Namespace)
	if err != nil {
		s.Status = drift.StatusUnknown
		s.DesiredHash = "unknown"
		s.ObservedHash = "unknown"
		s.Details = map[string]any{"error": err.Error(), "phase": "discover_observed"}
		_ = w.Repo.InsertSnapshot(ctx, s)
		return nil
	}

	obsHash, obsByKey, obsDetails, err := k8sout.IndexAndHashPolicies(observedPolicies)
	if err != nil {
		s.Status = drift.StatusUnknown
		s.DesiredHash = "unknown"
		s.ObservedHash = "unknown"
		s.Details = map[string]any{"error": err.Error(), "phase": "hash_observed"}
		_ = w.Repo.InsertSnapshot(ctx, s)
		return nil
	}
	s.ObservedHash = obsHash

	desiredHash, desiredByKey, desiredDetails, err := w.computeDesiredFromAppliedPlans(ctx)
	if err != nil {
		s.Status = drift.StatusUnknown
		s.DesiredHash = "unknown"
		s.Details = map[string]any{
			"observed": obsDetails,
			"error":    err.Error(),
			"phase":    "compute_desired",
		}
		_ = w.Repo.InsertSnapshot(ctx, s)
		return nil
	}
	s.DesiredHash = desiredHash

	missing, extra, changed := diffKeys(desiredByKey, obsByKey)

	missing = limitStrings(missing, 50)
	extra = limitStrings(extra, 50)
	changed = limitChanged(changed, 50)

	if len(missing) == 0 && len(extra) == 0 && len(changed) == 0 {
		s.Status = drift.StatusOK
	} else {
		s.Status = drift.StatusDrift
	}

	s.Details = map[string]any{
		"match":    desiredHash == obsHash,
		"desired":  desiredDetails,
		"observed": obsDetails,
		"diff": map[string]any{
			"missing": missing,
			"extra":   extra,
			"changed": changed,
		},
	}

	return w.Repo.InsertSnapshot(ctx, s)
}

func (w DriftWorker) computeDesiredFromAppliedPlans(ctx context.Context) (string, map[string]string, map[string]any, error) {
	if w.Plans == nil {
		return "", nil, nil, errors.New("plans repo is nil")
	}

	plans, err := w.Plans.ListLatestAppliedPlans(ctx, 200)
	if err != nil {
		return "", nil, nil, err
	}

	type item struct {
		p  k8sout.DiscoveredPolicy
		at int64
	}
	byKey := map[string]item{}

	for _, plan := range plans {
		k8sAny, ok := plan.Artifacts["k8s"]
		if !ok {
			continue
		}
		k8sMap, ok := k8sAny.(map[string]any)
		if !ok {
			continue
		}

		np, name, err := k8sout.RenderNetworkPolicy(k8sMap)
		if err != nil {

			return "", nil, nil, err
		}

		if w.Namespace != "" && w.Namespace != "*" && np.Namespace != w.Namespace {
			continue
		}

		key := np.Namespace + "/" + name
		cand := item{
			p: k8sout.DiscoveredPolicy{
				Namespace: np.Namespace,
				Name:      name,
				Labels:    np.Labels,
				Spec:      np.Spec,
			},
			at: plan.CreatedAtUnix,
		}

		prev, exists := byKey[key]
		if !exists || cand.at > prev.at {
			byKey[key] = cand
		}
	}

	desiredPolicies := make([]k8sout.DiscoveredPolicy, 0, len(byKey))
	for _, it := range byKey {
		desiredPolicies = append(desiredPolicies, it.p)
	}

	desiredHash, desiredByKey, details, err := k8sout.IndexAndHashPolicies(desiredPolicies)
	if err != nil {
		return "", nil, nil, err
	}

	details["source"] = "ncp_applied_plans"
	details["plansScanned"] = len(plans)

	return desiredHash, desiredByKey, details, nil
}

func diffKeys(desired, observed map[string]string) (missing, extra []string, changed []map[string]any) {
	for k, dh := range desired {
		oh, ok := observed[k]
		if !ok {
			missing = append(missing, k)
			continue
		}
		if oh != dh {
			changed = append(changed, map[string]any{
				"key":          k,
				"desiredHash":  dh,
				"observedHash": oh,
			})
		}
	}
	for k := range observed {
		if _, ok := desired[k]; !ok {
			extra = append(extra, k)
		}
	}

	sort.Strings(missing)
	sort.Strings(extra)
	sort.Slice(changed, func(i, j int) bool {
		return changed[i]["key"].(string) < changed[j]["key"].(string)
	})

	return
}

func limitStrings(xs []string, n int) []string {
	if n <= 0 || len(xs) <= n {
		return xs
	}
	return xs[:n]
}

func limitChanged(xs []map[string]any, n int) []map[string]any {
	if n <= 0 || len(xs) <= n {
		return xs
	}
	return xs[:n]
}
