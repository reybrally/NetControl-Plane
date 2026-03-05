package k8s

import (
	"context"
	"encoding/json"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

type Applier struct {
	Client       *Client
	FieldManager string
}

func NewApplier(client *Client) *Applier {
	return &Applier{Client: client, FieldManager: "ncp"}
}

func (a *Applier) ApplyNetworkPolicy(ctx context.Context, obj map[string]any, dryRun bool) (string, error) {
	if a == nil || a.Client == nil {
		return "", fmt.Errorf("k8s applier client is nil")
	}

	np, name, err := RenderNetworkPolicy(obj)
	if err != nil {
		return "", err
	}

	b, err := json.Marshal(np)
	if err != nil {
		return "", err
	}

	opts := metav1.PatchOptions{FieldManager: a.FieldManager, Force: ptr(true)}
	if dryRun {
		opts.DryRun = []string{metav1.DryRunAll}
	}

	_, err = a.Client.Must().NetworkingV1().NetworkPolicies(np.Namespace).Patch(
		ctx,
		name,
		types.ApplyPatchType,
		b,
		opts,
	)
	if err != nil {
		return "", err
	}
	return name, nil
}

func ptr[T any](v T) *T { return &v }

func (a *Applier) DeleteNetworkPolicy(ctx context.Context, namespace, name string, dryRun bool) error {
	if a == nil || a.Client == nil {
		return fmt.Errorf("k8s applier client is nil")
	}
	if namespace == "" || name == "" {
		return fmt.Errorf("namespace/name required")
	}

	opts := metav1.DeleteOptions{}
	if dryRun {
		opts.DryRun = []string{metav1.DryRunAll}
	}

	return a.Client.Must().NetworkingV1().NetworkPolicies(namespace).Delete(ctx, name, opts)
}
