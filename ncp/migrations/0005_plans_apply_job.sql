-- +goose Up
ALTER TABLE plans
    ADD COLUMN IF NOT EXISTS apply_job_id uuid;

CREATE INDEX IF NOT EXISTS idx_plans_apply_job_id ON plans(apply_job_id);

-- +goose Down
DROP INDEX IF EXISTS idx_plans_apply_job_id;
ALTER TABLE plans
DROP COLUMN IF EXISTS apply_job_id;