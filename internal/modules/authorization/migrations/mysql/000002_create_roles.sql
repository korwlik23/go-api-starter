-- +goose Up
CREATE TABLE roles (
    id BINARY(16) NOT NULL,
    owner_type VARCHAR(16) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    account_id BINARY(16) NULL,
    name VARCHAR(256) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL,
    version BIGINT NOT NULL DEFAULT 1,
    created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    CONSTRAINT roles_primary_key PRIMARY KEY (id),
    CONSTRAINT roles_account_id_id_unique UNIQUE (account_id, id),
    CONSTRAINT roles_owner_type_id_unique UNIQUE (owner_type, id),
    CONSTRAINT roles_owner_valid CHECK (
        (
            owner_type = 'system'
            AND OCTET_LENGTH(owner_type) = 6
            AND account_id IS NULL
        )
        OR
        (
            owner_type = 'account'
            AND OCTET_LENGTH(owner_type) = 7
            AND account_id IS NOT NULL
        )
    ),
    CONSTRAINT roles_name_valid CHECK (
        CHAR_LENGTH(name) BETWEEN 1 AND 128
        AND CHAR_LENGTH(name) = CHAR_LENGTH(TRIM(name))
    ),
    CONSTRAINT roles_version_positive CHECK (version > 0),
    CONSTRAINT roles_update_time_valid CHECK (updated_at >= created_at),
    CONSTRAINT roles_account_fk FOREIGN KEY (account_id)
        REFERENCES accounts (id)
) ENGINE=InnoDB DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;

-- +goose Down
DROP TABLE roles;
