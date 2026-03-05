package http

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/reybrally/NetControl-Plane/ncp/internal/ncp/app/usecases"
	ncperr "github.com/reybrally/NetControl-Plane/ncp/internal/ncp/domain/errors"
)

type JobHandlers struct {
	UC usecases.GetJob
}

func (h JobHandlers) Register(r interface {
	Get(pattern string, hfn http.HandlerFunc)
}) {
	r.Get("/jobs/{id}", h.get)
}

func (h JobHandlers) get(w http.ResponseWriter, r *http.Request) {
	actor := ActorFrom(r.Context())

	raw := chi.URLParam(r, "id")
	id, err := uuid.Parse(raw)
	if err != nil {
		writeError(w, ncperr.InvalidArgument("bad id", map[string]any{"id": raw}, err))
		return
	}

	j, err := h.UC.Handle(r.Context(), actor, id)
	if err != nil {
		writeError(w, err)
		return
	}

	resp := map[string]any{
		"id":            j.ID.String(),
		"kind":          string(j.Kind),
		"status":        j.Status,
		"payload":       j.Payload,
		"createdAtUnix": j.CreatedAt.Unix(),

		"runAtUnix": j.UpdatedAt.Unix(),
	}
	if j.Error != nil {
		resp["lastError"] = *j.Error
	}
	if j.LeasedBy != nil {
		resp["lockedBy"] = *j.LeasedBy
	}
	if j.LeasedAt != nil {
		resp["lockedAtUnix"] = j.LeasedAt.Unix()
	}

	resp["ageSeconds"] = int64(time.Since(j.CreatedAt).Seconds())

	writeJSON(w, http.StatusOK, resp)
}
