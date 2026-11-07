# ADR 0003: History for new members and new devices

- Status: **Proposed [REVIEW]**
- Date: 2026-11-07

## Context

MLS gives forward secrecy: a device that joins at epoch N cannot decrypt
messages from epochs before N. Nexus Chat needs history for three cases:

1. A user's **new device** reading the user's own past conversations.
2. **On-device search** backfilling older messages.
3. **New members** of channels that grant `READ_HISTORY_BEFORE_JOIN`.

## Options

| Option                                                  | How                                                                                                                                               | Cost / risk                                                                                                                                    |
| ------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------- |
| A. Re-encrypt on join                                   | An existing member's device re-encrypts past messages for the newcomer                                                                            | Proportional to history size for every join; needs a member online; impossible for large channels                                              |
| B. Keep MLS epoch secrets                               | Members export old epoch secret trees to newcomers                                                                                                | Hands over far more than needed (secrets that also decrypt handshake data); fragile; ties history to MLS internals                             |
| **C. Archive copy under exporter-derived history keys** | Each message also stored as AES-256-GCM under `MLS-Exporter("nexus-chat history v1", channelId‖epoch, 32)`; members share these keys when allowed | One extra small ciphertext per message (the live MLS copy is deleted after delivery, so long-term storage is ~1×); history keys are long-lived |
| D. Server-readable history                              | —                                                                                                                                                 | Violates the E2EE requirement                                                                                                                  |

## Decision (proposed)

**Option C**, as specified in [`e2ee-design.md`](../e2ee-design.md) §4–§6:

- Every message has a live MLS copy (deleted after delivery or 30 days) and
  an archive copy (kept for retention), signed by the sender.
- History keys go into the user's key backup, which handles cases 1 and 2.
- For case 3, the first online member device sends the newcomer a history
  bundle as an MLS message in the new epoch. Without the permission, no
  bundle is sent.
- The exporter is the MLS-sanctioned way to derive application keys
  (RFC 9420 §8.5), and AES-GCM comes from WebCrypto, so no new primitive or
  protocol is introduced.

## Consequences

- **Security tradeoff (must be accepted explicitly):** archived messages are
  protected by long-lived keys, not MLS's per-message ratchet. A compromised
  device, or a backup unlocked by an attacker, exposes every archive copy
  that device or user could read. Forward secrecy and post-compromise
  security still hold for live delivery, and for anyone who never held the
  relevant history keys (including removed members, for later epochs).
- A malicious member can leak history keys. This is no different from
  leaking the plaintext they can already read.
- New members get history only after some member device with the keys comes
  online. The UI shows "History will appear when a member is online."
- Moderator deletion removes both copies on the server. Devices that already
  decrypted a message keep their local copy until they sync the deletion,
  which honest clients do.
