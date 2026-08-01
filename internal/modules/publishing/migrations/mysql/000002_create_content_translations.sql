-- +goose Up
CREATE TABLE content_translations (
    id BINARY(16) NOT NULL,
    account_id BINARY(16) NOT NULL,
    content_id BINARY(16) NOT NULL,
    locale_id BINARY(16) NOT NULL,
    slug VARCHAR(160) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    path VARCHAR(512) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    title VARCHAR(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL,
    excerpt TEXT CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NULL,
    blocks JSON NOT NULL,
    seo_title VARCHAR(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NULL,
    seo_description VARCHAR(320) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NULL,
    geo_region VARCHAR(128) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NULL,
    geo_locality VARCHAR(128) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NULL,
    aeo_question VARCHAR(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NULL,
    aeo_answer TEXT CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NULL,
    status VARCHAR(16) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT 'draft',
    version BIGINT NOT NULL DEFAULT 1,
    published_at DATETIME(6) NULL,
    deleted_at DATETIME(6) NULL,
    created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    CONSTRAINT content_translations_primary_key PRIMARY KEY (id),
    CONSTRAINT content_translations_account_id_id_unique UNIQUE (account_id, id),
    CONSTRAINT content_translations_content_locale_unique UNIQUE (content_id, locale_id),
    CONSTRAINT content_translations_account_locale_path_unique
        UNIQUE (account_id, locale_id, path),
    CONSTRAINT content_translations_slug_valid CHECK (
        OCTET_LENGTH(slug) BETWEEN 1 AND 160
        AND OCTET_LENGTH(slug) = CHAR_LENGTH(slug)
        AND slug = LOWER(slug)
        AND NOT REGEXP_LIKE(slug, '[^a-z0-9_-]', 'c')
        AND REGEXP_LIKE(slug, '^[a-z][a-z0-9_-]*$', 'c')
    ),
    CONSTRAINT content_translations_path_valid CHECK (
        OCTET_LENGTH(path) BETWEEN 1 AND 512
        AND OCTET_LENGTH(path) = CHAR_LENGTH(path)
        AND path = LOWER(path)
        AND NOT REGEXP_LIKE(path, '[^a-z0-9/_-]', 'c')
        AND (path = '/' OR REGEXP_LIKE(path, '^/[a-z0-9][a-z0-9/_-]*$', 'c'))
        AND NOT REGEXP_LIKE(path, '//', 'c')
    ),
    CONSTRAINT content_translations_title_valid CHECK (
        CHAR_LENGTH(title) BETWEEN 1 AND 255
        AND CHAR_LENGTH(title) = CHAR_LENGTH(TRIM(title))
    ),
    CONSTRAINT content_translations_excerpt_valid CHECK (
        excerpt IS NULL OR CHAR_LENGTH(excerpt) <= 2000
    ),
    CONSTRAINT content_translations_blocks_valid CHECK (
        JSON_VALID(blocks) AND JSON_TYPE(blocks) = 'ARRAY'
        AND OCTET_LENGTH(blocks) <= 1048576
    ),
    CONSTRAINT content_translations_seo_title_valid CHECK (
        seo_title IS NULL OR CHAR_LENGTH(seo_title) BETWEEN 1 AND 255
    ),
    CONSTRAINT content_translations_seo_description_valid CHECK (
        seo_description IS NULL OR CHAR_LENGTH(seo_description) BETWEEN 1 AND 320
    ),
    CONSTRAINT content_translations_geo_region_valid CHECK (
        geo_region IS NULL OR CHAR_LENGTH(geo_region) BETWEEN 1 AND 128
    ),
    CONSTRAINT content_translations_geo_locality_valid CHECK (
        geo_locality IS NULL OR CHAR_LENGTH(geo_locality) BETWEEN 1 AND 128
    ),
    CONSTRAINT content_translations_aeo_question_valid CHECK (
        aeo_question IS NULL OR CHAR_LENGTH(aeo_question) BETWEEN 1 AND 255
    ),
    CONSTRAINT content_translations_aeo_answer_valid CHECK (
        aeo_answer IS NULL OR CHAR_LENGTH(aeo_answer) BETWEEN 1 AND 20000
    ),
    CONSTRAINT content_translations_status_valid CHECK (
        status IN ('draft', 'review', 'published', 'scheduled', 'archived')
    ),
    CONSTRAINT content_translations_version_positive CHECK (version > 0),
    CONSTRAINT content_translations_published_at_valid CHECK (
        status <> 'published' OR published_at IS NOT NULL
    ),
    CONSTRAINT content_translations_deleted_at_valid CHECK (
        deleted_at IS NULL OR deleted_at >= created_at
    ),
    CONSTRAINT content_translations_update_time_valid CHECK (updated_at >= created_at),
    CONSTRAINT content_translations_account_fk FOREIGN KEY (account_id)
        REFERENCES accounts (id) ON UPDATE RESTRICT ON DELETE RESTRICT,
    CONSTRAINT content_translations_content_fk FOREIGN KEY (account_id, content_id)
        REFERENCES content_items (account_id, id) ON UPDATE RESTRICT ON DELETE RESTRICT,
    CONSTRAINT content_translations_locale_fk FOREIGN KEY (locale_id)
        REFERENCES locales (id) ON UPDATE RESTRICT ON DELETE RESTRICT,
    INDEX content_translations_account_updated_id_index (account_id, updated_at, id),
    INDEX content_translations_locale_path_status_index (locale_id, path, status)
) ENGINE=InnoDB DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;

-- +goose Down
DROP TABLE content_translations;
