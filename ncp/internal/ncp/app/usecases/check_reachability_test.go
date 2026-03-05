package usecases

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	ncperr "github.com/reybrally/NetControl-Plane/ncp/internal/ncp/domain/errors"
	"github.com/reybrally/NetControl-Plane/ncp/internal/ncp/domain/intent"
	"github.com/reybrally/NetControl-Plane/ncp/internal/ncp/domain/reachability"
	"github.com/reybrally/NetControl-Plane/ncp/internal/ncp/ports/inbound"
	"github.com/reybrally/NetControl-Plane/ncp/internal/ncp/ports/outbound"
)

type fakeIntentRepo struct {
	intentsByID map[uuid.UUID]outbound.Intent
	listIntents []outbound.Intent
	revisions   map[string]outbound.Revision
}

func revKey(id uuid.UUID, rev int) string {
	return fmt.Sprintf("%s:%d", id.String(), rev)
}

func (f *fakeIntentRepo) CreateIntent(ctx context.Context, it outbound.Intent) error {
	return nil
}

func (f *fakeIntentRepo) GetIntent(ctx context.Context, id uuid.UUID) (outbound.Intent, error) {
	it, ok := f.intentsByID[id]
	if !ok {
		return outbound.Intent{}, ncperr.NotFound("intent not found", map[string]any{"id": id.String()}, nil)
	}
	return it, nil
}

func (f *fakeIntentRepo) ListIntents(ctx context.Context, limit int) ([]outbound.Intent, error) {
	return f.listIntents, nil
}

func (f *fakeIntentRepo) ListRevisions(ctx context.Context, intentID uuid.UUID, limit int) ([]outbound.Revision, error) {
	return nil, nil
}

func (f *fakeIntentRepo) CreateRevision(ctx context.Context, rev outbound.Revision) (int, error) {
	return 0, nil
}

func (f *fakeIntentRepo) GetRevision(ctx context.Context, intentID uuid.UUID, revision int) (outbound.Revision, error) {
	rev, ok := f.revisions[revKey(intentID, revision)]
	if !ok {
		return outbound.Revision{}, ncperr.NotFound("revision not found", map[string]any{
			"intentId": intentID.String(),
			"revision": revision,
		}, nil)
	}
	return rev, nil
}

func (f *fakeIntentRepo) RefreshNotAfterOnApply(ctx context.Context, revisionID int64) error {
	return nil
}

func (f *fakeIntentRepo) SetNotAfterOnFirstApply(ctx context.Context, revisionID int64, appliedAt time.Time) error {
	return nil
}

func baseReachabilitySpec() intent.Spec {
	return intent.Spec{
		Envs: []string{"dev"},
		Owner: intent.Owner{
			Team: "payments",
		},
		Subject: intent.WorkloadRef{
			Cluster:   "c1",
			Namespace: "payments",
			Selector:  map[string]string{"app": "svc-a"},
		},
		Destinations: []intent.Destination{
			{
				Type: "service",
				Service: &intent.ServiceRef{
					Cluster:   "c1",
					Namespace: "payments",
					Name:      "svc-b",
				},
			},
		},
		Rules: []intent.Rule{
			{
				Direction: intent.DirEgress,
				Protocol:  intent.ProtoTCP,
				Ports:     []int{443},
			},
		},
	}
}

func TestCheckReachabilitySingleIntentAllowed(t *testing.T) {
	id := uuid.New()
	cur := int64(1)
	spec := baseReachabilitySpec()

	repo := &fakeIntentRepo{
		intentsByID: map[uuid.UUID]outbound.Intent{
			id: {
				ID:              id,
				Key:             "payments.svc-a-to-svc-b",
				CurrentRevision: &cur,
			},
		},
		revisions: map[string]outbound.Revision{
			revKey(id, 1): {
				IntentID: id,
				Revision: 1,
				Spec:     spec,
			},
		},
	}

	uc := CheckReachability{Intents: repo}
	res, err := uc.Handle(context.Background(), inbound.Actor{ID: "dev"}, ReachabilityQuery{
		IntentID:      id,
		FromNamespace: "payments",
		FromSelector:  map[string]string{"app": "svc-a"},
		ToNamespace:   "payments",
		ToService:     "svc-b",
		Port:          443,
		Protocol:      intent.ProtoTCP,
		Direction:     intent.DirEgress,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Verdict != reachability.Allowed {
		t.Fatalf("expected allowed, got %s", res.Verdict)
	}
}

func TestCheckReachabilityGlobalModeAllowed(t *testing.T) {
	id := uuid.New()
	cur := int64(1)
	spec := baseReachabilitySpec()

	repo := &fakeIntentRepo{
		listIntents: []outbound.Intent{
			{
				ID:              id,
				Key:             "payments.svc-a-to-svc-b",
				CurrentRevision: &cur,
			},
		},
		revisions: map[string]outbound.Revision{
			revKey(id, 1): {
				IntentID: id,
				Revision: 1,
				Spec:     spec,
			},
		},
	}

	uc := CheckReachability{Intents: repo}
	res, err := uc.Handle(context.Background(), inbound.Actor{ID: "dev"}, ReachabilityQuery{
		FromNamespace: "payments",
		FromSelector:  map[string]string{"app": "svc-a"},
		ToNamespace:   "payments",
		ToService:     "svc-b",
		Port:          443,
		Protocol:      intent.ProtoTCP,
		Direction:     intent.DirEgress,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Verdict != reachability.Allowed {
		t.Fatalf("expected allowed, got %s", res.Verdict)
	}
}

func TestCheckReachabilityRevisionRequiresIntent(t *testing.T) {
	uc := CheckReachability{Intents: &fakeIntentRepo{}}
	rev := 1

	_, err := uc.Handle(context.Background(), inbound.Actor{ID: "dev"}, ReachabilityQuery{
		Revision:      &rev,
		FromNamespace: "payments",
		FromSelector:  map[string]string{"app": "svc-a"},
		ToNamespace:   "payments",
		ToService:     "svc-b",
		Port:          443,
		Protocol:      intent.ProtoTCP,
		Direction:     intent.DirEgress,
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if ncperr.As(err).Code != ncperr.CodeInvalidArgument {
		t.Fatalf("expected INVALID_ARGUMENT, got %s", ncperr.As(err).Code)
	}
}

func TestCheckReachabilityGlobalModeUnknownWhenNoCandidates(t *testing.T) {
	id := uuid.New()
	cur := int64(1)
	spec := baseReachabilitySpec()
	spec.Subject.Namespace = "other-namespace"

	repo := &fakeIntentRepo{
		listIntents: []outbound.Intent{
			{
				ID:              id,
				Key:             "payments.svc-a-to-svc-b",
				CurrentRevision: &cur,
			},
		},
		revisions: map[string]outbound.Revision{
			revKey(id, 1): {
				IntentID: id,
				Revision: 1,
				Spec:     spec,
			},
		},
	}

	uc := CheckReachability{Intents: repo}
	res, err := uc.Handle(context.Background(), inbound.Actor{ID: "dev"}, ReachabilityQuery{
		FromNamespace: "payments",
		FromSelector:  map[string]string{"app": "svc-a"},
		ToNamespace:   "payments",
		ToService:     "svc-b",
		Port:          443,
		Protocol:      intent.ProtoTCP,
		Direction:     intent.DirEgress,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Verdict != reachability.Unknown {
		t.Fatalf("expected unknown, got %s", res.Verdict)
	}
}
