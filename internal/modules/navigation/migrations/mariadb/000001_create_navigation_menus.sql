-- +goose Up
CREATE TABLE navigation_menus (
    id BINARY(16) NOT NULL,
    account_id BINARY(16) NOT NULL,
    locale_id BINARY(16) NOT NULL,
    menu_key VARCHAR(128) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    name VARCHAR(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL,
    enabled BOOLEAN NOT NULL DEFAULT FALSE,
    version BIGINT NOT NULL DEFAULT 1,
    deleted_at DATETIME(6) NULL,
    created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    CONSTRAINT navigation_menus_primary_key PRIMARY KEY (id),
    CONSTRAINT navigation_menus_account_id_unique UNIQUE (account_id, id),
    CONSTRAINT navigation_menus_account_locale_key_unique UNIQUE (account_id, locale_id, menu_key),
    CONSTRAINT navigation_menus_key_valid CHECK (CHAR_LENGTH(menu_key) BETWEEN 1 AND 128 AND menu_key = LOWER(menu_key) AND menu_key REGEXP '^[a-z][a-z0-9_-]*$'),
    CONSTRAINT navigation_menus_name_valid CHECK (CHAR_LENGTH(name) BETWEEN 1 AND 255 AND name = TRIM(name)),
    CONSTRAINT navigation_menus_version_positive CHECK (version > 0),
    CONSTRAINT navigation_menus_deleted_at_valid CHECK (deleted_at IS NULL OR deleted_at >= created_at),
    CONSTRAINT navigation_menus_update_time_valid CHECK (updated_at >= created_at),
    CONSTRAINT navigation_menus_account_fk FOREIGN KEY (account_id) REFERENCES accounts (id) ON UPDATE RESTRICT ON DELETE RESTRICT,
    CONSTRAINT navigation_menus_locale_fk FOREIGN KEY (locale_id) REFERENCES locales (id) ON UPDATE RESTRICT ON DELETE RESTRICT,
    INDEX navigation_menus_account_updated_id_index (account_id, updated_at, id),
    INDEX navigation_menus_visibility_index (account_id, locale_id, enabled, menu_key)
) ENGINE=InnoDB DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;

-- +goose Down
DROP TABLE navigation_menus;
