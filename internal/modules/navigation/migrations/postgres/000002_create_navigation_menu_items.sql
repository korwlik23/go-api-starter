-- +goose Up
CREATE TABLE navigation_menu_items (
    id UUID PRIMARY KEY,
    account_id UUID NOT NULL,
    menu_id UUID NOT NULL,
    parent_id UUID NULL,
    label VARCHAR(255) NOT NULL,
    target_kind VARCHAR(32) COLLATE "C" NOT NULL,
    target VARCHAR(1024) COLLATE "C" NOT NULL,
    position BIGINT NOT NULL,
    enabled BOOLEAN NOT NULL DEFAULT FALSE,
    version BIGINT NOT NULL DEFAULT 1,
    deleted_at TIMESTAMPTZ(6) NULL,
    created_at TIMESTAMPTZ(6) NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ(6) NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT navigation_menu_items_account_id_unique UNIQUE (account_id, id),
    CONSTRAINT navigation_menu_items_menu_id_unique UNIQUE (account_id, menu_id, id),
    CONSTRAINT navigation_menu_items_label_valid CHECK (
        CHAR_LENGTH(label) BETWEEN 1 AND 255
        AND CHAR_LENGTH(label) = CHAR_LENGTH(BTRIM(label))
    ),
    CONSTRAINT navigation_menu_items_target_kind_valid CHECK (
        target_kind IN ('internal_path', 'external_url', 'content_key')
    ),
    CONSTRAINT navigation_menu_items_target_valid CHECK (CHAR_LENGTH(target) BETWEEN 1 AND 1024),
    CONSTRAINT navigation_menu_items_position_valid CHECK (position >= 0),
    CONSTRAINT navigation_menu_items_version_positive CHECK (version > 0),
    CONSTRAINT navigation_menu_items_deleted_at_valid CHECK (deleted_at IS NULL OR deleted_at >= created_at),
    CONSTRAINT navigation_menu_items_update_time_valid CHECK (updated_at >= created_at),
    CONSTRAINT navigation_menu_items_menu_fk FOREIGN KEY (account_id, menu_id)
        REFERENCES navigation_menus (account_id, id) ON UPDATE RESTRICT ON DELETE RESTRICT,
    CONSTRAINT navigation_menu_items_parent_fk FOREIGN KEY (account_id, menu_id, parent_id)
        REFERENCES navigation_menu_items (account_id, menu_id, id) ON UPDATE RESTRICT ON DELETE RESTRICT
);

CREATE INDEX navigation_menu_items_menu_position_index
    ON navigation_menu_items (account_id, menu_id, parent_id, position, id);
CREATE INDEX navigation_menu_items_visibility_index
    ON navigation_menu_items (account_id, menu_id, enabled, parent_id, position, id);

-- +goose Down
DROP TABLE navigation_menu_items;
