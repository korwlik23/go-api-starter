-- +goose Up
CREATE TABLE discoverability_content_audits (
    id UUID PRIMARY KEY,
    account_id UUID NOT NULL,
    content_id UUID NOT NULL,
    translation_id UUID NOT NULL,
    locale_id UUID NOT NULL,
    content_version BIGINT NOT NULL,
    translation_version BIGINT NOT NULL,
    status VARCHAR(16) COLLATE "C" NOT NULL,
    checks JSONB NOT NULL,
    created_at TIMESTAMPTZ(6) NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ(6) NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT discoverability_content_audits_account_id_unique UNIQUE (account_id, id),
    CONSTRAINT discoverability_content_audits_revision_unique UNIQUE (account_id, translation_id, content_version, translation_version),
    CONSTRAINT discoverability_content_audits_status_valid CHECK (status IN ('pass', 'warning', 'error')),
    CONSTRAINT discoverability_content_audits_checks_valid CHECK (jsonb_typeof(checks) = 'array'),
    CONSTRAINT discoverability_content_audits_content_version_positive CHECK (content_version > 0),
    CONSTRAINT discoverability_content_audits_translation_version_positive CHECK (translation_version > 0),
    CONSTRAINT discoverability_content_audits_update_time_valid CHECK (updated_at >= created_at),
    CONSTRAINT discoverability_content_audits_account_fk FOREIGN KEY (account_id) REFERENCES accounts (id) ON UPDATE RESTRICT ON DELETE RESTRICT,
    CONSTRAINT discoverability_content_audits_content_fk FOREIGN KEY (account_id, content_id) REFERENCES content_items (account_id, id) ON UPDATE RESTRICT ON DELETE RESTRICT,
    CONSTRAINT discoverability_content_audits_translation_fk FOREIGN KEY (account_id, translation_id) REFERENCES content_translations (account_id, id) ON UPDATE RESTRICT ON DELETE RESTRICT,
    CONSTRAINT discoverability_content_audits_locale_fk FOREIGN KEY (locale_id) REFERENCES locales (id) ON UPDATE RESTRICT ON DELETE RESTRICT
);
CREATE INDEX discoverability_content_audits_account_updated_id_index ON discoverability_content_audits (account_id, updated_at, id);
CREATE INDEX discoverability_content_audits_account_content_index ON discoverability_content_audits (account_id, content_id, translation_id, updated_at);
-- +goose Down
DROP TABLE discoverability_content_audits;
