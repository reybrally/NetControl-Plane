package postgres

import (
	"context"
	"encoding/json"

	"github.com/reybrally/NetControl-Plane/ncp/internal/ncp/ports/outbound"
)

func (r AuditRepo) List(ctx context.Context, entityType, entityID string, limit int) ([]outbound.AuditEntry, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}

	rows, err := r.DB.Pool.Query(ctx, `
		SELECT id, extract(epoch from at)::bigint, actor, action, entity_type, entity_id, meta
		FROM audit_log
		WHERE ($1='' OR entity_type=$1)
		  AND ($2='' OR entity_id=$2)
		ORDER BY id DESC
		LIMIT $3`, entityType, entityID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []outbound.AuditEntry
	for rows.Next() {
		var e outbound.AuditEntry
		var metaBytes []byte
		if err := rows.Scan(&e.ID, &e.AtUnix, &e.Actor, &e.Action, &e.EntityType, &e.EntityID, &metaBytes); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(metaBytes, &e.Meta)
		out = append(out, e)
	}
	return out, rows.Err()
}
