-- +goose Up
CREATE TABLE content_items (
    id UUID PRIMARY KEY,
    account_id UUID NOT NULL,
    kind VARCHAR(16) COLLATE "C" NOT NULL,
    content_key VARCHAR(128) COLLATE "C" NOT NULL,
    status VARCHAR(16) COLLATE "C" NOT NULL DEFAULT 'draft',
    version BIGINT NOT NULL DEFAULT 1,
    deleted_at TIMESTAMPTZ(6) NULL,
    created_at TIMESTAMPTZ(6) NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ(6) NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT content_items_account_id_id_unique UNIQUE (account_id, id),
    CONSTRAINT content_items_account_key_unique UNIQUE (account_id, content_key),
    CONSTRAINT content_items_kind_valid CHECK (
        kind IN ('page', 'post')
        AND OCTET_LENGTH(kind) = CHAR_LENGTH(kind)
    ),
    CONSTRAINT content_items_key_valid CHECK (
        CHAR_LENGTH(content_key) BETWEEN 1 AND 128
        AND OCTET_LENGTH(content_key) = CHAR_LENGTH(content_key)
        AND content_key = LOWER(content_key)
        AND content_key !~ '[^a-z0-9_-]'
        AND content_key ~ '^[a-z][a-z0-9_-]*$'
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
        REFERENCES accounts (id) ON UPDATE RESTRICT ON DELETE RESTRICT
);

CREATE INDEX content_items_account_updated_id_index
    ON content_items (account_id, updated_at, id);
CREATE INDEX content_items_account_status_index
    ON content_items (account_id, status, updated_at, id);

-- +goose Down
DROP TABLE content_items;
