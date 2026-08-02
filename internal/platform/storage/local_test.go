package storage

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestM3B1StorageSentinelsRemainDistinct(t *testing.T) {
	t.Parallel()

	require.NotEqual(t, ErrInvalidContext, ErrInvalidRequest)
	require.NotEqual(t, ErrInvalidKey, ErrObjectNotFound)
	require.NotEqual(t, ErrObjectExists, ErrObjectNotFound)
	require.True(t, errors.Is(ErrObjectNotFound, ErrObjectNotFound))
}
