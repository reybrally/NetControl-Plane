package http

import (
	"net/http"
	"strconv"

	"github.com/reybrally/NetControl-Plane/ncp/internal/ncp/app/usecases"
	"github.com/reybrally/NetControl-Plane/ncp/internal/ncp/domain/drift"
	ncperr "github.com/reybrally/NetControl-Plane/ncp/internal/ncp/domain/errors"
)

type DriftHandlers struct {
	UC          usecases.ListDrift
	ReconcileUC usecases.ReconcileDrift
}

func (h DriftHandlers) Register(r interface {
	Get(pattern string, hfn http.HandlerFunc)
	Post(pattern string, hfn http.HandlerFunc)
}) {
	r.Get("/drift", h.list)
	r.Post("/drift/reconcile", h.reconcile)
}

func (h DriftHandlers) list(w http.ResponseWriter, r *http.Request) {
	actor := ActorFrom(r.Context())

	scope := r.URL.Query().Get("scope")

	limit := 100
	if v := r.URL.Query().Get("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n <= 0 || n > 1000 {
			writeError(w, ncperr.InvalidArgument("bad limit", map[string]any{"limit": v}, err))
			return
		}

		limit = n
	}

	items, err := h.UC.Handle(r.Context(), actor, scope, limit)
	if items == nil {
		items = []drift.Snapshot{}
	}
	if err != nil {
		writeError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h DriftHandlers) reconcile(w http.ResponseWriter, r *http.Request) {
	actor := ActorFrom(r.Context())

	scope := r.URL.Query().Get("scope")
	namespace := r.URL.Query().Get("namespace")

	dryRun := false
	if v := r.URL.Query().Get("dryRun"); v != "" {
		b, err := strconv.ParseBool(v)
		if err != nil {
			writeError(w, ncperr.InvalidArgument("bad dryRun", map[string]any{"dryRun": v}, err))
			return
		}
		dryRun = b
	}

	jobID, err := h.ReconcileUC.Handle(r.Context(), actor, scope, namespace, dryRun)
	if err != nil {
		writeError(w, err)
		return
	}

	writeJSON(w, http.StatusAccepted, map[string]any{
		"status": "queued",
		"jobId":  jobID.String(),
	})
}
