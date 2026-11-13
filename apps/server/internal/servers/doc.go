// Package servers holds servers (spec §4.4): their members, ordered roles,
// categories with text and voice channels, channel overrides and invites.
//
// Every permission check goes through internal/permissions, with what
// access.go loads for the member asking. Nobody can grant a permission
// they do not have, or manage a role at or above their own highest one,
// unless they are the owner or an administrator. Start reading at
// service.go, then access.go.
package servers
