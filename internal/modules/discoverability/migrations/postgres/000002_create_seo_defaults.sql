-- +goose Up
CREATE TABLE discoverability_seo_defaults (
    id UUID PRIMARY KEY,
    account_id UUID NOT NULL,
    locale_id UUID NOT NULL,
    title VARCHAR(255) NOT NULL,
    description VARCHAR(320) NULL,
    canonical_base VARCHAR(512) COLLATE "C" NOT NULL,
    robots VARCHAR(32) COLLATE "C" NOT NULL DEFAULT 'index,follow',
    version BIGINT NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ(6) NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ(6) NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT discoverability_seo_defaults_account_id_unique UNIQUE (account_id, id),
    CONSTRAINT discoverability_seo_defaults_account_locale_unique UNIQUE (account_id, locale_id),
    CONSTRAINT discoverability_seo_defaults_title_valid CHECK (CHAR_LENGTH(title) BETWEEN 1 AND 255 AND CHAR_LENGTH(title) = CHAR_LENGTH(BTRIM(title))),
    CONSTRAINT discoverability_seo_defaults_description_valid CHECK (description IS NULL OR CHAR_LENGTH(description) <= 320),
    CONSTRAINT discoverability_seo_defaults_canonical_valid CHECK (canonical_base ~ '^https://[^?#]+$'),
    CONSTRAINT discoverability_seo_defaults_robots_valid CHECK (robots IN ('index,follow', 'noindex,follow', 'index,nofollow', 'noindex,nofollow')),
    CONSTRAINT discoverability_seo_defaults_version_positive CHECK (version > 0),
    CONSTRAINT discoverability_seo_defaults_update_time_valid CHECK (updated_at >= created_at),
    CONSTRAINT discoverability_seo_defaults_account_fk FOREIGN KEY (account_id) REFERENCES accounts (id) ON UPDATE RESTRICT ON DELETE RESTRICT,
    CONSTRAINT discoverability_seo_defaults_locale_fk FOREIGN KEY (locale_id) REFERENCES locales (id) ON UPDATE RESTRICT ON DELETE RESTRICT
);
-- +goose Down
DROP TABLE discoverability_seo_defaults;
