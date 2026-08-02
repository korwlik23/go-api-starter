-- +goose Up
CREATE TABLE content_revisions (
    id UUID PRIMARY KEY,
    account_id UUID NOT NULL,
    content_id UUID NOT NULL,
    translation_id UUID NOT NULL,
    author_id UUID NOT NULL,
    snapshot_hash VARCHAR(64) COLLATE "C" NOT NULL,
    change_summary VARCHAR(1000) COLLATE "C" NOT NULL DEFAULT '',
    source_version BIGINT NOT NULL,
    snapshot JSONB NOT NULL,
    created_at TIMESTAMPTZ(6) NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT content_revisions_account_id_id_unique UNIQUE (account_id, id),
    CONSTRAINT content_revisions_snapshot_hash_valid CHECK (
        CHAR_LENGTH(snapshot_hash) = 64
        AND snapshot_hash ~ '^[a-f0-9]{64}$'
    ),
    CONSTRAINT content_revisions_summary_valid CHECK (CHAR_LENGTH(change_summary) <= 1000),
    CONSTRAINT content_revisions_source_version_positive CHECK (source_version > 0),
    CONSTRAINT content_revisions_snapshot_valid CHECK (jsonb_typeof(snapshot) = 'object'),
    CONSTRAINT content_revisions_account_fk FOREIGN KEY (account_id)
        REFERENCES accounts (id) ON UPDATE RESTRICT ON DELETE RESTRICT,
    CONSTRAINT content_revisions_content_fk FOREIGN KEY (account_id, content_id)
        REFERENCES content_items (account_id, id) ON UPDATE RESTRICT ON DELETE RESTRICT,
    CONSTRAINT content_revisions_translation_fk FOREIGN KEY (account_id, translation_id)
        REFERENCES content_translations (account_id, id) ON UPDATE RESTRICT ON DELETE RESTRICT
);

CREATE INDEX content_revisions_content_created_id_index
    ON content_revisions (account_id, content_id, translation_id, created_at, id);

-- +goose Down
DROP TABLE content_revisions;
