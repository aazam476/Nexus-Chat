package linkpreview

import (
	"errors"
	"fmt"
	"net"
	"net/netip"
	"syscall"
	"time"
)

// errForbiddenAddress means a fetch would reach an internal address.
var errForbiddenAddress = errors.New("the link points at a private or internal address")

// sharedAddressSpace is 100.64.0.0/10 (RFC 6598), used inside carrier and
// cloud networks; netip has no predicate for it.
var sharedAddressSpace = netip.MustParsePrefix("100.64.0.0/10") //nolint:gochecknoglobals // a constant prefix

// publicAddress reports whether address is on the public internet.
func publicAddress(address netip.Addr) bool {
	address = address.Unmap()
	return address.IsGlobalUnicast() && !address.IsPrivate() && !address.IsLoopback() &&
		!address.IsLinkLocalUnicast() && !sharedAddressSpace.Contains(address)
}

// guardedDialer connects only to addresses allow accepts. The check runs on
// the resolved address of every connection, redirects included, so DNS
// tricks cannot slip an internal address past it.
func guardedDialer(allow func(netip.Addr) bool) *net.Dialer {
	return &net.Dialer{
		Timeout: 5 * time.Second,
		Control: func(_ string, address string, _ syscall.RawConn) error {
			host, _, err := net.SplitHostPort(address)
			if err != nil {
				return fmt.Errorf("checking %q: %w", address, err)
			}
			parsed, err := netip.ParseAddr(host)
			if err != nil || !allow(parsed) {
				return errForbiddenAddress
			}
			return nil
		},
	}
}
