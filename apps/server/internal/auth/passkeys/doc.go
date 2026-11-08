// Package passkeys handles FIDO2 security keys and passkeys through
// WebAuthn (spec §4.1), using the go-webauthn library for every protocol
// check.
//
// Three ceremonies use it:
//   - registering a passkey or a security key (signed in)
//   - a security key as the second factor after a password
//   - logging in with a passkey alone (it already proves possession and
//     user verification, so it counts as both factors)
//
// Each ceremony is a challenge the server stores until the browser answers;
// a challenge can be answered once, within five minutes. WebAuthn needs the
// instance's domain, and browsers allow it only over HTTPS or localhost.
//
//   - service.go: relying-party setup and ceremony storage
//   - registration.go and login.go: the ceremonies
//   - user.go: the account as the library sees it
//
// Start reading at [Service.BeginRegistration].
package passkeys
