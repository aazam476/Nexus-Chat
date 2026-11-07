# ADR 0002: MLS library for React Native and web

- Status: **Proposed [REVIEW]**
- Date: 2026-11-07

## Context

MLS (RFC 9420) runs on every client: iOS and Android (React Native, Hermes),
web, and Electron. One library on every platform avoids interoperability
bugs between implementations. Hard rule: no custom crypto, MIT/Apache-
compatible license, latest stable version pinned.

Evidence gathered (November 2026):

| Library | Language | License | Latest stable | Platforms for us | Notes |
|---|---|---|---|---|---|
| **ts-mls** | TypeScript | MIT | 1.6.4 (2.0.0-rc.16 in progress) | Web, Electron, Node, Hermes (pure TS; relies on WebCrypto/`@hpke/*`, `@noble/*`) | Aims at full RFC 9420; a third-party run reports all mlswg interop vectors pass on Node, Chrome, Electron and Hermes. **No formal audit.** A high-severity advisory (GHSA-gwp3-968w-m7gv) was fixed in 1.6.4. An open issue (#858) reports a ratchet-tree validation check using `&&` where the RFC requires `\|\|`; a downstream fork backported a fix ahead of 1.6.5. Supports PQ suites (ML-KEM, X-Wing). |
| **OpenMLS** | Rust | MIT | 0.9.2 (`openmls_rust_crypto` 0.6.0, Aug 2026) | Native via UniFFI + `uniffi-bindgen-react-native` (JSI); web via WASM | Most widely used Rust implementation, active. Needs a Rust toolchain in every mobile and web build, plus our own binding layer (UniFFI annotations). `uniffi-bindgen-react-native` is MPL-2.0 (build tool, unmodified use) and still pre-1.0 (0.31.0-6). |
| **mls-rs** (AWS) | Rust | Apache-2.0 / MIT | 0.57.0 | Same binding story as OpenMLS | Its UniFFI wrapper `mls-rs-uniffi` is 0.1.0 from March 2024 and appears unmaintained; Swift/Kotlin only. |
| Go libraries (`emersion/go-mls`, `thomas-vilte/mls-go`) | Go | — | — | Server only | Not needed: the server is a delivery service and does not run MLS (see below). |

## Options

1. **ts-mls everywhere.** One TypeScript code path on all platforms.
   Native crypto speed on Hermes via `react-native-quick-crypto` (a WebCrypto
   implementation backed by OpenSSL).
2. **OpenMLS everywhere** via UniFFI (native) and WASM (web), with a shared
   Rust crate in `packages/crypto/native`.
3. Mixed: ts-mls on web, OpenMLS on native. Rejected: two implementations
   means interop bugs inside one product.

## Decision (proposed)

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
