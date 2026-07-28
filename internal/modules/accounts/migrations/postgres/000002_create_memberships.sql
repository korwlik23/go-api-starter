-- +goose Up
CREATE TABLE memberships (
    id UUID PRIMARY KEY,
    account_id UUID NOT NULL,
    user_id UUID NOT NULL,
    status VARCHAR(32) COLLATE "C" NOT NULL,
    authorization_version BIGINT NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ(6) NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ(6) NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT memberships_account_user_unique UNIQUE (account_id, user_id),
    CONSTRAINT memberships_account_id_id_unique UNIQUE (account_id, id),
    CONSTRAINT memberships_authorization_version_positive
        CHECK (authorization_version > 0),
    CONSTRAINT memberships_account_fk FOREIGN KEY (account_id)
        REFERENCES accounts (id) ON UPDATE RESTRICT ON DELETE RESTRICT,
    CONSTRAINT memberships_user_fk FOREIGN KEY (user_id)
        REFERENCES users (id) ON UPDATE RESTRICT ON DELETE RESTRICT
);

CREATE INDEX memberships_user_id_idx ON memberships (user_id);

-- +goose Down
DROP TABLE memberships;
