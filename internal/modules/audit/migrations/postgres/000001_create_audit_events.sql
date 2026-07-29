-- +goose Up
CREATE TABLE audit_events (
    id UUID NOT NULL,
    scope VARCHAR(16) COLLATE "C" NOT NULL,
    account_id UUID NULL,
    actor_type VARCHAR(16) COLLATE "C" NOT NULL,
    actor_id UUID NULL,
    action VARCHAR(128) COLLATE "C" NOT NULL,
    operation_id UUID NOT NULL,
    request_id VARCHAR(128) COLLATE "C" NULL,
    event_data TEXT COLLATE "C" NOT NULL,
    occurred_at TIMESTAMPTZ(6) NOT NULL,
    recorded_at TIMESTAMPTZ(6) NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT audit_events_primary_key PRIMARY KEY (id),
    CONSTRAINT audit_events_scope_valid CHECK (
        (scope = 'system' AND OCTET_LENGTH(scope) = 6)
        OR (scope = 'account' AND OCTET_LENGTH(scope) = 7)
    ),
    CONSTRAINT audit_events_scope_account_valid CHECK (
        (scope = 'system' AND account_id IS NULL)
        OR (scope = 'account' AND account_id IS NOT NULL)
    ),
    CONSTRAINT audit_events_actor_type_valid CHECK (
        (actor_type = 'user' AND OCTET_LENGTH(actor_type) = 4)
        OR (actor_type = 'system' AND OCTET_LENGTH(actor_type) = 6)
        OR (actor_type = 'anonymous' AND OCTET_LENGTH(actor_type) = 9)
    ),
    CONSTRAINT audit_events_actor_identity_valid CHECK (
        (actor_type = 'user' AND actor_id IS NOT NULL)
        OR (actor_type IN ('system', 'anonymous') AND actor_id IS NULL)
    ),
    CONSTRAINT audit_events_action_valid CHECK (
        OCTET_LENGTH(action) BETWEEN 3 AND 128
        AND OCTET_LENGTH(action) = CHAR_LENGTH(action)
        AND action ~ '^[a-z][a-z0-9]*([.][a-z][a-z0-9]*)+$'
    ),
    CONSTRAINT audit_events_request_id_valid CHECK (
        request_id IS NULL OR (
            OCTET_LENGTH(request_id) BETWEEN 1 AND 128
            AND OCTET_LENGTH(request_id) = CHAR_LENGTH(request_id)
            AND request_id ~ '^[A-Za-z0-9][A-Za-z0-9._:-]*$'
        )
    ),
    CONSTRAINT audit_events_event_data_valid CHECK (
        OCTET_LENGTH(event_data) BETWEEN 2 AND 16384
        AND jsonb_typeof(event_data::jsonb) = 'object'
    ),
    CONSTRAINT audit_events_account_fk FOREIGN KEY (account_id)
        REFERENCES accounts (id) ON UPDATE RESTRICT ON DELETE RESTRICT,
    CONSTRAINT audit_events_actor_fk FOREIGN KEY (actor_id)
        REFERENCES users (id) ON UPDATE RESTRICT ON DELETE RESTRICT
);

CREATE INDEX audit_events_account_time_index
    ON audit_events (account_id, recorded_at, id);
CREATE INDEX audit_events_actor_time_index
    ON audit_events (actor_id, actor_type, account_id, recorded_at, id);
CREATE INDEX audit_events_operation_time_index
    ON audit_events (account_id, operation_id, recorded_at, id);
CREATE INDEX audit_events_request_time_index
    ON audit_events (account_id, request_id, recorded_at, id);
CREATE INDEX audit_events_retention_index
    ON audit_events (recorded_at, id);

-- +goose Down
DROP TABLE audit_events;
