package runtime

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type HealthHandler struct {
	DB *pgxpool.Pool
}

func (h HealthHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 500*time.Millisecond)
	defer cancel()

	type resp struct {
		Status string `json:"status"`
		DB     string `json:"db"`
	}
	out := resp{Status: "ok", DB: "ok"}

	if h.DB != nil {
		if err := h.DB.Ping(ctx); err != nil {
			out.Status = "degraded"
			out.DB = "down"
		}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(out)
}
