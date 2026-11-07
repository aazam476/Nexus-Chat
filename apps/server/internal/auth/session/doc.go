// Package session manages logged-in sessions (spec §4.1): creating them at
// login, checking the bearer token on every request, listing a user's
// devices, and revoking any of them.
//
// A session token is 256 random bits, shown to the client once. The
// database stores only its SHA-256 hash, so a leaked database cannot be used
// to log in. Sessions expire after 90 days without use.
//
// Start reading at [Service.Create] and [Service.Authenticate]. The HTTP
// middleware that enforces authentication is in middleware.go.
package session
