-- +goose Up
CREATE TABLE workflow_events (
    id BINARY(16) NOT NULL,
    account_id BINARY(16) NOT NULL,
    content_id BINARY(16) NOT NULL,
    translation_id BINARY(16) NOT NULL,
    actor_id BINARY(16) NOT NULL,
    from_status VARCHAR(16) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    to_status VARCHAR(16) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    request_id VARCHAR(128) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    occurred_at DATETIME(6) NOT NULL,
    CONSTRAINT workflow_events_primary_key PRIMARY KEY (id),
    CONSTRAINT workflow_events_account_id_id_unique UNIQUE (account_id, id),
    CONSTRAINT workflow_events_status_valid CHECK (
        from_status IN ('draft', 'review', 'published', 'scheduled', 'archived')
        AND to_status IN ('draft', 'review', 'published', 'scheduled', 'archived')
        AND from_status <> to_status
    ),
    CONSTRAINT workflow_events_request_id_valid CHECK (
        CHAR_LENGTH(request_id) BETWEEN 1 AND 128
        AND request_id REGEXP '^[A-Za-z0-9][A-Za-z0-9._:-]*$'
    ),
    CONSTRAINT workflow_events_account_fk FOREIGN KEY (account_id)
        REFERENCES accounts (id) ON UPDATE RESTRICT ON DELETE RESTRICT,
    CONSTRAINT workflow_events_content_fk FOREIGN KEY (account_id, content_id)
        REFERENCES content_items (account_id, id) ON UPDATE RESTRICT ON DELETE RESTRICT,
    CONSTRAINT workflow_events_translation_fk FOREIGN KEY (account_id, translation_id)
        REFERENCES content_translations (account_id, id) ON UPDATE RESTRICT ON DELETE RESTRICT
) ENGINE=InnoDB;

CREATE INDEX workflow_events_content_time_index
    ON workflow_events (account_id, content_id, translation_id, occurred_at, id);

-- +goose Down
DROP TABLE workflow_events;
