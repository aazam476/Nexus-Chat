// Package delivery is the MLS delivery service (RFC 9420 §14;
// docs/e2ee-design.md §3–4): it orders and stores each conversation's
// commits and messages and hands them to member devices. It reads only
// the cleartext header (package mlsheader) and decrypts nothing.
//
// One commit is accepted per epoch, the first valid one; later ones get
// ErrEpochConflict and their devices catch up and retry. Committers upload
// a fresh GroupInfo so devices can join by external commit. When someone
// leaves, the service publishes a membership directive signed with the
// instance key, which tells the remaining devices to commit the removal.
// Start reading at service.go.
package delivery
