-- +goose Up
CREATE TABLE sessions (
    id BINARY(16) NOT NULL,
    token_hash VARBINARY(64) NOT NULL,
    csrf_token_hash VARBINARY(64) NOT NULL,
    state VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    user_id BINARY(16) NULL,
    authorization_version BIGINT NULL,
    idle_expires_at DATETIME(6) NOT NULL,
    absolute_expires_at DATETIME(6) NOT NULL,
    revoked_at DATETIME(6) NULL,
    created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    CONSTRAINT sessions_primary_key PRIMARY KEY (id),
    CONSTRAINT sessions_token_hash_unique UNIQUE (token_hash),
    CONSTRAINT sessions_user_fk FOREIGN KEY (user_id)
        REFERENCES users (id) ON UPDATE RESTRICT ON DELETE CASCADE,
    CONSTRAINT sessions_hash_lengths_valid
        CHECK (OCTET_LENGTH(token_hash) = 32 AND OCTET_LENGTH(csrf_token_hash) = 32),
    CONSTRAINT sessions_state_principal_valid CHECK (
        (state = 'anonymous' AND user_id IS NULL AND authorization_version IS NULL)
        OR
        (state IN ('mfa_pending', 'authenticated')
            AND user_id IS NOT NULL
            AND authorization_version IS NOT NULL
            AND authorization_version > 0)
    ),
    CONSTRAINT sessions_expiry_order_valid CHECK (
        created_at < idle_expires_at AND idle_expires_at <= absolute_expires_at
    ),
    CONSTRAINT sessions_revocation_time_valid
        CHECK (revoked_at IS NULL OR revoked_at >= created_at)
) ENGINE=InnoDB DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;

CREATE INDEX sessions_user_id_idx ON sessions (user_id);
CREATE INDEX sessions_absolute_expires_at_idx ON sessions (absolute_expires_at);

-- +goose Down
DROP TABLE sessions;
