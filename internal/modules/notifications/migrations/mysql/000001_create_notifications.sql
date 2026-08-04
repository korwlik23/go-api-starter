-- +goose Up
CREATE TABLE notifications (
    id BINARY(16) NOT NULL,
    user_id BINARY(16) NOT NULL,
    title VARCHAR(255) NOT NULL,
    body TEXT NOT NULL,
    kind VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT 'info',
    read_at DATETIME(6) NULL,
    created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    CONSTRAINT notifications_primary_key PRIMARY KEY (id),
    CONSTRAINT notifications_user_fk FOREIGN KEY (user_id) REFERENCES users (id) ON UPDATE RESTRICT ON DELETE CASCADE,
    CONSTRAINT notifications_title_valid CHECK (CHAR_LENGTH(title) BETWEEN 1 AND 255),
    CONSTRAINT notifications_body_valid CHECK (OCTET_LENGTH(body) BETWEEN 1 AND 16384),
    CONSTRAINT notifications_kind_valid CHECK (kind REGEXP '^[a-z][a-z0-9_.-]{0,31}$')
) ENGINE=InnoDB DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
CREATE INDEX notifications_user_unread_idx ON notifications (user_id, read_at, created_at, id);
-- +goose Down
DROP TABLE notifications;
