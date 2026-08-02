-- +goose Up
CREATE TABLE media_uploads (
    id UUID PRIMARY KEY,
    account_id UUID NOT NULL,
    storage_key VARCHAR(255) COLLATE "C" NOT NULL,
    original_name VARCHAR(255) NOT NULL,
    mime_type VARCHAR(128) COLLATE "C" NOT NULL,
    declared_size BIGINT NOT NULL,
    expected_sha256 CHAR(64) COLLATE "C" NOT NULL,
    state VARCHAR(16) COLLATE "C" NOT NULL DEFAULT 'pending',
    asset_id UUID NULL,
    version BIGINT NOT NULL DEFAULT 1,
    expires_at TIMESTAMPTZ(6) NOT NULL,
    created_at TIMESTAMPTZ(6) NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ(6) NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT media_uploads_account_id_unique UNIQUE (account_id, id),
    CONSTRAINT media_uploads_account_storage_key_unique UNIQUE (account_id, storage_key),
    CONSTRAINT media_uploads_storage_key_valid CHECK (CHAR_LENGTH(storage_key) BETWEEN 1 AND 255 AND storage_key !~ '[\\/]'),
    CONSTRAINT media_uploads_mime_valid CHECK (mime_type IN ('image/jpeg', 'image/png', 'image/gif', 'image/webp', 'application/pdf')),
    CONSTRAINT media_uploads_size_valid CHECK (declared_size BETWEEN 0 AND 52428800),
    CONSTRAINT media_uploads_sha_valid CHECK (expected_sha256 ~ '^[0-9a-f]{64}$'),
    CONSTRAINT media_uploads_state_valid CHECK (state IN ('pending', 'finalized', 'failed', 'expired')),
    CONSTRAINT media_uploads_version_positive CHECK (version > 0),
    CONSTRAINT media_uploads_account_fk FOREIGN KEY (account_id)
        REFERENCES accounts (id) ON UPDATE RESTRICT ON DELETE RESTRICT,
    CONSTRAINT media_uploads_asset_fk FOREIGN KEY (account_id, asset_id)
        REFERENCES media_assets (account_id, id) ON UPDATE RESTRICT ON DELETE RESTRICT
);

CREATE INDEX media_uploads_account_state_expiry_index
    ON media_uploads (account_id, state, expires_at, id);

-- +goose Down
DROP TABLE media_uploads;
