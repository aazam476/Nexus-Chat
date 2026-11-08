package oidc

import "errors"

// Errors the endpoints turn into API error codes.
var (
	ErrProviderNotFound   = errors.New("there is no sign-in provider with this ID")
	ErrProviderExists     = errors.New("a sign-in provider with this ID already exists")
	ErrRateLimited        = errors.New("too many sign-in attempts from this address; wait a minute")
	ErrInvalidState       = errors.New("this sign-in link is invalid or has expired; start again")
	ErrInvalidFlow        = errors.New("this sign-in is invalid or has expired; start again")
	ErrNoAccount          = errors.New("no account is linked to this login; log in another way and link the provider in settings")
	ErrRegistrationClosed = errors.New("registration is closed on this instance")
	ErrInviteRequired     = errors.New("this instance is invite-only; an invite code is required")
	ErrInvalidInvite      = errors.New("the invite is unknown, used up or expired")
	ErrEmailRequired      = errors.New("the provider did not share an email address, which an account needs")
	ErrEmailInUse         = errors.New("an account already uses this email; log in to it and link the provider in settings")
	ErrIdentityInUse      = errors.New("this provider login is already linked to another account")
	ErrAlreadyLinked      = errors.New("this account is already linked to another login at this provider; unlink it first")
	ErrNotLinked          = errors.New("this account is not linked to that provider")
	ErrLastLoginMethod    = errors.New("this is the account's only way to log in; set a password or add a passkey first")
)

// UnavailableError means the provider could not be reached or its
// configuration could not be loaded.
type UnavailableError struct {
	Err error
}

func (unavailable *UnavailableError) Error() string {
	return "the sign-in provider is not available right now: " + unavailable.Err.Error()
}

// Unwrap returns the underlying error.
func (unavailable *UnavailableError) Unwrap() error { return unavailable.Err }
