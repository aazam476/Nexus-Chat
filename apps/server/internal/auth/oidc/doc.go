// Package oidc signs users in with OAuth 2.0 / OpenID Connect providers that
// the instance admin configures (spec §4.1, ADR 0016).
//
// Layers:
//   - providers.go: the admin's provider settings, validated and stored
//     with the client secret encrypted
//   - connector.go: talking to a provider (discovery, code exchange, ID
//     token verification) through go-oidc and x/oauth2
//   - flow.go: starting a sign-in and handling the provider's callback
//   - complete.go: finishing it: logging in, creating the account, or
//     linking the identity to the logged-in account
//   - identities.go: listing and unlinking a user's providers
//   - handler.go and admin_handler.go: HTTP
//   - store/: queries, and the transaction that creates an account
//     together with its identity
//
// Start reading at [SignIn.Begin].
package oidc
