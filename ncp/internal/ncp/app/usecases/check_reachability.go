package usecases

import (
	"context"
	"strings"

	"github.com/google/uuid"
	ncperr "github.com/reybrally/NetControl-Plane/ncp/internal/ncp/domain/errors"
	"github.com/reybrally/NetControl-Plane/ncp/internal/ncp/domain/intent"
	"github.com/reybrally/NetControl-Plane/ncp/internal/ncp/domain/reachability"
	"github.com/reybrally/NetControl-Plane/ncp/internal/ncp/ports/inbound"
	"github.com/reybrally/NetControl-Plane/ncp/internal/ncp/ports/outbound"
)

type CheckReachability struct {
	Intents outbound.IntentRepo
}

type ReachabilityQuery struct {
	IntentID uuid.UUID
	Revision *int

	FromNamespace string
	FromSelector  map[string]string

	ToNamespace string
	ToService   string

	Port      int
	Protocol  intent.Protocol
	Direction intent.Direction
}

func (uc CheckReachability) Handle(ctx context.Context, actor inbound.Actor, q ReachabilityQuery) (reachability.Result, error) {
	if q.Port <= 0 || q.Port > 65535 {
		return reachability.Result{}, ncperr.InvalidArgument("bad port", map[string]any{
			"port": q.Port,
		}, nil)
	}
	if q.Protocol == "" {
		q.Protocol = intent.ProtoTCP
	}
	if q.Direction == "" {
		q.Direction = intent.DirEgress
	}
	if q.FromNamespace == "" || q.ToNamespace == "" || q.ToService == "" {
		return reachability.Result{}, ncperr.InvalidArgument("fromNamespace/toNamespace/toService are required", map[string]any{
			"required": []string{"fromNamespace", "toNamespace", "toService"},
		}, nil)
	}

	if q.IntentID == uuid.Nil {
		if q.Revision != nil {
			return reachability.Result{}, ncperr.InvalidArgument("revision requires intentId", nil, nil)
		}
		return uc.handleGlobal(ctx, q)
	}

	return uc.handleSingle(ctx, q.IntentID, q.Revision, q)
}

func (uc CheckReachability) handleGlobal(ctx context.Context, q ReachabilityQuery) (reachability.Result, error) {
	intents, err := uc.Intents.ListIntents(ctx, 200)
	if err != nil {
		return reachability.Result{}, err
	}

	var bestDenied *reachability.Result
	candidates := 0

	for _, it := range intents {
		if it.CurrentRevision == nil || *it.CurrentRevision <= 0 {
			continue
		}

		revNum := int(*it.CurrentRevision)
		rev, err := uc.Intents.GetRevision(ctx, it.ID, revNum)
		if err != nil {

			continue
		}

		spec := rev.Spec

		if spec.Subject.Namespace != q.FromNamespace {
			continue
		}
		if !selectorMatch(spec.Subject.Selector, q.FromSelector) {
			continue
		}

		destOK := false
		for _, d := range spec.Destinations {
			if strings.EqualFold(d.Type, "service") && d.Service != nil {
				if d.Service.Namespace == q.ToNamespace && d.Service.Name == q.ToService {
					destOK = true
					break
				}
			}
		}
		if !destOK {
			continue
		}

		candidates++

		for _, r := range spec.Rules {
			if !dirMatch(r.Direction, q.Direction) {
				continue
			}
			if r.Protocol != q.Protocol {
				continue
			}
			for _, p := range r.Ports {
				if p == q.Port {
					return reachability.Result{
						Verdict: reachability.Allowed,
						Why:     "matched rule in intent revision",
						Evidence: []any{
							evidence(it.ID, revNum),
							map[string]any{
								"rule": map[string]any{
									"direction": string(r.Direction),
									"protocol":  string(r.Protocol),
									"ports":     r.Ports,
								},
							},
						},
					}, nil
				}
			}
		}

		res := reachability.Result{
			Verdict:  reachability.Denied,
			Why:      "intent destination matched but no rule matches port/protocol/direction",
			Evidence: []any{evidence(it.ID, revNum)},
		}
		bestDenied = &res
	}

	if candidates == 0 {
		return reachability.Result{
			Verdict: reachability.Unknown,
			Why:     "no intents match subject+destination",
		}, nil
	}

	if bestDenied != nil {
		return *bestDenied, nil
	}

	return reachability.Result{
		Verdict: reachability.Unknown,
		Why:     "no intents match subject+destination",
	}, nil
}

func (uc CheckReachability) handleSingle(ctx context.Context, intentID uuid.UUID, revPtr *int, q ReachabilityQuery) (reachability.Result, error) {

	revNum := 0
	if revPtr != nil {
		revNum = *revPtr
		if revNum <= 0 {
			return reachability.Result{}, ncperr.InvalidArgument("bad revision", map[string]any{"revision": revNum}, nil)
		}
	} else {
		it, err := uc.Intents.GetIntent(ctx, intentID)
		if err != nil {
			return reachability.Result{}, err
		}
		if it.CurrentRevision == nil || *it.CurrentRevision <= 0 {
			return reachability.Result{
				Verdict: reachability.Unknown,
				Why:     "intent has no currentRevision",
			}, nil
		}
		revNum = int(*it.CurrentRevision)
	}

	rev, err := uc.Intents.GetRevision(ctx, intentID, revNum)
	if err != nil {
		return reachability.Result{}, err
	}

	spec := rev.Spec

	if spec.Subject.Namespace != q.FromNamespace {
		return reachability.Result{
			Verdict:  reachability.Denied,
			Why:      "subject namespace does not match",
			Evidence: []any{evidence(intentID, revNum)},
		}, nil
	}
	if !selectorMatch(spec.Subject.Selector, q.FromSelector) {
		return reachability.Result{
			Verdict:  reachability.Denied,
			Why:      "subject selector does not match",
			Evidence: []any{evidence(intentID, revNum)},
		}, nil
	}

	destOK := false
	for _, d := range spec.Destinations {
		if strings.EqualFold(d.Type, "service") && d.Service != nil {
			if d.Service.Namespace == q.ToNamespace && d.Service.Name == q.ToService {
				destOK = true
				break
			}
		}
	}
	if !destOK {
		return reachability.Result{
			Verdict:  reachability.Denied,
			Why:      "no destination matches target service",
			Evidence: []any{evidence(intentID, revNum)},
		}, nil
	}

	for _, r := range spec.Rules {
		if !dirMatch(r.Direction, q.Direction) {
			continue
		}
		if r.Protocol != q.Protocol {
			continue
		}
		for _, p := range r.Ports {
			if p == q.Port {
				return reachability.Result{
					Verdict: reachability.Allowed,
					Why:     "matched rule in intent revision",
					Evidence: []any{
						evidence(intentID, revNum),
						map[string]any{
							"rule": map[string]any{
								"direction": string(r.Direction),
								"protocol":  string(r.Protocol),
								"ports":     r.Ports,
							},
						},
					},
				}, nil
			}
		}
	}

	return reachability.Result{
		Verdict:  reachability.Denied,
		Why:      "intent destination matched but no rule matches port/protocol/direction",
		Evidence: []any{evidence(intentID, revNum)},
	}, nil
}

func selectorMatch(actual map[string]string, want map[string]string) bool {
	for k, v := range want {
		if actual[k] != v {
			return false
		}
	}
	return true
}

func dirMatch(ruleDir intent.Direction, qDir intent.Direction) bool {
	if qDir == intent.DirBoth {
		return ruleDir == intent.DirBoth
	}
	if ruleDir == intent.DirBoth {
		return qDir == intent.DirIngress || qDir == intent.DirEgress
	}
	return ruleDir == qDir
}

func evidence(intentID uuid.UUID, rev int) map[string]any {
	return map[string]any{
		"intentId": intentID.String(),
		"revision": rev,
	}
}
