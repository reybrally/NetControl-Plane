package wiring

import (
	"context"

	inhttp "github.com/reybrally/NetControl-Plane/ncp/internal/ncp/adapters/inbound/http"
	k8sout "github.com/reybrally/NetControl-Plane/ncp/internal/ncp/adapters/outbound/k8s"
	pg "github.com/reybrally/NetControl-Plane/ncp/internal/ncp/adapters/outbound/postgres"

	"github.com/reybrally/NetControl-Plane/ncp/internal/ncp/app/usecases"
	"github.com/reybrally/NetControl-Plane/ncp/internal/ncp/bootstrap/config"
	"github.com/reybrally/NetControl-Plane/ncp/internal/ncp/ports/inbound"
	"github.com/reybrally/NetControl-Plane/ncp/internal/ncp/ports/outbound"
)

type Container struct {
	DB *pg.DB

	Tx       pg.TxRunner
	JobsRepo pg.JobRepo
	Intents  pg.IntentRepo
	Plans    pg.PlanRepo
	Audit    pg.AuditRepo
	Jobs     pg.JobQueue
	Drift    pg.DriftRepo

	IntentSvc       inbound.IntentService
	ReachabilitySvc inbound.ReachabilityService

	K8sApplier      *k8sout.Applier
	K8sYAMLRenderer outbound.K8sYAMLRenderer

	HTTPAuth inhttp.AuthConfig
	Cfg      config.Config
}

func Build(ctx context.Context, cfg config.Config, auth inhttp.AuthConfig) (*Container, error) {
	db, err := pg.New(ctx, cfg.DBDSN)
	if err != nil {
		return nil, err
	}

	c := &Container{
		DB:              db,
		Tx:              pg.TxRunner{DB: db},
		Intents:         pg.IntentRepo{DB: db},
		Plans:           pg.PlanRepo{DB: db},
		Audit:           pg.AuditRepo{DB: db},
		Jobs:            pg.JobQueue{DB: db},
		Drift:           pg.DriftRepo{DB: db},
		JobsRepo:        pg.JobRepo{DB: db},
		HTTPAuth:        auth,
		Cfg:             cfg,
		K8sYAMLRenderer: k8sout.ArtifactYAMLRenderer{},
	}

	c.IntentSvc = &usecases.IntentService{
		Tx:      c.Tx,
		Intents: &c.Intents,
		Plans:   c.Plans,
		Jobs:    c.Jobs,
		Audit:   c.Audit,
	}

	c.ReachabilitySvc = usecases.ReachabilityService{
		UC: usecases.CheckReachability{Intents: &c.Intents},
	}

	if cfg.ApplyMode == "k8s" {
		client, err := k8sout.New(ctx, k8sout.Config{Kubeconfig: cfg.Kubeconfig, Context: cfg.KubeContext})
		if err != nil {
			return nil, err
		}
		c.K8sApplier = k8sout.NewApplier(client)
	}

	return c, nil
}
