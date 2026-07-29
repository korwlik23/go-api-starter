package config

import (
	"bytes"
	"encoding/base64"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRecoveryKeyringsRetainVersionedKeys(t *testing.T) {
	t.Parallel()

	encoded1 := base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{0x21}, 32))
	encoded2 := base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{0x22}, 32))
	serialized := "2:" + encoded2 + ",1:" + encoded1

	tokenKeys, err := ParseRecoveryTokenKeyring(2, serialized)
	require.NoError(t, err)
	activeVersion, activeKey := tokenKeys.Active()
	require.Equal(t, int64(2), activeVersion)
	require.Equal(t, bytes.Repeat([]byte{0x22}, 32), activeKey)
	oldKey, ok := tokenKeys.Key(1)
	require.True(t, ok)
	require.Equal(t, bytes.Repeat([]byte{0x21}, 32), oldKey)

	deliveryKeys, err := ParseRecoveryDeliveryKeyring(2, serialized)
	require.NoError(t, err)
	_, deliveryKey := deliveryKeys.Active()
	require.Equal(t, activeKey, deliveryKey)
}

func TestRecoveryKeyringsRejectMissingActiveVersion(t *testing.T) {
	t.Parallel()

	encoded := base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{0x21}, 32))
	_, err := ParseRecoveryTokenKeyring(2, "1:"+encoded)
	require.ErrorIs(t, err, ErrInvalidRecoveryTokenKeyring)
	_, err = ParseRecoveryDeliveryKeyring(2, "1:"+encoded)
	require.ErrorIs(t, err, ErrInvalidRecoveryDeliveryKeyring)
}

func TestSecurityConfigRejectsKeyReuseAcrossRecoveryPurposes(t *testing.T) {
	t.Parallel()

	cfg := validSecurityConfig()
	cfg.Authentication.Recovery.DeliveryEncryptionKeys =
		cfg.Authentication.Recovery.TokenHMACKeys

	err := Validate(cfg)

	require.EqualError(
		t,
		err,
		"config: recovery token HMAC and delivery encryption keys must be distinct",
	)
}
