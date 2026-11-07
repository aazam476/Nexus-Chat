// Package accounts owns local user accounts: the rules for usernames,
// display names and email addresses (spec §4.1), and creating accounts.
//
// Layers:
//   - validation.go: the field rules, used by every feature that creates or
//     edits an account
//   - store/: SQL queries compiled by sqlc
//
// Start reading at [Validator.ValidateNewAccount].
package accounts
