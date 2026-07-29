package gorm

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCatalogIdentityValidationMatchesDatabaseConstraints(t *testing.T) {
	for _, category := range []string{"common", "admin.navigation", "auth-errors"} {
		require.True(t, validCatalogCategory(category), category)
	}
	for _, category := range []string{"", "Common", "prototype.items", "a..b"} {
		require.False(t, validCatalogCategory(category), category)
	}
	for _, key := range []string{"actions.save", "validation.email_invalid"} {
		require.True(t, validTranslationKey(key), key)
	}
	for _, key := range []string{"", "Actions.save", "constructor.name", "a..b"} {
		require.False(t, validTranslationKey(key), key)
	}
}

func TestCatalogReplacementRejectsEmptyInitialVersion(t *testing.T) {
	repository := &CatalogRepository{}

	_, err := repository.ReplaceCategory(
		context.Background(),
		"locale-id",
		"common",
		map[string]string{},
		0,
		"editor",
	)

	require.ErrorIs(t, err, ErrInvalidCatalogRepository)
}
