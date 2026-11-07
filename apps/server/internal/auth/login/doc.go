// Package login checks a username (or email) and password and starts a
// session (spec §4.1).
//
// Protections against guessing:
//   - per-IP rate limiting (the caller passes the client's IP)
//   - progressive delays per account: from the fifth wrong password in a
//     row, each further attempt doubles a lockout window, up to 15 minutes
//   - unknown usernames take as long as known ones, so the response time
//     does not reveal which accounts exist
//
// Start reading at [Service.Login].
package login
