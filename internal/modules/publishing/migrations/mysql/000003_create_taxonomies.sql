-- +goose Up
CREATE TABLE taxonomies (
    id BINARY(16) NOT NULL,
    account_id BINARY(16) NOT NULL,
    kind VARCHAR(16) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    name VARCHAR(128) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL,
    slug VARCHAR(128) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    version BIGINT NOT NULL DEFAULT 1,
    deleted_at DATETIME(6) NULL,
    created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    CONSTRAINT taxonomies_primary_key PRIMARY KEY (id),
    CONSTRAINT taxonomies_account_id_id_unique UNIQUE (account_id, id),
    CONSTRAINT taxonomies_account_kind_slug_unique UNIQUE (account_id, kind, slug),
    CONSTRAINT taxonomies_kind_valid CHECK (kind IN ('category', 'tag')),
    CONSTRAINT taxonomies_name_valid CHECK (
        CHAR_LENGTH(name) BETWEEN 1 AND 128
        AND CHAR_LENGTH(name) = CHAR_LENGTH(TRIM(name))
    ),
    CONSTRAINT taxonomies_slug_valid CHECK (
        OCTET_LENGTH(slug) BETWEEN 1 AND 128
        AND OCTET_LENGTH(slug) = CHAR_LENGTH(slug)
        AND slug = LOWER(slug)
        AND NOT REGEXP_LIKE(slug, '[^a-z0-9_-]', 'c')
        AND REGEXP_LIKE(slug, '^[a-z][a-z0-9_-]*$', 'c')
    ),
    CONSTRAINT taxonomies_version_positive CHECK (version > 0),
    CONSTRAINT taxonomies_deleted_at_valid CHECK (
        deleted_at IS NULL OR deleted_at >= created_at
    ),
    CONSTRAINT taxonomies_update_time_valid CHECK (updated_at >= created_at),
    CONSTRAINT taxonomies_account_fk FOREIGN KEY (account_id)
        REFERENCES accounts (id) ON UPDATE RESTRICT ON DELETE RESTRICT,
    INDEX taxonomies_account_kind_updated_id_index (account_id, kind, updated_at, id)
) ENGINE=InnoDB DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;

-- +goose Down
DROP TABLE taxonomies;
