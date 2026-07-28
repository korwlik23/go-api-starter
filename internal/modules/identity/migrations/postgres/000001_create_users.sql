-- +goose Up
CREATE TABLE users (
    id UUID PRIMARY KEY,
    normalized_email VARCHAR(320) COLLATE "C" NOT NULL,
    status VARCHAR(32) COLLATE "C" NOT NULL,
    authorization_version BIGINT NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ(6) NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ(6) NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT users_normalized_email_unique UNIQUE (normalized_email),
    CONSTRAINT users_authorization_version_positive CHECK (authorization_version > 0)
);

-- +goose Down
DROP TABLE users;
