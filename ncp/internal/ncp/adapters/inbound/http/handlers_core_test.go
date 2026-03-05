package http

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	ncperr "github.com/reybrally/NetControl-Plane/ncp/internal/ncp/domain/errors"
	"github.com/reybrally/NetControl-Plane/ncp/internal/ncp/domain/intent"
	"github.com/reybrally/NetControl-Plane/ncp/internal/ncp/domain/reachability"
	"github.com/reybrally/NetControl-Plane/ncp/internal/ncp/ports/inbound"
)

type fakeIntentService struct {
	createIntentFn   func(ctx context.Context, actor inbound.Actor, key, title, ownerTeam string, labels map[string]string) (uuid.UUID, error)
	createRevisionFn func(ctx context.Context, actor inbound.Actor, intentID uuid.UUID, spec intent.Spec, ticketRef, justification string, ttlSeconds *int) (int, error)
	planIntentFn     func(ctx context.Context, actor inbound.Actor, intentID uuid.UUID, revision int) (uuid.UUID, error)
	applyPlanFn      func(ctx context.Context, actor inbound.Actor, planID uuid.UUID) error
	getPlanFn        func(ctx context.Context, actor inbound.Actor, planID uuid.UUID) (map[string]any, error)
	getIntentFn      func(ctx context.Context, actor inbound.Actor, intentID uuid.UUID) (map[string]any, error)
	listRevisionsFn  func(ctx context.Context, actor inbound.Actor, intentID uuid.UUID) ([]map[string]any, error)
	listAuditFn      func(ctx context.Context, actor inbound.Actor, entityType, entityID string, limit int) ([]map[string]any, error)
}

func (f fakeIntentService) CreateIntent(ctx context.Context, actor inbound.Actor, key, title, ownerTeam string, labels map[string]string) (uuid.UUID, error) {
	if f.createIntentFn != nil {
		return f.createIntentFn(ctx, actor, key, title, ownerTeam, labels)
	}
	return uuid.Nil, nil
}

func (f fakeIntentService) CreateRevision(ctx context.Context, actor inbound.Actor, intentID uuid.UUID, spec intent.Spec, ticketRef, justification string, ttlSeconds *int) (int, error) {
	if f.createRevisionFn != nil {
		return f.createRevisionFn(ctx, actor, intentID, spec, ticketRef, justification, ttlSeconds)
	}
	return 0, nil
}

func (f fakeIntentService) PlanIntent(ctx context.Context, actor inbound.Actor, intentID uuid.UUID, revision int) (uuid.UUID, error) {
	if f.planIntentFn != nil {
		return f.planIntentFn(ctx, actor, intentID, revision)
	}
	return uuid.Nil, nil
}

func (f fakeIntentService) ApplyPlan(ctx context.Context, actor inbound.Actor, planID uuid.UUID) error {
	if f.applyPlanFn != nil {
		return f.applyPlanFn(ctx, actor, planID)
	}
	return nil
}

func (f fakeIntentService) GetPlan(ctx context.Context, actor inbound.Actor, planID uuid.UUID) (map[string]any, error) {
	if f.getPlanFn != nil {
		return f.getPlanFn(ctx, actor, planID)
	}
	return map[string]any{}, nil
}

func (f fakeIntentService) GetIntent(ctx context.Context, actor inbound.Actor, intentID uuid.UUID) (map[string]any, error) {
	if f.getIntentFn != nil {
		return f.getIntentFn(ctx, actor, intentID)
	}
	return map[string]any{}, nil
}

func (f fakeIntentService) ListRevisions(ctx context.Context, actor inbound.Actor, intentID uuid.UUID) ([]map[string]any, error) {
	if f.listRevisionsFn != nil {
		return f.listRevisionsFn(ctx, actor, intentID)
	}
	return nil, nil
}

func (f fakeIntentService) ListAudit(ctx context.Context, actor inbound.Actor, entityType, entityID string, limit int) ([]map[string]any, error) {
	if f.listAuditFn != nil {
		return f.listAuditFn(ctx, actor, entityType, entityID, limit)
	}
	return nil, nil
}

type fakeReachabilityService struct {
	checkFn func(ctx context.Context, actor inbound.Actor, q inbound.ReachabilityQuery) (reachability.Result, error)
}

func (f fakeReachabilityService) CheckReachability(ctx context.Context, actor inbound.Actor, q inbound.ReachabilityQuery) (reachability.Result, error) {
	if f.checkFn != nil {
		return f.checkFn(ctx, actor, q)
	}
	return reachability.Result{}, nil
}

func withActor(req *http.Request) *http.Request {
	return req.WithContext(context.WithValue(req.Context(), actorKey, inbound.Actor{
		ID:    "dev",
		Roles: []string{"admin"},
		Team:  "platform",
	}))
}

func decodeBody(t *testing.T, body string) map[string]any {
	t.Helper()
	var out map[string]any
	if err := json.Unmarshal([]byte(body), &out); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	return out
}

func TestIntentCreateBadJSON(t *testing.T) {
	r := chi.NewRouter()
	IntentHandlers{Svc: fakeIntentService{}}.Register(r)

	req := httptest.NewRequest(http.MethodPost, "/intents", strings.NewReader("{bad"))
	req = withActor(req)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
	body := decodeBody(t, w.Body.String())
	if body["code"] != string(ncperr.CodeInvalidArgument) {
		t.Fatalf("expected INVALID_ARGUMENT, got %v", body["code"])
	}
}

func TestIntentCreateSuccess(t *testing.T) {
	newID := uuid.New()
	r := chi.NewRouter()
	IntentHandlers{Svc: fakeIntentService{
		createIntentFn: func(ctx context.Context, actor inbound.Actor, key, title, ownerTeam string, labels map[string]string) (uuid.UUID, error) {
			if key != "demo.svc-a-to-svc-b" || title == "" || ownerTeam == "" {
				t.Fatalf("unexpected input: key=%s title=%s ownerTeam=%s", key, title, ownerTeam)
			}
			return newID, nil
		},
	}}.Register(r)

	req := httptest.NewRequest(http.MethodPost, "/intents", strings.NewReader(`{"key":"demo.svc-a-to-svc-b","title":"svc-a to svc-b","ownerTeam":"payments"}`))
	req = withActor(req)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body=%s", w.Code, w.Body.String())
	}
	body := decodeBody(t, w.Body.String())
	if body["id"] != newID.String() {
		t.Fatalf("expected id %s, got %v", newID.String(), body["id"])
	}
}

func TestPlanIntentRequiresRevisionQuery(t *testing.T) {
	r := chi.NewRouter()
	PlanHandlers{Svc: fakeIntentService{}}.Register(r)
	id := uuid.New()

	req := httptest.NewRequest(http.MethodPost, "/intents/"+id.String()+"/plan", nil)
	req = withActor(req)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
	body := decodeBody(t, w.Body.String())
	if body["code"] != string(ncperr.CodeInvalidArgument) {
		t.Fatalf("expected INVALID_ARGUMENT, got %v", body["code"])
	}
}

func TestReachabilityBadProtocol(t *testing.T) {
	r := chi.NewRouter()
	ReachabilityHandlers{Svc: fakeReachabilityService{}}.Register(r)
	req := httptest.NewRequest(http.MethodGet, "/reachability?fromNamespace=payments&toNamespace=payments&toService=svc-b&port=443&protocol=icmp", nil)
	req = withActor(req)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
	body := decodeBody(t, w.Body.String())
	if body["code"] != string(ncperr.CodeInvalidArgument) {
		t.Fatalf("expected INVALID_ARGUMENT, got %v", body["code"])
	}
}

func TestReachabilitySuccessDefaultsApplied(t *testing.T) {
	r := chi.NewRouter()
	ReachabilityHandlers{Svc: fakeReachabilityService{
		checkFn: func(ctx context.Context, actor inbound.Actor, q inbound.ReachabilityQuery) (reachability.Result, error) {
			if q.Protocol != intent.ProtoTCP {
				t.Fatalf("expected default protocol TCP, got %s", q.Protocol)
			}
			if q.Direction != intent.DirEgress {
				t.Fatalf("expected default direction egress, got %s", q.Direction)
			}
			if q.FromSelector["app"] != "svc-a" {
				t.Fatalf("expected selector app=svc-a, got %+v", q.FromSelector)
			}
			return reachability.Result{
				Verdict: reachability.Allowed,
				Why:     "ok",
			}, nil
		},
	}}.Register(r)

	req := httptest.NewRequest(http.MethodGet, "/reachability?fromNamespace=payments&fromSelector=app=svc-a&toNamespace=payments&toService=svc-b&port=443", nil)
	req = withActor(req)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body=%s", w.Code, w.Body.String())
	}
	body := decodeBody(t, w.Body.String())
	if body["verdict"] != string(reachability.Allowed) {
		t.Fatalf("expected verdict allowed, got %v", body["verdict"])
	}
}

func TestAuthMiddlewareMissingToken(t *testing.T) {
	r := chi.NewRouter()
	r.Use(AuthMiddleware(AuthConfig{
		Mode:     "dev",
		DevToken: "secret",
	}))
	r.Get("/x", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	})

	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
	body := decodeBody(t, w.Body.String())
	if body["code"] != string(ncperr.CodeUnauthenticated) {
		t.Fatalf("expected UNAUTHENTICATED, got %v", body["code"])
	}
}

func TestAuthMiddlewareNonDevMode(t *testing.T) {
	r := chi.NewRouter()
	r.Use(AuthMiddleware(AuthConfig{
		Mode:     "jwt",
		DevToken: "secret",
	}))
	r.Get("/x", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	})

	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.Header.Set("Authorization", "Bearer secret")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
	body := decodeBody(t, w.Body.String())
	if body["code"] != string(ncperr.CodeInternal) {
		t.Fatalf("expected INTERNAL, got %v", body["code"])
	}
}
