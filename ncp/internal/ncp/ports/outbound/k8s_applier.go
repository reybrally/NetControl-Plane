package outbound

import "context"

type K8sApplier interface {
	ApplyNetworkPolicy(ctx context.Context, obj map[string]any, dryRun bool) (appliedName string, err error)
	DeleteNetworkPolicy(ctx context.Context, namespace, name string, dryRun bool) error
}
