-- +goose Up
CREATE TABLE auth_attempts (
    key_type VARCHAR(16) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    key_hash VARBINARY(64) NOT NULL,
    failure_count BIGINT NOT NULL,
    window_started_at DATETIME(6) NOT NULL,
    window_expires_at DATETIME(6) NOT NULL,
    last_failed_at DATETIME(6) NOT NULL,
    locked_until DATETIME(6) NULL,
    created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    CONSTRAINT auth_attempts_primary_key PRIMARY KEY (key_type, key_hash),
    CONSTRAINT auth_attempts_key_type_valid
        CHECK (key_type IN ('subject', 'ip')),
    CONSTRAINT auth_attempts_key_hash_length_valid
        CHECK (OCTET_LENGTH(key_hash) = 32),
    CONSTRAINT auth_attempts_failure_count_valid
        CHECK (failure_count > 0),
    CONSTRAINT auth_attempts_window_valid CHECK (
        window_started_at <= last_failed_at
        AND last_failed_at < window_expires_at
    ),
    CONSTRAINT auth_attempts_lock_time_valid
        CHECK (locked_until IS NULL OR locked_until > last_failed_at)
) ENGINE=InnoDB DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;

CREATE INDEX auth_attempts_window_expires_at_idx
    ON auth_attempts (window_expires_at);

-- +goose Down
DROP TABLE auth_attempts;
