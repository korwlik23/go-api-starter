-- +goose Up
CREATE TABLE membership_roles (
    account_id UUID NOT NULL,
    membership_id UUID NOT NULL,
    role_id UUID NOT NULL,
    created_at TIMESTAMPTZ(6) NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT membership_roles_primary_key
        PRIMARY KEY (account_id, membership_id, role_id),
    CONSTRAINT membership_roles_membership_fk
        FOREIGN KEY (account_id, membership_id)
        REFERENCES memberships (account_id, id)
        ON UPDATE RESTRICT ON DELETE CASCADE,
    CONSTRAINT membership_roles_role_fk
        FOREIGN KEY (account_id, role_id)
        REFERENCES roles (account_id, id)
        ON UPDATE RESTRICT ON DELETE RESTRICT
);

CREATE INDEX membership_roles_account_role_idx
    ON membership_roles (account_id, role_id);

CREATE INDEX membership_roles_role_account_membership_idx
    ON membership_roles (role_id, account_id, membership_id);

-- +goose Down
DROP TABLE membership_roles;
