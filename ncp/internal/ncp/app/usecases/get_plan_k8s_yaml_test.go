package usecases

import (
	"context"
	"testing"

	"github.com/google/uuid"
	ncperr "github.com/reybrally/NetControl-Plane/ncp/internal/ncp/domain/errors"
	"github.com/reybrally/NetControl-Plane/ncp/internal/ncp/ports/outbound"
)

type fakePlanRepoForYAML struct {
	plan outbound.Plan
	err  error
}

func (f fakePlanRepoForYAML) CreatePlan(ctx context.Context, p outbound.Plan) error {
	return nil
}
func (f fakePlanRepoForYAML) GetPlan(ctx context.Context, id uuid.UUID) (outbound.Plan, error) {
	return f.plan, f.err
}
func (f fakePlanRepoForYAML) UpdatePlanStatus(ctx context.Context, id uuid.UUID, status string) error {
	return nil
}
func (f fakePlanRepoForYAML) SetApplyJobOnce(ctx context.Context, planID uuid.UUID, jobID uuid.UUID) (ok bool, err error) {
	return false, nil
}
func (f fakePlanRepoForYAML) ListLatestAppliedPlans(ctx context.Context, limit int) ([]outbound.Plan, error) {
	return nil, nil
}
func (f fakePlanRepoForYAML) ListLatestAppliedPlansPerIntentWithTTL(ctx context.Context, limit int) ([]outbound.ExpirablePlan, error) {
	return nil, nil
}
func (f fakePlanRepoForYAML) ListAppliedPlansByIntent(ctx context.Context, intentID uuid.UUID, limit int) ([]outbound.Plan, error) {
	return nil, nil
}
func (f fakePlanRepoForYAML) ListTTLExpiredCandidates(ctx context.Context, limit int) ([]uuid.UUID, error) {
	return nil, nil
}
func (f fakePlanRepoForYAML) MarkPlanExpiredOnce(ctx context.Context, planID uuid.UUID) (bool, error) {
	return false, nil
}
func (f fakePlanRepoForYAML) SetAppliedK8sRef(ctx context.Context, planID uuid.UUID, namespace, name string) error {
	return nil
}

type fakeYAMLRenderer struct {
	called bool
	got    map[string]any
	out    []byte
	err    error
}

func (f *fakeYAMLRenderer) RenderNetworkPolicyYAML(obj map[string]any) ([]byte, error) {
	f.called = true
	f.got = obj
	return f.out, f.err
}

func TestGetPlanK8sYAMLNoK8sArtifact(t *testing.T) {
	uc := GetPlanK8sYAML{
		Plans: fakePlanRepoForYAML{
			plan: outbound.Plan{
				ID:        uuid.New(),
				Artifacts: map[string]any{"aws": map[string]any{"kind": "x"}},
			},
		},
		Renderer: &fakeYAMLRenderer{},
	}

	_, err := uc.Handle(context.Background(), uuid.New())
	if err == nil {
		t.Fatal("expected error")
	}
	if ncperr.As(err).Code != ncperr.CodeInvalidArgument {
		t.Fatalf("expected INVALID_ARGUMENT, got %s", ncperr.As(err).Code)
	}
}

func TestGetPlanK8sYAMLSuccess(t *testing.T) {
	renderer := &fakeYAMLRenderer{out: []byte("kind: NetworkPolicy\n")}
	uc := GetPlanK8sYAML{
		Plans: fakePlanRepoForYAML{
			plan: outbound.Plan{
				ID: uuid.New(),
				Artifacts: map[string]any{
					"k8s": map[string]any{
						"kind": "NetworkPolicy",
					},
				},
			},
		},
		Renderer: renderer,
	}

	out, err := uc.Handle(context.Background(), uuid.New())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(out) != "kind: NetworkPolicy\n" {
		t.Fatalf("unexpected renderer output: %s", string(out))
	}
	if !renderer.called {
		t.Fatal("renderer was not called")
	}
}
