package password

import (
	"errors"
	"unicode/utf8"
)

// Password length rules (spec §4.1). The maximum stops someone from making
// the server hash megabytes; it is far above any real password.
const (
	MinimumLength      = 10
	maximumLengthBytes = 1024
)

// Errors returned by [CheckPolicy]. Callers show them to the user.
var (
	ErrTooShort = errors.New("password must be at least 10 characters")
	ErrTooLong  = errors.New("password must be at most 1024 bytes")
	ErrInvalid  = errors.New("password must be valid UTF-8 text")
)

// CheckPolicy checks a new password against the length rules. Length is
// counted in characters, so "ü" counts once, not twice.
//
// The breached-password check from the spec is added in milestone M2,
// together with the bundled list it needs.
func CheckPolicy(password string) error {
	if !utf8.ValidString(password) {
		return ErrInvalid
	}
	if len(password) > maximumLengthBytes {
		return ErrTooLong
	}
	if utf8.RuneCountInString(password) < MinimumLength {
		return ErrTooShort
	}
	return nil
}
