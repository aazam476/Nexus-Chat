# Dependencies

Every runtime dependency has a one-line justification here or in an ADR
(spec §0.3). Versions are pinned exactly in `go.mod` and `package.json`;
Dependabot proposes updates weekly.

## Go: shipped in the server binary

| Module                        | Version | License | Why                                                                                      |
| ----------------------------- | ------- | ------- | ---------------------------------------------------------------------------------------- |
| `github.com/jackc/pgx/v5`     | v5.12.0 | MIT     | PostgreSQL driver and connection pool required by the spec; fastest maintained Go driver |
| `github.com/pressly/goose/v3` | v3.29.0 | MIT     | Migrations embedded in the binary and applied on start, as the spec requires             |

Their own dependencies (`pgpassfile`, `pgservicefile`, `puddle`,
`interpolate`, `go-retry`, `multierr`, `golang.org/x/sync`,
`golang.org/x/text`) come with them and are covered by the license check.

## Go: shipped in the push relay binary

None beyond the standard library.

## Go: tests and build tools (not shipped)

| Module                                                   | Version  | License    | Why                                                                                       |
| -------------------------------------------------------- | -------- | ---------- | ----------------------------------------------------------------------------------------- |
| `github.com/stretchr/testify`                            | v1.12.1  | MIT        | Assertions, as the code standards specify                                                 |
| `github.com/testcontainers/testcontainers-go`            | v0.45.0  | MIT        | Starts PostgreSQL for database and contract tests                                         |
| `github.com/getkin/kin-openapi`                          | v0.153.0 | MIT        | Contract tests validate responses against `openapi.yaml`                                  |
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
