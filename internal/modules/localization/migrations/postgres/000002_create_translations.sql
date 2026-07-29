-- +goose Up
CREATE TABLE translations (
    locale_id UUID NOT NULL,
    category VARCHAR(64) COLLATE "C" NOT NULL,
    translation_key VARCHAR(255) COLLATE "C" NOT NULL,
    value TEXT NULL,
    source VARCHAR(16) COLLATE "C" NOT NULL,
    version BIGINT NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ(6) NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ(6) NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT translations_primary_key
        PRIMARY KEY (locale_id, category, translation_key),
    CONSTRAINT translations_category_valid CHECK (
        CHAR_LENGTH(category) BETWEEN 1 AND 64
        AND OCTET_LENGTH(category) = CHAR_LENGTH(category)
        AND category = LOWER(category)
        AND category !~ '[^a-z0-9._-]'
        AND category ~ '^[a-z][a-z0-9]*([._-][a-z0-9]+)*$'
        AND category !~ '(^|[._-])(prototype|constructor)($|[._-])'
    ),
    CONSTRAINT translations_key_valid CHECK (
        CHAR_LENGTH(translation_key) BETWEEN 1 AND 255
        AND OCTET_LENGTH(translation_key) = CHAR_LENGTH(translation_key)
        AND translation_key = LOWER(translation_key)
        AND translation_key !~ '[^a-z0-9._-]'
        AND translation_key ~ '^[a-z][a-z0-9]*([_-][a-z0-9]+)*([.][a-z][a-z0-9]*([_-][a-z0-9]+)*)*$'
        AND translation_key !~ '(^|[.])(prototype|constructor)($|[.])'
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
);

-- +goose Down
DROP TABLE translations;
