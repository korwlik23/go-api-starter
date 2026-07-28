-- +goose Up
CREATE TABLE password_credentials (
    user_id UUID NOT NULL,
    password_hash VARCHAR(255) COLLATE "C" NOT NULL,
    changed_at TIMESTAMPTZ(6) NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT password_credentials_primary_key PRIMARY KEY (user_id),
    CONSTRAINT password_credentials_user_fk FOREIGN KEY (user_id)
        REFERENCES users (id) ON UPDATE RESTRICT ON DELETE CASCADE
);

-- +goose Down
DROP TABLE password_credentials;
