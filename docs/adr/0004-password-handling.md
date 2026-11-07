# ADR 0004: Password handling — Argon2id now, OPAQUE later

- Status: **Proposed [REVIEW]**
- Date: 2026-11-07

## Context

The spec requires a minimum of 10 characters, a breached-password check, and
Argon2id, and asks Phase 0 to evaluate OPAQUE (an asymmetric PAKE, published
as RFC 9807 in 2025) so the server never sees the password.

## Options

| | Argon2id on the server | OPAQUE |
|---|---|---|
| Server sees password | Yes, briefly, over TLS | Never |
| Offline attack after a DB leak | Argon2id-hard guessing | Still possible (OPAQUE does not prevent offline guessing after a leak, it only forces precomputation to happen after the leak) |
| Go library | `golang.org/x/crypto/argon2` (BSD-3) | `github.com/bytemare/opaque` v0.18.0 (MIT, pre-1.0) |
| Web/Electron library | — | `@serenity-kit/opaque` 1.1.0 (MIT, WASM build of the Rust `opaque-ke` crate) |
| React Native library | — | `react-native-opaque` 0.3.1, last released October 2023, so effectively unmaintained |
| Breached-password check | On the server, against a bundled list | Must move to the client (the server never sees the password), so the list ships in every app or is queried with k-anonymity, which leaks a hash prefix |
| Cross-implementation interop (Go server ↔ Rust client) | n/a | Needs both sides at the same RFC version with identical configuration; must be proven with test vectors |

## Decision (proposed)

**Argon2id on the server for beta** (`m=64 MiB, t=3, p=1` minimum, tuned so
a hash takes about 250 ms on the reference 4-vCPU host, with parameters stored
per hash so they can be raised later). The breached-password check runs
server-side against a bundled list of the most common breached passwords
(for example the top 1 million from Have I Been Pwned's Pwned Passwords
set). The data license is reviewed in M2 against the license rule, and if it
is not compatible a different public list is chosen and recorded here.

Revisit OPAQUE after beta, once there is a maintained React Native binding
and a stable Go implementation that interoperates with it. The login API is
versioned (`/auth/v1/password`), so OPAQUE can be added as `/auth/v2`
without breaking existing clients.

## Consequences

- Listed as a known limitation in the threat model: the password reaches the
  server at login, protected only by TLS.
- Not affected: E2EE. The key-backup password derivation is entirely
  client-side, with its own salt and context, and never sent anywhere
  ([`e2ee-design.md`](../e2ee-design.md) §5).
- Passkeys are offered prominently as the phishing-resistant,
  password-free option.
