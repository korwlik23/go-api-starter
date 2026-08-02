package cache

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestM3C3KeyCanonicalIncludesVersionAndScopes(t *testing.T) {
	t.Parallel()

	first := Key{
		Version: 1, Namespace: "publishing.content", AccountID: "account-1",
		ContentID: "content-1", Locale: "th", Path: "/home",
	}
	second := first
	second.Version = 2

	require.NoError(t, first.Validate())
	require.NoError(t, second.Validate())
	require.NotEqual(t, first.Canonical(), second.Canonical())
	require.Contains(t, first.Canonical(), "v1")
	require.Error(t, (Key{}).Validate())
	require.Error(t, (Invalidation{}).Validate())
	require.True(t, (Invalidation{
		Namespace: "publishing.content", AccountID: "account-1", ContentID: "content-1",
	}).Matches(first))
	require.False(t, (Invalidation{
		Namespace: "publishing.content", AccountID: "account-2",
	}).Matches(first))

	_, err := NewMemory(MemoryOptions{})
	require.ErrorIs(t, err, ErrInvalidMemoryOptions)
	_, err = (&Memory{}).Get(context.Background(), first)
	require.ErrorIs(t, err, ErrInvalidMemory)
}
