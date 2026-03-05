-- +goose Up
CREATE TABLE IF NOT EXISTS jobs (
                                    id uuid PRIMARY KEY,
                                    kind text NOT NULL,
                                    payload jsonb NOT NULL,
                                    status text NOT NULL,
                                    attempts int NOT NULL DEFAULT 0,
                                    run_at timestamptz NOT NULL,
                                    locked_by text,
                                    locked_at timestamptz,
                                    last_error text,
                                    created_at timestamptz NOT NULL
);

CREATE INDEX IF NOT EXISTS jobs_pick_idx
    ON jobs(status, run_at) WHERE status='queued';

-- +goose Down
DROP INDEX IF EXISTS jobs_pick_idx;
DROP TABLE IF EXISTS jobs;