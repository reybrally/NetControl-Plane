package http

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/reybrally/NetControl-Plane/ncp/internal/ncp/app/usecases"
	ncperr "github.com/reybrally/NetControl-Plane/ncp/internal/ncp/domain/errors"
)

type PlanArtifactsHandlers struct {
	UC usecases.GetPlanK8sYAML
}

func (h PlanArtifactsHandlers) Register(r chi.Router) {
	r.Get("/plans/{id}/artifacts/k8s.yaml", h.getK8sYAML)
}

func (h PlanArtifactsHandlers) getK8sYAML(w http.ResponseWriter, r *http.Request) {
	planID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, ncperr.InvalidArgument("bad id", nil, err))
		return
	}

	y, err := h.UC.Handle(r.Context(), planID)
	if err != nil {
		writeError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/x-yaml")
	_, _ = w.Write(y)
}
