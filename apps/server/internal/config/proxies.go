package config

import (
	"fmt"
	"net/netip"
	"strings"
)

// TrustedProxies is the list of networks allowed to set X-Forwarded-For and
// X-Forwarded-Proto (spec §3.3). Requests from anywhere else have those
// headers ignored, so a client cannot spoof its IP address for rate limiting.
type TrustedProxies []netip.Prefix

// ParseTrustedProxies parses a comma-separated list of CIDR ranges or single
// IP addresses, for example "172.16.0.0/12, 10.0.0.5".
func ParseTrustedProxies(value string) (TrustedProxies, error) {
	var proxies TrustedProxies
	for _, entry := range strings.Split(value, ",") {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		prefix, err := parseProxyEntry(entry)
		if err != nil {
			return nil, fmt.Errorf("TRUSTED_PROXIES entry %q is not an IP address or CIDR range", entry)
		}
		proxies = append(proxies, prefix)
	}
	return proxies, nil
}

func parseProxyEntry(entry string) (netip.Prefix, error) {
	if strings.Contains(entry, "/") {
		prefix, err := netip.ParsePrefix(entry)
		if err != nil {
			return netip.Prefix{}, fmt.Errorf("parsing prefix: %w", err)
		}
		// Masked() turns "10.0.0.5/8" into "10.0.0.0/8" so Contains behaves
		// the way an admin expects.
		return prefix.Masked(), nil
	}
	address, err := netip.ParseAddr(entry)
	if err != nil {
		return netip.Prefix{}, fmt.Errorf("parsing address: %w", err)
	}
	return netip.PrefixFrom(address, address.BitLen()), nil
}

// Contains reports whether the address belongs to a trusted proxy. IPv4
// addresses written in IPv6 form (::ffff:10.0.0.1) are normalized first.
func (proxies TrustedProxies) Contains(address netip.Addr) bool {
	address = address.Unmap()
	for _, prefix := range proxies {
		if prefix.Contains(address) {
			return true
		}
	}
	return false
}
