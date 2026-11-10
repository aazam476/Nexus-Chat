// Package keybackup stores each user's encrypted key backup
// (docs/e2ee-design.md §5): a salt for deriving wrapping keys, the backup
// key wrapped under each unlock method, and named blobs sealed with the
// backup key. Everything is encrypted on the client; the server checks
// only sizes, names and who is asking.
//
// Adding or removing an unlock method needs a recent login, because it
// changes who can open the backup. A password reset by email deletes the
// password copy (see SetPassword in internal/accounts/store). Start
// reading at service.go.
package keybackup
