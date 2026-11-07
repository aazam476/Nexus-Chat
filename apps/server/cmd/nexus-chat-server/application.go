package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"runtime"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/aazam476/nexus-chat/apps/server/internal/auth/password"
	"github.com/aazam476/nexus-chat/apps/server/internal/config"
	"github.com/aazam476/nexus-chat/apps/server/internal/health"
	"github.com/aazam476/nexus-chat/apps/server/internal/httpserver"
	"github.com/aazam476/nexus-chat/apps/server/internal/instanceconfig"
	instanceconfigstore "github.com/aazam476/nexus-chat/apps/server/internal/instanceconfig/store"
	"github.com/aazam476/nexus-chat/apps/server/internal/platform/clock"
	"github.com/aazam476/nexus-chat/apps/server/internal/platform/database"
	"github.com/aazam476/nexus-chat/apps/server/internal/platform/ratelimit"
	"github.com/aazam476/nexus-chat/apps/server/internal/secrets"
	"github.com/aazam476/nexus-chat/apps/server/internal/setup"
	setupstore "github.com/aazam476/nexus-chat/apps/server/internal/setup/store"
)

// shutdownTimeout is how long in-flight requests get to finish after a stop
// signal. Docker waits 10 s by default before killing the container, so we
// stay below that.
const shutdownTimeout = 8 * time.Second

// application holds every long-lived dependency of the running server.
type application struct {
	logger  *slog.Logger
	pool    *pgxpool.Pool
	secrets secrets.Secrets
	server  *http.Server
}

// newApplication prepares everything the server needs before it accepts
// traffic: secrets, database connection, migrations and the HTTP handler.
func newApplication(ctx context.Context, configuration config.Config, logger *slog.Logger) (*application, error) {
	instanceSecrets, err := secrets.LoadOrCreate(secrets.DirectoryIn(configuration.DataDirectory))
	if err != nil {
		return nil, fmt.Errorf("loading instance secrets: %w", err)
	}
	pool, err := database.Open(ctx, configuration.DatabaseURL)
	if err != nil {
		return nil, err
	}
	app := &application{logger: logger, pool: pool, secrets: instanceSecrets}
	if err := app.prepareDatabase(ctx); err != nil {
		app.close()
		return nil, err
	}

	features, err := app.buildFeatures(ctx, configuration)
	if err != nil {
		app.close()
		return nil, err
	}
	handler := httpserver.NewHandler(features, httpserver.Options{Logger: logger, TrustedProxies: configuration.TrustedProxies})
	app.server = httpserver.NewServer("", handler, logger)
	return app, nil
}

// buildFeatures creates every feature's service and handler.
func (app *application) buildFeatures(ctx context.Context, configuration config.Config) (httpserver.Features, error) {
	// One hash per CPU at a time keeps memory bounded under a login flood
	// (each hash uses 64 MiB).
	hasher, err := password.NewHasher(password.DefaultParams(), runtime.NumCPU())
	if err != nil {
		return httpserver.Features{}, err
	}
	setupService := setup.NewService(setup.Dependencies{
		Store:             setupstore.New(app.pool),
		Hasher:            hasher,
		Limiter:           ratelimit.New(ratelimit.PerMinute(10, 5), clock.System{}),
		Clock:             clock.System{},
		Logger:            app.logger,
		EnvironmentDomain: configuration.Domain,
	})
	if err := setupService.Start(ctx); err != nil {
		return httpserver.Features{}, err
	}
	healthService := health.NewService([]health.NamedChecker{{Name: "database", Checker: app.pool}})
	return httpserver.Features{
		Health: health.NewHandler(healthService),
		Setup:  setup.NewHandler(setupService),
	}, nil
}

func (app *application) prepareDatabase(ctx context.Context) error {
	if err := database.Migrate(ctx, app.pool, app.logger); err != nil {
		return err
	}
	settings := instanceconfig.NewService(instanceconfigstore.New(app.pool), clock.System{})
	firstBoot, err := settings.RecordFirstBoot(ctx)
	if err != nil {
		return err
	}
	app.logger.InfoContext(ctx, "instance ready", slog.Time("first_boot_at", firstBoot))
	return nil
}

// serve handles HTTP requests on listener until ctx is cancelled, then shuts
// down gracefully.
func (app *application) serve(ctx context.Context, listener net.Listener) error {
	serveErrors := make(chan error, 1)
	// This goroutine ends when Serve returns, which happens when Shutdown
	// below is called or the listener fails.
	go func() { serveErrors <- app.server.Serve(listener) }()

	select {
	case err := <-serveErrors:
		return fmt.Errorf("serving HTTP: %w", err)
	case <-ctx.Done():
	}

	app.logger.Info("shutting down")
	// ctx is already cancelled here, so shutdown gets a fresh deadline that
	// keeps ctx's values but not its cancellation.
	shutdownContext, cancel := context.WithTimeout(context.WithoutCancel(ctx), shutdownTimeout)
	defer cancel()
	if err := app.server.Shutdown(shutdownContext); err != nil {
		return fmt.Errorf("shutting down HTTP server: %w", err)
	}
	if err := <-serveErrors; !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("serving HTTP: %w", err)
	}
	return nil
}

// close releases resources. It is safe to call on a partly built application.
func (app *application) close() {
	app.pool.Close()
}
