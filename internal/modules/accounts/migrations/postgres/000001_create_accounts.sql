-- +goose Up
CREATE TABLE accounts (
    id UUID PRIMARY KEY,
    slug VARCHAR(63) COLLATE "C" NOT NULL,
    status VARCHAR(32) COLLATE "C" NOT NULL,
    created_at TIMESTAMPTZ(6) NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ(6) NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT accounts_slug_unique UNIQUE (slug)
);

-- +goose Down
DROP TABLE accounts;
