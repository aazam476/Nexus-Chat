// Package social holds friends, blocks and privacy settings (spec §4.2).
//
// Friend requests go one way and become a friendship when accepted;
// blocking ends any friendship and pending request and stops new ones and
// DMs in both directions. Privacy settings say who may send a user a
// friend request or start a DM with them; CanDirectMessage is what the
// conversations feature asks. Start reading at service.go, then friends.go.
package social
