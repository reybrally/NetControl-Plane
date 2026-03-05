package postgres

import (
	"context"
	"encoding/json"

	"github.com/reybrally/NetControl-Plane/ncp/internal/ncp/domain/drift"
)

type DriftRepo struct {
	DB *DB
}

func (r DriftRepo) InsertSnapshot(ctx context.Context, s drift.Snapshot) error {
	detailsBytes, _ := json.Marshal(s.Details)

	_, err := r.DB.Pool.Exec(ctx, `
		INSERT INTO drift_snapshots (id, at, scope, status, desired_hash, observed_hash, details)
		VALUES ($1, to_timestamp($2), $3, $4, $5, $6, $7::jsonb)
	`, s.ID, s.AtUnix, s.Scope, string(s.Status), s.DesiredHash, s.ObservedHash, detailsBytes)

	return err
}

func (r DriftRepo) ListSnapshots(ctx context.Context, scope string, limit int) ([]drift.Snapshot, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}

	var (
		rows any
		err  error
	)

	if scope != "" {
		rows, err = r.DB.Pool.Query(ctx, `
			SELECT id, extract(epoch from at)::bigint, scope, status, desired_hash, observed_hash, details
			FROM drift_snapshots
			WHERE scope = $1
			ORDER BY at DESC
			LIMIT $2
		`, scope, limit)
	} else {
		rows, err = r.DB.Pool.Query(ctx, `
			SELECT id, extract(epoch from at)::bigint, scope, status, desired_hash, observed_hash, details
			FROM drift_snapshots
			ORDER BY at DESC
			LIMIT $1
		`, limit)
	}
	if err != nil {
		return nil, err
	}
	rs := rows.(interface {
		Next() bool
		Scan(dest ...any) error
		Close()
		Err() error
	})
	defer rs.Close()

	var out []drift.Snapshot
	for rs.Next() {
		var s drift.Snapshot
		var status string
		var detailsBytes []byte

		if err := rs.Scan(&s.ID, &s.AtUnix, &s.Scope, &status, &s.DesiredHash, &s.ObservedHash, &detailsBytes); err != nil {
			return nil, err
		}
		s.Status = drift.Status(status)
		_ = json.Unmarshal(detailsBytes, &s.Details)
		out = append(out, s)
	}

	return out, rs.Err()
}
