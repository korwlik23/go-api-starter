-- +goose Up
CREATE TABLE permissions (
    id UUID PRIMARY KEY,
    permission_key VARCHAR(255) COLLATE "C" NOT NULL,
    module_id VARCHAR(40) COLLATE "C" NOT NULL,
    scope VARCHAR(16) COLLATE "C" NOT NULL,
    tier VARCHAR(16) COLLATE "C" NOT NULL,
    delegable BOOLEAN NOT NULL DEFAULT FALSE,
    active BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ(6) NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ(6) NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT permissions_permission_key_unique UNIQUE (permission_key),
    CONSTRAINT permissions_module_id_valid
        CHECK (module_id ~ '^[a-z][a-z0-9_]{0,39}$'),
    CONSTRAINT permissions_scope_valid
        CHECK (scope IN ('own', 'any', 'system')),
    CONSTRAINT permissions_tier_valid
        CHECK (tier IN ('core', 'default', 'optional')),
    CONSTRAINT permissions_permission_key_valid CHECK (
        permission_key ~
            '^[a-z][a-z0-9_]*[.][a-z][a-z0-9_]*[.][a-z][a-z0-9_]*[.](own|any|system)$'
        AND split_part(permission_key, '.', 1) = module_id
        AND split_part(permission_key, '.', 4) = scope
    ),
    CONSTRAINT permissions_update_time_valid
        CHECK (updated_at >= created_at)
);

CREATE INDEX permissions_module_active_idx
    ON permissions (module_id, active);

-- +goose Down
DROP TABLE permissions;
