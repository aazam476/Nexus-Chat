// Package password hashes and verifies login passwords with Argon2id
// (RFC 9106), and checks the password rules from spec §4.1.
//
// Hashes are stored in the PHC string format used by most Argon2 tools:
//
//	$argon2id$v=19$m=65536,t=3,p=1$<salt>$<hash>
//
// The parameters live inside each hash, so they can be raised later and old
// hashes still verify; [Hasher.NeedsRehash] reports when to upgrade one.
// Start reading at [NewHasher].
//
// Login passwords are separate from the client-side key-backup password
// derivation (docs/e2ee-design.md §5), which never reaches the server.
package password
