// Package databasetest gives each test its own empty PostgreSQL database.
//
// Tests call [NewDatabaseURL] (or [NewPool]) and get a database that is
// dropped when the test ends, so tests never see each other's rows and can
// run in parallel.
//
// Where the PostgreSQL server comes from:
//   - If NEXUS_TEST_DATABASE_URL is set, that server is used. The URL must
//     point at a user allowed to CREATE DATABASE.
//   - Otherwise a postgres container is started with Testcontainers, which
//     needs Docker. This is what CI does.
//
// If neither is available the test fails with an explanation. It is never
// skipped.
package databasetest

import (
	"context"
	"crypto/rand"
	"fmt"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
)

// PostgresImage is the image used when no server URL is given. Keep it in
// step with infra/compose/docker-compose.yml.
const PostgresImage = "postgres:18.6-alpine"

// serverURLEnvironmentVariable names the variable that points at an existing
// PostgreSQL server.
const serverURLEnvironmentVariable = "NEXUS_TEST_DATABASE_URL"

// sharedServer starts at most one container per test binary. Starting one per
// test would take several seconds each. This is the one piece of
// package-level state in the codebase; it exists only in tests.
var sharedServer = sync.OnceValues(startServer)

// NewDatabaseURL creates an empty database and returns its connection URL.
// The database is dropped when the test finishes.
func NewDatabaseURL(t testing.TB) string {
	t.Helper()
	serverURL, err := sharedServer()
	if err != nil {
		t.Fatalf("no PostgreSQL server for tests: %v\nSet %s to a server URL, or make Docker available.",
			err, serverURLEnvironmentVariable)
	}

	ctx := context.Background()
	name := "test_" + strings.ToLower(rand.Text())
	administration, err := pgx.Connect(ctx, serverURL)
	if err != nil {
		t.Fatalf("connecting to test PostgreSQL server: %v", err)
	}
	defer func() { _ = administration.Close(ctx) }()
	// The name comes from rand.Text (A–Z, 2–7) so it is safe to splice in;
	// CREATE DATABASE cannot take a bind parameter.
	if _, err := administration.Exec(ctx, "CREATE DATABASE "+name); err != nil {
		t.Fatalf("creating test database: %v", err)
	}
	t.Cleanup(func() { dropDatabase(t, serverURL, name) })

	return withDatabaseName(t, serverURL, name)
}

// NewPool creates an empty database and returns a connection pool for it.
func NewPool(t testing.TB) *pgxpool.Pool {
	t.Helper()
	pool, err := pgxpool.New(context.Background(), NewDatabaseURL(t))
	if err != nil {
		t.Fatalf("opening test database pool: %v", err)
	}
	// Registered after the drop, so it runs first (cleanups run in reverse).
	t.Cleanup(pool.Close)
	return pool
}

func startServer() (string, error) {
	if serverURL := os.Getenv(serverURLEnvironmentVariable); serverURL != "" {
		return serverURL, nil
	}
	ctx := context.Background()
	container, err := postgres.Run(ctx, PostgresImage,
		postgres.WithUsername("nexuschat"),
		postgres.WithPassword("nexuschat"),
		postgres.WithDatabase("nexuschat"),
		postgres.BasicWaitStrategies(),
	)
	if err != nil {
		return "", fmt.Errorf("starting %s with Testcontainers: %w", PostgresImage, err)
	}
	// Testcontainers' reaper removes the container when the test binary
	// exits, so there is nothing to stop here.
	connection, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		return "", fmt.Errorf("reading container connection string: %w", err)
	}
	return connection, nil
}

func dropDatabase(t testing.TB, serverURL string, name string) {
	ctx := context.Background()
	administration, err := pgx.Connect(ctx, serverURL)
	if err != nil {
		t.Errorf("connecting to drop test database: %v", err)
		return
	}
	defer func() { _ = administration.Close(ctx) }()
	// FORCE disconnects pools a failed test may have left open.
	if _, err := administration.Exec(ctx, "DROP DATABASE IF EXISTS "+name+" WITH (FORCE)"); err != nil {
		t.Errorf("dropping test database %s: %v", name, err)
	}
}

func withDatabaseName(t testing.TB, serverURL string, name string) string {
	parsed, err := url.Parse(serverURL)
	if err != nil {
		t.Fatalf("parsing test server URL: %v", err)
	}
	parsed.Path = "/" + name
	return parsed.String()
}
