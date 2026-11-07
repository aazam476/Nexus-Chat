# Nexus Chat — Roadmap to Beta

Each milestone is independently testable and complete only when every gate in
the spec's §7 that applies to it passes in CI (coverage, mutation, property,
fuzz, black-box, accessibility, worst-case UI). After each milestone,
`docs/progress.md` is updated. Milestones marked **[REVIEW]** pause for
maintainer review before the next one starts.

| #   | Milestone                                   | Depends on |
| --- | ------------------------------------------- | ---------- |
| M0  | Foundation                                  | —          |
| M1  | Deployable instance                         | M0         |
| M2  | Accounts and client shell                   | M1         |
| M3  | Crypto core **[REVIEW]**                    | M2         |
| M4  | DMs and E2EE messaging                      | M3         |
| M5  | Servers, channels, permissions              | M4         |
| M6  | Search                                      | M5         |
| M7  | Moderation and admin                        | M5         |
| M8  | Push notifications                          | M4         |
| M9  | Voice, video, screen share                  | M5         |
| M10 | Federation                                  | M5, M8, M9 |
| M11 | Desktop and store distribution **[REVIEW]** | M9         |
| M12 | Account lifecycle, hardening, beta release  | all        |

## M0 — Foundation

- Monorepo: pnpm workspaces + Turborepo, `apps/*`, `packages/*`, Go module
  in `apps/server` and `apps/push-relay`.
- CI: build, `gofmt`, `golangci-lint`, ESLint/Prettier, `tsc --strict`,
  tests with `-race`, coverage gates, license check, CodeQL, secret
  scanning, dependency scan.
- `openapi.yaml` skeleton, `oapi-codegen` and API client generation
  wired into the build, `sqlc` + `goose` with one migration.
- Server: `/healthz`, `/readyz`, structured logging, config loading,
  first-boot secret generation.
- Docs: README, CONTRIBUTING, code standards, getting-started, SECURITY.md,
  CODE_OF_CONDUCT.md.
- **Test:** CI green on an empty feature set; `go test` for config and
  secret generation; the license check fails on a deliberately added GPL
  fixture in a test job.

## M1 — Deployable instance

- `infra/compose/docker-compose.yml` (server, postgres, livekit; `p2p`
  profile with coturn), `.env.example` with the two required variables;
  media server config generated from first-boot secrets.
- Multi-arch images, cosign signing, SBOM and provenance, Trivy.
- Setup token and setup API (first admin, instance name, domain,
  registration mode); HTTPS from installed certificate files with
  validation, hot reload and expiry warnings; `TRUSTED_PROXIES`.
- `docs/self-hosting.md` with Nginx, Caddy and Traefik examples.
- Release gate: clean install from only `docker-compose.yml` and
  `.env.example`, on localhost and through each of the three proxies.
- **Test:** release gate passes in under 10 minutes; certificate
  validation table tests; contract tests for setup.
- _Moved to M2_ (they need the client UI and admin sessions): the setup
  wizard screens, certificate upload in the Admin panel, the HTTPS warning
  banner, expiry emails, the browser-driven release gate, and the ZAP scan
  (which needs pages to scan).

## M2 — Accounts and client shell

- Registration modes, username and profile rules, email verification,
  avatar cropping and resizing, status and custom status.
- Password (Argon2id, breached-password list), passkeys, TOTP, security
  keys, backup codes, OIDC providers, sessions list and revoke, rate limits
  and progressive delays, password reset.
- WebSocket gateway with resume; presence; `docs/realtime-protocol.md`.
- Expo app (dev builds), Expo Router, instance picker, login and signup,
  theme tokens, light/dark/system, i18n setup, `packages/ui` primitives,
  `motion.ts`, `break-ui` worst-case fixtures, axe checks, visual snapshots.
- Electron shell that loads the web build (no native features yet).
- From M1: setup wizard screens, Admin panel certificate upload with
  expiry banner and email, the HTTPS warning on admin pages and the login
  page, the Playwright release gate, and the ZAP baseline scan.
- **Test:** auth at 100% branch coverage and ≥ 70% mutation; Playwright
  signup/login on web; Maestro login flows; axe zero serious/critical.

## M3 — Crypto core **[REVIEW]**

- `packages/crypto`: MLS wrapper, RFC 9420 test vectors on Node and Hermes,
  identity keys and cross-signing, device linking by QR and comparison code,
  key backup with phrase, password and passkey PRF, history keys, archive
  envelopes, attachment encryption.
- Server: key-package directory, delivery service (one commit per epoch,
  signed membership directives, GroupInfo storage).
- Encrypted local storage on native, web and Electron.
- **Test:** 100% branch and ≥ 70% mutation in `packages/crypto`; property
  round-trips; E2E "signup → recovery phrase → second device verifies".
- Review point: confirm the MLS library choice against real performance
  numbers before building features on it.

## M4 — DMs and E2EE messaging

- Friends, blocks, privacy settings, DMs, group DMs (name, icon, add, leave).
- Every §4.5 feature: Markdown subset (parser fuzzed and property-tested),
  mentions, edit/delete, attachments with client thumbnails (streaming
  encryption above 64 MiB, optional lossy compression, instance size
  limit; ADR 0014), reactions,
  inline replies, typing, read state and jump to unread. Link previews on
  native and desktop; web preview proxy toggle.
- **Test:** Playwright multi-user DM flows; reaction, reply flows; WebSocket
  frame fuzzing; Markdown fuzzing.

## M5 — Servers, channels, permissions

- Servers, categories, text and voice channels, drag-and-drop ordering,
  invites, join by address, roles, channel overrides, the full permission
  list in one module, `docs/permissions.md` with a worked example.
- Channel audiences → MLS groups; `READ_HISTORY_BEFORE_JOIN` history bundles.
- Per-server attachment limit and "compress media by default" setting
  (ADR 0014).
- Threads (follow/unfollow), custom emoji.
- **Test:** exhaustive table-driven and property tests for permission
  resolution; Playwright permission-denial paths and thread flows; E2E
  "new member reads history"; k6 load smoke test (500 WebSocket users,
  p95 < 300 ms).

## M6 — Search

- Encrypted local FTS5 index, recent window, backfill (Wi-Fi + charging on
  mobile), encrypted index sync by channel and month, desktop full indexing.
- Server-side metadata search (directory, channel names, members).
- **Test:** search filters; index sync between two devices; backfill resume
  after interruption.

## M7 — Moderation and admin

- Kick, ban with message deletion, timeout, audit log, reports with
  signature + server-receipt verification, instance admin panel (users,
  servers, registration, federation lists, stats).
- **Test:** report forgery attempts are rejected; audit log filters;
  Playwright admin flows.

## M8 — Push notifications

- `apps/push-relay` with instance registration, handles, signed sends, rate
  limits, cleanup, blocklist.
- Instance push service, Web Push (VAPID), UnifiedPush, iOS Notification
  Service Extension, Android data-message handler, per-user settings.
- **Test:** relay suite including wrong-instance signature, forged
  handle, deleted handle, and the federation example.

## M9 — Voice, video, screen share

- Starts with two spikes: (a) P2P frame-encryption interop between web
  transforms and native `FrameCryptor`; (b) LiveKit E2EE with MLS-derived
  keys and epoch rotation.
- SFU mode, P2P mesh, Auto mode with the switching rules in
  [ADR 0009](adr/0009-voice-transport.md), device selection, per-user volume,
  mute/deafen, server mute/deafen, move members, push-to-talk on web,
  speaking and quality indicators, screen share on web, Android and iOS.
- **Test:** Playwright voice join with fake media devices in all three
  modes; mode switch while in a call.

## M10 — Federation

- Discovery, RFC 9421 signing, all v1 endpoints, outbound queues, remote
  key packages, remote voice tokens, cross-instance DMs, policy lists.
- **Test:** two-instance Docker suite: remote user joins, chats, reacts,
  joins voice, gets banned; fuzzing of federation request parsing.

## M11 — Desktop and store distribution **[REVIEW]**

- Electron native features: screen/window picker, system audio, tray,
  global push-to-talk, `safeStorage`.
- MAS, MSIX and Flatpak builds; mobile builds per
  [ADR 0011](adr/0011-mobile-build-pipeline.md); TestFlight, Play internal
  testing, and Store flights; `store/` folder generating listings and privacy
  labels.
- Starts with a sandbox spike on a signed MAS build (see
  [`platform-support.md`](platform-support.md)).
- **Test:** Playwright for Electron, including the window/screen picker.

## M12 — Account lifecycle, hardening, beta release

- Data export (assembled on device), account deletion with grace period,
  ownership transfer checks, federated deletion notices.
- Remaining gates: ZAP, Trivy, full release gate, docs site (`pkgsite`),
  accessibility manual checklist.
- **Beta release** `0.1.0-beta.1`.
