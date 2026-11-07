// Package auth serves the login, logout and session endpoints (spec §4.1).
//
// The rules live in the subpackages:
//   - password: Argon2id hashing and password rules
//   - session: session tokens, and the Principal of an authenticated request
//   - login: checking credentials, rate limits and progressive delays
//
// This package only translates between HTTP and those services; start
// reading at [Handler.Login].
package auth
