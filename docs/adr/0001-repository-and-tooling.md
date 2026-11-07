# ADR 0001: Repository layout and tooling

- Status: Accepted (fixed by the project specification)
- Date: 2026-11-07

## Context

The spec fixes most of the stack. This ADR records the pinned versions found
at the start of Phase 0 and the few choices the spec leaves open.

## Decision

| Tool | Version (pinned at Phase 0) | Note |
|---|---|---|
| Go | 1.27.3 | Server and push relay |
| pgx | v5.12.0 | |
| sqlc | v1.31.1 | |
| goose | v3.29.0 | Migrations embedded via `embed.FS` |
| oapi-codegen | v2.8.2 | `std-http` strict server |
| testify | v1.12.1 | Assertions only; no `mock` package |
| rapid | v1.3.0 | Property tests |
| Node.js | 22 LTS line (installed 22.23.4; re-checked at M0) | |
| pnpm | 10.28.0 (re-checked at M0) | |
| Turborepo | 2.11.10 | |
| TypeScript | 7.0.2 | |
| Expo SDK | 58 (`expo` 58.0.23) | React Native 0.88 via the SDK |
| Electron | 45.2.3 | |
| Jest | 30.5.2 | |
| fast-check | 4.10.2 | |
| Stryker | 10.1.1 | |
| Playwright | 1.65.0 | |

Versions are re-verified and pinned exactly (no ranges) when each package is
first added, and Renovate keeps them current.

- **Licenses:** CI runs `go-licenses` for Go and `license-checker-rseidelsohn`
  for pnpm, with an allowlist (Apache-2.0, MIT, BSD-2/3-Clause, ISC,
  MPL-2.0, 0BSD, Unlicense, CC0-1.0). Anything else fails the build.
- **Go mutation testing:** gremlins, if it is maintained when M0 starts.
  Otherwise the most active maintained Go mutation tool, recorded in this ADR.
- **No git hook managers.** Lint, format, and test checks run in CI only.

## Consequences

The pinned versions are a snapshot. Every later dependency addition records
its version and justification in `docs/dependencies.md`.
