package k8s

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

type Client struct {
	Kube *kubernetes.Clientset
}

type Config struct {
	Kubeconfig string
	Context    string
}

func New(ctx context.Context, cfg Config) (*Client, error) {
	restCfg, err := buildRestConfig(cfg)
	if err != nil {
		return nil, err
	}
	k, err := kubernetes.NewForConfig(restCfg)
	if err != nil {
		return nil, err
	}
	return &Client{Kube: k}, nil
}

func buildRestConfig(cfg Config) (*rest.Config, error) {

	if cfg.Kubeconfig != "" {
		path := cfg.Kubeconfig
		if path[:1] == "~" {
			home, _ := os.UserHomeDir()
			path = filepath.Join(home, path[1:])
		}
		loading := &clientcmd.ClientConfigLoadingRules{ExplicitPath: path}
		overrides := &clientcmd.ConfigOverrides{}
		if cfg.Context != "" {
			overrides.CurrentContext = cfg.Context
		}
		return clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loading, overrides).ClientConfig()
	}

	return rest.InClusterConfig()
}

func (c *Client) Must() *kubernetes.Clientset {
	if c == nil || c.Kube == nil {
		panic(fmt.Errorf("k8s client is nil"))
	}
	return c.Kube
}
