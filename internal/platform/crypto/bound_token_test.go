package crypto

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBoundTokenUsesAtLeast256BitsAndBindsIdentityFields(t *testing.T) {
	t.Parallel()

	keyring := boundTokenKeyringStub{
		active: 3,
		keys: map[int64][]byte{
			3: bytes.Repeat([]byte{0x42}, 32),
		},
	}
	codec, err := newBoundTokenCodec(
		keyring,
		bytes.NewReader(bytes.Repeat([]byte{0x24}, TokenEntropyBytes)),
	)
	require.NoError(t, err)

	token, err := codec.Generate(
		"password_reset",
		"018f1f64-7b2a-7c10-8bf2-3c40d5f67585",
		"018f1f64-7b2a-7c10-8bf2-3c40d5f67586",
	)
	require.NoError(t, err)
	require.NotEmpty(t, token.Raw)
	require.Len(t, token.Hash, 32)
	require.Equal(t, int16(1), token.HashVersion)
	require.Equal(t, int64(3), token.HashKeyVersion)

	require.NotEqual(
		t,
		token.Hash,
		codec.Hash(
			"password_reset",
			"018f1f64-7b2a-7c10-8bf2-3c40d5f67585",
			"different-user",
			token.Raw,
			3,
		),
	)
	require.NotEqual(
		t,
		token.Hash,
		codec.Hash(
			"user_activation",
			"018f1f64-7b2a-7c10-8bf2-3c40d5f67585",
			"018f1f64-7b2a-7c10-8bf2-3c40d5f67586",
			token.Raw,
			3,
		),
	)
}

func TestBoundTokenRetainsOldKeyForVerification(t *testing.T) {
	t.Parallel()

	codec, err := NewBoundTokenCodec(boundTokenKeyringStub{
		active: 2,
		keys: map[int64][]byte{
			1: bytes.Repeat([]byte{0x11}, 32),
			2: bytes.Repeat([]byte{0x22}, 32),
		},
	})
	require.NoError(t, err)

	hash := codec.Hash("password_reset", "token-id", "user-id", "raw", 1)

	require.True(
		t,
		codec.Matches(
			"password_reset",
			"token-id",
			"user-id",
			"raw",
			1,
			hash,
		),
	)
	require.False(
		t,
		codec.Matches(
			"password_reset",
			"token-id",
			"user-id",
			"wrong",
			1,
			hash,
		),
	)
}

type boundTokenKeyringStub struct {
	active int64
	keys   map[int64][]byte
}

func (stub boundTokenKeyringStub) Active() (int64, []byte) {
	return stub.active, stub.keys[stub.active]
}

func (stub boundTokenKeyringStub) Key(version int64) ([]byte, bool) {
	key, ok := stub.keys[version]
	return key, ok
}
