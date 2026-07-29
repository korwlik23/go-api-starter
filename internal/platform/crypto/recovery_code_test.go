package crypto

import (
	"bytes"
	"encoding/base64"
	"testing"

	"github.com/korwlik23/go-api-starter/internal/platform/config"
	"github.com/stretchr/testify/require"
)

func TestRecoveryCodeCodecGeneratesHighEntropyVersionedHashes(t *testing.T) {
	keyring, err := config.ParseRecoveryCodeKeyring(
		2,
		"1:"+base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{0x31}, 32))+
			",2:"+base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{0x32}, 32)),
	)
	require.NoError(t, err)
	codec, err := newRecoveryCodeCodec(
		keyring,
		bytes.NewReader(bytes.Repeat([]byte{0x42}, RecoveryCodeEntropyBytes)),
	)
	require.NoError(t, err)

	code, err := codec.Generate("user-id")
	require.NoError(t, err)
	require.Len(t, code.Raw, 27)
	require.Equal(t, RecoveryCodeHashVersion, code.HashVersion)
	require.Equal(t, int64(2), code.HashKeyVersion)
	require.NotEqual(t, [32]byte{}, code.Hash)
	require.True(t, codec.Matches("user-id", code.Raw, 2, code.Hash))
	require.False(t, codec.Matches("other-user", code.Raw, 2, code.Hash))
	require.False(t, codec.Matches("user-id", "wrong-code", 2, code.Hash))
}
