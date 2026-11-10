# ADR 0002: MLS library for React Native and web

- Status: Accepted (M3 review, 2026-11-10)
- Date: 2026-11-07

## Context

MLS (RFC 9420) runs on every client: iOS and Android (React Native, Hermes),
web, and Electron. One library on every platform avoids interoperability
bugs between implementations. Hard rule: no custom crypto, MIT/Apache-
compatible license, latest stable version pinned.

Evidence gathered (November 2026):

| Library                                                 | Language   | License          | Latest stable                                 | Platforms for us                                                                 | Notes                                                                                                                                                                                                                                                                                                                                                                                                        |
| ------------------------------------------------------- | ---------- | ---------------- | --------------------------------------------- | -------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| **ts-mls**                                              | TypeScript | MIT              | 1.6.4 (2.0.0-rc.16 in progress)               | Web, Electron, Node, Hermes (pure TS; relies on WebCrypto/`@hpke/*`, `@noble/*`) | Aims at full RFC 9420; a third-party run reports all mlswg interop vectors pass on Node, Chrome, Electron and Hermes. **No formal audit.** A high-severity advisory (GHSA-gwp3-968w-m7gv) was fixed in 1.6.4. An open issue (#858) reports a ratchet-tree validation check using `&&` where the RFC requires `\|\|`; a downstream fork backported a fix ahead of 1.6.5. Supports PQ suites (ML-KEM, X-Wing). |
| **OpenMLS**                                             | Rust       | MIT              | 0.9.2 (`openmls_rust_crypto` 0.6.0, Aug 2026) | Native via UniFFI + `uniffi-bindgen-react-native` (JSI); web via WASM            | Most widely used Rust implementation, active. Needs a Rust toolchain in every mobile and web build, plus our own binding layer (UniFFI annotations). `uniffi-bindgen-react-native` is MPL-2.0 (build tool, unmodified use) and still pre-1.0 (0.31.0-6).                                                                                                                                                     |
| **mls-rs** (AWS)                                        | Rust       | Apache-2.0 / MIT | 0.57.0                                        | Same binding story as OpenMLS                                                    | Its UniFFI wrapper `mls-rs-uniffi` is 0.1.0 from March 2024 and appears unmaintained; Swift/Kotlin only.                                                                                                                                                                                                                                                                                                     |
| Go libraries (`emersion/go-mls`, `thomas-vilte/mls-go`) | Go         | —                | —                                             | Server only                                                                      | Not needed: the server is a delivery service and does not run MLS (see below).                                                                                                                                                                                                                                                                                                                               |

## Options

1. **ts-mls everywhere.** One TypeScript code path on all platforms.
   Native crypto speed on Hermes via `react-native-quick-crypto` (a WebCrypto
   implementation backed by OpenSSL).
2. **OpenMLS everywhere** via UniFFI (native) and WASM (web), with a shared
   Rust crate in `packages/crypto/native`.
3. Mixed: ts-mls on web, OpenMLS on native. Rejected: two implementations
   means interop bugs inside one product.

## Decision

**Option 1, ts-mls, pinned to 1.6.4**, behind the `packages/crypto` interface
`MlsEngine`, so the library can be replaced without touching callers. Move to
2.0 once it is released as stable, and only after the test vectors pass.

Conditions attached:

- CI runs the full mlswg interop test vectors against our wrapper on Node and
  on Hermes (spec §5.1).
- We add our own regression test for the issue #858 tree-validation case, and
  carry a fix upstream if 1.6.5 has not shipped when M3 starts.
- We subscribe to the repository's security advisories and add a CI check
  for advisories that are not in the GitHub Advisory Database (the 1.6.x
  advisory was missed by `npm audit`).
- M3 ends with a review point: if performance on a mid-range Android phone
  is unacceptable for large groups (decrypt, commit, external join at 1,000+
  devices), we switch to Option 2 behind the same interface.

## Why not OpenMLS first

OpenMLS is the stronger implementation on paper, but on our platforms it
brings a Rust cross-compilation toolchain into every iOS, Android, web, and
Electron build, a pre-1.0 binding generator, and a binding layer we would
write and maintain ourselves. That binding layer is not crypto, but it does
sit on the key-material path. ts-mls avoids all of that, passes the interop
vectors, and keeps the codebase readable for the contributors the spec
targets. The `MlsEngine` interface keeps OpenMLS as a planned fallback.

## Consequences

- Neither candidate has a published independent audit; this is listed as a
  known limitation in the threat model. The pre-1.0 audit (spec §5.6) must
  cover the MLS library as used by us.
- JavaScript cannot guarantee secret erasure from memory; also listed.
- The server needs no Go MLS library. It reads only the cleartext header
  fields of `PublicMessage`/`PrivateMessage` (`group_id`, `epoch`,
  `content_type`; RFC 9420 §6) with a small TLS-presentation-language
  decoder, which is fuzz-tested like every other parser of untrusted input.
  It verifies no MLS signatures. A client that lies in those fields can only
  get its own messages rejected by the other members. Keeping the Go side
  MLS-free shrinks its crypto surface.

## Measurements for the M3 review (November 2026)

`packages/crypto` wraps ts-mls 1.6.4 behind `mls/engine.ts`. The interop
vectors pass: all 148 passive-client cases (welcome, handling commits, and
random sequences, ciphersuites 1–7) from mlswg/mls-implementations at
commit `2034fe72`. Issue #858 is reproduced by a regression test and fixed
by `patches/ts-mls@1.6.4.patch` (`&&` → `||`, one character).

`pnpm --filter @nexus-chat/crypto bench` on Node 26, 4 vCPUs, with native
WebCrypto (milliseconds unless noted):

| Devices | Key packages (all) | Add all, one commit | Welcome size | Join  | Commit with path | Apply that commit | Encrypt / decrypt |
| ------- | ------------------ | ------------------- | ------------ | ----- | ---------------- | ----------------- | ----------------- |
| 10      | 19                 | 44                  | 3 KiB        | 22    | 31               | 21                | 2 / 1             |
| 100     | 58                 | 203                 | 34 KiB       | 131   | 184              | 102               | 2 / 1             |
| 1,000   | 701                | 2,842               | 337 KiB      | 936   | 1,450            | 815               | 2 / 1             |
| 2,000   | 1,165              | 7,600               | 675 KiB      | 1,598 | 2,594            | 1,410             | 2 / 1             |

What the numbers say:

- Sending and reading messages costs the same at any group size.
- Joining and applying a commit grow linearly with the group. A CPU
  profile of the 1,000-device run puts most of it in tree hashing and the
  TLS encoding under it: ts-mls recomputes the whole tree's hash on every
  commit instead of caching unchanged subtrees (as OpenMLS does), and it
  imports a fresh WebCrypto key for every primitive call.
- ts-mls uses native WebCrypto for Ed25519, X25519, AES-GCM, HKDF and
  hashing when `crypto.subtle` has them, and pure JavaScript (@noble)
  otherwise. Pure-JS Ed25519 verification costs 1.5 ms per signature on
  this machine, so a phone without native Ed25519 would spend seconds per
  1,000-device join on signatures alone.
- Not measured yet: a real mid-range Android phone with Hermes and
  `react-native-quick-crypto`. Hermes interprets where Node compiles, so
  expect these numbers several times higher there. The same benchmark runs
  on a device from a development build.

Options for the review:

1. **Keep ts-mls** and fix the linear costs where they are: cache subtree
   hashes and reuse imported keys (a contained change, offered upstream and
   carried as a patch meanwhile). Messages are already constant-time.
2. **Keep ts-mls and bound the problem**: membership changes in very large
   channels are batched (one commit per batch, which the delivery service
   already enforces per epoch), so a device applies a few commits per
   minute at most.
3. **Switch to OpenMLS** behind the same `MlsEngine` interface, accepting
   the Rust toolchain in every build.

Recommendation: 1 and 2 together. They keep one readable code path, and
the costs measured are an implementation detail of one function, not a
limit of the protocol. Measure on a device before M4 either way.

## Review outcome (November 2026)

The maintainer left the choice to the project; we keep **ts-mls** and take
options 1 and 2. OpenMLS stays the fallback behind `MlsEngine` if the
device measurement says otherwise.

First fix, done: `patches/ts-mls@1.6.4.patch` now also caches subtree
hashes. Tree updates in ts-mls copy the node array and put new node
objects in it, never changing a node in place, so a hash is reused only
for the same node object at the same index over children whose hashes
match; blank nodes are cached by index. The 148 interop vectors still
pass (they run in one process, so they exercise the cache across many
trees), and `treeValidation.test.ts` checks that a changed node is hashed
again. Same machine, same benchmark (milliseconds):

| Devices | Join, before | Join, after | Apply commit, before | Apply commit, after |
| ------- | ------------ | ----------- | -------------------- | ------------------- |
| 1,000   | 936          | 1,031       | 815                  | 592                 |
| 2,000   | 1,598        | 2,067       | 1,410                | 1,031               |

Applying a commit, which every member does for every membership change, is
about 27% cheaper. Joining is unchanged within this machine's noise (pure-JS
Ed25519 verification ran 40% slower in this run than in the first one): a
device that joins hashes and verifies every node once, so there is nothing
to reuse yet.

Still to do, before M4:

- Reuse imported WebCrypto keys instead of importing one per call (the
  other half of option 1), and offer both changes upstream.
- Measure on a mid-range Android phone with Hermes from a development
  build; that number decides whether the fallback is needed.
