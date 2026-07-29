package crypto

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEnvelopeCipherEncryptsWithFreshNonceAndBoundAssociatedData(t *testing.T) {
	keys := envelopeKeyringStub{
		activeVersion: "v2",
		keys: map[string][]byte{
			"v1": bytes.Repeat([]byte{0x11}, 32),
			"v2": bytes.Repeat([]byte{0x22}, 32),
		},
	}
	cipher, err := NewEnvelopeCipher(keys)
	require.NoError(t, err)
	aad := []byte("user-id\x00totp\x00envelope-v1")

	first, err := cipher.Encrypt([]byte("totp-secret"), aad)
	require.NoError(t, err)
	second, err := cipher.Encrypt([]byte("totp-secret"), aad)
	require.NoError(t, err)

	require.Equal(t, "v2", first.KeyVersion)
	require.Len(t, first.Nonce, EnvelopeNonceBytes)
	require.NotEqual(t, first.Nonce, second.Nonce)
	require.NotEqual(t, first.Ciphertext, second.Ciphertext)

	plaintext, err := cipher.Decrypt(first, aad)
	require.NoError(t, err)
	require.Equal(t, []byte("totp-secret"), plaintext)

	_, err = cipher.Decrypt(first, []byte("other-user\x00totp\x00envelope-v1"))
	require.ErrorIs(t, err, ErrEnvelopeAuthentication)
}

func TestEnvelopeCipherDecryptsRetainedOldKeyVersion(t *testing.T) {
	oldKeys := envelopeKeyringStub{
		activeVersion: "v1",
		keys:          map[string][]byte{"v1": bytes.Repeat([]byte{0x11}, 32)},
	}
	oldCipher, err := NewEnvelopeCipher(oldKeys)
	require.NoError(t, err)
	envelope, err := oldCipher.Encrypt([]byte("secret"), []byte("aad"))
	require.NoError(t, err)

	rotated, err := NewEnvelopeCipher(envelopeKeyringStub{
		activeVersion: "v2",
		keys: map[string][]byte{
			"v1": bytes.Repeat([]byte{0x11}, 32),
			"v2": bytes.Repeat([]byte{0x22}, 32),
		},
	})
	require.NoError(t, err)
	plaintext, err := rotated.Decrypt(envelope, []byte("aad"))
	require.NoError(t, err)
	require.Equal(t, []byte("secret"), plaintext)
}

type envelopeKeyringStub struct {
	activeVersion string
	keys          map[string][]byte
}

func (keyring envelopeKeyringStub) Active() (string, []byte) {
	return keyring.activeVersion, keyring.keys[keyring.activeVersion]
}

func (keyring envelopeKeyringStub) Key(version string) ([]byte, bool) {
	key, ok := keyring.keys[version]
	return key, ok
}
