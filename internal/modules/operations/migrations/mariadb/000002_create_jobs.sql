-- +goose Up
CREATE TABLE operation_jobs (
    id BINARY(16) NOT NULL,
    name VARCHAR(128) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    payload JSON NOT NULL,
    state VARCHAR(16) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT 'queued',
    attempts INT NOT NULL DEFAULT 0,
    last_error VARCHAR(2048) NULL,
    created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    CONSTRAINT operation_jobs_primary_key PRIMARY KEY (id),
    CONSTRAINT operation_jobs_name_valid CHECK (name REGEXP '^[a-z][a-z0-9_.-]{0,127}$'),
    CONSTRAINT operation_jobs_state_valid CHECK (state IN ('queued','running','failed','done')),
    CONSTRAINT operation_jobs_attempts_valid CHECK (attempts >= 0)
) ENGINE=InnoDB DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
CREATE INDEX operation_jobs_state_time_idx ON operation_jobs (state, created_at, id);
-- +goose Down
DROP TABLE operation_jobs;
