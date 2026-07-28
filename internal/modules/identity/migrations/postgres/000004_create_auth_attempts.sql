-- +goose Up
CREATE TABLE auth_attempts (
    key_type VARCHAR(16) COLLATE "C" NOT NULL,
    key_hash BYTEA NOT NULL,
    failure_count BIGINT NOT NULL,
    window_started_at TIMESTAMPTZ(6) NOT NULL,
    window_expires_at TIMESTAMPTZ(6) NOT NULL,
    last_failed_at TIMESTAMPTZ(6) NOT NULL,
    locked_until TIMESTAMPTZ(6) NULL,
    created_at TIMESTAMPTZ(6) NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ(6) NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT auth_attempts_primary_key PRIMARY KEY (key_type, key_hash),
    CONSTRAINT auth_attempts_key_type_valid
        CHECK (key_type IN ('subject', 'ip')),
    CONSTRAINT auth_attempts_key_hash_length_valid
        CHECK (octet_length(key_hash) = 32),
    CONSTRAINT auth_attempts_failure_count_valid
        CHECK (failure_count > 0),
    CONSTRAINT auth_attempts_window_valid CHECK (
        window_started_at <= last_failed_at
        AND last_failed_at < window_expires_at
    ),
    CONSTRAINT auth_attempts_lock_time_valid
        CHECK (locked_until IS NULL OR locked_until > last_failed_at)
);

CREATE INDEX auth_attempts_window_expires_at_idx
    ON auth_attempts (window_expires_at);

-- +goose Down
DROP TABLE auth_attempts;
