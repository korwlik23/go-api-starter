-- +goose Up
CREATE TABLE publication_schedules (
    id BINARY(16) NOT NULL,
    account_id BINARY(16) NOT NULL,
    translation_id BINARY(16) NOT NULL,
    publish_at DATETIME(6) NOT NULL,
    unpublish_at DATETIME(6) NULL,
    expected_version BIGINT NOT NULL,
    idempotency_key VARCHAR(128) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    state VARCHAR(16) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT 'pending',
    lease_until DATETIME(6) NULL,
    attempts INT NOT NULL DEFAULT 0,
    failure_reason VARCHAR(2000) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
    created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    CONSTRAINT publication_schedules_primary_key PRIMARY KEY (id),
    CONSTRAINT publication_schedules_account_key_unique UNIQUE (account_id, idempotency_key),
    CONSTRAINT publication_schedules_account_id_unique UNIQUE (account_id, id),
    CONSTRAINT publication_schedules_key_valid CHECK (REGEXP_LIKE(idempotency_key, '^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$')),
    CONSTRAINT publication_schedules_version_positive CHECK (expected_version > 0),
    CONSTRAINT publication_schedules_state_valid CHECK (state IN ('pending', 'leased', 'succeeded', 'failed', 'canceled')),
    CONSTRAINT publication_schedules_attempts_valid CHECK (attempts >= 0),
    CONSTRAINT publication_schedules_failure_valid CHECK (CHAR_LENGTH(failure_reason) <= 2000 AND NOT REGEXP_LIKE(failure_reason, '[\r\n]')),
    CONSTRAINT publication_schedules_time_valid CHECK (unpublish_at IS NULL OR unpublish_at > publish_at),
    CONSTRAINT publication_schedules_account_fk FOREIGN KEY (account_id)
        REFERENCES accounts (id) ON UPDATE RESTRICT ON DELETE RESTRICT,
    CONSTRAINT publication_schedules_translation_fk FOREIGN KEY (account_id, translation_id)
        REFERENCES content_translations (account_id, id) ON UPDATE RESTRICT ON DELETE RESTRICT
) ENGINE=InnoDB;

CREATE INDEX publication_schedules_due_index
    ON publication_schedules (state, publish_at, lease_until, id);

-- +goose Down
DROP TABLE publication_schedules;
