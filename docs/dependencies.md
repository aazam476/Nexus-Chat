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
| `github.com/coder/websocket` | v1.8.15 | ISC | The WebSocket gateway ([ADR 0012](adr/0012-websocket-and-realtime-state.md)); `context`-based, no dependencies |
| `golang.org/x/image` | v0.47.0 | BSD-3-Clause | WebP decoding and high-quality resizing of profile pictures, from the Go project |
| `golang.org/x/sync` | v0.24.0 | BSD-3-Clause | A weighted semaphore that bounds how many uploaded pictures are decoded at once |
| `github.com/pquerna/otp` | v1.5.0 | Apache-2.0 | TOTP (RFC 6238) enrollment URIs and code checks for two-factor login |
| `github.com/go-webauthn/webauthn` | v0.18.2 | BSD-3-Clause | WebAuthn ceremonies for passkeys and security keys; the maintained Go implementation, verifies attestation and assertions |

Their own dependencies (`pgpassfile`, `pgservicefile`, `puddle`,
`interpolate`, `go-retry`, `multierr`,
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

## TypeScript: `@nexus-chat/client` and `@nexus-chat/ui`

Versions are the ones Expo SDK 58 is tested with (from `expo`'s own list of
compatible native modules), so the app builds on iOS, Android and the web.

| Package                                                                                      | Version                        | License | Why                                                                                                                                       |
| -------------------------------------------------------------------------------------------- | ------------------------------ | ------- | ----------------------------------------------------------------------------------------------------------------------------------------- |
| `expo`, `expo-router`, `expo-constants`, `expo-linking`, `expo-status-bar`, `expo-system-ui` | 58.0.x                         | MIT     | The app framework and file-based navigation the spec names (spec §2)                                                                      |
| `react`, `react-dom`, `react-native`, `react-native-web`                                     | 19.3.0, 19.3.0, 0.88.1, 0.21.4 | MIT     | React Native, and its web renderer for the web build                                                                                      |
| `react-native-reanimated`, `react-native-worklets`                                           | 4.7.1, 0.13.0                  | MIT     | Animations on the UI thread (ADR 0017)                                                                                                    |
| `react-native-gesture-handler`, `react-native-screens`, `react-native-safe-area-context`     | 3.3.0, 4.30.1, 5.11.1          | MIT     | Native gestures, native screen containers and notch-safe layouts, required by Expo Router                                                 |
| `expo-secure-store`                                                                          | 58.0.2                         | MIT     | Keychain / Keystore storage for the session on iOS and Android (spec §5.3)                                                                |
| `expo-web-browser`                                                                           | 58.0.7                         | MIT     | The in-app browser session for provider sign-in on mobile (ADR 0016)                                                                      |
| `i18next`, `react-i18next`                                                                   | 26.7.2, 17.1.13                | MIT     | Every user-facing string comes from translation files (spec §4.13); the newest majors were released the day before and are left to settle |

| Package (dev)                                              | Version         | License    | Why                                                                                                             |
| ---------------------------------------------------------- | --------------- | ---------- | --------------------------------------------------------------------------------------------------------------- |
| `jest-expo`, `@testing-library/react-native`               | 58.0.14, 14.1.0 | MIT        | Component and hook tests in React Native's environment (spec §7.1)                                              |
| `fake-indexeddb`                                           | 6.2.7           | Apache-2.0 | Tests the web storage without a browser                                                                         |
| `@eslint-react/eslint-plugin`, `eslint-plugin-react-hooks` | 5.28.8, 7.1.1   | MIT        | React and hooks rules                                                                                           |
| `eslint-plugin-react-native-a11y`                          | 3.5.1           | MIT        | Accessibility rules for React Native (spec §7.2); its peer range predates ESLint 10 but its rules run unchanged |
| `eslint-plugin-i18next`                                    | 6.1.5           | ISC        | Rejects text written directly into screens (spec §4.13)                                                         |

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
