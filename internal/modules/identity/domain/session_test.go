package domain

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSessionStoresOnlyTokenHash(t *testing.T) {
	hashBytes := [32]byte{1, 2, 3}
	session := Session{
		ID:        "session-id",
		TokenHash: TokenHash(hashBytes),
	}

	require.Equal(t, TokenHash(hashBytes), session.TokenHash)

	sessionType := reflect.TypeOf(session)
	for index := 0; index < sessionType.NumField(); index++ {
		require.NotEqual(t, "RawToken", sessionType.Field(index).Name)
	}
}

func TestTokenHashRequiresSHA256Length(t *testing.T) {
	hash, err := NewTokenHash(make([]byte, 32))
	require.NoError(t, err)
	require.Equal(t, TokenHash{}, hash)

	_, err = NewTokenHash(make([]byte, 31))
	require.ErrorIs(t, err, ErrInvalidTokenHash)
}
