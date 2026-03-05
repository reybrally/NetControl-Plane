package http

import (
	"encoding/json"
	"net/http"

	ncperr "github.com/reybrally/NetControl-Plane/ncp/internal/ncp/domain/errors"
)

type errorResponse struct {
	Error   string         `json:"error"`
	Code    ncperr.Code    `json:"code"`
	Details map[string]any `json:"details,omitempty"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, err error) {
	e := ncperr.As(err)
	status := ncperr.HTTPStatus(e.Code)

	msg := e.Message
	if e.Code == ncperr.CodeInternal && (msg == "" || msg == "internal error") {
		msg = "internal error"
	}

	writeJSON(w, status, errorResponse{
		Error:   msg,
		Code:    e.Code,
		Details: e.Details,
	})
}
