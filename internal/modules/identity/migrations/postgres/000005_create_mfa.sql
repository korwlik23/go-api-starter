-- +goose Up
CREATE TABLE mfa_totp_credentials (
    user_id UUID PRIMARY KEY,
    enrollment_id UUID NOT NULL,
    secret_envelope_version SMALLINT NOT NULL,
    secret_key_version BIGINT NOT NULL,
    secret_nonce BYTEA NOT NULL,
    secret_ciphertext BYTEA NOT NULL,
    enrollment_expires_at TIMESTAMPTZ(6) NULL,
    confirmed_at TIMESTAMPTZ(6) NULL,
    last_used_step BIGINT NULL,
    created_at TIMESTAMPTZ(6) NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ(6) NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT mfa_totp_credentials_key_nonce_unique
        UNIQUE (secret_key_version, secret_nonce),
    CONSTRAINT mfa_totp_credentials_enrollment_id_unique
        UNIQUE (enrollment_id),
    CONSTRAINT mfa_totp_credentials_user_fk FOREIGN KEY (user_id)
        REFERENCES users (id) ON UPDATE RESTRICT ON DELETE CASCADE,
    CONSTRAINT mfa_totp_credentials_envelope_version_valid
        CHECK (secret_envelope_version = 1),
    CONSTRAINT mfa_totp_credentials_key_version_valid
        CHECK (secret_key_version > 0),
    CONSTRAINT mfa_totp_credentials_envelope_lengths_valid CHECK (
        octet_length(secret_nonce) = 12
        AND octet_length(secret_ciphertext) BETWEEN 17 AND 512
    ),
    CONSTRAINT mfa_totp_credentials_lifecycle_valid CHECK (
        (
            confirmed_at IS NULL
            AND last_used_step IS NULL
            AND enrollment_expires_at IS NOT NULL
            AND enrollment_expires_at > created_at
        )
        OR
        (
            confirmed_at IS NOT NULL
            AND confirmed_at >= created_at
            AND last_used_step IS NOT NULL
            AND last_used_step > 0
            AND enrollment_expires_at IS NULL
        )
    ),
    CONSTRAINT mfa_totp_credentials_update_time_valid
        CHECK (updated_at >= created_at)
);

CREATE INDEX mfa_totp_credentials_enrollment_expiry_idx
    ON mfa_totp_credentials (enrollment_expires_at);

CREATE TABLE mfa_recovery_codes (
    user_id UUID NOT NULL,
    code_hash BYTEA NOT NULL,
    hash_version SMALLINT NOT NULL,
    hash_key_version BIGINT NOT NULL,
    consumed_at TIMESTAMPTZ(6) NULL,
    created_at TIMESTAMPTZ(6) NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT mfa_recovery_codes_primary_key PRIMARY KEY (user_id, code_hash),
    CONSTRAINT mfa_recovery_codes_credential_fk FOREIGN KEY (user_id)
        REFERENCES mfa_totp_credentials (user_id)
        ON UPDATE RESTRICT ON DELETE CASCADE,
    CONSTRAINT mfa_recovery_codes_hash_length_valid
        CHECK (octet_length(code_hash) = 32),
    CONSTRAINT mfa_recovery_codes_hash_versions_valid
        CHECK (hash_version > 0 AND hash_key_version > 0),
    CONSTRAINT mfa_recovery_codes_consumption_time_valid
        CHECK (consumed_at IS NULL OR consumed_at >= created_at)
);

-- +goose Down
DROP TABLE mfa_recovery_codes;
DROP TABLE mfa_totp_credentials;
