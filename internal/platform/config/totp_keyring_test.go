package config

import (
	"bytes"
	"encoding/base64"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseTOTPKeyringSelectsActiveVersionAndRetainsOldKeys(t *testing.T) {
	v1 := base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{0x11}, 32))
	v2 := base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{0x22}, 32))

	keyring, err := ParseTOTPKeyring("v2", "v2:"+v2+",v1:"+v1)
	require.NoError(t, err)
	version, key := keyring.Active()
	require.Equal(t, "v2", version)
	require.Equal(t, bytes.Repeat([]byte{0x22}, 32), key)

	old, ok := keyring.Key("v1")
	require.True(t, ok)
	require.Equal(t, bytes.Repeat([]byte{0x11}, 32), old)
}

func TestParseTOTPKeyringRejectsInvalidConfigurationWithoutEchoingKeys(t *testing.T) {
	secret := base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{0x33}, 31))
	testCases := []struct {
		active string
		keys   string
	}{
		{active: "", keys: "v1:" + secret},
		{active: "v1", keys: ""},
		{active: "v1", keys: "v1:" + secret},
		{active: "v2", keys: "v1:" + secret},
		{active: "v1", keys: "v1:" + secret + ",v1:" + secret},
	}

	for _, testCase := range testCases {
		_, err := ParseTOTPKeyring(testCase.active, testCase.keys)
		require.ErrorIs(t, err, ErrInvalidTOTPKeyring)
		require.NotContains(t, err.Error(), secret)
	}
}
