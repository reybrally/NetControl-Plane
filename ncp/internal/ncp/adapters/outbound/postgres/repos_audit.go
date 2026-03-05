package postgres

import (
	"context"
	"encoding/json"
)

type AuditRepo struct{ DB *DB }

func (r AuditRepo) Append(ctx context.Context, actor, action, entityType, entityID string, meta map[string]any) error {
	b, _ := json.Marshal(meta)
	return exec(ctx, r.DB,
		`INSERT INTO audit_log(at,actor,action,entity_type,entity_id,meta)
		 VALUES(now(),$1,$2,$3,$4,$5)`,
		actor, action, entityType, entityID, b,
	)
}
