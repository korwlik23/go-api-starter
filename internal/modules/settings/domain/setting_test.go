package domain

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSettingUpdateRejectsSecretKeysAndInvalidVersions(t *testing.T) {
	setting := Setting{Key: "app.name", Value: "Starter", Version: 2}

	updated, err := setting.Update("app.display_name", "Go Starter", 2)
	require.NoError(t, err)
	require.Equal(t, "app.display_name", updated.Key)
	require.Equal(t, "Go Starter", updated.Value)
	require.Equal(t, int64(3), updated.Version)

	_, err = setting.Update("database.password", "changed", 2)
	require.ErrorIs(t, err, ErrSecretSetting)

	_, err = setting.Update("app.name", "changed", 1)
	require.ErrorIs(t, err, ErrVersionConflict)
}

func TestValidateSettingKeyAcceptsOnlyAllowlistedNonSecretShape(t *testing.T) {
	require.NoError(t, ValidateKey("app.name"))
	require.NoError(t, ValidateKey("public.site_title"))
	require.Error(t, ValidateKey(""))
	require.Error(t, ValidateKey("APP.NAME"))
	require.ErrorIs(t, ValidateKey("smtp.password"), ErrSecretSetting)
}
