-- +goose Up
CREATE TABLE system_role_assignments (
    user_id BINARY(16) NOT NULL,
    role_owner_type VARCHAR(16) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    role_id BINARY(16) NOT NULL,
    created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    CONSTRAINT system_role_assignments_primary_key
        PRIMARY KEY (user_id, role_id),
    CONSTRAINT system_role_assignments_owner_valid CHECK (
        role_owner_type = 'system'
        AND OCTET_LENGTH(role_owner_type) = 6
    ),
    INDEX system_role_assignments_owner_role_idx (role_owner_type, role_id),
    INDEX system_role_assignments_role_user_idx (role_id, user_id),
    CONSTRAINT system_role_assignments_user_fk FOREIGN KEY (user_id)
        REFERENCES users (id) ON UPDATE RESTRICT ON DELETE CASCADE,
    CONSTRAINT system_role_assignments_role_fk
        FOREIGN KEY (role_owner_type, role_id)
        REFERENCES roles (owner_type, id)
) ENGINE=InnoDB DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;

-- +goose Down
DROP TABLE system_role_assignments;
