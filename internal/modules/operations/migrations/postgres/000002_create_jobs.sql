-- +goose Up
CREATE TABLE operation_jobs (
    id UUID PRIMARY KEY,
    name VARCHAR(128) NOT NULL,
    payload JSONB NOT NULL,
    state VARCHAR(16) NOT NULL DEFAULT 'queued',
    attempts INTEGER NOT NULL DEFAULT 0,
    last_error VARCHAR(2048) NULL,
    created_at TIMESTAMPTZ(6) NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ(6) NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT operation_jobs_name_valid CHECK (name ~ '^[a-z][a-z0-9_.-]{0,127}$'),
    CONSTRAINT operation_jobs_state_valid CHECK (state IN ('queued','running','failed','done')),
    CONSTRAINT operation_jobs_attempts_valid CHECK (attempts >= 0),
    CONSTRAINT operation_jobs_payload_valid CHECK (jsonb_typeof(payload) IN ('object','array'))
);
CREATE INDEX operation_jobs_state_time_idx ON operation_jobs (state, created_at, id);
-- +goose Down
DROP TABLE operation_jobs;
