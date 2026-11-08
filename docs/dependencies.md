# Dependencies

Every runtime dependency has a one-line justification here or in an ADR
(spec §0.3). Versions are pinned exactly in `go.mod` and `package.json`;
Dependabot proposes updates weekly.

## Go: shipped in the server binary

| Module                        | Version | License | Why                                                                                      |
| ----------------------------- | ------- | ------- | ---------------------------------------------------------------------------------------- |
| `github.com/jackc/pgx/v5`     | v5.12.0 | MIT     | PostgreSQL driver and connection pool required by the spec; fastest maintained Go driver |
| `github.com/pressly/goose/v3` | v3.29.0 | MIT     | Migrations embedded in the binary and applied on start, as the spec requires             |

| `golang.org/x/crypto` | v0.58.0 | BSD-3-Clause | Argon2id password hashing (`argon2`), from the Go project |
| `golang.org/x/time` | v0.16.0 | BSD-3-Clause | Token-bucket rate limiting (`rate`), from the Go project |
| `github.com/google/uuid` | v1.6.0 | BSD-3-Clause | UUIDv7 identifiers; also the UUID type used by the generated API code |
| `github.com/oapi-codegen/runtime` | v1.8.0 | Apache-2.0 | Small runtime support library required by the code `oapi-codegen` generates |
| `github.com/coreos/go-oidc/v3` | v3.21.0 | Apache-2.0 | OIDC discovery and ID token verification for provider sign-in ([ADR 0016](adr/0016-oidc-sign-in.md)); brings `go-jose/v4` (Apache-2.0) |
| `golang.org/x/oauth2` | v0.37.0 | BSD-3-Clause | Authorization code exchange with PKCE for provider sign-in, from the Go project |
| `github.com/pquerna/otp` | v1.5.0 | Apache-2.0 | TOTP (RFC 6238) enrollment URIs and code checks for two-factor login |
| `github.com/go-webauthn/webauthn` | v0.18.2 | BSD-3-Clause | WebAuthn ceremonies for passkeys and security keys; the maintained Go implementation, verifies attestation and assertions |

Their own dependencies (`pgpassfile`, `pgservicefile`, `puddle`,
`interpolate`, `go-retry`, `multierr`, `golang.org/x/sync`,
`golang.org/x/text`) come with them and are covered by the license check.

## Bundled data

| Data                                                                                  | License | Why                                                                                                        |
| ------------------------------------------------------------------------------------- | ------- | ---------------------------------------------------------------------------------------------------------- |
| Common breached passwords (from SecLists `Pwdb_top-1000000.txt`, 10+ characters only) | MIT     | Breached-password check required by spec §4.1; see `apps/server/internal/auth/password/breached/README.md` |

## Container images

| Image                               | Pinned version                     | Why                                                               |
| ----------------------------------- | ---------------------------------- | ----------------------------------------------------------------- |
| `gcr.io/distroless/static-debian13` | by digest                          | Base of our images ([ADR 0015](adr/0015-container-deployment.md)) |
| `golang`                            | 1.27.3-trixie, by digest           | Build stage only                                                  |
| `postgres`                          | 18.6-alpine, by digest             | The database the spec requires                                    |
| `livekit/livekit-server`            | v1.13.11, by digest                | SFU and built-in TURN (spec §2)                                   |
| `coturn/coturn`                     | 4.18.0, by digest                  | TURN for P2P mode (`--profile p2p`)                               |
| `caddy`, `nginx`, `traefik`         | 2.11.9, 1.30.6, v3.7.17, by digest | Release-gate tests of the reverse-proxy examples only             |

## Go: shipped in the push relay binary

None beyond the standard library.

## Go: tests and build tools (not shipped)

| Module                                                   | Version  | License    | Why                                                                                       |
| -------------------------------------------------------- | -------- | ---------- | ----------------------------------------------------------------------------------------- |
| `github.com/stretchr/testify`                            | v1.12.1  | MIT        | Assertions, as the code standards specify                                                 |
| `github.com/testcontainers/testcontainers-go`            | v0.45.0  | MIT        | Starts PostgreSQL for database and contract tests                                         |
| `github.com/getkin/kin-openapi`                          | v0.153.0 | MIT        | Contract tests validate responses against `openapi.yaml`                                  |
| `github.com/fxamacker/cbor/v2`                           | v2.9.4   | MIT        | Builds WebAuthn authenticator responses in the passkey tests                              |
| `github.com/oapi-codegen/oapi-codegen/v2`                | v2.8.2   | Apache-2.0 | Generates server interfaces from the OpenAPI file (Go `tool`)                             |
| `github.com/sqlc-dev/sqlc`                               | v1.31.1  | MIT        | Compiles SQL queries to typed Go (Go `tool`)                                              |
| golangci-lint, gobco, go-licenses, govulncheck, gitleaks | see CI   | GPL-free   | Lint, branch coverage, license and vulnerability checks; installed in CI, not in `go.mod` |

## TypeScript: `@nexus-chat/api-client`

No runtime dependencies: the generated client uses the platform `fetch`.

| Package (dev)                    | Version                 | License | Why                                                                                                 |
| -------------------------------- | ----------------------- | ------- | --------------------------------------------------------------------------------------------------- |
| `@hey-api/openapi-ts`            | 0.99.0                  | MIT     | Generates the client from `openapi.yaml`; supports TypeScript 6 (`openapi-typescript` does not yet) |
| `jest`, `ts-jest`, `@types/jest` | 30.5.2, 29.4.15, 30.0.0 | MIT     | Test runner required by the spec                                                                    |

## TypeScript: workspace tooling (dev only)

`typescript` 6.0.3, `eslint` 10.14.0, `typescript-eslint` 8.72.1,
`@eslint/js`, `eslint-plugin-jsdoc` (BSD-3-Clause), `eslint-plugin-tsdoc`,
`globals`, `prettier` 3.9.10, `turbo` 2.11.10 and
`license-checker-rseidelsohn` (BSD-3-Clause). All MIT unless noted.

**Why TypeScript 6.0.3 rather than 7.0.2:** TypeScript 7 is the new native
compiler and does not yet provide the JavaScript API that typed ESLint rules
(`typescript-eslint`, which supports `<6.1`) and `ts-jest` (`<7`) depend on.
Using two compilers would mean two sets of type errors. We move to 7 when
both tools support it.

## Security and compatibility overrides

Declared in `pnpm-workspace.yaml`. Each one is removed once the parent package
ships the fix.

| Override                                                  | Reason                                                                               |
| --------------------------------------------------------- | ------------------------------------------------------------------------------------ |
| `js-yaml` 4.x → 4.3.2                                     | Three high-severity CPU-exhaustion advisories, reached through `@hey-api/openapi-ts` |
| `eslint-plugin-tsdoc` → `@typescript-eslint/utils` 8.72.1 | The plugin pins a version that predates TypeScript 6 support                         |

Known, not yet fixed (development tools only, below the `high` audit
threshold): `deepmerge` (low, via Jest), `glob` and `minimatch` (moderate, via
Jest and ESLint). None of them ships in an app or image.

## Allowed licenses

The license check (`scripts/check-licenses.sh`) allows: Apache-2.0, MIT,
BSD-2-Clause, BSD-3-Clause, ISC, MPL-2.0 (unmodified use), 0BSD, Unlicense
and CC0-1.0. For JavaScript tooling it also allows these permissive
licenses, which appear only in development dependencies:

- BlueOak-1.0.0: a permissive license similar to MIT
- Python-2.0: the PSF license of `argparse`, a permissive license
- CC-BY-3.0 and CC-BY-4.0: data files (SPDX license lists, browser data)
  that require attribution only

GPL, LGPL and AGPL are never allowed. CI proves the check works by running it
against `scripts/license-fixtures/gpl-dependency`, which must fail.
