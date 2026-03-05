package main

import (
	"context"
	"log"
	"os/signal"
	"syscall"
	"time"

	inhttp "github.com/reybrally/NetControl-Plane/ncp/internal/ncp/adapters/inbound/http"
	inworker "github.com/reybrally/NetControl-Plane/ncp/internal/ncp/adapters/inbound/worker"
	"k8s.io/client-go/kubernetes"

	"github.com/reybrally/NetControl-Plane/ncp/internal/ncp/bootstrap/config"
	"github.com/reybrally/NetControl-Plane/ncp/internal/ncp/bootstrap/wiring"
)

func main() {
	cfg := config.Load()
	if cfg.DBDSN == "" {
		log.Fatal("NCP_DB_DSN is required")
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	auth := inhttp.AuthConfig{Mode: cfg.AuthMode, DevToken: cfg.DevToken}
	c, err := wiring.Build(ctx, cfg, auth)
	if err != nil {
		log.Fatal(err)
	}
	defer c.DB.Close()

	ctxName := cfg.KubeContext
	if ctxName == "" {
		ctxName = "default"
	}

	var kubeClient kubernetes.Interface
	if cfg.Kubeconfig != "" {
		kc, err := buildKubeClient(ctx, cfg.Kubeconfig, cfg.KubeContext)
		if err != nil {
			log.Printf("drift: failed to init kube client: %v", err)
		} else {
			kubeClient = kc
		}
	} else {
		log.Printf("drift: kubeconfig is empty, will record unknown snapshots")
	}

	apply := inworker.ApplyWorker{
		Tx:      c.Tx,
		Plans:   c.Plans,
		Audit:   c.Audit,
		Mode:    c.Cfg.ApplyMode,
		DryRun:  c.Cfg.DryRun,
		K8s:     c.K8sApplier,
		Intents: &c.Intents,
	}

	expireTTL := inworker.ExpireTTLWorker{
		Tx:     c.Tx,
		Plans:  c.Plans,
		Audit:  c.Audit,
		Mode:   c.Cfg.ApplyMode,
		DryRun: c.Cfg.DryRun,
		K8s:    c.K8sApplier,
	}

	reconcileWorker := inworker.ReconcileDriftWorker{
		Plans:     c.Plans,
		Audit:     c.Audit,
		K8sClient: kubeClient,
		Applier:   c.K8sApplier,
	}

	runner := inworker.Runner{
		Queue:          c.Jobs,
		WorkerID:       "worker-1",
		ApplyPlan:      apply.Handle,
		ExpireTTL:      expireTTL.Handle,
		ReconcileDrift: reconcileWorker.Handle,
	}

	ns := "default"
	driftWorker := inworker.DriftWorker{
		Repo:      c.Drift,
		Plans:     c.Plans,
		K8s:       kubeClient,
		Namespace: ns,
		Scope:     "k8s:" + ctxName + "/" + ns,
		Interval:  20 * time.Second,
	}

	ttlScheduler := inworker.TTLWorker{
		Plans:    c.Plans,
		Jobs:     c.Jobs,
		Interval: 10 * time.Second,
		Limit:    50,
	}

	log.Printf(
		"worker started (apply_mode=%s dry_run=%v drift_scope=%s ttl_interval=%s)",
		c.Cfg.ApplyMode, c.Cfg.DryRun, driftWorker.Scope, ttlScheduler.Interval,
	)

	go runner.Run(ctx)

	go func() {
		if err := driftWorker.Run(ctx); err != nil && ctx.Err() == nil {
			log.Fatalf("drift worker failed: %v", err)
		}
	}()

	go func() {
		if err := ttlScheduler.Run(ctx); err != nil && ctx.Err() == nil {
			log.Fatalf("ttl scheduler failed: %v", err)
		}
	}()

	<-ctx.Done()
	log.Println("worker stopped")
}
