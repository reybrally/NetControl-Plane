package wiring

import (
	inhttp "github.com/reybrally/NetControl-Plane/ncp/internal/ncp/adapters/inbound/http"
	"github.com/reybrally/NetControl-Plane/ncp/internal/ncp/bootstrap/config"
)

func AuthConfig(cfg config.Config) inhttp.AuthConfig {
	return inhttp.AuthConfig{
		Mode:     cfg.AuthMode,
		DevToken: cfg.DevToken,
	}
}
