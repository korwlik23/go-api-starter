-- +goose Up
CREATE TABLE membership_roles (
    account_id BINARY(16) NOT NULL,
    membership_id BINARY(16) NOT NULL,
    role_id BINARY(16) NOT NULL,
    created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    CONSTRAINT membership_roles_primary_key
        PRIMARY KEY (account_id, membership_id, role_id),
    INDEX membership_roles_account_role_idx (account_id, role_id),
    INDEX membership_roles_role_account_membership_idx
        (role_id, account_id, membership_id),
    CONSTRAINT membership_roles_membership_fk
        FOREIGN KEY (account_id, membership_id)
        REFERENCES memberships (account_id, id)
        ON UPDATE RESTRICT ON DELETE CASCADE,
    CONSTRAINT membership_roles_role_fk
        FOREIGN KEY (account_id, role_id)
        REFERENCES roles (account_id, id)
) ENGINE=InnoDB DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;

-- +goose Down
DROP TABLE membership_roles;
