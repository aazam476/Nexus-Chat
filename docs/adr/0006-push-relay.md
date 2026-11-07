# ADR 0006: Push relay protocol, payload format, limits, and hosting

- Status: **Proposed [REVIEW]**
- Date: 2026-11-07

## Context

Store apps can only receive APNs/FCM pushes signed with the publisher's
credentials, so instances need a relay the publisher runs. The relay must
never see plaintext and must stop instances from pushing to devices that did
not register with them (spec §4.10).

## Decision (proposed)

Full detail in [`push-notifications.md`](../push-notifications.md).

1. **Instance identity:** an Ed25519 key per instance, registered
   automatically. The relay confirms domain control by fetching the key from
   the instance's `/.well-known/nexus-chat`.
2. **Handles:** devices register raw tokens with the relay directly and get
   a 256-bit random handle bound to one instance key. Instances only ever see
   handles.
3. **Requests:** batched `POST /v1/push`, signed with the same RFC 9421
   profile as federation (one signing implementation to maintain and test).
4. **Payload:** a versioned binary envelope of at most 3,072 bytes,
   carrying the message's archive copy, a fetch pointer, or a sync
   signal. The archive copy is used because decrypting it needs only
   read-only keys, which suits an iOS Notification Service Extension.
5. **Rate limits:** 30/min and 600/h per handle; per instance
   `max(20, active_handles/50)` requests/s with burst ×5 and a 2,000/s cap;
   registration limits per IP.
6. **Hosting:** one small VM plus managed Postgres, swap disabled, payloads
   in RAM only, logs as counters only, push tokens encrypted at rest.

## Alternatives considered

- Relay keyed by instance API tokens instead of signatures: needs a manual
  signup step, which the spec forbids ("automatic, no admin or user steps").
- Handles scoped per channel: rejected by the spec's own reasoning (relay
  can't verify, metadata leak, no added protection).
- Payload always a fetch pointer: simpler, but every notification then needs
  a network round trip inside the iOS extension's time budget, which fails on
  poor connections. Inline payloads first, pointer as fallback.

## Consequences

- The relay is a small but real service for the publisher to operate, with
  persistent storage for handles.
- Self-hosters on iOS still need the official relay or their own app build.
