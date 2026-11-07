# ADR 0001: Repository layout and tooling

- Status: Accepted (fixed by the project specification)
- Date: 2026-11-07 (updated in milestone M0)

## Context

The spec fixes most of the stack. This ADR records the pinned versions found
at the start of Phase 0 and the few choices the spec leaves open.

## Decision

| Tool         | Version (pinned at Phase 0)                  | Note                               |
| ------------ | -------------------------------------------- | ---------------------------------- |
| Go           | 1.27.3                                       | Server and push relay              |
| pgx          | v5.12.0                                      |                                    |
| sqlc         | v1.31.1                                      |                                    |
| goose        | v3.29.0                                      | Migrations embedded via `embed.FS` |
| oapi-codegen | v2.8.2                                       | `std-http` strict server           |
| testify      | v1.12.1                                      | Assertions only; no `mock` package |
| rapid        | v1.3.0                                       | Property tests                     |
| Node.js      | 26.13.0 (current LTS, `.node-version`)       |                                    |
| pnpm         | 12.18.0 (`packageManager` in `package.json`) |                                    |
| Turborepo    | 2.11.10                                      |                                    |
| TypeScript   | 6.0.3                                        | Not 7.0.2; see below               |
| Expo SDK     | 58 (`expo` 58.0.23)                          | React Native 0.88 via the SDK      |
| Electron     | 45.2.3                                       |                                    |
| Jest         | 30.5.2                                       |                                    |
| fast-check   | 4.10.2                                       |                                    |
| Stryker      | 10.1.1                                       |                                    |
| Playwright   | 1.65.0                                       |                                    |

Versions are re-verified and pinned exactly (no ranges) when each package is
first added, and Dependabot (built into GitHub, no extra app to install)
proposes updates weekly.

- **TypeScript 6, not 7.** TypeScript 7.0.2 is the latest release, but it is
  the new native compiler and lacks the JavaScript API that `typescript-eslint`
  (supports `<6.1`) and `ts-jest` (`<7`) need for typed lint rules and tests.
  We use the newest version those tools support and move to 7 when they do.
- **Licenses:** CI runs `go-licenses` for Go and `license-checker-rseidelsohn`
  for pnpm, with the allowlist in [`dependencies.md`](../dependencies.md).
  Anything else fails the build, and a GPL fixture proves the check works.
- **Go branch coverage:** Go's built-in coverage counts statements only, so
  branch coverage is measured with gobco v1.3.4 (BSD-2-Clause).
- **Linting:** golangci-lint v2.15.0 with the configuration in
  `.golangci.yml`, which encodes the code standards' size and style limits.
- **Go mutation testing:** decided in M2, when the first package that needs
  it (authentication) exists. gremlins' latest release, v0.5.1, is from May
  2024, so its maintenance status is checked again then and the choice is
  recorded here.
- **No git hook managers.** Lint, format, and test checks run in CI only.

## Consequences

The pinned versions are a snapshot. Every later dependency addition records
its version and justification in `docs/dependencies.md`.
