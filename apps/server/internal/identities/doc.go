// Package identities stores each user's published identity: the public
// halves of their user master key and self-signing key, and the master
// key's signature over the self-signing key (docs/e2ee-design.md §2).
//
// Clients pin a user's master key on first contact and check every device
// against the self-signing key; the server only keeps the latest identity
// and refuses ones whose signature does not verify. Replacing the master
// key, which every contact sees as an identity change, needs a recent
// login. Start reading at service.go.
package identities
