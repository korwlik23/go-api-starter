-- +goose Up
CREATE TABLE discoverability_redirects (
    id UUID PRIMARY KEY,
    account_id UUID NOT NULL,
    locale_id UUID NOT NULL,
    source_path VARCHAR(512) COLLATE "C" NOT NULL,
    destination_path VARCHAR(512) COLLATE "C" NOT NULL,
    status_code SMALLINT NOT NULL,
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    version BIGINT NOT NULL DEFAULT 1,
    deleted_at TIMESTAMPTZ(6) NULL,
    created_at TIMESTAMPTZ(6) NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ(6) NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT discoverability_redirects_account_id_unique UNIQUE (account_id, id),
    CONSTRAINT discoverability_redirects_account_locale_source_unique UNIQUE (account_id, locale_id, source_path),
    CONSTRAINT discoverability_redirects_source_valid CHECK (source_path = LOWER(source_path) AND CHAR_LENGTH(source_path) BETWEEN 1 AND 512 AND (source_path = '/' OR source_path ~ '^/[a-z0-9][a-z0-9/_-]*$') AND source_path !~ '//'),
    CONSTRAINT discoverability_redirects_destination_valid CHECK (destination_path = LOWER(destination_path) AND CHAR_LENGTH(destination_path) BETWEEN 1 AND 512 AND (destination_path = '/' OR destination_path ~ '^/[a-z0-9][a-z0-9/_-]*$') AND destination_path !~ '//'),
    CONSTRAINT discoverability_redirects_no_loop CHECK (source_path <> destination_path),
    CONSTRAINT discoverability_redirects_status_valid CHECK (status_code IN (301, 302, 307, 308)),
    CONSTRAINT discoverability_redirects_version_positive CHECK (version > 0),
    CONSTRAINT discoverability_redirects_deleted_at_valid CHECK (deleted_at IS NULL OR deleted_at >= created_at),
    CONSTRAINT discoverability_redirects_update_time_valid CHECK (updated_at >= created_at),
    CONSTRAINT discoverability_redirects_account_fk FOREIGN KEY (account_id) REFERENCES accounts (id) ON UPDATE RESTRICT ON DELETE RESTRICT,
    CONSTRAINT discoverability_redirects_locale_fk FOREIGN KEY (locale_id) REFERENCES locales (id) ON UPDATE RESTRICT ON DELETE RESTRICT
);
CREATE INDEX discoverability_redirects_lookup_index ON discoverability_redirects (account_id, locale_id, source_path, enabled);
CREATE INDEX discoverability_redirects_updated_id_index ON discoverability_redirects (account_id, updated_at, id);
-- +goose Down
DROP TABLE discoverability_redirects;
