-- +goose Up
CREATE TABLE IF NOT EXISTS drift_snapshots (
                                               id uuid PRIMARY KEY,
                                               at timestamptz NOT NULL,
                                               scope text NOT NULL,
                                               status text NOT NULL,
                                               desired_hash text NOT NULL,
                                               observed_hash text NOT NULL,
                                               details jsonb NOT NULL DEFAULT '{}'::jsonb
);

CREATE INDEX IF NOT EXISTS drift_snapshots_at_idx ON drift_snapshots(at DESC);
CREATE INDEX IF NOT EXISTS drift_snapshots_scope_at_idx ON drift_snapshots(scope, at DESC);

-- +goose Down
DROP INDEX IF EXISTS drift_snapshots_scope_at_idx;
DROP INDEX IF EXISTS drift_snapshots_at_idx;
DROP TABLE IF EXISTS drift_snapshots;
