-- +goose Up
CREATE TABLE IF NOT EXISTS intents (
                                       id uuid PRIMARY KEY,
                                       key text NOT NULL UNIQUE,
                                       title text NOT NULL,
                                       owner_team text NOT NULL,
                                       status text NOT NULL,
                                       current_revision bigint,
                                       labels jsonb NOT NULL DEFAULT '{}'::jsonb,
                                       created_by text NOT NULL,
                                       created_at timestamptz NOT NULL,
                                       updated_at timestamptz NOT NULL
);

CREATE TABLE IF NOT EXISTS intent_revisions (
                                                id bigserial PRIMARY KEY,
                                                intent_id uuid NOT NULL REFERENCES intents(id) ON DELETE CASCADE,
    revision int NOT NULL,
    spec jsonb NOT NULL,
    spec_hash text NOT NULL,
    state text NOT NULL,
    justification text NOT NULL,
    ticket_ref text NOT NULL,
    ttl_seconds int,
    not_after timestamptz,
    created_by text NOT NULL,
    created_at timestamptz NOT NULL,
    approved_by text,
    approved_at timestamptz,
    UNIQUE(intent_id, revision)
    );

CREATE TABLE IF NOT EXISTS plans (
                                     id uuid PRIMARY KEY,
                                     intent_id uuid NOT NULL REFERENCES intents(id) ON DELETE CASCADE,
    revision_id bigint NOT NULL REFERENCES intent_revisions(id) ON DELETE CASCADE,
    status text NOT NULL,
    diff jsonb NOT NULL,
    blast_radius jsonb NOT NULL,
    artifacts jsonb NOT NULL,
    created_by text NOT NULL,
    created_at timestamptz NOT NULL
    );

CREATE TABLE IF NOT EXISTS artifacts (
                                         id uuid PRIMARY KEY,
                                         plan_id uuid NOT NULL REFERENCES plans(id) ON DELETE CASCADE,
    target text NOT NULL,
    kind text NOT NULL,
    content jsonb NOT NULL,
    content_hash text NOT NULL,
    applied_at timestamptz,
    UNIQUE(plan_id, target, content_hash)
    );

CREATE TABLE IF NOT EXISTS audit_log (
                                         id bigserial PRIMARY KEY,
                                         at timestamptz NOT NULL,
                                         actor text NOT NULL,
                                         action text NOT NULL,
                                         entity_type text NOT NULL,
                                         entity_id text NOT NULL,
                                         meta jsonb NOT NULL DEFAULT '{}'::jsonb
);

-- +goose Down
DROP TABLE IF EXISTS audit_log;
DROP TABLE IF EXISTS artifacts;
DROP TABLE IF EXISTS plans;
DROP TABLE IF EXISTS intent_revisions;
DROP TABLE IF EXISTS intents;