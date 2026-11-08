// Package mfa manages second factors (spec §4.1): TOTP authenticator apps
// (RFC 6238) and single-use backup codes. FIDO2 security keys and passkeys
// are in package webauthn.
//
//   - secretbox.go: encrypting TOTP secrets before they are stored
//   - totp.go: enrolling an app and checking its codes
//   - backupcodes.go: the ten single-use recovery codes
//
// Start reading at [Service.EnrollTOTP].
package mfa
