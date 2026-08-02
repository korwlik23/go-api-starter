-- +goose Up
CREATE TABLE media_assets (
    id BINARY(16) NOT NULL,
    account_id BINARY(16) NOT NULL,
    storage_key VARCHAR(255) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    original_name VARCHAR(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL,
    mime_type VARCHAR(128) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    byte_size BIGINT NOT NULL,
    sha256 CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    width INT NULL,
    height INT NULL,
    version BIGINT NOT NULL DEFAULT 1,
    deleted_at DATETIME(6) NULL,
    created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    CONSTRAINT media_assets_primary_key PRIMARY KEY (id),
    CONSTRAINT media_assets_account_id_unique UNIQUE (account_id, id),
    CONSTRAINT media_assets_account_storage_key_unique UNIQUE (account_id, storage_key),
    CONSTRAINT media_assets_storage_key_valid CHECK (CHAR_LENGTH(storage_key) BETWEEN 1 AND 255 AND storage_key NOT REGEXP '[\\/]'),
    CONSTRAINT media_assets_mime_valid CHECK (mime_type IN ('image/jpeg', 'image/png', 'image/gif', 'image/webp', 'application/pdf')),
    CONSTRAINT media_assets_size_valid CHECK (byte_size BETWEEN 0 AND 52428800),
    CONSTRAINT media_assets_sha_valid CHECK (sha256 REGEXP '^[0-9a-f]{64}$'),
    CONSTRAINT media_assets_dimensions_valid CHECK ((width IS NULL AND height IS NULL) OR (width > 0 AND height > 0 AND width <= 100000 AND height <= 100000)),
    CONSTRAINT media_assets_version_positive CHECK (version > 0),
    CONSTRAINT media_assets_account_fk FOREIGN KEY (account_id) REFERENCES accounts (id) ON UPDATE RESTRICT ON DELETE RESTRICT,
    INDEX media_assets_account_updated_id_index (account_id, updated_at, id)
) ENGINE=InnoDB DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;

-- +goose Down
DROP TABLE media_assets;
