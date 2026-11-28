// Package reports lets users report messages to moderators (spec §4.7,
// ADR 0010). A reporter's device hands over the message as it decrypted
// it, with the sender's signature from the archive copy (the evidence).
// The instance adds a signed receipt for the copy it stores: who sent it,
// where, when, and its hash. Moderators' devices check the signature, the
// sender's credential and the receipt; the server never needs to read
// messages that nobody reported.
package reports
