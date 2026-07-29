-- +goose Up
CREATE TABLE one_time_tokens (
    id BINARY(16) NOT NULL,
    user_id BINARY(16) NOT NULL,
    purpose VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    token_hash VARBINARY(64) NOT NULL,
    hash_version SMALLINT NOT NULL,
    hash_key_version BIGINT NOT NULL,
    expires_at DATETIME(6) NOT NULL,
    consumed_at DATETIME(6) NULL,
    revoked_at DATETIME(6) NULL,
    created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    CONSTRAINT one_time_tokens_primary_key PRIMARY KEY (id),
    CONSTRAINT one_time_tokens_token_hash_unique UNIQUE (token_hash),
    CONSTRAINT one_time_tokens_user_fk FOREIGN KEY (user_id)
        REFERENCES users (id) ON UPDATE RESTRICT ON DELETE CASCADE,
    CONSTRAINT one_time_tokens_purpose_valid
        CHECK (purpose IN ('user_activation', 'password_reset')),
    CONSTRAINT one_time_tokens_hash_length_valid
        CHECK (OCTET_LENGTH(token_hash) = 32),
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
) ENGINE=InnoDB DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;

CREATE INDEX one_time_tokens_user_purpose_expiry_idx
    ON one_time_tokens (user_id, purpose, expires_at);

CREATE INDEX one_time_tokens_expiry_idx
    ON one_time_tokens (expires_at);

CREATE INDEX one_time_tokens_key_expiry_idx
    ON one_time_tokens (hash_key_version, expires_at);

CREATE TABLE one_time_token_deliveries (
    token_id BINARY(16) NOT NULL,
    envelope_version SMALLINT NOT NULL,
    key_version BIGINT NOT NULL,
    nonce VARBINARY(64) NOT NULL,
    ciphertext VARBINARY(2048) NOT NULL,
    expires_at DATETIME(6) NOT NULL,
    delivered_at DATETIME(6) NULL,
    created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    CONSTRAINT one_time_token_deliveries_primary_key PRIMARY KEY (token_id),
    CONSTRAINT one_time_token_deliveries_token_fk FOREIGN KEY (token_id)
        REFERENCES one_time_tokens (id) ON UPDATE RESTRICT ON DELETE CASCADE,
    CONSTRAINT one_time_token_deliveries_envelope_version_valid
        CHECK (envelope_version = 1 AND key_version > 0),
    CONSTRAINT one_time_token_deliveries_envelope_lengths_valid CHECK (
        OCTET_LENGTH(nonce) = 12
        AND OCTET_LENGTH(ciphertext) BETWEEN 17 AND 2048
    ),
    CONSTRAINT one_time_token_deliveries_lifetime_valid
        CHECK (expires_at > created_at),
    CONSTRAINT one_time_token_deliveries_delivery_time_valid
        CHECK (delivered_at IS NULL OR (delivered_at >= created_at AND delivered_at < expires_at))
) ENGINE=InnoDB DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;

CREATE INDEX one_time_token_deliveries_expiry_idx
    ON one_time_token_deliveries (expires_at);

-- +goose Down
DROP TABLE one_time_token_deliveries;
DROP TABLE one_time_tokens;
