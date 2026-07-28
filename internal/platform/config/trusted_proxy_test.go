package config

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestA11A(t *testing.T) {
	t.Parallel()

	t.Run("empty config disables proxy trust", func(t *testing.T) {
		t.Parallel()

		trustedProxies, err := ParseTrustedProxyCIDRs(nil)
		require.NoError(t, err)
		require.False(t, trustedProxies.Enabled())
		require.Empty(t, trustedProxies.Prefixes())
		require.False(t, trustedProxies.Contains(netip.MustParseAddr("127.0.0.1")))
	})

	t.Run("explicit CIDRs are parsed", func(t *testing.T) {
		t.Parallel()

		trustedProxies, err := ParseTrustedProxyCIDRs([]string{
			"10.0.0.0/8",
			"2001:db8::/32",
		})
		require.NoError(t, err)
		require.True(t, trustedProxies.Enabled())
		require.Equal(t, []netip.Prefix{
			netip.MustParsePrefix("10.0.0.0/8"),
			netip.MustParsePrefix("2001:db8::/32"),
		}, trustedProxies.Prefixes())
		require.True(t, trustedProxies.Contains(netip.MustParseAddr("10.20.30.40")))
		require.True(t, trustedProxies.Contains(netip.MustParseAddr("2001:db8::1")))
		require.False(t, trustedProxies.Contains(netip.MustParseAddr("203.0.113.5")))
	})

	invalidTests := []struct {
		name          string
		value         string
		expectedError string
	}{
		{
			name:          "malformed CIDR",
			value:         "not-a-cidr",
			expectedError: "config: TRUSTED_PROXY_CIDRS must contain valid CIDR prefixes",
		},
		{
			name:          "IPv4 trust all",
			value:         "0.0.0.0/0",
			expectedError: "config: TRUSTED_PROXY_CIDRS must not trust all addresses",
		},
		{
			name:          "IPv6 trust all",
			value:         "::/0",
			expectedError: "config: TRUSTED_PROXY_CIDRS must not trust all addresses",
		},
		{
			name:          "implicit wildcard",
			value:         "*",
			expectedError: "config: TRUSTED_PROXY_CIDRS must not use wildcard values",
		},
	}

	for _, test := range invalidTests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			_, err := ParseTrustedProxyCIDRs([]string{test.value})
			require.EqualError(t, err, test.expectedError)
		})
	}
}
