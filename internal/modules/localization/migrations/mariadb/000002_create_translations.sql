-- +goose Up
CREATE TABLE translations (
    locale_id BINARY(16) NOT NULL,
    category VARCHAR(128) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    translation_key VARCHAR(512) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    value TEXT CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NULL,
    source VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    version BIGINT NOT NULL DEFAULT 1,
    created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    CONSTRAINT translations_primary_key
        PRIMARY KEY (locale_id, category, translation_key),
    CONSTRAINT translations_category_valid CHECK (
        OCTET_LENGTH(category) BETWEEN 1 AND 64
        AND OCTET_LENGTH(category) = CHAR_LENGTH(category)
        AND category = LOWER(category)
        AND category NOT REGEXP '[^a-z0-9._-]'
        AND category REGEXP '^[a-z][a-z0-9]*([._-][a-z0-9]+)*$'
        AND category NOT REGEXP '(^|[._-])(prototype|constructor)($|[._-])'
    ),
    CONSTRAINT translations_key_valid CHECK (
        OCTET_LENGTH(translation_key) BETWEEN 1 AND 255
        AND OCTET_LENGTH(translation_key) = CHAR_LENGTH(translation_key)
        AND translation_key = LOWER(translation_key)
        AND translation_key NOT REGEXP '[^a-z0-9._-]'
        AND translation_key REGEXP '^[a-z][a-z0-9]*([_-][a-z0-9]+)*([.][a-z][a-z0-9]*([_-][a-z0-9]+)*)*$'
        AND translation_key NOT REGEXP '(^|[.])(prototype|constructor)($|[.])'
    ),
    CONSTRAINT translations_value_valid CHECK (
        value IS NULL OR OCTET_LENGTH(value) BETWEEN 1 AND 16384
    ),
    CONSTRAINT translations_source_valid CHECK (
        source IN ('editor', 'import')
        AND OCTET_LENGTH(source) = 6
    ),
    CONSTRAINT translations_version_positive CHECK (version > 0),
    CONSTRAINT translations_update_time_valid CHECK (updated_at >= created_at),
    CONSTRAINT translations_locale_fk FOREIGN KEY (locale_id)
        REFERENCES locales (id) ON UPDATE RESTRICT ON DELETE RESTRICT
) ENGINE=InnoDB DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;

-- +goose Down
DROP TABLE translations;
