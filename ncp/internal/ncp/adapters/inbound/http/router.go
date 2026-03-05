package http

import (
	"fmt"
	stdhttp "net/http"

	"github.com/go-chi/chi/v5"
	"github.com/reybrally/NetControl-Plane/ncp/internal/ncp/app/usecases"
	"github.com/reybrally/NetControl-Plane/ncp/internal/ncp/ports/inbound"
	"github.com/reybrally/NetControl-Plane/ncp/internal/ncp/ports/outbound"
)

type RouterDeps struct {
	Auth AuthConfig

	HealthHandler   stdhttp.HandlerFunc
	OpenAPISpecPath string

	IntentSvc       inbound.IntentService
	ReachabilitySvc inbound.ReachabilityService

	JobsRepo outbound.JobRepo
	Plans    outbound.PlanRepo
	Drift    outbound.DriftRepo
	Tx       outbound.Tx
	Jobs     outbound.JobQueue
	Audit    outbound.AuditRepo

	K8sYAMLRenderer outbound.K8sYAMLRenderer
}

func BuildRouter(deps RouterDeps) (*chi.Mux, error) {
	if deps.HealthHandler == nil {
		return nil, fmt.Errorf("health handler is required")
	}
	if deps.IntentSvc == nil {
		return nil, fmt.Errorf("intent service is required")
	}
	if deps.ReachabilitySvc == nil {
		return nil, fmt.Errorf("reachability service is required")
	}
	if deps.JobsRepo == nil || deps.Plans == nil || deps.Drift == nil || deps.Tx == nil || deps.Jobs == nil || deps.Audit == nil {
		return nil, fmt.Errorf("router dependencies are incomplete")
	}

	r := chi.NewRouter()
	r.Get("/health", deps.HealthHandler)

	if deps.OpenAPISpecPath != "" {
		r.Get("/openapi.yaml", func(w stdhttp.ResponseWriter, req *stdhttp.Request) {
			w.Header().Set("Content-Type", "application/yaml")
			stdhttp.ServeFile(w, req, deps.OpenAPISpecPath)
		})
	}

	r.Group(func(pr chi.Router) {
		pr.Use(AuthMiddleware(deps.Auth))

		ih := IntentHandlers{Svc: deps.IntentSvc}
		ih.Register(pr)
		ih.RegisterRead(pr)

		jobGetUC := usecases.GetJob{Repo: deps.JobsRepo}
		k8sArtifactUC := usecases.GetPlanK8sYAML{
			Plans:    deps.Plans,
			Renderer: deps.K8sYAMLRenderer,
		}
		driftListUC := usecases.ListDrift{Repo: deps.Drift}
		driftReconcileUC := usecases.ReconcileDrift{Tx: deps.Tx, Jobs: deps.Jobs, Audit: deps.Audit}

		PlanHandlers{Svc: deps.IntentSvc}.Register(pr)
		AuditHandlers{Svc: deps.IntentSvc}.Register(pr)
		PlanArtifactsHandlers{UC: k8sArtifactUC}.Register(pr)
		ReachabilityHandlers{Svc: deps.ReachabilitySvc}.Register(pr)
		JobHandlers{UC: jobGetUC}.Register(pr)
		DriftHandlers{
			UC:          driftListUC,
			ReconcileUC: driftReconcileUC,
		}.Register(pr)
	})

	return r, nil
}
