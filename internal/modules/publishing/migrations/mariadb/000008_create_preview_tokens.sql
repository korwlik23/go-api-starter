-- +goose Up
CREATE TABLE publishing_preview_tokens (
    id BINARY(16) NOT NULL,
    user_id BINARY(16) NOT NULL,
    account_id BINARY(16) NOT NULL,
    content_id BINARY(16) NOT NULL,
    translation_id BINARY(16) NOT NULL,
    revision_id BINARY(16) NOT NULL,
    token_hash BINARY(32) NOT NULL,
    hash_version SMALLINT NOT NULL,
    hash_key_version BIGINT NOT NULL,
    audience VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    expires_at DATETIME(6) NOT NULL,
    consumed_at DATETIME(6) NULL,
    created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    CONSTRAINT publishing_preview_tokens_primary_key PRIMARY KEY (id),
    CONSTRAINT publishing_preview_tokens_account_id_id_unique UNIQUE (account_id, id),
    CONSTRAINT publishing_preview_tokens_hash_valid CHECK (OCTET_LENGTH(token_hash) = 32),
    CONSTRAINT publishing_preview_tokens_versions_valid CHECK (hash_version > 0 AND hash_key_version > 0),
    CONSTRAINT publishing_preview_tokens_audience_valid CHECK (audience = 'public'),
    CONSTRAINT publishing_preview_tokens_time_valid CHECK (expires_at > created_at AND (consumed_at IS NULL OR consumed_at >= created_at)),
    CONSTRAINT publishing_preview_tokens_user_fk FOREIGN KEY (user_id) REFERENCES users (id) ON UPDATE RESTRICT ON DELETE RESTRICT,
    CONSTRAINT publishing_preview_tokens_account_fk FOREIGN KEY (account_id) REFERENCES accounts (id) ON UPDATE RESTRICT ON DELETE RESTRICT,
    CONSTRAINT publishing_preview_tokens_content_fk FOREIGN KEY (account_id, content_id) REFERENCES content_items (account_id, id) ON UPDATE RESTRICT ON DELETE RESTRICT,
    CONSTRAINT publishing_preview_tokens_translation_fk FOREIGN KEY (account_id, translation_id) REFERENCES content_translations (account_id, id) ON UPDATE RESTRICT ON DELETE RESTRICT,
    CONSTRAINT publishing_preview_tokens_revision_fk FOREIGN KEY (account_id, revision_id) REFERENCES content_revisions (account_id, id) ON UPDATE RESTRICT ON DELETE RESTRICT
) ENGINE=InnoDB;

CREATE INDEX publishing_preview_tokens_hash_index ON publishing_preview_tokens (token_hash);
CREATE INDEX publishing_preview_tokens_expiry_index ON publishing_preview_tokens (expires_at, consumed_at);

-- +goose Down
DROP TABLE publishing_preview_tokens;
