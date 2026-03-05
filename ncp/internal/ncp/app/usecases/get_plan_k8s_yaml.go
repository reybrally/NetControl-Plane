package usecases

import (
	"context"

	"github.com/google/uuid"
	ncperr "github.com/reybrally/NetControl-Plane/ncp/internal/ncp/domain/errors"
	"github.com/reybrally/NetControl-Plane/ncp/internal/ncp/ports/outbound"
)

type GetPlanK8sYAML struct {
	Plans    outbound.PlanRepo
	Renderer outbound.K8sYAMLRenderer
}

func (uc GetPlanK8sYAML) Handle(ctx context.Context, planID uuid.UUID) ([]byte, error) {
	p, err := uc.Plans.GetPlan(ctx, planID)
	if err != nil {
		return nil, err
	}

	artifacts := p.Artifacts
	if artifacts == nil {
		return nil, ncperr.InvalidArgument("no artifacts", nil, nil)
	}

	k8sAny, ok := artifacts["k8s"]
	if !ok {
		return nil, ncperr.InvalidArgument("no k8s artifact", nil, nil)
	}

	k8sMap, ok := k8sAny.(map[string]any)
	if !ok {
		return nil, ncperr.InvalidArgument("bad k8s artifact", nil, nil)
	}

	if uc.Renderer == nil {
		return nil, ncperr.Internal("k8s yaml renderer is not configured", nil, nil)
	}

	y, err := uc.Renderer.RenderNetworkPolicyYAML(k8sMap)
	if err != nil {
		return nil, err
	}
	return y, nil
}
