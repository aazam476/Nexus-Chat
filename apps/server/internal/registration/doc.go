// Package registration creates new accounts (spec §4.1 and §3.5's
// registration modes) and manages the invites that invite-only instances
// require.
//
// Layers:
//   - handler.go: HTTP
//   - service.go: registration rules
//   - invites.go: creating invite codes
//   - store/: queries, and the transaction that uses an invite and creates
//     the account together
//
// Start reading at [Service.Register].
package registration
