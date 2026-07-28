-- +goose Up
CREATE TABLE one_time_tokens (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL,
    purpose VARCHAR(32) COLLATE "C" NOT NULL,
    token_hash BYTEA NOT NULL,
    hash_version SMALLINT NOT NULL,
    hash_key_version BIGINT NOT NULL,
    expires_at TIMESTAMPTZ(6) NOT NULL,
    consumed_at TIMESTAMPTZ(6) NULL,
    revoked_at TIMESTAMPTZ(6) NULL,
    created_at TIMESTAMPTZ(6) NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT one_time_tokens_token_hash_unique UNIQUE (token_hash),
    CONSTRAINT one_time_tokens_user_fk FOREIGN KEY (user_id)
        REFERENCES users (id) ON UPDATE RESTRICT ON DELETE CASCADE,
    CONSTRAINT one_time_tokens_purpose_valid
        CHECK (purpose IN ('user_activation', 'password_reset')),
    CONSTRAINT one_time_tokens_hash_length_valid
        CHECK (octet_length(token_hash) = 32),
    CONSTRAINT one_time_tokens_hash_versions_valid
        CHECK (hash_version > 0 AND hash_key_version > 0),
    CONSTRAINT one_time_tokens_lifetime_valid
        CHECK (expires_at > created_at),
    CONSTRAINT one_time_tokens_consumption_time_valid CHECK (
        consumed_at IS NULL
        OR (consumed_at >= created_at AND consumed_at < expires_at)
    ),
    CONSTRAINT one_time_tokens_revocation_time_valid
        CHECK (revoked_at IS NULL OR revoked_at >= created_at),
    CONSTRAINT one_time_tokens_terminal_state_valid
        CHECK (consumed_at IS NULL OR revoked_at IS NULL)
);

CREATE INDEX one_time_tokens_user_purpose_expiry_idx
    ON one_time_tokens (user_id, purpose, expires_at);

CREATE INDEX one_time_tokens_expiry_idx
    ON one_time_tokens (expires_at);

CREATE INDEX one_time_tokens_key_expiry_idx
    ON one_time_tokens (hash_key_version, expires_at);

-- +goose Down
DROP TABLE one_time_tokens;
