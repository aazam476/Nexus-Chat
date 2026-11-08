package admin

import (
	"errors"
	"fmt"
	"strings"
	"unicode"

	"github.com/aazam476/nexus-chat/apps/server/internal/platform/mail"
	"github.com/aazam476/nexus-chat/apps/server/internal/platform/validation"
)

// Limits on SMTP fields. A host name is at most 253 characters (RFC 1035).
const (
	maximumHostLength     = 253
	maximumUsernameLength = 320
	maximumPasswordLength = 1024
	maximumPort           = 65535
	defaultSubmissionPort = 587
)

var (
	errHostRequired      = errors.New("is required")
	errHostInvalid       = fmt.Errorf("must be a host name or IP address of at most %d characters", maximumHostLength)
	errPortInvalid       = fmt.Errorf("must be between 1 and %d", maximumPort)
	errSecurityInvalid   = errors.New("must be starttls, tls or none")
	errPasswordUnsafe    = errors.New("a password is only sent over an encrypted connection; use starttls or tls, or a relay on localhost")
	errUsernameTooLong   = fmt.Errorf("must be at most %d characters", maximumUsernameLength)
	errPasswordTooLong   = fmt.Errorf("must be at most %d characters", maximumPasswordLength)
	errFromAddressFormat = errors.New("must be a plain email address such as chat@example.com")
)

// check validates settings about to be saved. newPassword is nil when
// the stored password is kept.
func check(config storedSMTP, newPassword *string) error {
	var problems validation.Errors
	problems.Add("host", checkHost(config.Host))
	if config.Port < 1 || config.Port > maximumPort {
		problems.Add("port", errPortInvalid)
	}
	problems.Add("security", checkSecurity(config, newPassword))
	if len(config.Username) > maximumUsernameLength {
		problems.Add("username", errUsernameTooLong)
	}
	if newPassword != nil && len(*newPassword) > maximumPasswordLength {
		problems.Add("password", errPasswordTooLong)
	}
	if mail.CheckAddress(config.From) != nil {
		problems.Add("from", errFromAddressFormat)
	}
	return problems.Err()
}

func checkHost(host string) error {
	if host == "" {
		return errHostRequired
	}
	if len(host) > maximumHostLength || strings.ContainsFunc(host, func(r rune) bool {
		return unicode.IsSpace(r) || unicode.IsControl(r) || r == '/'
	}) {
		return errHostInvalid
	}
	return nil
}

// isLoopback matches the hosts net/smtp.PlainAuth accepts without TLS.
func isLoopback(host string) bool {
	return host == "localhost" || host == "127.0.0.1" || host == "::1"
}

func willHavePassword(config storedSMTP, newPassword *string) bool {
	if newPassword != nil {
		return *newPassword != ""
	}
	return config.SealedPassword != nil
}

func checkSecurity(config storedSMTP, newPassword *string) error {
	switch config.Security {
	case mail.SecurityStartTLS, mail.SecurityTLS:
		return nil
	case mail.SecurityNone:
		// net/smtp refuses to send a password in the clear to any other
		// host, so saving such settings would only fail later.
		if willHavePassword(config, newPassword) && !isLoopback(config.Host) {
			return errPasswordUnsafe
		}
		return nil
	default:
		return errSecurityInvalid
	}
}
