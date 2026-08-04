-- +goose Up
CREATE TABLE system_settings (
    key VARCHAR(128) COLLATE "C" PRIMARY KEY,
    value TEXT NOT NULL,
    version BIGINT NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ(6) NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ(6) NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT system_settings_key_valid CHECK (key ~ '^[a-z][a-z0-9]*(?:[._-][a-z0-9]+)*$'),
    CONSTRAINT system_settings_value_size CHECK (OCTET_LENGTH(value) <= 16384),
    CONSTRAINT system_settings_version_positive CHECK (version > 0)
);
-- +goose Down
DROP TABLE system_settings;
