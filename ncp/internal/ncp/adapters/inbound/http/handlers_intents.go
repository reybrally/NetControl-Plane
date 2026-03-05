package http

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	ncperr "github.com/reybrally/NetControl-Plane/ncp/internal/ncp/domain/errors"
	"github.com/reybrally/NetControl-Plane/ncp/internal/ncp/domain/intent"
	"github.com/reybrally/NetControl-Plane/ncp/internal/ncp/ports/inbound"
)

type IntentHandlers struct {
	Svc inbound.IntentService
}

type createIntentReq struct {
	Key       string            `json:"key"`
	Title     string            `json:"title"`
	OwnerTeam string            `json:"ownerTeam"`
	Labels    map[string]string `json:"labels"`
}

type createIntentResp struct {
	ID string `json:"id"`
}

func (h IntentHandlers) Register(r chi.Router) {
	r.Post("/intents", h.createIntent)
	r.Post("/intents/{id}/revisions", h.createRevision)
}

func (h IntentHandlers) createIntent(w http.ResponseWriter, r *http.Request) {
	actor := ActorFrom(r.Context())

	var req createIntentReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, ncperr.InvalidArgument("bad json", nil, err))
		return
	}

	if req.Key == "" || req.Title == "" || req.OwnerTeam == "" {
		writeError(w, ncperr.InvalidArgument(
			"key/title/ownerTeam required",
			map[string]any{"required": []string{"key", "title", "ownerTeam"}},
			nil,
		))
		return
	}

	id, err := h.Svc.CreateIntent(r.Context(), actor, req.Key, req.Title, req.OwnerTeam, req.Labels)
	if err != nil {
		writeError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, createIntentResp{ID: id.String()})
}

type createRevisionReq struct {
	Spec          intent.Spec `json:"spec"`
	TicketRef     string      `json:"ticketRef"`
	Justification string      `json:"justification"`
	TTLSeconds    *int        `json:"ttlSeconds,omitempty"`
}

func (h IntentHandlers) createRevision(w http.ResponseWriter, r *http.Request) {
	actor := ActorFrom(r.Context())

	intentID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, ncperr.InvalidArgument("bad id", nil, err))
		return
	}

	var req createRevisionReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, ncperr.InvalidArgument("bad json", nil, err))
		return
	}

	if req.TicketRef == "" || req.Justification == "" {
		writeError(w, ncperr.InvalidArgument(
			"ticketRef/justification required",
			map[string]any{"required": []string{"ticketRef", "justification"}},
			nil,
		))
		return
	}
	rev, err := h.Svc.CreateRevision(r.Context(), actor, intentID, req.Spec, req.TicketRef, req.Justification, req.TTLSeconds)

	if err != nil {
		writeError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"revision": rev})
}
