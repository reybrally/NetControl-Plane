package http

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	ncperr "github.com/reybrally/NetControl-Plane/ncp/internal/ncp/domain/errors"
	"github.com/reybrally/NetControl-Plane/ncp/internal/ncp/ports/inbound"
)

type PlanHandlers struct {
	Svc inbound.IntentService
}

func (h PlanHandlers) Register(r chi.Router) {
	r.Post("/intents/{id}/plan", h.planIntent)
	r.Post("/plans/{id}/apply", h.applyPlan)
	r.Get("/plans/{id}", h.getPlan)
}

func (h PlanHandlers) planIntent(w http.ResponseWriter, r *http.Request) {
	actor := ActorFrom(r.Context())

	intentID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, ncperr.InvalidArgument("bad id", nil, err))
		return
	}

	revStr := r.URL.Query().Get("revision")
	rev, err := strconv.Atoi(revStr)
	if err != nil || rev <= 0 {
		writeError(w, ncperr.InvalidArgument("revision query param required", map[string]any{
			"param": "revision",
		}, err))
		return
	}

	planID, err := h.Svc.PlanIntent(r.Context(), actor, intentID, rev)
	if err != nil {
		writeError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"planId": planID.String()})
}

func (h PlanHandlers) applyPlan(w http.ResponseWriter, r *http.Request) {
	actor := ActorFrom(r.Context())

	planID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, ncperr.InvalidArgument("bad id", nil, err))
		return
	}

	if err := h.Svc.ApplyPlan(r.Context(), actor, planID); err != nil {
		writeError(w, err)
		return
	}

	out, err := h.Svc.GetPlan(r.Context(), actor, planID)
	if err != nil {
		writeError(w, err)
		return
	}

	status := out["status"]
	writeJSON(w, http.StatusOK, map[string]any{"status": status})
}

func (h PlanHandlers) getPlan(w http.ResponseWriter, r *http.Request) {
	actor := ActorFrom(r.Context())

	planID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, ncperr.InvalidArgument("bad id", nil, err))
		return
	}

	out, err := h.Svc.GetPlan(r.Context(), actor, planID)
	if err != nil {
		writeError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(out)
}
