-- +goose Up
CREATE TABLE role_permissions (
    role_id UUID NOT NULL,
    permission_id UUID NOT NULL,
    created_at TIMESTAMPTZ(6) NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT role_permissions_primary_key PRIMARY KEY (role_id, permission_id),
    CONSTRAINT role_permissions_role_fk FOREIGN KEY (role_id)
        REFERENCES roles (id) ON UPDATE RESTRICT ON DELETE CASCADE,
    CONSTRAINT role_permissions_permission_fk FOREIGN KEY (permission_id)
        REFERENCES permissions (id) ON UPDATE RESTRICT ON DELETE RESTRICT
);

CREATE INDEX role_permissions_permission_role_idx
    ON role_permissions (permission_id, role_id);

-- +goose Down
DROP TABLE role_permissions;
