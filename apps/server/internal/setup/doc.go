// Package setup runs the first-run setup of a fresh instance (spec §3.5).
//
// On first boot the server prints a one-time setup token to its logs. The
// setup wizard sends that token together with the first admin account and
// basic instance settings; only someone who can read the logs can claim
// the instance.
//
// Layers:
//   - handler.go: HTTP, using the generated OpenAPI types
//   - service.go: the rules (token, rate limit, validation, hashing)
//   - store/: one database transaction that saves everything
//
// Start reading at [Service.Complete].
package setup
