package domain

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPermissionKeyUsesCanonicalFourSegmentGrammar(t *testing.T) {
	t.Parallel()

	key, err := ParsePermissionKey("content.articles.update.own")

	require.NoError(t, err)
	require.Equal(t, "content", key.Module)
	require.Equal(t, PermissionScopeOwn, key.Scope)
	require.Equal(t, "content.articles.update.own", key.String())
}

func TestPermissionKeyRejectsNonCanonicalOrUnsupportedScopes(t *testing.T) {
	t.Parallel()

	for _, value := range []string{
		"",
		"Content.articles.update.own",
		"content.articles.update",
		"content.articles.update.cross_account",
		"content..update.own",
		"content.articles.update.own.extra",
		"content.articles.update.ทุกคน",
	} {
		_, err := ParsePermissionKey(value)
		require.ErrorIs(t, err, ErrInvalidPermissionKey, value)
	}
}
