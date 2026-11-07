// Package instanceconfig manages instance-wide settings stored in the
// database: everything the setup wizard configures and the Admin panel edits
// later (spec §3.5).
//
// Layers, from the outside in:
//   - service.go: business rules ([Service])
//   - store/: SQL queries compiled by sqlc (do not edit *.gen.go by hand)
//
// In M0 the only setting is the time of the first boot, which later
// milestones show in instance statistics. Start reading at [NewService].
package instanceconfig
