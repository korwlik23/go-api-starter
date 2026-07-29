-- +goose Up
CREATE TABLE locales (
    id BINARY(16) NOT NULL,
    tag VARCHAR(512) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    name VARCHAR(256) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL,
    direction VARCHAR(16) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    fallback_locale_id BINARY(16) NULL,
    enabled BOOLEAN NOT NULL DEFAULT FALSE,
    user_selectable BOOLEAN NOT NULL DEFAULT FALSE,
    default_slot SMALLINT NULL,
    version BIGINT NOT NULL DEFAULT 1,
    created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    CONSTRAINT locales_primary_key PRIMARY KEY (id),
    CONSTRAINT locales_tag_unique UNIQUE (tag),
    CONSTRAINT locales_default_slot_unique UNIQUE (default_slot),
    CONSTRAINT locales_tag_valid CHECK (
        OCTET_LENGTH(tag) BETWEEN 1 AND 255
        AND OCTET_LENGTH(tag) = CHAR_LENGTH(tag)
        AND tag = LOWER(tag)
        AND NOT REGEXP_LIKE(tag, '[^a-z0-9-]', 'c')
        AND REGEXP_LIKE(tag, '^([a-z]{2,8}|[xi])(-[a-z0-9]{1,8})*$', 'c')
        AND tag NOT IN ('x', 'i')
    ),
    CONSTRAINT locales_name_valid CHECK (
        CHAR_LENGTH(name) BETWEEN 1 AND 128
        AND CHAR_LENGTH(name) = CHAR_LENGTH(TRIM(name))
    ),
    CONSTRAINT locales_direction_valid CHECK (
        direction IN ('ltr', 'rtl')
        AND OCTET_LENGTH(direction) = 3
    ),
    CONSTRAINT locales_fallback_not_self CHECK (
        fallback_locale_id IS NULL OR fallback_locale_id <> id
    ),
    CONSTRAINT locales_boolean_state_valid CHECK (
        enabled IN (0, 1) AND user_selectable IN (0, 1)
    ),
    CONSTRAINT locales_selectable_enabled CHECK (
        user_selectable = 0 OR enabled = 1
    ),
    CONSTRAINT locales_default_valid CHECK (
        default_slot IS NULL
        OR (
            default_slot = 1
            AND enabled = 1
            AND fallback_locale_id IS NULL
        )
    ),
    CONSTRAINT locales_version_positive CHECK (version > 0),
    CONSTRAINT locales_update_time_valid CHECK (updated_at >= created_at),
    INDEX locales_fallback_locale_id_index (fallback_locale_id),
    INDEX locales_visibility_tag_index (enabled, user_selectable, tag),
    CONSTRAINT locales_fallback_fk FOREIGN KEY (fallback_locale_id)
        REFERENCES locales (id)
) ENGINE=InnoDB DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;

-- +goose Down
DROP TABLE locales;
