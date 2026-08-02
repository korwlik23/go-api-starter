-- +goose Up
CREATE TABLE media_assets (
    id UUID PRIMARY KEY,
    account_id UUID NOT NULL,
    storage_key VARCHAR(255) COLLATE "C" NOT NULL,
    original_name VARCHAR(255) NOT NULL,
    mime_type VARCHAR(128) COLLATE "C" NOT NULL,
    byte_size BIGINT NOT NULL,
    sha256 CHAR(64) COLLATE "C" NOT NULL,
    width INTEGER NULL,
    height INTEGER NULL,
    version BIGINT NOT NULL DEFAULT 1,
    deleted_at TIMESTAMPTZ(6) NULL,
    created_at TIMESTAMPTZ(6) NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ(6) NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT media_assets_account_id_unique UNIQUE (account_id, id),
    CONSTRAINT media_assets_account_storage_key_unique UNIQUE (account_id, storage_key),
    CONSTRAINT media_assets_storage_key_valid CHECK (CHAR_LENGTH(storage_key) BETWEEN 1 AND 255 AND storage_key !~ '[\\/]'),
    CONSTRAINT media_assets_mime_valid CHECK (mime_type IN ('image/jpeg', 'image/png', 'image/gif', 'image/webp', 'application/pdf')),
    CONSTRAINT media_assets_size_valid CHECK (byte_size BETWEEN 0 AND 52428800),
    CONSTRAINT media_assets_sha_valid CHECK (sha256 ~ '^[0-9a-f]{64}$'),
    CONSTRAINT media_assets_dimensions_valid CHECK ((width IS NULL AND height IS NULL) OR (width > 0 AND height > 0 AND width <= 100000 AND height <= 100000)),
    CONSTRAINT media_assets_version_positive CHECK (version > 0),
    CONSTRAINT media_assets_account_fk FOREIGN KEY (account_id)
        REFERENCES accounts (id) ON UPDATE RESTRICT ON DELETE RESTRICT
);

CREATE INDEX media_assets_account_updated_id_index
    ON media_assets (account_id, updated_at, id);

-- +goose Down
DROP TABLE media_assets;
