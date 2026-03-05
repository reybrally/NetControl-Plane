package postgres

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
	"github.com/reybrally/NetControl-Plane/ncp/internal/ncp/ports/outbound"
)

func (r IntentRepo) ListIntents(ctx context.Context, limit int) ([]outbound.Intent, error) {
	if limit <= 0 || limit > 500 {
		limit = 200
	}

	rows, err := r.DB.Pool.Query(ctx, `
		SELECT id,key,title,owner_team,status,current_revision,labels,created_by,
		       extract(epoch from created_at)::bigint,
		       extract(epoch from updated_at)::bigint
		FROM intents
		ORDER BY updated_at DESC
		LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []outbound.Intent
	for rows.Next() {
		var it outbound.Intent
		var labelsBytes []byte
		var cur *int64

		if err := rows.Scan(
			&it.ID,
			&it.Key,
			&it.Title,
			&it.OwnerTeam,
			&it.Status,
			&cur,
			&labelsBytes,
			&it.CreatedBy,
			&it.CreatedAtUnix,
			&it.UpdatedAtUnix,
		); err != nil {
			return nil, err
		}

		it.CurrentRevision = cur
		_ = json.Unmarshal(labelsBytes, &it.Labels)
		out = append(out, it)
	}

	return out, rows.Err()
}

func (r IntentRepo) ListRevisions(ctx context.Context, intentID uuid.UUID, limit int) ([]outbound.Revision, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}

	rows, err := r.DB.Pool.Query(ctx, `
		SELECT id,intent_id,revision,spec,spec_hash,state,ticket_ref,justification,ttl_seconds,
		       CASE WHEN not_after IS NULL THEN NULL ELSE extract(epoch from not_after)::bigint END,
		       created_by,extract(epoch from created_at)::bigint
		FROM intent_revisions
		WHERE intent_id=$1
		ORDER BY revision DESC
		LIMIT $2`, intentID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []outbound.Revision
	for rows.Next() {
		var rev outbound.Revision
		var specBytes []byte
		var ttl *int
		var notAfter *int64

		if err := rows.Scan(&rev.ID, &rev.IntentID, &rev.Revision, &specBytes, &rev.SpecHash, &rev.State,
			&rev.TicketRef, &rev.Justification, &ttl, &notAfter, &rev.CreatedBy, &rev.CreatedAtUnix); err != nil {
			return nil, err
		}
		rev.TTLSeconds = ttl
		rev.NotAfterUnix = notAfter
		_ = json.Unmarshal(specBytes, &rev.Spec)
		out = append(out, rev)
	}

	return out, rows.Err()
}
