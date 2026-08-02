-- +goose Up
CREATE TABLE content_revisions (
    id BINARY(16) NOT NULL,
    account_id BINARY(16) NOT NULL,
    content_id BINARY(16) NOT NULL,
    translation_id BINARY(16) NOT NULL,
    author_id BINARY(16) NOT NULL,
    snapshot_hash VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    change_summary VARCHAR(1000) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
    source_version BIGINT NOT NULL,
    snapshot JSON NOT NULL,
    created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    CONSTRAINT content_revisions_primary_key PRIMARY KEY (id),
    CONSTRAINT content_revisions_account_id_id_unique UNIQUE (account_id, id),
    CONSTRAINT content_revisions_snapshot_hash_valid CHECK (
        CHAR_LENGTH(snapshot_hash) = 64
        AND REGEXP_LIKE(snapshot_hash, '^[a-f0-9]{64}$')
    ),
    CONSTRAINT content_revisions_summary_valid CHECK (CHAR_LENGTH(change_summary) <= 1000),
    CONSTRAINT content_revisions_source_version_positive CHECK (source_version > 0),
    CONSTRAINT content_revisions_snapshot_valid CHECK (JSON_VALID(snapshot) AND JSON_TYPE(snapshot) = 'OBJECT'),
    CONSTRAINT content_revisions_account_fk FOREIGN KEY (account_id)
        REFERENCES accounts (id) ON UPDATE RESTRICT ON DELETE RESTRICT,
    CONSTRAINT content_revisions_content_fk FOREIGN KEY (account_id, content_id)
        REFERENCES content_items (account_id, id) ON UPDATE RESTRICT ON DELETE RESTRICT,
    CONSTRAINT content_revisions_translation_fk FOREIGN KEY (account_id, translation_id)
        REFERENCES content_translations (account_id, id) ON UPDATE RESTRICT ON DELETE RESTRICT
) ENGINE=InnoDB;

CREATE INDEX content_revisions_content_created_id_index
    ON content_revisions (account_id, content_id, translation_id, created_at, id);

-- +goose Down
DROP TABLE content_revisions;
