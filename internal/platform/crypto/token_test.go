package crypto

import (
	"bytes"
	"encoding/base64"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestOpaqueTokenGenerationAndVerification(t *testing.T) {
	codec, err := NewTokenCodec(bytes.Repeat([]byte{0x42}, 32), "identity.session.v1")
	require.NoError(t, err)

	first, err := codec.Generate()
	require.NoError(t, err)
	second, err := codec.Generate()
	require.NoError(t, err)

	firstBytes, err := base64.RawURLEncoding.DecodeString(first.Raw)
	require.NoError(t, err)
	require.Len(t, firstBytes, TokenEntropyBytes)
	require.NotEqual(t, first.Raw, second.Raw)
	require.NotEqual(t, []byte(first.Raw), first.Hash[:])
	require.True(t, codec.Matches(first.Raw, first.Hash))
	require.False(t, codec.Matches(second.Raw, first.Hash))
}

func TestOpaqueTokenHashIsDomainSeparated(t *testing.T) {
	key := bytes.Repeat([]byte{0x24}, 32)
	sessionCodec, err := NewTokenCodec(key, "identity.session.v1")
	require.NoError(t, err)
	csrfCodec, err := NewTokenCodec(key, "identity.csrf.v1")
	require.NoError(t, err)

	require.NotEqual(t, sessionCodec.Hash("same-token"), csrfCodec.Hash("same-token"))
}

func TestOpaqueTokenCodecRejectsUnsafeConfiguration(t *testing.T) {
	_, err := NewTokenCodec([]byte("short"), "identity.session.v1")
	require.ErrorIs(t, err, ErrTokenKeyTooShort)

	_, err = NewTokenCodec(bytes.Repeat([]byte{0x42}, 32), "")
	require.ErrorIs(t, err, ErrTokenDomainRequired)
}
