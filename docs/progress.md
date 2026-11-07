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

## Next

**M1: Deployable instance.** Docker images, Compose file, setup wizard,
HTTPS by certificate upload or reverse proxy, release-gate job.

## Known issues

- Mutation testing tool for Go is chosen in M2 (see ADR 0001).
- Moderate and low `pnpm audit` findings remain in development-only tools
  (`glob`, `minimatch`, `deepmerge`); see
  [dependencies.md](dependencies.md#security-and-compatibility-overrides).
- The license check's self-test covers the JavaScript checker; the Go
  checker uses the same allowlist logic from `go-licenses` but has no GPL
  fixture of its own, because that would need a GPL module in `go.mod`.
- Fuzz and property-based tests start with the first untrusted-input
  parsers (M2/M4); M0 has none.
