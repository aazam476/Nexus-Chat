package oidc

import (
	"errors"
	"net/url"
	"strings"

	"github.com/aazam476/nexus-chat/apps/server/internal/platform/validation"
)

// appScheme is the URL scheme the mobile apps register.
const appScheme = "nexuschat"

const maximumReturnTo = 512

var errReturnTo = errors.New("must be a path on this instance such as /login/oidc, or a nexuschat:// address")

// resolveReturnTo turns the client's return address into an absolute URL.
// Only this instance's own pages and the app's scheme are allowed, so a
// callback can never send a completion code to another site.
func resolveReturnTo(base string, returnTo string) (string, error) {
	if !validReturnTo(returnTo) {
		var problems validation.Errors
		problems.Add("returnTo", errReturnTo)
		return "", problems.Err()
	}
	if strings.HasPrefix(returnTo, "/") {
		return base + returnTo, nil
	}
	return returnTo, nil
}

func validReturnTo(returnTo string) bool {
	if returnTo == "" || len(returnTo) > maximumReturnTo || strings.ContainsAny(returnTo, "#\\") ||
		strings.ContainsFunc(returnTo, func(r rune) bool { return r < 0x21 || r == 0x7F }) {
		return false
	}
	if strings.HasPrefix(returnTo, "/") {
		// "//host" is a link to another site.
		return !strings.HasPrefix(returnTo, "//")
	}
	parsed, err := url.Parse(returnTo)
	return err == nil && parsed.Scheme == appScheme
}

// withFragment appends values as the URL fragment. A fragment is never
// sent to a server, so the code stays out of logs and Referer headers.
func withFragment(address string, values url.Values) string {
	return address + "#" + values.Encode()
}
