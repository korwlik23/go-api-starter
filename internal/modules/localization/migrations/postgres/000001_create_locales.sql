-- +goose Up
CREATE TABLE locales (
    id UUID PRIMARY KEY,
    tag VARCHAR(255) COLLATE "C" NOT NULL,
    name VARCHAR(128) NOT NULL,
    direction VARCHAR(16) COLLATE "C" NOT NULL,
    fallback_locale_id UUID NULL,
    enabled BOOLEAN NOT NULL DEFAULT FALSE,
    user_selectable BOOLEAN NOT NULL DEFAULT FALSE,
    default_slot SMALLINT NULL,
    version BIGINT NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ(6) NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ(6) NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT locales_tag_unique UNIQUE (tag),
    CONSTRAINT locales_default_slot_unique UNIQUE (default_slot),
    CONSTRAINT locales_tag_valid CHECK (
        CHAR_LENGTH(tag) BETWEEN 1 AND 255
        AND OCTET_LENGTH(tag) = CHAR_LENGTH(tag)
        AND tag = LOWER(tag)
        AND tag !~ '[^a-z0-9-]'
        AND tag ~ '^([a-z]{2,8}|[xi])(-[a-z0-9]{1,8})*$'
        AND tag NOT IN ('x', 'i')
    ),
    CONSTRAINT locales_name_valid CHECK (
        CHAR_LENGTH(name) BETWEEN 1 AND 128
        AND CHAR_LENGTH(name) = CHAR_LENGTH(BTRIM(name))
    ),
    CONSTRAINT locales_direction_valid CHECK (
        direction IN ('ltr', 'rtl')
        AND OCTET_LENGTH(direction) = 3
    ),
    CONSTRAINT locales_fallback_not_self CHECK (
        fallback_locale_id IS NULL OR fallback_locale_id <> id
    ),
    CONSTRAINT locales_selectable_enabled CHECK (
        NOT user_selectable OR enabled
    ),
    CONSTRAINT locales_default_valid CHECK (
        default_slot IS NULL
        OR (
            default_slot = 1
            AND enabled
            AND fallback_locale_id IS NULL
        )
    ),
    CONSTRAINT locales_version_positive CHECK (version > 0),
    CONSTRAINT locales_update_time_valid CHECK (updated_at >= created_at),
    CONSTRAINT locales_fallback_fk FOREIGN KEY (fallback_locale_id)
        REFERENCES locales (id) ON UPDATE RESTRICT ON DELETE RESTRICT
);

CREATE INDEX locales_fallback_locale_id_index
    ON locales (fallback_locale_id);
CREATE INDEX locales_visibility_tag_index
    ON locales (enabled, user_selectable, tag);

-- +goose Down
DROP TABLE locales;
