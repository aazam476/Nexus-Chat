// Package profiles lets users change their public profile (display name,
// bio, picture) and their status (spec §4.1), and lets others read it.
//
// Layers:
//   - handler.go: HTTP
//   - service.go: profile and status rules
//   - avatar/: turning an uploaded image into the stored 512×512 picture
//
// The rows live in the users table, so the queries are in accounts/store.
// Start reading at [Service.SetStatus].
package profiles
