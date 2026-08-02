-- +goose Up
CREATE TABLE navigation_menu_items (
    id BINARY(16) NOT NULL,
    account_id BINARY(16) NOT NULL,
    menu_id BINARY(16) NOT NULL,
    parent_id BINARY(16) NULL,
    label VARCHAR(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL,
    target_kind VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    target VARCHAR(1024) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL,
    position BIGINT NOT NULL,
    enabled BOOLEAN NOT NULL DEFAULT FALSE,
    version BIGINT NOT NULL DEFAULT 1,
    deleted_at DATETIME(6) NULL,
    created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    CONSTRAINT navigation_menu_items_primary_key PRIMARY KEY (id),
    CONSTRAINT navigation_menu_items_account_id_unique UNIQUE (account_id, id),
    CONSTRAINT navigation_menu_items_menu_id_unique UNIQUE (account_id, menu_id, id),
    CONSTRAINT navigation_menu_items_label_valid CHECK (CHAR_LENGTH(label) BETWEEN 1 AND 255 AND label = TRIM(label)),
    CONSTRAINT navigation_menu_items_target_kind_valid CHECK (target_kind IN ('internal_path', 'external_url', 'content_key')),
    CONSTRAINT navigation_menu_items_target_valid CHECK (CHAR_LENGTH(target) BETWEEN 1 AND 1024),
    CONSTRAINT navigation_menu_items_position_valid CHECK (position >= 0),
    CONSTRAINT navigation_menu_items_version_positive CHECK (version > 0),
    CONSTRAINT navigation_menu_items_deleted_at_valid CHECK (deleted_at IS NULL OR deleted_at >= created_at),
    CONSTRAINT navigation_menu_items_update_time_valid CHECK (updated_at >= created_at),
    CONSTRAINT navigation_menu_items_menu_fk FOREIGN KEY (account_id, menu_id) REFERENCES navigation_menus (account_id, id) ON UPDATE RESTRICT ON DELETE RESTRICT,
    CONSTRAINT navigation_menu_items_parent_fk FOREIGN KEY (account_id, menu_id, parent_id) REFERENCES navigation_menu_items (account_id, menu_id, id) ON UPDATE RESTRICT ON DELETE RESTRICT,
    INDEX navigation_menu_items_menu_position_index (account_id, menu_id, parent_id, position, id),
    INDEX navigation_menu_items_visibility_index (account_id, menu_id, enabled, parent_id, position, id)
) ENGINE=InnoDB DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;

-- +goose Down
DROP TABLE navigation_menu_items;
