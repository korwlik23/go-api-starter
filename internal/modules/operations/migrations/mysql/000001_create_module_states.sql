-- +goose Up
CREATE TABLE module_states (
    module_id VARCHAR(80) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    enabled TINYINT NOT NULL,
    module_checksum VARBINARY(64) NOT NULL,
    config_checksum VARBINARY(64) NOT NULL,
    catalog_checksum VARBINARY(64) NOT NULL,
    catalog_epoch BIGINT NOT NULL,
    reconcile_revision BIGINT NOT NULL,
    reconciled_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    CONSTRAINT module_states_primary_key PRIMARY KEY (module_id),
    CONSTRAINT module_states_module_id_valid CHECK (
        OCTET_LENGTH(module_id) BETWEEN 1 AND 40
        AND module_id = LOWER(module_id)
        AND REGEXP_LIKE(module_id, '^[a-z][a-z0-9_]{0,39}$', 'c')
    ),
    CONSTRAINT module_states_enabled_valid CHECK (enabled IN (0, 1)),
    CONSTRAINT module_states_module_checksum_valid CHECK (
        OCTET_LENGTH(module_checksum) = 32
    ),
    CONSTRAINT module_states_config_checksum_valid CHECK (
        OCTET_LENGTH(config_checksum) = 32
    ),
    CONSTRAINT module_states_catalog_checksum_valid CHECK (
        OCTET_LENGTH(catalog_checksum) = 32
    ),
    CONSTRAINT module_states_catalog_epoch_positive CHECK (catalog_epoch > 0),
    CONSTRAINT module_states_reconcile_revision_positive CHECK (reconcile_revision > 0)
) ENGINE=InnoDB DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;

-- +goose Down
DROP TABLE module_states;
