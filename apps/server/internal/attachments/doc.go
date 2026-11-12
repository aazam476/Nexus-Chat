// Package attachments stores encrypted attachments (spec §4.5,
// docs/e2ee-design.md §7, ADR 0014). Devices encrypt each file before
// uploading it, so the server sees only ciphertext and its size; the key
// travels inside the MLS-encrypted message that refers to the file.
//
// A member who may send in a conversation uploads a file to it; any member
// may download it. The instance's attachment limit, set in the Admin
// panel, is checked against the declared size before the upload is read
// and against the bytes actually received. Start reading at service.go.
package attachments
