package postgres

import (
	"context"

	"github.com/google/uuid"
)

func (r PlanRepo) SetApplyJobOnce(ctx context.Context, planID uuid.UUID, jobID uuid.UUID) (bool, error) {
	q := `UPDATE plans SET apply_job_id=$2 WHERE id=$1 AND apply_job_id IS NULL`

	if tx, ok := getTx(ctx); ok {
		tag, err := tx.Exec(ctx, q, planID, jobID)
		if err != nil {
			return false, err
		}
		return tag.RowsAffected() == 1, nil
	}

	tag, err := r.DB.Pool.Exec(ctx, q, planID, jobID)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() == 1, nil
}
