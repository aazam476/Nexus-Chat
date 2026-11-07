# ADR 0014: Attachment size limits and lossy compression

- Status: Accepted
- Date: 2026-11-07

## Context

The spec sets a default attachment limit of 25 MB that the admin can change.
Phase 0 asked whether to allow files above 64 MiB, which need a streaming
encryption format. The maintainer approved large files, asked for a
per-server limit set by server owners, and suggested optional lossy
compression.

Attachments are encrypted on the sender's device (spec §5.3), so the server
sees only ciphertext and its size. It cannot inspect, resize, or recompress
anything.

## Decision

### Limits

| Setting           | Who sets it                       | Default          | Applies to                                                                       |
| ----------------- | --------------------------------- | ---------------- | -------------------------------------------------------------------------------- |
| Instance maximum  | Instance admin                    | 25 MB            | Everything on the instance                                                       |
| Server limit      | Server owner (or `MANAGE_SERVER`) | Instance maximum | Channels and threads in that server; may only be lower than the instance maximum |
| DMs and group DMs | (none)                            | Instance maximum | Private conversations                                                            |

- The server checks the declared ciphertext size when an upload starts
  and the actual size when it finishes, and rejects anything over the
  limit that applies. Clients read the limits from the API and refuse
  oversized files before encrypting, with a message naming the limit and
  who set it.
- For a remote user in a federated server, the hosting instance's limits
  apply.
- Files over 64 MiB use the Tink streaming format
  ([e2ee-design.md §7](../e2ee-design.md#7-attachments)).

### Lossy compression (sender side, before encryption)

- **Images** (JPEG, PNG without transparency, HEIC, WebP): re-encoded as
  WebP at quality 82 and at most 2560 px on the longest side. Animated
  images and images with transparency are sent as they are. Metadata
  (including location) is stripped either way.
- **Video:** re-encoded to H.264 720p at about 2.5 Mbit/s, using the
  platform's own encoder (AVFoundation on iOS and macOS, Media3 Transformer
  on Android, WebCodecs on web and desktop where available). Where no
  encoder is available, the original is sent and the user is told.
- **Audio and other files** are never recompressed.
- **Who decides:** the sender. Each upload shows a "Send original quality"
  toggle. A user setting chooses the default (compress on mobile data, send
  originals on Wi-Fi and desktop). A server owner can set "compress media by
  default" for their server, which flips the toggle's default there.
  Because the server cannot see content, this is a client default, not
  something the server can enforce. Only the size limit is enforced.
- If the compressed result is not smaller, the original is sent.

## Consequences

- Large files depend on the streaming format, which must pass Tink's test
  vectors in CI.
- Image and video compression adds platform-specific code to the client
  (M4). Its exact quality settings live in named constants and may be tuned
  after testing.
- Stripping image metadata improves privacy even when compression is off.
