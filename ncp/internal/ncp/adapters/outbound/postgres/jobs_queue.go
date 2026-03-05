package postgres

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/reybrally/NetControl-Plane/ncp/internal/ncp/ports/outbound"
)

type JobQueue struct{ DB *DB }

func (q JobQueue) Enqueue(ctx context.Context, kind outbound.JobKind, payload map[string]any) (uuid.UUID, error) {
	id := uuid.New()
	b, _ := json.Marshal(payload)
	err := exec(ctx, q.DB, `INSERT INTO jobs(id,kind,payload,status,run_at,created_at) VALUES($1,$2,$3,'queued',now(),now())`,
		id, string(kind), b)
	return id, err
}

func (q JobQueue) LeaseNext(ctx context.Context, workerID string) (uuid.UUID, outbound.JobKind, map[string]any, bool, error) {
	tx, err := q.DB.Pool.Begin(ctx)
	if err != nil {
		return uuid.Nil, "", nil, false, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var id uuid.UUID
	var kind string
	var payloadBytes []byte

	err = tx.QueryRow(ctx, `
		SELECT id,kind,payload FROM jobs
		WHERE status='queued' AND run_at <= now()
		ORDER BY run_at ASC
		FOR UPDATE SKIP LOCKED
		LIMIT 1`).Scan(&id, &kind, &payloadBytes)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return uuid.Nil, "", nil, false, tx.Commit(ctx)
		}
		return uuid.Nil, "", nil, false, err
	}

	if _, err := tx.Exec(ctx, `UPDATE jobs SET status='running', locked_by=$2, locked_at=now() WHERE id=$1`, id, workerID); err != nil {
		return uuid.Nil, "", nil, false, err
	}

	if err := tx.Commit(ctx); err != nil {
		return uuid.Nil, "", nil, false, err
	}

	var payload map[string]any
	_ = json.Unmarshal(payloadBytes, &payload)
	return id, outbound.JobKind(kind), payload, true, nil
}

func (q JobQueue) MarkDone(ctx context.Context, jobID uuid.UUID) error {
	return exec(ctx, q.DB, `UPDATE jobs SET status='done' WHERE id=$1`, jobID)
}

func (q JobQueue) MarkFailed(ctx context.Context, jobID uuid.UUID, errMsg string) error {

	return exec(ctx, q.DB, `
		UPDATE jobs
		SET status=CASE WHEN attempts+1 >= 5 THEN 'failed' ELSE 'queued' END,
		    attempts=attempts+1,
		    last_error=$2,
		    run_at=CASE WHEN attempts+1 >= 5 THEN run_at ELSE now() + interval '10 seconds' END,
		    locked_by=NULL, locked_at=NULL
		WHERE id=$1`, jobID, errMsg)
}

func (q JobQueue) FindActiveReconcileDrift(ctx context.Context, scope, namespace string, dryRun bool) (uuid.UUID, bool, error) {
	var id uuid.UUID
	err := q.DB.Pool.QueryRow(ctx, `
		SELECT id
		FROM jobs
		WHERE kind = $1
		  AND status IN ('queued','running')
		  AND payload->>'scope' = $2
		  AND payload->>'namespace' = $3
		  AND COALESCE((payload->>'dryRun')::boolean, false) = $4
		ORDER BY created_at DESC
		LIMIT 1
	`, string(outbound.JobReconcileDrift), scope, namespace, dryRun).Scan(&id)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return uuid.Nil, false, nil
		}
		return uuid.Nil, false, err
	}

	return id, true, nil
}
func (q JobQueue) FindActiveExpireTTL(ctx context.Context, planID uuid.UUID) (uuid.UUID, bool, error) {
	var id uuid.UUID
	err := q.DB.Pool.QueryRow(ctx, `
		SELECT id
		FROM jobs
		WHERE kind = $1
		  AND status IN ('queued','running')
		  AND payload->>'planId' = $2
		ORDER BY created_at DESC
		LIMIT 1
	`, string(outbound.JobExpireTTL), planID.String()).Scan(&id)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return uuid.Nil, false, nil
		}
		return uuid.Nil, false, err
	}
	return id, true, nil
}
