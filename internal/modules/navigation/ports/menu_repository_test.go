package ports

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestM3B4ValidateMenuListQueryRequiresAccountAndBounds(t *testing.T) {
	t.Parallel()

	require.ErrorIs(t, ValidateMenuListQuery(MenuListQuery{Limit: 25}), ErrInvalidMenuQuery)
	require.ErrorIs(t, ValidateMenuListQuery(MenuListQuery{AccountID: "not-a-uuid", Limit: 25}), ErrInvalidMenuQuery)
	require.ErrorIs(t, ValidateMenuListQuery(MenuListQuery{AccountID: "018f1f64-7b2a-7c10-8bf2-3c40d5f67580", Limit: 0}), ErrInvalidMenuQuery)
	require.NoError(t, ValidateMenuListQuery(MenuListQuery{AccountID: "018f1f64-7b2a-7c10-8bf2-3c40d5f67580", Limit: 25}))
}
