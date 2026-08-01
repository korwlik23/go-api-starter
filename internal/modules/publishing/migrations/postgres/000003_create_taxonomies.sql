-- +goose Up
CREATE TABLE taxonomies (
    id UUID PRIMARY KEY,
    account_id UUID NOT NULL,
    kind VARCHAR(16) COLLATE "C" NOT NULL,
    name VARCHAR(128) NOT NULL,
    slug VARCHAR(128) COLLATE "C" NOT NULL,
    version BIGINT NOT NULL DEFAULT 1,
    deleted_at TIMESTAMPTZ(6) NULL,
    created_at TIMESTAMPTZ(6) NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ(6) NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT taxonomies_account_id_id_unique UNIQUE (account_id, id),
    CONSTRAINT taxonomies_account_kind_slug_unique UNIQUE (account_id, kind, slug),
    CONSTRAINT taxonomies_kind_valid CHECK (kind IN ('category', 'tag')),
    CONSTRAINT taxonomies_name_valid CHECK (
        CHAR_LENGTH(name) BETWEEN 1 AND 128
        AND CHAR_LENGTH(name) = CHAR_LENGTH(BTRIM(name))
    ),
    CONSTRAINT taxonomies_slug_valid CHECK (
        CHAR_LENGTH(slug) BETWEEN 1 AND 128
        AND OCTET_LENGTH(slug) = CHAR_LENGTH(slug)
        AND slug = LOWER(slug)
        AND slug !~ '[^a-z0-9_-]'
        AND slug ~ '^[a-z][a-z0-9_-]*$'
    ),
    CONSTRAINT taxonomies_version_positive CHECK (version > 0),
    CONSTRAINT taxonomies_deleted_at_valid CHECK (
        deleted_at IS NULL OR deleted_at >= created_at
    ),
    CONSTRAINT taxonomies_update_time_valid CHECK (updated_at >= created_at),
    CONSTRAINT taxonomies_account_fk FOREIGN KEY (account_id)
        REFERENCES accounts (id) ON UPDATE RESTRICT ON DELETE RESTRICT
);

CREATE INDEX taxonomies_account_kind_updated_id_index
    ON taxonomies (account_id, kind, updated_at, id);

-- +goose Down
DROP TABLE taxonomies;
