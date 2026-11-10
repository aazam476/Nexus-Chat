# ADR 0013: Encrypted storage on devices

- Status: Accepted (built in M3)
- Date: 2026-11-07

## Context

Spec §5.3: native databases encrypted with SQLCipher or equivalent, with
the key in the Keychain/Keystore; web and Electron IndexedDB encrypted under
a non-extractable WebCrypto key, with Electron using `safeStorage`. On-device
search needs SQLite FTS5.

## Decision

- **Native:** `@op-engineering/op-sqlite` 19.0.5 (MIT) built with SQLCipher
  and FTS5 enabled. It is fast (JSI) and works with Expo dev builds. The
  database key is random, stored with `expo-secure-store`
  (Keychain/Keystore, device-only accessibility).
  `expo-sqlite` 58.0.15 (MIT) also offers SQLCipher; op-sqlite is chosen for
  its performance on large search indexes, and the choice is re-checked with
  a benchmark in M6.
- **Web and Electron:** IndexedDB records encrypted with AES-256-GCM, with
  each record's collection and ID as additional data. On the web the data
  key is a non-extractable WebCrypto `CryptoKey`, which persists in
  IndexedDB. In Electron the data key is instead random bytes wrapped with
  `safeStorage` (Keychain, DPAPI or the Secret Service) and imported as
  non-extractable at start, since a non-extractable key's material can't be
  handed to the OS. Where Linux has no keyring (`basic_text`), the desktop
  app keeps the non-extractable key.
- **iOS extension sharing:** history keys needed by the Notification Service
  Extension live in an App Group keychain item, not in the database.

## Consequences

- Web storage cannot defend against code running in the page (XSS, or a
  malicious instance serving modified code); this is listed in the threat
  model.
