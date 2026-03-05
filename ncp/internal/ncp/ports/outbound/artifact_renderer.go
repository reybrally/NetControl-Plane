package outbound

type K8sYAMLRenderer interface {
	RenderNetworkPolicyYAML(obj map[string]any) ([]byte, error)
}
