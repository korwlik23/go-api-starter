-- +goose Up
CREATE TABLE notifications (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL,
    title VARCHAR(255) NOT NULL,
    body TEXT NOT NULL,
    kind VARCHAR(32) NOT NULL DEFAULT 'info',
    read_at TIMESTAMPTZ(6) NULL,
    created_at TIMESTAMPTZ(6) NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT notifications_user_fk FOREIGN KEY (user_id) REFERENCES users (id) ON UPDATE RESTRICT ON DELETE CASCADE,
    CONSTRAINT notifications_title_valid CHECK (CHAR_LENGTH(title) BETWEEN 1 AND 255),
    CONSTRAINT notifications_body_valid CHECK (OCTET_LENGTH(body) BETWEEN 1 AND 16384),
    CONSTRAINT notifications_kind_valid CHECK (kind ~ '^[a-z][a-z0-9_.-]{0,31}$')
);
CREATE INDEX notifications_user_unread_idx ON notifications (user_id, read_at, created_at DESC, id DESC);
-- +goose Down
DROP TABLE notifications;
