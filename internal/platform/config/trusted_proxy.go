package config

import (
	"net/netip"
	"strings"
)

type TrustedProxySet struct {
	prefixes []netip.Prefix
}

func ParseTrustedProxyCIDRs(values []string) (TrustedProxySet, error) {
	prefixes := make([]netip.Prefix, 0, len(values))
	seen := make(map[netip.Prefix]struct{}, len(values))

	for _, rawValue := range values {
		value := strings.TrimSpace(rawValue)
		switch strings.ToLower(value) {
		case "*", "all", "any":
			return TrustedProxySet{}, invalidKeyError(
				"TRUSTED_PROXY_CIDRS",
				"must not use wildcard values",
			)
		}

		prefix, err := netip.ParsePrefix(value)
		if err != nil {
			return TrustedProxySet{}, invalidKeyError(
				"TRUSTED_PROXY_CIDRS",
				"must contain valid CIDR prefixes",
			)
		}
		prefix = prefix.Masked()
		if prefix.Bits() == 0 {
			return TrustedProxySet{}, invalidKeyError(
				"TRUSTED_PROXY_CIDRS",
				"must not trust all addresses",
			)
		}
		if _, exists := seen[prefix]; exists {
			continue
		}
		seen[prefix] = struct{}{}
		prefixes = append(prefixes, prefix)
	}

	return TrustedProxySet{prefixes: prefixes}, nil
}

func (trustedProxies TrustedProxySet) Enabled() bool {
	return len(trustedProxies.prefixes) > 0
}

func (trustedProxies TrustedProxySet) Prefixes() []netip.Prefix {
	return append([]netip.Prefix(nil), trustedProxies.prefixes...)
}

func (trustedProxies TrustedProxySet) Strings() []string {
	values := make([]string, len(trustedProxies.prefixes))
	for index, prefix := range trustedProxies.prefixes {
		values[index] = prefix.String()
	}
	return values
}

func (trustedProxies TrustedProxySet) Contains(address netip.Addr) bool {
	address = address.Unmap()
	for _, prefix := range trustedProxies.prefixes {
		if prefix.Contains(address) {
			return true
		}
	}
	return false
}
