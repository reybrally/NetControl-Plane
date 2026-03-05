package postgres

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/google/uuid"
	"github.com/reybrally/NetControl-Plane/ncp/internal/ncp/ports/outbound"
)

type PlanRepo struct{ DB *DB }

func (r PlanRepo) CreatePlan(ctx context.Context, p outbound.Plan) error {
	diff, _ := json.Marshal(p.Diff)
	blast, _ := json.Marshal(p.Blast)
	art, _ := json.Marshal(p.Artifacts)

	q := `INSERT INTO plans(id,intent_id,revision_id,status,diff,blast_radius,artifacts,created_by,created_at)
	      VALUES($1,$2,$3,$4,$5,$6,$7,$8,now())`
	return exec(ctx, r.DB, q, p.ID, p.IntentID, p.RevisionID, p.Status, diff, blast, art, p.CreatedBy)
}

func (r PlanRepo) GetPlan(ctx context.Context, id uuid.UUID) (outbound.Plan, error) {
	row := queryRow(ctx, r.DB, `
		SELECT id,intent_id,revision_id,status,diff,blast_radius,artifacts,created_by,
		       extract(epoch from created_at)::bigint,
		       apply_job_id
		FROM plans WHERE id=$1`, id)

	var p outbound.Plan
	var diff, blast, art []byte
	var applyJobID *uuid.UUID

	if err := row.Scan(&p.ID, &p.IntentID, &p.RevisionID, &p.Status, &diff, &blast, &art,
		&p.CreatedBy, &p.CreatedAtUnix, &applyJobID); err != nil {
		return outbound.Plan{}, err
	}
	p.ApplyJobID = applyJobID

	_ = json.Unmarshal(diff, &p.Diff)
	_ = json.Unmarshal(blast, &p.Blast)
	_ = json.Unmarshal(art, &p.Artifacts)
	return p, nil
}

func (r PlanRepo) UpdatePlanStatus(ctx context.Context, id uuid.UUID, status string) error {
	return exec(ctx, r.DB, `UPDATE plans SET status=$2 WHERE id=$1`, id, status)
}

func (r PlanRepo) ListLatestAppliedPlans(ctx context.Context, limit int) ([]outbound.Plan, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}

	rows, err := r.DB.Pool.Query(ctx, `
		SELECT id,intent_id,revision_id,status,diff,blast_radius,artifacts,created_by,
		       extract(epoch from created_at)::bigint,
		       apply_job_id
		FROM plans
		WHERE status='applied'
		ORDER BY created_at DESC
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]outbound.Plan, 0, limit)

	for rows.Next() {
		var p outbound.Plan
		var diff, blast, art []byte
		var applyJobID *uuid.UUID

		if err := rows.Scan(&p.ID, &p.IntentID, &p.RevisionID, &p.Status,
			&diff, &blast, &art, &p.CreatedBy, &p.CreatedAtUnix, &applyJobID); err != nil {
			return nil, err
		}
		p.ApplyJobID = applyJobID

		_ = json.Unmarshal(diff, &p.Diff)
		_ = json.Unmarshal(blast, &p.Blast)
		_ = json.Unmarshal(art, &p.Artifacts)

		out = append(out, p)
	}
	return out, rows.Err()
}

func (r PlanRepo) ListLatestAppliedPlansPerIntentWithTTL(ctx context.Context, limit int) ([]outbound.ExpirablePlan, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}

	rows, err := r.DB.Pool.Query(ctx, `
		SELECT DISTINCT ON (p.intent_id)
			p.id, p.intent_id, p.revision_id, p.status, p.diff, p.blast_radius, p.artifacts, p.created_by,
			extract(epoch from p.created_at)::bigint,
			p.apply_job_id,
			r.ttl_seconds,
			CASE WHEN r.not_after IS NULL THEN NULL ELSE extract(epoch from r.not_after)::bigint END
		FROM plans p
		JOIN intent_revisions r ON r.id = p.revision_id
		WHERE p.status='applied'
		ORDER BY p.intent_id, p.created_at DESC
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]outbound.ExpirablePlan, 0, limit)

	for rows.Next() {
		var p outbound.Plan
		var diff, blast, art []byte
		var applyJobID *uuid.UUID
		var ttl *int
		var notAfter *int64

		if err := rows.Scan(
			&p.ID, &p.IntentID, &p.RevisionID, &p.Status,
			&diff, &blast, &art,
			&p.CreatedBy, &p.CreatedAtUnix,
			&applyJobID,
			&ttl, &notAfter,
		); err != nil {
			return nil, err
		}
		p.ApplyJobID = applyJobID
		_ = json.Unmarshal(diff, &p.Diff)
		_ = json.Unmarshal(blast, &p.Blast)
		_ = json.Unmarshal(art, &p.Artifacts)

		out = append(out, outbound.ExpirablePlan{
			Plan:         p,
			TTLSeconds:   ttl,
			NotAfterUnix: notAfter,
		})
	}
	return out, rows.Err()
}

func (r PlanRepo) ListAppliedPlansByIntent(ctx context.Context, intentID uuid.UUID, limit int) ([]outbound.Plan, error) {
	if limit <= 0 || limit > 200 {
		limit = 20
	}

	rows, err := r.DB.Pool.Query(ctx, `
		SELECT id,intent_id,revision_id,status,diff,blast_radius,artifacts,created_by,
		       extract(epoch from created_at)::bigint,
		       apply_job_id
		FROM plans
		WHERE status='applied' AND intent_id=$1
		ORDER BY created_at DESC
		LIMIT $2
	`, intentID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]outbound.Plan, 0, limit)
	for rows.Next() {
		var p outbound.Plan
		var diff, blast, art []byte
		var applyJobID *uuid.UUID

		if err := rows.Scan(&p.ID, &p.IntentID, &p.RevisionID, &p.Status,
			&diff, &blast, &art, &p.CreatedBy, &p.CreatedAtUnix, &applyJobID); err != nil {
			return nil, err
		}
		p.ApplyJobID = applyJobID
		_ = json.Unmarshal(diff, &p.Diff)
		_ = json.Unmarshal(blast, &p.Blast)
		_ = json.Unmarshal(art, &p.Artifacts)
		out = append(out, p)
	}
	return out, rows.Err()
}

func (r PlanRepo) ListTTLExpiredCandidates(ctx context.Context, limit int) ([]uuid.UUID, error) {
	if limit <= 0 {
		limit = 100
	}

	rows, err := r.DB.Pool.Query(ctx, `
		SELECT p.id
		FROM plans p
		JOIN intent_revisions r ON r.id = p.revision_id
		WHERE p.status = 'applied'
		  AND r.ttl_seconds IS NOT NULL
		  AND r.ttl_seconds > 0
		  AND r.not_after IS NOT NULL
		  AND r.not_after <= now()
		ORDER BY r.not_after ASC
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func (r PlanRepo) MarkPlanExpiredOnce(ctx context.Context, planID uuid.UUID) (bool, error) {
	ra, err := execResult(ctx, r.DB, `
		UPDATE plans
		SET status='expired'
		WHERE id=$1
		  AND status <> 'expired'
	`, planID)
	if err != nil {
		return false, err
	}
	return ra == 1, nil
}

func (r PlanRepo) SetAppliedK8sRef(ctx context.Context, planID uuid.UUID, namespace, name string) error {
	if namespace == "" || name == "" {
		return errors.New("SetAppliedK8sRef: namespace/name required")
	}

	return exec(ctx, r.DB, `
		UPDATE plans
		SET artifacts =
			jsonb_set(
				COALESCE(artifacts, '{}'::jsonb),
				'{k8s,applied}',
				jsonb_build_object(
					'namespace', $2::text,
					'name',      $3::text
				),
				true
			)
		WHERE id=$1
	`, planID, namespace, name)
}
