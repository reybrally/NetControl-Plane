package usecases

import (
	"context"

	"github.com/google/uuid"
	"github.com/reybrally/NetControl-Plane/ncp/internal/ncp/ports/inbound"
)

func (s *IntentService) GetPlan(ctx context.Context, actor inbound.Actor, planID uuid.UUID) (map[string]any, error) {
	p, err := s.Plans.GetPlan(ctx, planID)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"id":         p.ID.String(),
		"intentId":   p.IntentID.String(),
		"revisionId": p.RevisionID,
		"status":     p.Status,
		"diff":       p.Diff,
		"blast":      p.Blast,
		"artifacts":  p.Artifacts,
	}, nil
}

func (s *IntentService) GetIntent(ctx context.Context, actor inbound.Actor, intentID uuid.UUID) (map[string]any, error) {
	it, err := s.Intents.GetIntent(ctx, intentID)
	if err != nil {
		return nil, err
	}
	var cur any = nil
	if it.CurrentRevision != nil {
		cur = *it.CurrentRevision
	}
	return map[string]any{
		"id":              it.ID.String(),
		"key":             it.Key,
		"title":           it.Title,
		"ownerTeam":       it.OwnerTeam,
		"status":          it.Status,
		"currentRevision": cur,
		"labels":          it.Labels,
		"createdBy":       it.CreatedBy,
	}, nil
}

func (s *IntentService) ListRevisions(ctx context.Context, actor inbound.Actor, intentID uuid.UUID) ([]map[string]any, error) {
	revs, err := s.Intents.ListRevisions(ctx, intentID, 50)
	if err != nil {
		return nil, err
	}
	out := make([]map[string]any, 0, len(revs))
	for _, r := range revs {
		out = append(out, map[string]any{
			"id":            r.ID,
			"revision":      r.Revision,
			"state":         r.State,
			"ticketRef":     r.TicketRef,
			"justification": r.Justification,
			"ttlSeconds":    r.TTLSeconds,
			"notAfterUnix":  r.NotAfterUnix,
			"createdBy":     r.CreatedBy,
			"createdAtUnix": r.CreatedAtUnix,
		})
	}
	return out, nil
}

func (s *IntentService) ListAudit(ctx context.Context, actor inbound.Actor, entityType, entityID string, limit int) ([]map[string]any, error) {
	entries, err := s.Audit.List(ctx, entityType, entityID, limit)
	if err != nil {
		return nil, err
	}
	out := make([]map[string]any, 0, len(entries))
	for _, e := range entries {
		out = append(out, map[string]any{
			"id":         e.ID,
			"atUnix":     e.AtUnix,
			"actor":      e.Actor,
			"action":     e.Action,
			"entityType": e.EntityType,
			"entityId":   e.EntityID,
			"meta":       e.Meta,
		})
	}
	return out, nil
}
