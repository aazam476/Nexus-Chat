# Code Standards

**Who this is for:** a college junior who knows Java or Python from
coursework, is new to Go, and knows basic React. If that reader would need to
look something up to understand your code, add a comment or simplify.

Most rules below are enforced automatically (golangci-lint, ESLint, coverage
scripts). The ones that cannot be automated are checked in review.

## 1. General

- Prefer explicit, boring code over clever code.
- **Limits:** functions ≤ 40 lines, files ≤ 300 lines, nesting ≤ 3 levels,
  ≤ 4 parameters (use a parameter object beyond that). An exception needs a
  comment explaining why.
- **Full-word names:** `messageRepository`, not `msgRepo`. Allowed
  abbreviations: `id`, `url`, `api`, `dto`, `ui`.
- **Comments explain why, not what.** Security and crypto code cites the
  spec section it implements, for example `// RFC 9420 §12.4.3`.
- **Organize by feature, not by layer:** `messages/`, `channels/`,
  `voice/`, each with its own layers inside.
- No reflection tricks, no metaprogramming, and no code generation other than
  `sqlc` and the OpenAPI generators.

## 2. Go

- **Doc comments** on every exported identifier, starting with its name
  (`// Load reads ...`). Every package has a `doc.go` saying what it is for and
  where to start reading.
- **Layers inside a feature:** `handler` (HTTP/WebSocket only) → `service`
  (business logic) → `store` (database). No business logic in handlers, and
  no SQL outside stores.
- **Explicit wiring:** dependencies are passed to constructors
  (`NewService(store, clock)`) and assembled in `main.go`. No
  dependency-injection frameworks, no global mutable state, and no `init()`
  side effects.
- **Errors** are always handled and wrapped with context:
  `fmt.Errorf("loading channel %s: %w", id, err)`. Never ignore one silently;
  if ignoring is correct, assign to `_` and say why in a comment. No `panic`
  in request paths.
- `context.Context` is the first parameter of every function that does I/O.
- **Interfaces** are small and defined where they are used, not where they
  are implemented (see `instanceconfig.settingsStore`).
- **Avoid features that confuse newcomers:** no named or naked returns, no
  generics where a concrete type works, no clever channel patterns where a
  mutex is clearer. Every goroutine has a comment saying who stops it and
  how.
- Time comes from a `clock.Clock`, so tests never sleep waiting for it.
- **Tests:** standard `testing` with `testify` assertions, table-driven
  where there are several cases, and hand-written fakes instead of mocking
  frameworks. Always run with `-race`.
- `gofmt` and `golangci-lint` (configured in `.golangci.yml`); the build
  fails on any issue.

## 3. TypeScript and React Native

- `strict: true`, and `any` is an ESLint error.
- TSDoc on every exported function, component, hook and type.
- Function components and hooks only, one component per file. Business
  logic lives in hooks or services, not in JSX.
- ESLint + Prettier; the build fails on violations.
- **Tests:** Jest, plus React Native Testing Library for components.

## 4. Testing gates

| Gate                                                         | Requirement                                                          |
| ------------------------------------------------------------ | -------------------------------------------------------------------- |
| Overall coverage                                             | ≥ 85% lines/statements, ≥ 75% branches                               |
| `packages/crypto`, permission resolution, authentication     | 100% branch coverage, ≥ 70% mutation score                           |
| Permissions, Markdown parsing, serialization round-trips     | Property-based tests (`rapid` in Go, `fast-check` in TypeScript)     |
| Parsers of untrusted input (WebSocket, federation, Markdown) | Go fuzz tests                                                        |
| Black-box tests                                              | Only public interfaces (HTTP, WebSocket, UI); never import internals |

How Go coverage is measured: `scripts/go-coverage.sh` uses `go test
-coverprofile` for statements and [gobco](https://github.com/rillig/gobco)
for branches, because Go's built-in tool only counts statements. Generated
code is excluded. Packages that need 100% branch coverage are listed in each
module's `coverage-critical.txt`.

Flaky tests are fixed, or quarantined with a linked issue. Automatic retries
may never hide a failure.

## 5. Documentation

- Every app and package has a README covering what it does, how to run it,
  how to test it, and the three files to read first.
- Mermaid diagrams in `docs/` are kept up to date with the code.
- Conventional Commits; small, focused commits.

## 6. User interface

UI code follows the accessibility requirements (WCAG 2.1 AA: contrast,
keyboard access, screen-reader labels, reduced motion, 200% font scaling)
and keeps motion values in named constants in `packages/ui/motion.ts`.
Every screen is tested with the worst-case fixtures (very long names, empty
states, right-to-left text, huge counts) in light and dark themes.
