package http

import (
	"encoding/json"
	"net/http"
	"strconv"

	ncperr "github.com/reybrally/NetControl-Plane/ncp/internal/ncp/domain/errors"
	"github.com/reybrally/NetControl-Plane/ncp/internal/ncp/ports/inbound"
)

type AuditHandlers struct {
	Svc inbound.IntentService
}

func (h AuditHandlers) Register(r interface {
	Get(pattern string, hfn http.HandlerFunc)
}) {

	r.Get("/audits", h.list)
}

func (h AuditHandlers) list(w http.ResponseWriter, r *http.Request) {
	actor := ActorFrom(r.Context())

	entityType := r.URL.Query().Get("entityType")
	entityID := r.URL.Query().Get("entityId")

	limit := 100
	if v := r.URL.Query().Get("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n <= 0 || n > 1000 {
			writeError(w, ncperr.InvalidArgument("bad limit", map[string]any{
				"limit": v,
			}, err))
			return
		}
		limit = n
	}

	out, err := h.Svc.ListAudit(r.Context(), actor, entityType, entityID, limit)
	if err != nil {
		writeError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"items": out})
}
