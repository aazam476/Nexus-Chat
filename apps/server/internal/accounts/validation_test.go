package accounts

import (
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/aazam476/nexus-chat/apps/server/internal/platform/validation"
)

// stubBreached treats one password as breached.
type stubBreached struct{}

func (stubBreached) Check(password string) error {
	if password == "password123" {
		return errors.New("this password appears in lists of leaked passwords")
	}
	return nil
}

var testValidator = NewValidator(stubBreached{})

func validAccount() NewAccount {
	return NewAccount{Username: "alice", DisplayName: "Alice", Email: "Alice@Example.com", Password: "correct horse battery"}
}

func TestValidateNewAccountNormalizesEmail(t *testing.T) {
	account, err := testValidator.ValidateNewAccount(validAccount())

	require.NoError(t, err)
	assert.Equal(t, "alice@example.com", account.Email)
	assert.Equal(t, "alice", account.Username)
}

func TestValidateNewAccountFieldRules(t *testing.T) {
	tests := []struct {
		name      string
		change    func(*NewAccount)
		wantField string
	}{
		{"username too short", func(a *NewAccount) { a.Username = "al" }, "username"},
		{"username too long", func(a *NewAccount) { a.Username = strings.Repeat("a", 33) }, "username"},
		{"username uppercase", func(a *NewAccount) { a.Username = "Alice" }, "username"},
		{"username dash", func(a *NewAccount) { a.Username = "al-ice" }, "username"},
		{"display name empty", func(a *NewAccount) { a.DisplayName = "" }, "displayName"},
		{"display name too long", func(a *NewAccount) { a.DisplayName = strings.Repeat("é", 33) }, "displayName"},
		{"display name padded", func(a *NewAccount) { a.DisplayName = " Alice" }, "displayName"},
		{"display name control char", func(a *NewAccount) { a.DisplayName = "Al\u0007ice" }, "displayName"},
		{"email missing at", func(a *NewAccount) { a.Email = "alice.example.com" }, "email"},
		{"email with name", func(a *NewAccount) { a.Email = "Alice <alice@example.com>" }, "email"},
		{"email too long", func(a *NewAccount) { a.Email = strings.Repeat("a", 250) + "@example.com" }, "email"},
		{"password too short", func(a *NewAccount) { a.Password = "short" }, "password"},
		{"password breached", func(a *NewAccount) { a.Password = "password123" }, "password"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			account := validAccount()
			test.change(&account)

			_, err := testValidator.ValidateNewAccount(account)

			var validationError *validation.Error
			require.ErrorAs(t, err, &validationError)
			require.Len(t, validationError.Fields, 1)
			assert.Equal(t, test.wantField, validationError.Fields[0].Field)
		})
	}
}

func TestValidateNewAccountAcceptsBoundaryValues(t *testing.T) {
	account := NewAccount{
		Username:    "a._",
		DisplayName: strings.Repeat("é", 32),
		Email:       "x@y.example",
		Password:    "0123456789",
	}

	_, err := testValidator.ValidateNewAccount(account)

	assert.NoError(t, err)
}

func TestValidationErrorListsEveryField(t *testing.T) {
	_, err := testValidator.ValidateNewAccount(NewAccount{})

	var validationError *validation.Error
	require.ErrorAs(t, err, &validationError)
	assert.Len(t, validationError.Fields, 4)
	assert.Equal(t,
		"invalid input: username: must be 3–32 characters using only a–z, 0–9, underscores and dots; "+
			"displayName: must be 1–32 characters; email: must be a valid email address; "+
			"password: password must be at least 10 characters",
		err.Error())
}
