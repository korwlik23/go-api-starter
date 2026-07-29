-- +goose Up
CREATE TABLE user_locale_preferences (
    user_id BINARY(16) NOT NULL,
    all_languages BOOLEAN NOT NULL,
    version BIGINT NOT NULL DEFAULT 1,
    created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    CONSTRAINT user_locale_preferences_primary_key PRIMARY KEY (user_id),
    CONSTRAINT user_locale_preferences_boolean_state_valid CHECK (all_languages IN (0, 1)),
    CONSTRAINT user_locale_preferences_version_positive CHECK (version > 0),
    CONSTRAINT user_locale_preferences_update_time_valid CHECK (updated_at >= created_at),
    CONSTRAINT user_locale_preferences_user_fk FOREIGN KEY (user_id)
        REFERENCES users (id) ON UPDATE RESTRICT ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;

CREATE TABLE user_locale_preference_locales (
    user_id BINARY(16) NOT NULL,
    locale_id BINARY(16) NOT NULL,
    CONSTRAINT user_locale_preference_locales_primary_key PRIMARY KEY (user_id, locale_id),
    INDEX user_locale_preference_locales_locale_index (locale_id, user_id),
    CONSTRAINT user_locale_preference_locales_user_fk FOREIGN KEY (user_id)
        REFERENCES user_locale_preferences (user_id) ON UPDATE RESTRICT ON DELETE CASCADE,
    CONSTRAINT user_locale_preference_locales_locale_fk FOREIGN KEY (locale_id)
        REFERENCES locales (id)
) ENGINE=InnoDB DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;

-- +goose Down
DROP TABLE user_locale_preference_locales;
DROP TABLE user_locale_preferences;
