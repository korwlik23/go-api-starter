-- +goose Up
CREATE TABLE user_profiles (
    user_id BINARY(16) NOT NULL,
    display_name VARCHAR(128) NOT NULL DEFAULT '',
    timezone VARCHAR(64) NOT NULL DEFAULT 'UTC',
    preferred_locale VARCHAR(32) NOT NULL DEFAULT '',
    version BIGINT NOT NULL DEFAULT 1,
    created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    CONSTRAINT user_profiles_primary_key PRIMARY KEY (user_id),
    CONSTRAINT user_profiles_user_fk FOREIGN KEY (user_id) REFERENCES users (id) ON UPDATE RESTRICT ON DELETE CASCADE,
    CONSTRAINT user_profiles_version_positive CHECK (version > 0),
    CONSTRAINT user_profiles_display_name_size CHECK (CHAR_LENGTH(display_name) <= 128),
    CONSTRAINT user_profiles_timezone_size CHECK (CHAR_LENGTH(timezone) <= 64),
    CONSTRAINT user_profiles_locale_size CHECK (CHAR_LENGTH(preferred_locale) <= 32)
) ENGINE=InnoDB DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
-- +goose Down
DROP TABLE user_profiles;
