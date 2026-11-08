package accounts

import (
	"errors"
	"net/mail"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/aazam476/nexus-chat/apps/server/internal/auth/password"
	"github.com/aazam476/nexus-chat/apps/server/internal/platform/validation"
)

// usernamePattern is spec §4.1: 3–32 characters from a–z, 0–9, "_" and ".".
// The users table enforces the same rule with a CHECK constraint.
var usernamePattern = regexp.MustCompile(`^[a-z0-9_.]{3,32}$`) //nolint:gochecknoglobals // compiled once, never changed

// Field length limits.
const (
	maximumDisplayNameLength = 32
	maximumEmailLength       = 254 // RFC 5321 §4.5.3.1.3 path limit
)

// NewAccount is the user-supplied data for a new password account.
type NewAccount struct {
	Username    string
	DisplayName string
	Email       string
	Password    string
}

// PasswordChecker rejects known-bad passwords; *breached.List satisfies it.
type PasswordChecker interface {
	Check(password string) error
}

// Validator checks new account data.
type Validator struct {
	breached PasswordChecker
}

// NewValidator returns a Validator that also rejects passwords found by
// breached.
func NewValidator(breached PasswordChecker) *Validator {
	return &Validator{breached: breached}
}

// ValidateNewAccount checks every field and returns the account with its
// email normalized to lower case. On failure the error is a
// *validation.Error naming each bad field.
func (validator *Validator) ValidateNewAccount(account NewAccount) (NewAccount, error) {
	var problems validation.Errors
	problems.Add("username", validateUsername(account.Username))
	problems.Add("displayName", ValidateDisplayName(account.DisplayName))
	email, err := normalizeEmail(account.Email)
	problems.Add("email", err)
	problems.Add("password", validator.CheckPassword(account.Password))

	if err := problems.Err(); err != nil {
		return NewAccount{}, err
	}
	account.Email = email
	return account, nil
}

// CheckPassword applies the length rules first, so the breach list is only
// searched for passwords that are otherwise acceptable.
func (validator *Validator) CheckPassword(plain string) error {
	if err := password.CheckPolicy(plain); err != nil {
		return err
	}
	return validator.breached.Check(plain)
}

func validateUsername(username string) error {
	if !usernamePattern.MatchString(username) {
		return errors.New("must be 3–32 characters using only a–z, 0–9, underscores and dots")
	}
	return nil
}

// ValidateDisplayName checks the display name rules (spec §4.1): 1–32
// characters, no surrounding spaces, no control characters.
func ValidateDisplayName(displayName string) error {
	trimmed := strings.TrimSpace(displayName)
	if trimmed != displayName {
		return errors.New("must not start or end with spaces")
	}
	length := utf8.RuneCountInString(displayName)
	if length < 1 || length > maximumDisplayNameLength {
		return errors.New("must be 1–32 characters")
	}
	if strings.ContainsFunc(displayName, unicode.IsControl) {
		return errors.New("must not contain control characters")
	}
	return nil
}

// normalizeEmail accepts a bare address ("a@example.com", not
// "Name <a@example.com>") and lower-cases it, so the same mailbox cannot
// register twice with different capitalization.
func normalizeEmail(email string) (string, error) {
	parsed, err := mail.ParseAddress(email)
	if err != nil || parsed.Address != email || len(email) > maximumEmailLength {
		return "", errors.New("must be a valid email address")
	}
	return strings.ToLower(email), nil
}
