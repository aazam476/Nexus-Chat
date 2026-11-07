# Nexus Chat server (`apps/server`)

The Go backend of a Nexus Chat instance. It will serve the REST API,
WebSocket gateway, federation, the MLS delivery service and the web client.
Today (milestone M0) it loads its configuration, creates its secrets on first
boot, migrates the database, and serves `/healthz` and `/readyz`.

## Run it

You need PostgreSQL. See
[getting started](../../docs/contributing/getting-started.md#4-run-the-server)
for the exact command; the short form is:

```bash
ADMIN_EMAIL=admin@example.com DATABASE_URL="postgres://…" DATA_DIR=./data go run ./cmd/nexus-chat-server
```

Environment variables are documented in `internal/config/config.go`.

## Test it

```bash
../../scripts/go-coverage.sh .
```

This runs every test with the race detector and enforces the coverage gates.
Database tests use Docker (Testcontainers) or `NEXUS_TEST_DATABASE_URL`.
`test/contract` contains black-box tests that start the real binary and
check every response against `api/openapi.yaml`.

After editing `api/openapi.yaml` or any SQL, run `go generate ./...` and
commit the generated files.

## Read these three files first

1. `cmd/nexus-chat-server/application.go`: how every part is wired together.
2. `api/openapi.yaml`: the API contract everything else is generated from.
3. `internal/health/`: the smallest complete feature (handler → service),
   a template for new features.
