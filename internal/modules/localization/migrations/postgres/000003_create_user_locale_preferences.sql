-- +goose Up
CREATE TABLE user_locale_preferences (
    user_id UUID NOT NULL,
    all_languages BOOLEAN NOT NULL,
    version BIGINT NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ(6) NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ(6) NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT user_locale_preferences_primary_key PRIMARY KEY (user_id),
    CONSTRAINT user_locale_preferences_version_positive CHECK (version > 0),
    CONSTRAINT user_locale_preferences_update_time_valid CHECK (updated_at >= created_at),
    CONSTRAINT user_locale_preferences_user_fk FOREIGN KEY (user_id)
        REFERENCES users (id) ON UPDATE RESTRICT ON DELETE CASCADE
);

CREATE TABLE user_locale_preference_locales (
    user_id UUID NOT NULL,
    locale_id UUID NOT NULL,
    CONSTRAINT user_locale_preference_locales_primary_key PRIMARY KEY (user_id, locale_id),
    CONSTRAINT user_locale_preference_locales_user_fk FOREIGN KEY (user_id)
        REFERENCES user_locale_preferences (user_id) ON UPDATE RESTRICT ON DELETE CASCADE,
    CONSTRAINT user_locale_preference_locales_locale_fk FOREIGN KEY (locale_id)
        REFERENCES locales (id) ON UPDATE RESTRICT ON DELETE RESTRICT
);

CREATE INDEX user_locale_preference_locales_locale_index
    ON user_locale_preference_locales (locale_id, user_id);

-- +goose Down
DROP TABLE user_locale_preference_locales;
DROP TABLE user_locale_preferences;
