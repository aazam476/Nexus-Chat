// Package conversations holds DMs and group DMs (spec §4.3): who is in
// each one, its name, and the MLS group that encrypts it (one group per
// conversation, docs/e2ee-design.md §3.1).
//
// Starting a DM or adding someone to a group DM asks the social feature
// whether they accept DMs from the caller. A pair of users has one DM.
// Group DMs have 3 members up to the instance's limit
// (GROUP_DM_MAX_MEMBERS). Start reading at
// service.go.
package conversations
