# Contributing to Nexus Chat

Thanks for helping. This page explains how changes get into the project.

## Start here

1. [Getting started](docs/contributing/getting-started.md): from clone to
   running tests locally in about 15 minutes.
2. [Code standards](docs/contributing/code-standards.md): the rules every
   change follows. Reviews check against this document.
3. [Architecture](docs/architecture.md) and the
   [decision records](docs/adr/README.md): why the project is built the way
   it is.

Security problems go through [SECURITY.md](SECURITY.md), never a public
issue. Everyone taking part follows the [Code of Conduct](CODE_OF_CONDUCT.md).

## Making a change

- **Small, focused commits** using [Conventional Commits](https://www.conventionalcommits.org/):
  `feat(server): add invite expiry`, `fix(client): keep focus after send`,
  `docs: explain backup codes`.
- **Tests come with the change.** Coverage gates are enforced in CI (see the
  code standards). Never delete, skip or weaken a test to make a build pass;
  fix the code or ask for help in the pull request.
- **API changes start in the contract.** Edit
  `apps/server/api/openapi.yaml`, then regenerate (`go generate ./...` in
  `apps/server`, `pnpm generate` at the root). Never hand-write a type that
  duplicates the spec.
- **No custom cryptography.** Use the vetted libraries listed in
  [docs/e2ee-design.md](docs/e2ee-design.md). If you think something new is
  needed, open an issue first.
- **New dependencies** need a one-line justification in
  [docs/dependencies.md](docs/dependencies.md), an exact pinned version, and
  an Apache-2.0-compatible license (CI checks this).

## Checks

All checks run in CI on every pull request: formatting, linting, type
checking, tests with the race detector, coverage, license checks, secret
scanning, CodeQL and vulnerability scans. The repository installs no git
hooks; run the commands in the getting-started guide before pushing to catch
problems early.

## License

By contributing, you agree that your contributions are licensed under the
[Apache License 2.0](LICENSE).
