# Getting Started

From a fresh clone to passing tests in about 15 minutes.

## 1. Install the tools

| Tool                                          | Version                           | Why                                            |
| --------------------------------------------- | --------------------------------- | ---------------------------------------------- |
| [Go](https://go.dev/dl/)                      | 1.27.3 (see `apps/server/go.mod`) | Server and push relay                          |
| [Node.js](https://nodejs.org/)                | 26.13.0 LTS (see `.node-version`) | Client, packages, tooling                      |
| [pnpm](https://pnpm.io/installation)          | 12.18.0 (see `package.json`)      | JavaScript package manager                     |
| [Docker](https://docs.docker.com/get-docker/) | any recent version                | Database tests start PostgreSQL in a container |

Install pnpm with `npm install -g pnpm@12.18.0`.

Go linting and coverage tools:

```bash
go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.15.0
```

```bash
go install github.com/rillig/gobco@v1.3.4
```

```bash
go install github.com/google/go-licenses/v2@v2.0.1
```

## 2. Install JavaScript dependencies

```bash
pnpm install
```

## 3. Run the checks

TypeScript (lint, type check, tests, build) for every package:

```bash
pnpm lint && pnpm typecheck && pnpm test && pnpm build
```

Go, for one module (`apps/server` or `apps/push-relay`):

```bash
cd apps/server && golangci-lint run ./... && cd ../..
```

```bash
scripts/go-coverage.sh apps/server
```

The coverage script runs every test with the race detector. Database tests
start a PostgreSQL container through Docker. If you would rather use an
existing PostgreSQL server, point the tests at it with a user that may
create databases:

```bash
export NEXUS_TEST_DATABASE_URL="postgres://user:password@localhost:5432/postgres?sslmode=disable"
```

## 4. Run the server

The server needs a PostgreSQL database. With one available:

```bash
cd apps/server
```

```bash
ADMIN_EMAIL=admin@example.com DATABASE_URL="postgres://user:password@localhost:5432/nexuschat?sslmode=disable" DATA_DIR=./data go run ./cmd/nexus-chat-server
```

Then check it: `curl http://localhost:8080/readyz`. A Docker Compose file
that runs everything arrives in milestone M1.

## 5. After changing the API

The OpenAPI file `apps/server/api/openapi.yaml` is the contract. After
editing it, regenerate both sides and commit the result:

```bash
(cd apps/server && go generate ./...) && pnpm generate
```

CI fails if generated code is out of date.

## Where to go next

- [Code standards](code-standards.md)
- [Architecture](../architecture.md)
- The README of the app or package you are working on
