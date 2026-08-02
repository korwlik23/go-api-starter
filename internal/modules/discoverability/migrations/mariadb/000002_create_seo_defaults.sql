-- +goose Up
CREATE TABLE discoverability_seo_defaults (
    id BINARY(16) NOT NULL,
    account_id BINARY(16) NOT NULL,
    locale_id BINARY(16) NOT NULL,
    title VARCHAR(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL,
    description VARCHAR(320) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NULL,
    canonical_base VARCHAR(512) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    robots VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT 'index,follow',
    version BIGINT NOT NULL DEFAULT 1,
    created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    CONSTRAINT discoverability_seo_defaults_primary_key PRIMARY KEY (id),
    CONSTRAINT discoverability_seo_defaults_account_id_unique UNIQUE (account_id, id),
    CONSTRAINT discoverability_seo_defaults_account_locale_unique UNIQUE (account_id, locale_id),
    CONSTRAINT discoverability_seo_defaults_title_valid CHECK (CHAR_LENGTH(title) BETWEEN 1 AND 255 AND title = TRIM(title)),
    CONSTRAINT discoverability_seo_defaults_description_valid CHECK (description IS NULL OR CHAR_LENGTH(description) <= 320),
    CONSTRAINT discoverability_seo_defaults_canonical_valid CHECK (canonical_base LIKE 'https://%' AND canonical_base NOT LIKE '%?%' AND canonical_base NOT LIKE '%#%'),
    CONSTRAINT discoverability_seo_defaults_robots_valid CHECK (robots IN ('index,follow', 'noindex,follow', 'index,nofollow', 'noindex,nofollow')),
    CONSTRAINT discoverability_seo_defaults_version_positive CHECK (version > 0),
    CONSTRAINT discoverability_seo_defaults_update_time_valid CHECK (updated_at >= created_at),
    CONSTRAINT discoverability_seo_defaults_account_fk FOREIGN KEY (account_id) REFERENCES accounts (id) ON UPDATE RESTRICT ON DELETE RESTRICT,
    CONSTRAINT discoverability_seo_defaults_locale_fk FOREIGN KEY (locale_id) REFERENCES locales (id) ON UPDATE RESTRICT ON DELETE RESTRICT
) ENGINE=InnoDB DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
-- +goose Down
DROP TABLE discoverability_seo_defaults;
