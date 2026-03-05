package k8s

import "sigs.k8s.io/yaml"

type ArtifactYAMLRenderer struct{}

func (ArtifactYAMLRenderer) RenderNetworkPolicyYAML(obj map[string]any) ([]byte, error) {
	np, _, err := RenderNetworkPolicy(obj)
	if err != nil {
		return nil, err
	}
	return yaml.Marshal(np)
}
