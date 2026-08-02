package ports

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestM3B5ValidateRedirectListQueryScopesAccountAndBounds(t *testing.T) {
	t.Parallel()
	require.ErrorIs(t, ValidateRedirectListQuery(RedirectListQuery{Limit: 25}), ErrInvalidRedirectQuery)
	require.ErrorIs(t, ValidateRedirectListQuery(RedirectListQuery{AccountID: "not-a-uuid", Limit: 25}), ErrInvalidRedirectQuery)
	require.NoError(t, ValidateRedirectListQuery(RedirectListQuery{AccountID: "018f1f64-7b2a-7c10-8bf2-3c40d5f67580", Limit: 25}))
}
