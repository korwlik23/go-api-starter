-- +goose Up
CREATE TABLE navigation_menus (
    id UUID PRIMARY KEY,
    account_id UUID NOT NULL,
    locale_id UUID NOT NULL,
    menu_key VARCHAR(128) COLLATE "C" NOT NULL,
    name VARCHAR(255) NOT NULL,
    enabled BOOLEAN NOT NULL DEFAULT FALSE,
    version BIGINT NOT NULL DEFAULT 1,
    deleted_at TIMESTAMPTZ(6) NULL,
    created_at TIMESTAMPTZ(6) NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ(6) NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT navigation_menus_account_id_unique UNIQUE (account_id, id),
    CONSTRAINT navigation_menus_account_locale_key_unique UNIQUE (account_id, locale_id, menu_key),
    CONSTRAINT navigation_menus_key_valid CHECK (
        CHAR_LENGTH(menu_key) BETWEEN 1 AND 128
        AND OCTET_LENGTH(menu_key) = CHAR_LENGTH(menu_key)
        AND menu_key = LOWER(menu_key)
        AND menu_key ~ '^[a-z][a-z0-9_-]*$'
    ),
    CONSTRAINT navigation_menus_name_valid CHECK (
        CHAR_LENGTH(name) BETWEEN 1 AND 255
        AND CHAR_LENGTH(name) = CHAR_LENGTH(BTRIM(name))
    ),
    CONSTRAINT navigation_menus_version_positive CHECK (version > 0),
    CONSTRAINT navigation_menus_deleted_at_valid CHECK (deleted_at IS NULL OR deleted_at >= created_at),
    CONSTRAINT navigation_menus_update_time_valid CHECK (updated_at >= created_at),
    CONSTRAINT navigation_menus_account_fk FOREIGN KEY (account_id)
        REFERENCES accounts (id) ON UPDATE RESTRICT ON DELETE RESTRICT,
    CONSTRAINT navigation_menus_locale_fk FOREIGN KEY (locale_id)
        REFERENCES locales (id) ON UPDATE RESTRICT ON DELETE RESTRICT
);

CREATE INDEX navigation_menus_account_updated_id_index
    ON navigation_menus (account_id, updated_at, id);
CREATE INDEX navigation_menus_visibility_index
    ON navigation_menus (account_id, locale_id, enabled, menu_key);

-- +goose Down
DROP TABLE navigation_menus;
