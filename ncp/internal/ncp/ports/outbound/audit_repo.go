package outbound

import "context"

type AuditEntry struct {
	ID         int64
	AtUnix     int64
	Actor      string
	Action     string
	EntityType string
	EntityID   string
	Meta       map[string]any
}

type AuditRepo interface {
	Append(ctx context.Context, actor, action, entityType, entityID string, meta map[string]any) error
	List(ctx context.Context, entityType, entityID string, limit int) ([]AuditEntry, error)
}
