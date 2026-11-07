package config

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseTrustedProxies(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  []string
	}{
		{"empty", "", nil},
		{"only separators and spaces", " , ,", nil},
		{"single cidr", "172.16.0.0/12", []string{"172.16.0.0/12"}},
		{"host bits are masked", "10.1.2.3/8", []string{"10.0.0.0/8"}},
		{"single ipv4 address", "10.0.0.5", []string{"10.0.0.5/32"}},
		{"single ipv6 address", "fd00::1", []string{"fd00::1/128"}},
		{"several with spaces", " 10.0.0.0/8 , fd00::/8 ", []string{"10.0.0.0/8", "fd00::/8"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			proxies, err := ParseTrustedProxies(test.value)

			require.NoError(t, err)
			var got []string
			for _, prefix := range proxies {
				got = append(got, prefix.String())
			}
			assert.Equal(t, test.want, got)
		})
	}
}

func TestParseTrustedProxiesRejectsInvalidEntries(t *testing.T) {
	for _, value := range []string{"not-an-ip", "10.0.0.0/33", "10.0.0/8", "300.1.1.1"} {
		t.Run(value, func(t *testing.T) {
			_, err := ParseTrustedProxies(value)

			require.Error(t, err)
			assert.Contains(t, err.Error(), value)
		})
	}
}

func TestTrustedProxiesContains(t *testing.T) {
	proxies, err := ParseTrustedProxies("172.16.0.0/12, 10.0.0.5, fd00::/8")
	require.NoError(t, err)

	tests := []struct {
		address string
		want    bool
	}{
		{"172.20.1.1", true},
		{"172.32.0.1", false},
		{"10.0.0.5", true},
		{"10.0.0.6", false},
		{"::ffff:10.0.0.5", true}, // IPv4-mapped form of a trusted address
		{"fd12::1", true},
		{"2001:db8::1", false},
	}
	for _, test := range tests {
		t.Run(test.address, func(t *testing.T) {
			assert.Equal(t, test.want, proxies.Contains(netip.MustParseAddr(test.address)))
		})
	}
}

func TestEmptyTrustedProxiesTrustNobody(t *testing.T) {
	var proxies TrustedProxies

	assert.False(t, proxies.Contains(netip.MustParseAddr("127.0.0.1")))
}
