package http

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	ncperr "github.com/reybrally/NetControl-Plane/ncp/internal/ncp/domain/errors"
	"github.com/reybrally/NetControl-Plane/ncp/internal/ncp/ports/inbound"
)

func (h IntentHandlers) RegisterRead(r chi.Router) {
	r.Get("/intents/{id}", h.getIntent)
	r.Get("/intents/{id}/revisions", h.listRevisions)
}

func (h IntentHandlers) getIntent(w http.ResponseWriter, r *http.Request) {
	actor := ActorFrom(r.Context())

	intentID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, ncperr.InvalidArgument("bad id", nil, err))
		return
	}

	out, err := h.Svc.GetIntent(r.Context(), actor, intentID)
	if err != nil {
		writeError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(out)
}

func (h IntentHandlers) listRevisions(w http.ResponseWriter, r *http.Request) {
	actor := ActorFrom(r.Context())

	intentID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, ncperr.InvalidArgument("bad id", nil, err))
		return
	}

	out, err := h.Svc.ListRevisions(r.Context(), actor, intentID)
	if err != nil {
		writeError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"items": out})
}

var _ inbound.IntentService = nil
