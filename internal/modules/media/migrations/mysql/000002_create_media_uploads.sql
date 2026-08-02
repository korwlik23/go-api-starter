-- +goose Up
CREATE TABLE media_uploads (
    id BINARY(16) NOT NULL,
    account_id BINARY(16) NOT NULL,
    storage_key VARCHAR(255) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    original_name VARCHAR(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL,
    mime_type VARCHAR(128) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    declared_size BIGINT NOT NULL,
    expected_sha256 CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    state VARCHAR(16) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT 'pending',
    asset_id BINARY(16) NULL,
    version BIGINT NOT NULL DEFAULT 1,
    expires_at DATETIME(6) NOT NULL,
    created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    CONSTRAINT media_uploads_primary_key PRIMARY KEY (id),
    CONSTRAINT media_uploads_account_id_unique UNIQUE (account_id, id),
    CONSTRAINT media_uploads_account_storage_key_unique UNIQUE (account_id, storage_key),
    CONSTRAINT media_uploads_storage_key_valid CHECK (CHAR_LENGTH(storage_key) BETWEEN 1 AND 255 AND storage_key NOT REGEXP '[\\/]'),
    CONSTRAINT media_uploads_mime_valid CHECK (mime_type IN ('image/jpeg', 'image/png', 'image/gif', 'image/webp', 'application/pdf')),
    CONSTRAINT media_uploads_size_valid CHECK (declared_size BETWEEN 0 AND 52428800),
    CONSTRAINT media_uploads_sha_valid CHECK (expected_sha256 REGEXP '^[0-9a-f]{64}$'),
    CONSTRAINT media_uploads_state_valid CHECK (state IN ('pending', 'finalized', 'failed', 'expired')),
    CONSTRAINT media_uploads_version_positive CHECK (version > 0),
    CONSTRAINT media_uploads_account_fk FOREIGN KEY (account_id) REFERENCES accounts (id) ON UPDATE RESTRICT ON DELETE RESTRICT,
    CONSTRAINT media_uploads_asset_fk FOREIGN KEY (account_id, asset_id) REFERENCES media_assets (account_id, id) ON UPDATE RESTRICT ON DELETE RESTRICT,
    INDEX media_uploads_account_state_expiry_index (account_id, state, expires_at, id)
) ENGINE=InnoDB DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;

-- +goose Down
DROP TABLE media_uploads;
