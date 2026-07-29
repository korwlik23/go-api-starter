-- +goose Up
CREATE TABLE permissions (
    id BINARY(16) NOT NULL,
    permission_key VARCHAR(512) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    module_id VARCHAR(80) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    scope VARCHAR(16) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    tier VARCHAR(16) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    delegable BOOLEAN NOT NULL DEFAULT FALSE,
    active BOOLEAN NOT NULL DEFAULT FALSE,
    created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    CONSTRAINT permissions_primary_key PRIMARY KEY (id),
    CONSTRAINT permissions_permission_key_unique UNIQUE (permission_key),
    CONSTRAINT permissions_module_id_valid CHECK (
        OCTET_LENGTH(module_id) <= 40
        AND module_id REGEXP '^[a-z][a-z0-9_]{0,39}$'
        AND module_id = LOWER(module_id)
    ),
    CONSTRAINT permissions_scope_valid
        CHECK (scope IN ('own', 'any', 'system')),
    CONSTRAINT permissions_tier_valid
        CHECK (tier IN ('core', 'default', 'optional')),
    CONSTRAINT permissions_permission_key_valid CHECK (
        OCTET_LENGTH(permission_key) <= 255
        AND permission_key REGEXP
            '^[a-z][a-z0-9_]*[.][a-z][a-z0-9_]*[.][a-z][a-z0-9_]*[.](own|any|system)$'
        AND permission_key = LOWER(permission_key)
        AND SUBSTRING_INDEX(permission_key, '.', 1) = module_id
        AND SUBSTRING_INDEX(permission_key, '.', -1) = scope
    ),
    CONSTRAINT permissions_flags_valid
        CHECK (delegable IN (0, 1) AND active IN (0, 1)),
    CONSTRAINT permissions_update_time_valid
        CHECK (updated_at >= created_at)
) ENGINE=InnoDB DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;

CREATE INDEX permissions_module_active_idx
    ON permissions (module_id, active);

-- +goose Down
DROP TABLE permissions;
