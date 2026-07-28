-- +goose Up
CREATE TABLE users (
    id BINARY(16) NOT NULL,
    normalized_email VARCHAR(320) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NOT NULL,
    status VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    authorization_version BIGINT NOT NULL DEFAULT 1,
    created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    CONSTRAINT users_primary_key PRIMARY KEY (id),
    CONSTRAINT users_normalized_email_unique UNIQUE (normalized_email),
    CONSTRAINT users_authorization_version_positive CHECK (authorization_version > 0)
) ENGINE=InnoDB DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;

-- +goose Down
DROP TABLE users;
