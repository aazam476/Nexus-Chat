package database

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"log/slog"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/pressly/goose/v3/lock"
)

// migrationFiles holds every *.sql file in migrations/. The embed directive
// is how the SQL ends up inside the binary.
//
//go:embed migrations/*.sql
var migrationFiles embed.FS

// Open connects to PostgreSQL and checks the connection works before
// returning, so a wrong password fails at startup with a clear message.
func Open(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		// pgx error messages never include the password.
		return nil, fmt.Errorf("configuring database connection: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("connecting to database: %w", err)
	}
	return pool, nil
}

// Migrate applies every migration that has not run yet. goose records
// applied versions in its own table and takes a PostgreSQL advisory lock, so
// two servers starting together cannot apply the same migration twice.
func Migrate(ctx context.Context, pool *pgxpool.Pool, logger *slog.Logger) error {
	// goose works with database/sql, so we wrap the pgx pool. Closing the
	// wrapper does not close the pool.
	sqlDatabase := stdlib.OpenDBFromPool(pool)
	defer func() { _ = sqlDatabase.Close() }()

	// goose expects the migration files at the root of the file system.
	migrations, err := fs.Sub(migrationFiles, "migrations")
	if err != nil {
		return fmt.Errorf("opening embedded migrations: %w", err)
	}
	locker, err := lock.NewPostgresSessionLocker()
	if err != nil {
		return fmt.Errorf("preparing migration lock: %w", err)
	}
	provider, err := goose.NewProvider(goose.DialectPostgres, sqlDatabase, migrations,
		goose.WithSessionLocker(locker),
	)
	if err != nil {
		return fmt.Errorf("preparing migrations: %w", err)
	}
	results, err := provider.Up(ctx)
	if err != nil {
		return fmt.Errorf("applying migrations: %w", err)
	}
	for _, result := range results {
		logger.InfoContext(ctx, "applied database migration",
			slog.Int64("version", result.Source.Version),
			slog.Duration("duration", result.Duration))
	}
	return nil
}

// InTransaction runs work inside a database transaction. If work returns an
// error the transaction is rolled back, otherwise it is committed. Stores
// pass the transaction to sqlc's Queries via New(transaction).
func InTransaction(ctx context.Context, pool *pgxpool.Pool, work func(transaction pgx.Tx) error) error {
	if err := pgx.BeginFunc(ctx, pool, work); err != nil {
		return fmt.Errorf("database transaction: %w", err)
	}
	return nil
}
