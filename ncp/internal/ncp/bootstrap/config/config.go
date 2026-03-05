package config

import "os"

type Config struct {
	HTTPAddr string
	DBDSN    string

	AuthMode string
	DevToken string

	ApplyMode   string
	Kubeconfig  string
	KubeContext string
	DryRun      bool
}

func Load() Config {
	LoadDotEnv(".env")
	return Config{
		HTTPAddr: env("NCP_HTTP_ADDR", ":8080"),
		DBDSN:    env("NCP_DB_DSN", ""),

		AuthMode:    env("NCP_AUTH_MODE", "dev"),
		DevToken:    env("NCP_DEV_TOKEN", "devtoken"),
		ApplyMode:   env("NCP_APPLY_MODE", "noop"),
		Kubeconfig:  env("NCP_KUBECONFIG", ""),
		KubeContext: env("NCP_KUBE_CONTEXT", ""),
		DryRun:      envBool("NCP_DRY_RUN", false),
	}
}

func env(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func envBool(k string, def bool) bool {
	v := os.Getenv(k)
	if v == "" {
		return def
	}
	return v == "1" || v == "true" || v == "TRUE" || v == "yes"
}
