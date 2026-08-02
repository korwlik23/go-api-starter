-- +goose Up
CREATE TABLE publication_schedules (
    id UUID PRIMARY KEY,
    account_id UUID NOT NULL,
    translation_id UUID NOT NULL,
    publish_at TIMESTAMPTZ(6) NOT NULL,
    unpublish_at TIMESTAMPTZ(6) NULL,
    expected_version BIGINT NOT NULL,
    idempotency_key VARCHAR(128) COLLATE "C" NOT NULL,
    state VARCHAR(16) COLLATE "C" NOT NULL DEFAULT 'pending',
    lease_until TIMESTAMPTZ(6) NULL,
    attempts INTEGER NOT NULL DEFAULT 0,
    failure_reason VARCHAR(2000) COLLATE "C" NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ(6) NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ(6) NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT publication_schedules_account_key_unique UNIQUE (account_id, idempotency_key),
    CONSTRAINT publication_schedules_account_id_unique UNIQUE (account_id, id),
    CONSTRAINT publication_schedules_key_valid CHECK (idempotency_key ~ '^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$'),
    CONSTRAINT publication_schedules_version_positive CHECK (expected_version > 0),
    CONSTRAINT publication_schedules_state_valid CHECK (state IN ('pending', 'leased', 'succeeded', 'failed', 'canceled')),
    CONSTRAINT publication_schedules_attempts_valid CHECK (attempts >= 0),
    CONSTRAINT publication_schedules_failure_valid CHECK (CHAR_LENGTH(failure_reason) <= 2000 AND failure_reason !~ '[\r\n]'),
    CONSTRAINT publication_schedules_time_valid CHECK (unpublish_at IS NULL OR unpublish_at > publish_at),
    CONSTRAINT publication_schedules_account_fk FOREIGN KEY (account_id)
        REFERENCES accounts (id) ON UPDATE RESTRICT ON DELETE RESTRICT,
    CONSTRAINT publication_schedules_translation_fk FOREIGN KEY (account_id, translation_id)
        REFERENCES content_translations (account_id, id) ON UPDATE RESTRICT ON DELETE RESTRICT
);

CREATE INDEX publication_schedules_due_index
    ON publication_schedules (state, publish_at, lease_until, id);

-- +goose Down
DROP TABLE publication_schedules;
