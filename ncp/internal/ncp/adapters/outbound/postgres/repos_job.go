package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/reybrally/NetControl-Plane/ncp/internal/ncp/ports/outbound"
)

type JobRepo struct {
	DB *DB
}

func (r JobRepo) GetByID(ctx context.Context, id uuid.UUID) (*outbound.Job, error) {
	var (
		kind      string
		status    string
		payload   []byte
		lastErr   *string
		lockedBy  *string
		lockedAt  *time.Time
		runAt     time.Time
		createdAt time.Time
		attempts  int
	)

	row := r.DB.Pool.QueryRow(ctx, `
		SELECT kind, status, payload, last_error, locked_by, locked_at, run_at, created_at, attempts
		FROM jobs
		WHERE id = $1
	`, id)

	if err := row.Scan(&kind, &status, &payload, &lastErr, &lockedBy, &lockedAt, &runAt, &createdAt, &attempts); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	var payloadMap map[string]any
	if len(payload) > 0 {
		_ = json.Unmarshal(payload, &payloadMap)
	}
	if payloadMap == nil {
		payloadMap = map[string]any{}
	}

	j := &outbound.Job{
		ID:        id,
		Kind:      outbound.JobKind(kind),
		Status:    status,
		Payload:   payloadMap,
		Error:     lastErr,
		LeasedBy:  lockedBy,
		LeasedAt:  lockedAt,
		CreatedAt: createdAt,
		UpdatedAt: runAt,
	}

	j.Payload["attempts"] = attempts
	j.Payload["runAtUnix"] = runAt.Unix()

	return j, nil
}
