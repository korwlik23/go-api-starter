-- +goose Up
CREATE TABLE memberships (
    id BINARY(16) NOT NULL,
    account_id BINARY(16) NOT NULL,
    user_id BINARY(16) NOT NULL,
    status VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    authorization_version BIGINT NOT NULL DEFAULT 1,
    created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    CONSTRAINT memberships_primary_key PRIMARY KEY (id),
    CONSTRAINT memberships_account_user_unique UNIQUE (account_id, user_id),
    CONSTRAINT memberships_account_id_id_unique UNIQUE (account_id, id),
    CONSTRAINT memberships_authorization_version_positive
        CHECK (authorization_version > 0),
    CONSTRAINT memberships_account_fk FOREIGN KEY (account_id)
        REFERENCES accounts (id) ON UPDATE RESTRICT ON DELETE RESTRICT,
    CONSTRAINT memberships_user_fk FOREIGN KEY (user_id)
        REFERENCES users (id) ON UPDATE RESTRICT ON DELETE RESTRICT
) ENGINE=InnoDB DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;

CREATE INDEX memberships_user_id_idx ON memberships (user_id);

-- +goose Down
DROP TABLE memberships;
