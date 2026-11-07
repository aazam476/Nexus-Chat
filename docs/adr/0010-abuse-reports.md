# ADR 0010: Verifiable user reports (message franking)

- Status: **Proposed [REVIEW]**
- Date: 2026-11-07

## Context

The reporter's client submits a decrypted message to moderators with proof
that the reported user really sent it, without the server ever reading
messages that were not reported.

## Options

| Option | Mechanism | Assessment |
|---|---|---|
| A. Commitment-based franking (Facebook, Grubbs–Lu–Ristenpart 2017) | Sender embeds a random franking key and sends an HMAC commitment to the plaintext; the server MACs the commitment with context at delivery; a reporter reveals plaintext + franking key | Well studied, but adds a parallel commitment scheme to MLS that we would implement ourselves on top of HMAC |
| **B. Sender signatures + server receipt** | MLS already signs every message's content with the sender's leaf key (RFC 9420 §6.1); our archive copy carries the same kind of signature. The server adds a signed receipt at delivery | Uses only signatures that already exist plus a server Ed25519 signature. No new construction |
| C. Asymmetric message franking (Tyagi et al. 2019) | Designated-verifier signatures preserving deniability | Research grade; no vetted libraries |

## Decision (proposed)

**Option B.**

- At send time the server returns and stores a **receipt**:
  `Sign_instance(sha256(archiveCopy) ‖ conversationId ‖ messageId ‖ senderDevice ‖ serverTimestamp)`.
- A **report** contains: the plaintext payload, the sender's signature
  over it (from the archive copy), the sender's device certificate chain to
  their UMK, the receipt, and an optional comment.
- The moderator's client (not the server) verifies: the signature over the
  plaintext matches the sender's device key; that device key chains to the
  sender's published UMK; the receipt is valid and its hash matches the
  stored archive copy, which the server still has. A forged plaintext fails
  the signature; a message the reporter wrote themselves fails the sender
  check; a replayed old message shows its true timestamp.
- Instance admins receive reports when the server's moderators escalate
  them, or directly when the instance setting requires it.

## Consequences

- **Not deniable:** anyone holding a message and its signature can prove
  who sent it. MLS already has this property, so nothing is lost, and the
  threat model lists it.
- Reports reveal plaintext to moderators by design, only for the reported
  message and only at the reporter's choice.
- For federated servers, the hosting instance's moderators verify, and the
  sender's home instance publishes the UMK used in the check.
