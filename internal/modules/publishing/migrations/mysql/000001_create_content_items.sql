-- +goose Up
CREATE TABLE content_items (
    id BINARY(16) NOT NULL,
    account_id BINARY(16) NOT NULL,
    kind VARCHAR(16) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    content_key VARCHAR(128) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    status VARCHAR(16) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT 'draft',
    version BIGINT NOT NULL DEFAULT 1,
    deleted_at DATETIME(6) NULL,
    created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    CONSTRAINT content_items_primary_key PRIMARY KEY (id),
    CONSTRAINT content_items_account_id_id_unique UNIQUE (account_id, id),
    CONSTRAINT content_items_account_key_unique UNIQUE (account_id, content_key),
    CONSTRAINT content_items_kind_valid CHECK (
        kind IN ('page', 'post')
        AND OCTET_LENGTH(kind) = CHAR_LENGTH(kind)
    ),
    CONSTRAINT content_items_key_valid CHECK (
        OCTET_LENGTH(content_key) BETWEEN 1 AND 128
        AND OCTET_LENGTH(content_key) = CHAR_LENGTH(content_key)
        AND content_key = LOWER(content_key)
        AND NOT REGEXP_LIKE(content_key, '[^a-z0-9_-]', 'c')
        AND REGEXP_LIKE(content_key, '^[a-z][a-z0-9_-]*$', 'c')
    ),
    CONSTRAINT content_items_status_valid CHECK (
        status IN ('draft', 'review', 'published', 'scheduled', 'archived')
    ),
    CONSTRAINT content_items_version_positive CHECK (version > 0),
    CONSTRAINT content_items_deleted_at_valid CHECK (
        deleted_at IS NULL OR deleted_at >= created_at
    ),
    CONSTRAINT content_items_update_time_valid CHECK (updated_at >= created_at),
    CONSTRAINT content_items_account_fk FOREIGN KEY (account_id)
        REFERENCES accounts (id) ON UPDATE RESTRICT ON DELETE RESTRICT,
    INDEX content_items_account_updated_id_index (account_id, updated_at, id),
    INDEX content_items_account_status_index (account_id, status, updated_at, id)
) ENGINE=InnoDB DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;

-- +goose Down
DROP TABLE content_items;
