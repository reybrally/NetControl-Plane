package k8s

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"

	netv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type DiscoveredPolicy struct {
	Namespace string                  `json:"namespace"`
	Name      string                  `json:"name"`
	Labels    map[string]string       `json:"labels,omitempty"`
	Spec      netv1.NetworkPolicySpec `json:"spec"`
}

func DiscoverManagedNetworkPolicies(ctx context.Context, cs kubernetes.Interface, namespace string) ([]DiscoveredPolicy, error) {
	selector := "managed-by=ncp"

	var list *netv1.NetworkPolicyList
	var err error

	if namespace == "" || namespace == "*" {
		list, err = cs.NetworkingV1().NetworkPolicies(metav1.NamespaceAll).List(ctx, metav1.ListOptions{
			LabelSelector: selector,
		})
	} else {
		list, err = cs.NetworkingV1().NetworkPolicies(namespace).List(ctx, metav1.ListOptions{
			LabelSelector: selector,
		})
	}
	if err != nil {
		return nil, err
	}

	out := make([]DiscoveredPolicy, 0, len(list.Items))
	for i := range list.Items {
		np := &list.Items[i]
		out = append(out, DiscoveredPolicy{
			Namespace: np.Namespace,
			Name:      np.Name,
			Labels:    np.Labels,
			Spec:      np.Spec,
		})
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].Namespace == out[j].Namespace {
			return out[i].Name < out[j].Name
		}
		return out[i].Namespace < out[j].Namespace
	})

	return out, nil
}

func PolicyKey(p DiscoveredPolicy) string {
	return p.Namespace + "/" + p.Name
}

func HashPolicy(p DiscoveredPolicy) (string, error) {

	type stable struct {
		Namespace string                  `json:"namespace"`
		Name      string                  `json:"name"`
		Spec      netv1.NetworkPolicySpec `json:"spec"`
	}
	b, err := json.Marshal(stable{
		Namespace: p.Namespace,
		Name:      p.Name,
		Spec:      p.Spec,
	})
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:]), nil
}

func IndexAndHashPolicies(policies []DiscoveredPolicy) (overallHash string, byKey map[string]string, details map[string]any, err error) {
	byKey = make(map[string]string, len(policies))

	sort.Slice(policies, func(i, j int) bool {
		ki := PolicyKey(policies[i])
		kj := PolicyKey(policies[j])
		return ki < kj
	})

	type pair struct {
		Key  string `json:"key"`
		Hash string `json:"hash"`
	}
	pairs := make([]pair, 0, len(policies))

	for _, p := range policies {
		k := PolicyKey(p)
		h, err := HashPolicy(p)
		if err != nil {
			return "", nil, nil, err
		}
		byKey[k] = h
		pairs = append(pairs, pair{Key: k, Hash: h})
	}

	b, err := json.Marshal(pairs)
	if err != nil {
		return "", nil, nil, err
	}
	sum := sha256.Sum256(b)
	overallHash = hex.EncodeToString(sum[:])

	details = map[string]any{
		"count": len(policies),
	}
	if len(policies) > 0 {
		preview := make([]map[string]string, 0, min(len(policies), 20))
		for _, p := range policies {
			preview = append(preview, map[string]string{
				"namespace": p.Namespace,
				"name":      p.Name,
			})
			if len(preview) >= 20 {
				break
			}
		}
		details["policies"] = preview
	}

	return overallHash, byKey, details, nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
