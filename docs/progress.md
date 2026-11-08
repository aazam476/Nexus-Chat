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

Still to do in M2: OIDC providers; profile, avatar and status; the
WebSocket gateway and presence; and the client: Expo app shell, setup wizard, login screens, HTTPS
banner, Electron shell, Playwright release gate and ZAP scan.

## Known issues

- Removing a second factor or regenerating backup codes needs only a valid
  session; it should also ask for recent re-authentication (password or
  passkey). Planned together with account deletion's re-authentication.

- Mutation testing tool for Go is chosen in M2 (see ADR 0001).
- Moderate and low `pnpm audit` findings remain in development-only tools
  (`glob`, `minimatch`, `deepmerge`); see
  [dependencies.md](dependencies.md#security-and-compatibility-overrides).
- The license check's self-test covers the JavaScript checker; the Go
  checker uses the same allowlist logic from `go-licenses` but has no GPL
  fixture of its own, because that would need a GPL module in `go.mod`.
- Fuzz and property-based tests start with the first untrusted-input
  parsers (M2/M4); there are none yet.
- Moved from M1 to M2 (they need the client UI or admin sessions): setup
  wizard screens, certificate upload in the Admin panel, the HTTPS warning
  banner, expiry emails, the browser-driven release gate, and the ZAP scan.
