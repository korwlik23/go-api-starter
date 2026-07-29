-- +goose Up
CREATE TABLE system_role_assignments (
    user_id UUID NOT NULL,
    role_owner_type VARCHAR(16) COLLATE "C" NOT NULL,
    role_id UUID NOT NULL,
    created_at TIMESTAMPTZ(6) NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT system_role_assignments_primary_key
        PRIMARY KEY (user_id, role_id),
    CONSTRAINT system_role_assignments_owner_valid CHECK (
        role_owner_type = 'system'
        AND OCTET_LENGTH(role_owner_type) = 6
    ),
    CONSTRAINT system_role_assignments_user_fk FOREIGN KEY (user_id)
        REFERENCES users (id) ON UPDATE RESTRICT ON DELETE CASCADE,
    CONSTRAINT system_role_assignments_role_fk
        FOREIGN KEY (role_owner_type, role_id)
        REFERENCES roles (owner_type, id)
        ON UPDATE RESTRICT ON DELETE RESTRICT
);

CREATE INDEX system_role_assignments_owner_role_idx
    ON system_role_assignments (role_owner_type, role_id);

CREATE INDEX system_role_assignments_role_user_idx
    ON system_role_assignments (role_id, user_id);

-- +goose Down
DROP TABLE system_role_assignments;
