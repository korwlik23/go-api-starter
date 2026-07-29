-- +goose Up
CREATE TABLE module_states (
    module_id VARCHAR(40) COLLATE "C" NOT NULL,
    enabled BOOLEAN NOT NULL,
    module_checksum BYTEA NOT NULL,
    config_checksum BYTEA NOT NULL,
    catalog_checksum BYTEA NOT NULL,
    catalog_epoch BIGINT NOT NULL,
    reconcile_revision BIGINT NOT NULL,
    reconciled_at TIMESTAMPTZ(6) NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT module_states_primary_key PRIMARY KEY (module_id),
    CONSTRAINT module_states_module_id_valid CHECK (
        module_id ~ '^[a-z][a-z0-9_]{0,39}$'
    ),
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
);

-- +goose Down
DROP TABLE module_states;
