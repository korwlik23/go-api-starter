package storage

import (
	"context"
	"io"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestM3B1ObjectStoreContractIsContextAware(t *testing.T) {
	t.Parallel()

	var store ObjectStore
	require.Nil(t, store)
	var request PutRequest
	require.Nil(t, request.Body)
	require.Equal(t, int64(0), request.Size)
	require.ErrorIs(t, ValidateContext(nil), ErrInvalidContext)
	require.ErrorIs(t, ValidateContext(context.Background()), nil)
	var _ io.Reader = request.Body
}
