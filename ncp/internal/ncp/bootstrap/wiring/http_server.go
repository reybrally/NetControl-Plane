package wiring

import (
	inhttp "github.com/reybrally/NetControl-Plane/ncp/internal/ncp/adapters/inbound/http"
	"github.com/reybrally/NetControl-Plane/ncp/internal/ncp/bootstrap/runtime"
)

const openAPISpecPath = "api/openapi.yaml"

func BuildHTTPRouter(c *Container) (*inhttp.Server, error) {
	health := runtime.HealthHandler{DB: c.DB.Pool}

	r, err := inhttp.BuildRouter(inhttp.RouterDeps{
		Auth:            c.HTTPAuth,
		HealthHandler:   health.ServeHTTP,
		OpenAPISpecPath: openAPISpecPath,
		IntentSvc:       c.IntentSvc,
		ReachabilitySvc: c.ReachabilitySvc,
		JobsRepo:        c.JobsRepo,
		Plans:           c.Plans,
		Drift:           c.Drift,
		Tx:              c.Tx,
		Jobs:            c.Jobs,
		Audit:           c.Audit,
		K8sYAMLRenderer: c.K8sYAMLRenderer,
	})
	if err != nil {
		return nil, err
	}

	return &inhttp.Server{Router: r}, nil
}
