# Progress

Updated at the end of every milestone. The plan is in
[roadmap.md](roadmap.md).

## Done

### Phase 0: Research and design

Architecture, threat model, E2EE, federation and push designs, roadmap,
platform support research, and ADRs 0001–0013. Decisions marked
**[REVIEW]** are still waiting for maintainer approval.

### M0: Foundation

- pnpm + Turborepo workspace, Go workspace (`go.work`) with `apps/server`
  and `apps/push-relay`.
- Server: configuration from the environment, first-boot secret
  generation (token signing, federation, push relay, VAPID, LiveKit, TURN),
  PostgreSQL via pgx, embedded goose migrations, the first sqlc store,
  `/healthz` and `/readyz` generated from `openapi.yaml`, graceful shutdown,
  request logging without sensitive data.
- Push relay: configuration and `/healthz`.
- `@nexus-chat/api-client`, generated from the same OpenAPI file.
- Tests: unit tests, database tests (Testcontainers or
  `NEXUS_TEST_DATABASE_URL`), and black-box contract tests that run the real
  binary and validate responses against the OpenAPI file.
  Coverage at completion: server 92.9% statements / 85.7% branches; push
  relay 93.8% / 90.0%; api-client 100%.
- CI: lint and format (golangci-lint, ESLint, Prettier), type check, tests
  with `-race`, coverage gates, generated-code drift check, license checks
  with a GPL self-test, govulncheck, `pnpm audit`, gitleaks, CodeQL,
  Dependabot.
- Contributor docs: README, CONTRIBUTING, code standards, getting started,
  SECURITY, Code of Conduct, per-app READMEs, dependency register.

### Decisions approved after Phase 0

- Desktop notifications arrive over the app's live connection (no Web Push
  in Electron).
- One MLS group per channel audience.
- Membership changes through signed directives.
- Attachments above 64 MiB with streaming encryption, per-server limits, and
  optional sender-side lossy compression (ADR 0014).

Still open: the remaining §10 items (MLS library, history sharing, password
handling, federation, push relay details, search, server address syntax,
voice switching, reports, Mac App Store limits). Work continues on the
recommended options; M2 uses Argon2id, which is the spec's baseline.

### M1: Deployable instance

- `docker compose up -d` with only `.env` (two required values) brings up
  server, PostgreSQL and LiveKit; `--profile p2p` adds coturn. LiveKit and
  coturn configs are generated from first-boot secrets.
- Setup: one-time token in the logs, `POST /api/v1/setup` creates the first
  admin (Argon2id) and saves name, domain and registration mode in one
  transaction; rate limited per client IP.
- HTTPS from certificate files in `DATA_DIR/tls`: validated (key, chain,
  validity, domain), hot-reloaded, expiry warnings at 30/7/1 days.
- `TRUSTED_PROXIES`: real client IP and scheme behind proxies.
- Images: distroless, privilege drop after taking ownership of `/data`,
  built-in healthcheck; release workflow with multi-arch builds, SBOM and
  provenance attestations, keyless cosign signatures; Trivy gate.
- Release gate in CI: clean install from `docker-compose.yml` and
  `.env.example`, on localhost and behind Caddy, Nginx and Traefik (each
  example in `docs/self-hosting.md` is the tested file).
- Coverage at completion: server 95.6% statements / 91.7% branches.

## Next

**M2: Accounts and client shell** (in progress). Server side done so far:

- Sessions: hashed 256-bit tokens, sliding 90-day expiry, list and revoke,
  default-deny authentication checked against the OpenAPI contract.
- Password login with per-IP rate limits, progressive per-account delays
  and timing-equalized unknown accounts; breached-password check (bundled
  SecLists list, MIT).
- Registration (open, invite-only, closed) with admin-managed invites;
  email verification and password reset through an SMTP mailer with
  embedded templates; reset logs out every session.
- Second factors: TOTP apps (secrets encrypted at rest, replay-proof),
  ten single-use backup codes, FIDO2 security keys, and passwordless
  passkey login (go-webauthn, tested end to end with a software
  authenticator). An MFA challenge allows exactly one attempt.
- Admin APIs: upload, inspect and remove the HTTPS certificate (validated
  before anything is saved, served at once); SMTP settings (the password is
  never returned, and stored AES-256-GCM encrypted with its own key) and a
  rate-limited test email to the admin's own address.
- Public `GET /api/v1/instance`: name, registration mode and whether the
  connection is secure, which drives the client's HTTPS warning.

- Sign-in with OpenID Connect providers configured by the admin (ADR 0016):
  authorization code with PKCE and nonce, completion bound to the client
  that started the flow (no login or link CSRF), sign-up following the
  registration mode, linking and unlinking, client secrets encrypted.

- Profiles: display name and bio, profile pictures (PNG, JPEG, WebP, GIF up
  to 8 MB, cropped and resized to 512×512, re-encoded so no metadata such
  as GPS positions survives, animated GIFs kept animated), status and
  custom status with expiry.
- WebSocket gateway (`docs/realtime-protocol.md`): identify, heartbeats
  that re-check the session, resume with replay of the last 500 events,
  presence with automatic idle, frame limits; fuzz-tested frame parser.

- Client (ADR 0017): the Expo app with Expo Router, `packages/ui` (tokens
  checked for WCAG AA contrast in both themes, motion constants with
  reduced motion, accessible primitives), translations from `locales/`,
  the instance picker, setup wizard, sign-up, password login with second
  factor, passkey login on web, provider sign-in (web redirect and mobile
  in-app browser), the HTTPS warning, and a home screen with status and
  live presence over the gateway. The session is kept in the Keychain or
  Keystore on mobile and in IndexedDB under a non-extractable WebCrypto key
  on web. The server serves the web build with a strict Content Security
  Policy.

- Desktop shell (`apps/desktop`, Electron): the web build served from a
  secure `app://` origin with a strict Content Security Policy, a
  sandboxed window that can only navigate within the app (links open in
  the system browser), all permission requests refused, and provider
  sign-in in its own window, returning to `nexuschat://oidc-return`.

- Browser release gate (`tests/e2e`, Playwright): setup wizard → admin →
  second user → live status on a second device → reload and logout, plus
  the desktop app, with axe-core on every screen (no serious or critical
  violations). It runs in the release gate over localhost and through
  Caddy, Nginx and Traefik, where Chromium trusts the test certificate's
  key so the page stays a secure context. An OWASP ZAP baseline scan runs
  against the set-up instance; it led to `connect-src 'self'`,
  Permissions-Policy, Cross-Origin-Embedder-Policy and
  Cross-Origin-Resource-Policy headers.

- Certificate expiry emails: 30, 7 and 1 days before the HTTPS certificate
  expires, `ADMIN_EMAIL` gets an email through the instance's SMTP
  settings, once per threshold and certificate, even across restarts.

- Recent re-authentication: removing an authenticator app or a security
  key or passkey, replacing backup codes, adding a passkey or security key,
  and linking or unlinking a provider need the session to have logged in,
  or confirmed its password (`POST /api/v1/auth/reauthenticate`), within
  the last 10 minutes, so a stolen session cannot add its own way back in.
  Account deletion (later) uses the same check.

- Mutation testing with gremlins in CI on every authentication package:
  89–100% of mutants caught, against a 70% minimum (ADR 0001).

- Admin panel in the client: the HTTPS certificate (status, names, issuer,
  expiry, a warning banner from 30 days before expiry, install by pasting
  the PEM files, removal after a confirmation) and the email settings (the
  saved password is never shown; a test email goes to the admin), under
  the plain-HTTP warning. The release gate opens it and runs axe on it.

- Worst-case UI (code standards §8.5): permanent demo, worst-case and
  right-to-left fixtures (`tests/fixtures`) used by component tests and by
  Playwright visual checks of every screen in both themes at 320, 390 and
  1280px, with layout checks (no clipped text, sideways scrolling or
  overlapping controls), axe, and pixel snapshots taken in the pinned
  Playwright image. Running them found and fixed: the custom status was
  never shown or settable, and changing status would have cleared it;
  wrapped button labels were left-aligned; certificate names ran together
  in one paragraph; names in sentences lacked bidirectional isolation.

- Motion pass for the milestone (code standards §8.5): finding
  opportunities, removing motion and planning improvements. Nothing needed
  removing (only the press scale and the platform's screen transitions
  move). Added: links dim while pressed (120ms); errors, confirmations
  and new form steps fade in (150ms and 200ms, opacity only, so reduced
  motion keeps them); a busy button shows its spinner only after 250ms, so
  fast requests no longer flash one. Rejected: motion on the status and
  option pickers (frequent, and the check mark already shows the choice),
  on the HTTPS and expiry warnings (they must be read at once), and on
  keyboard focus. The final animation review skill is not available in
  this environment; the changes were reviewed against the same rules by
  hand, and press feel still needs a check on a real device.
- Maestro flows (`apps/client/.maestro`): setup wizard, wrong password,
  login and logout, and sign-up, run in CI on an Android emulator against
  a fresh instance with the release APK. The same flows run on the iOS
  Simulator once the macOS build pipeline exists (M11), since iOS builds
  need macOS runners.

## M3: Crypto core [REVIEW] (in progress)

- `packages/crypto` with the MLS engine on ts-mls 1.6.4: create a group,
  add and remove devices, join from a Welcome, refresh keys, encrypt and
  decrypt, at 100% branch coverage.
- All 148 mlswg passive-client interop vectors pass in CI (pinned commit,
  checksum-verified download); issue #858 has a regression test and a
  one-character patch; CI checks reviewed and unreviewed advisories
  against the pinned version.
- Server key-package directory (`internal/keypackages`): each device (for
  now, each session) uploads one-time key packages and one last-resort
  package (`POST /api/v1/key-packages`), sees its stock and when to refill
  (`GET`), and others claim one per device of a user
  (`POST /api/v1/users/{userId}/key-packages/claim`), with one-time ones
  used up and the last resort kept. The server stays MLS-free and checks
  only the header (MLS 1.0, key package, ciphersuite 0x0001) and size.
  Claims are rate-limited per claiming user; packages go with the session.
- Attachment encryption (`attachments/attachments.ts`): a fresh key per
  file; one-shot AES-256-GCM (WebCrypto) up to 64 MiB, libsodium
  secretstream in 1 MiB pieces above, read and written as streams. Cut-off,
  reordered, extended or changed files are refused, and the ciphertext
  SHA-256 is checked. Tink's streaming format was dropped: there is no
  maintained JavaScript implementation (ADR 0014).
- Identity keys (`identity/identity.ts`): master and self-signing keys,
  device certificates over `@user@instance/deviceId` and the device's MLS
  signature key, chain checks, self-signing key rotation, and the safety
  number. Every group checks credentials (`identity/credential.ts`): a
  device is admitted, and a Welcome accepted, only if each member's
  certificate chains to its user's trusted identity.
- Published identities on the server (`internal/identities`, migration
  00011): `PUT /api/v1/identity` stores a user's master and self-signing
  public keys after checking the master key's Ed25519 signature (the same
  statement the client signs; a test checks a signature made by the
  TypeScript code). A new self-signing key under the same master key is
  accepted as is; a new master key needs a recent login.
  `GET /api/v1/users/{userId}/identity` returns it. The package is under the
  100% branch and mutation gates. Pinning on first contact is client work.
- Key-backup storage (`internal/keybackup`, migration 00012):
  `GET /api/v1/key-backup` returns the user's salt (made on first use),
  wrapped backup keys and blob list; `PUT`/`DELETE` on
  `/api/v1/key-backup/wrapped-keys/{method}` turn unlock methods on and off
  (recent login required); `/api/v1/key-backup/blobs/{name}` stores sealed
  blobs (1 MiB each, 10,000 per user). An email password reset deletes the
  password copy in the same statement that sets the new password. Under
  the 100% branch and mutation gates.
- Every request body is now capped at 16 MiB (`MaximumRequestBody`).
- History keys and archive copies (`history/archive.ts`): the history key
  is the MLS exporter over the channel and epoch; an archive copy seals the
  plaintext with it (AES-256-GCM) together with the sender's credential,
  MLS signature key and labelled signature, so it can be checked without
  that epoch's tree. History bundles (`history/bundle.ts`) carry a
  channel's keys to new members (ADR 0003's recommended design, built
  while the ADR is still Proposed).
- Encrypted device database (ADR 0013, `apps/client/src/storage`): records
  by collection and ID in SQLCipher (op-sqlite, key in the Keychain or
  Keystore) on native, and AES-GCM-sealed IndexedDB records on web, bound
  to their place. The desktop app wraps the data key with `safeStorage`
  (only for the app's own pages; declined where Linux has no keyring).
- Mutation testing of `packages/crypto` with Stryker (CI job "Mutation
  testing (crypto)", break at 70%): 90.9% of 640 mutants caught. The
  interop vectors are left out of mutation runs, since they test ts-mls.
- Property-based round-trips (fast-check) for the length-prefixed layout,
  device credentials, history bundles, and attachment streams split at
  random points on both sides.
- Key backup (`backup/backup.ts`): a random backup key seals named blobs
  (XChaCha20-Poly1305) and is wrapped under each unlock method: a 24-word
  recovery phrase (BIP-39, 256 bits, HKDF with the user's salt), the
  password (Argon2id, 3 passes, 256 MiB by default and never under 64 MiB,
  then HKDF with the backup context), or a passkey's PRF output. The
  additional data names each blob and method, so copies can't be swapped.
  Server storage and the signup screens come next.
- Device linking (`linking/linking.ts`): `crypto_kx` between the two
  devices, started from a QR code or through the server with a commitment
  to the new device's key; a 6×4-digit comparison code over both keys; and
  sealed transfers each way. Tests cover a relay in the middle. The server
  relay and the screens come with the device list.
- Performance at 10–2,000 devices is in ADR 0002. The M3 review kept
  ts-mls (ADR 0002 accepted): the patch now caches subtree hashes, which
  makes applying a commit about 27% cheaper at 1,000–2,000 devices, and
  very large channels batch membership changes. Still to do: reuse
  imported WebCrypto keys, and measure on a mid-range Android phone.

## M4: DMs and E2EE messaging (in progress)

- Friends, blocks and privacy (`internal/social`, migration 00013):
  friend requests (send, accept, decline, cancel; requesting someone who
  requested you accepts theirs), removing friends, blocking (ends any
  friendship and request, stops requests and DMs both ways, not revealed
  to the blocked user), and who may DM you (everyone / friends / friends
  and server members, the default) or friend-request you (everyone, the
  default / server members / nobody). Friend requests are rate-limited per
  sender. Both users' devices get a `relationship_update` event.
- DMs and group DMs (`internal/conversations`, migration 00014): one DM
  per pair of users, created only if the other's privacy setting allows
  and neither blocked the other (an existing DM stays readable after a
  block, but nobody can send); group DMs of 3 to 25 members with a name,
  adding members, leaving and renaming. Each conversation is one MLS group
  whose ID the server picks. Group DM icons and an instance setting for
  the 25-member cap are still to come.
- The MLS delivery service (`internal/delivery`, migration 00015): one
  commit per epoch (first wins, others get `wrong_epoch` or
  `epoch_conflict`), with the GroupInfo kept for external joins; welcomes;
  application messages with their archive copies (duplicate message IDs
  refused); fetching live copies by ID and archive copies by time; and a
  membership directive, signed with the instance key, when someone leaves
  a group DM. The server reads only the cleartext MLS header
  (`mlsheader`, fuzz-tested, checked against messages from ts-mls). Devices
  hear about changes through `conversation_update` and `mls_messages`.
- Two M3 items move here, since they need the key-setup screens and the
  native crypto stack: the E2E "signup → recovery phrase → second device
  verifies", and running the RFC 9420 vectors on Hermes.

## Known issues

- **Left out by the maintainer's decision (November 2026):** publishing to
  the App Store, Mac App Store, Google Play, Microsoft Store and Flathub;
  deploying the hosted official push relay; signing images with cosign; and
  running Maestro on the iOS Simulator (needs macOS runners). The build
  pipelines are written so these can be switched on with the publisher's
  accounts and keys, but they are not run.

- Moderate and low `pnpm audit` findings remain in development-only tools
  (`glob`, `minimatch`, `deepmerge`); see
  [dependencies.md](dependencies.md#security-and-compatibility-overrides).
- The license check's self-test covers the JavaScript checker; the Go
  checker uses the same allowlist logic from `go-licenses` but has no GPL
  fixture of its own, because that would need a GPL module in `go.mod`.
- Fuzz and property-based tests start with the first untrusted-input
  parsers (M2/M4); there are none yet.
