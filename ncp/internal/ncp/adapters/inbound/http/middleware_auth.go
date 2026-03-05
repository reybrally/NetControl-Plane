package http

import (
	"context"
	"net/http"
	"strings"

	ncperr "github.com/reybrally/NetControl-Plane/ncp/internal/ncp/domain/errors"
	"github.com/reybrally/NetControl-Plane/ncp/internal/ncp/ports/inbound"
)

type ctxKey string

const actorKey ctxKey = "actor"

type AuthConfig struct {
	Mode     string
	DevToken string
}

func AuthMiddleware(cfg AuthConfig) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if cfg.Mode != "dev" {
				writeError(w, ncperr.Internal("auth mode not configured", nil, nil))
				return
			}

			h := r.Header.Get("Authorization")
			if !strings.HasPrefix(h, "Bearer ") || strings.TrimPrefix(h, "Bearer ") != cfg.DevToken {

				writeError(w, ncperr.Unauthenticated("unauthorized", nil, nil))
				return
			}

			actor := inbound.Actor{ID: "dev", Roles: []string{"admin"}, Team: "platform"}
			ctx := context.WithValue(r.Context(), actorKey, actor)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func ActorFrom(ctx context.Context) inbound.Actor {
	a, _ := ctx.Value(actorKey).(inbound.Actor)
	return a
}
