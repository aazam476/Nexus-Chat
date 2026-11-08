package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"path/filepath"
	"runtime"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/aazam476/nexus-chat/apps/server/internal/accounts"
	"github.com/aazam476/nexus-chat/apps/server/internal/accounts/emails"
	accountsstore "github.com/aazam476/nexus-chat/apps/server/internal/accounts/store"
	"github.com/aazam476/nexus-chat/apps/server/internal/accounts/tokens"
	tokensstore "github.com/aazam476/nexus-chat/apps/server/internal/accounts/tokens/store"
	"github.com/aazam476/nexus-chat/apps/server/internal/auth"
	"github.com/aazam476/nexus-chat/apps/server/internal/auth/login"
	"github.com/aazam476/nexus-chat/apps/server/internal/auth/mfa"
	mfastore "github.com/aazam476/nexus-chat/apps/server/internal/auth/mfa/store"
	"github.com/aazam476/nexus-chat/apps/server/internal/auth/password"
	"github.com/aazam476/nexus-chat/apps/server/internal/auth/password/breached"
	"github.com/aazam476/nexus-chat/apps/server/internal/auth/passwordreset"
	"github.com/aazam476/nexus-chat/apps/server/internal/auth/session"
	sessionstore "github.com/aazam476/nexus-chat/apps/server/internal/auth/session/store"
	"github.com/aazam476/nexus-chat/apps/server/internal/config"
	"github.com/aazam476/nexus-chat/apps/server/internal/health"
	"github.com/aazam476/nexus-chat/apps/server/internal/httpserver"
	"github.com/aazam476/nexus-chat/apps/server/internal/instanceconfig"
	instanceconfigstore "github.com/aazam476/nexus-chat/apps/server/internal/instanceconfig/store"
	"github.com/aazam476/nexus-chat/apps/server/internal/platform/clock"
	"github.com/aazam476/nexus-chat/apps/server/internal/platform/database"
	"github.com/aazam476/nexus-chat/apps/server/internal/platform/mail"
	"github.com/aazam476/nexus-chat/apps/server/internal/platform/ratelimit"
	"github.com/aazam476/nexus-chat/apps/server/internal/registration"
	registrationstore "github.com/aazam476/nexus-chat/apps/server/internal/registration/store"
	"github.com/aazam476/nexus-chat/apps/server/internal/secrets"
	"github.com/aazam476/nexus-chat/apps/server/internal/setup"
	setupstore "github.com/aazam476/nexus-chat/apps/server/internal/setup/store"
	"github.com/aazam476/nexus-chat/apps/server/internal/tlsconfig"
	"github.com/aazam476/nexus-chat/apps/server/internal/voice"
)

// shutdownTimeout is how long in-flight requests get to finish after a stop
// signal. Docker waits 10 s by default before killing the container, so we
// stay below that.
const shutdownTimeout = 8 * time.Second

// certificateCheckInterval is how often the HTTPS certificate files are
// checked for changes and the certificate for upcoming expiry.
const certificateCheckInterval = 30 * time.Second

// application holds every long-lived dependency of the running server.
type application struct {
	logger       *slog.Logger
	pool         *pgxpool.Pool
	secrets      secrets.Secrets
	server       *http.Server
	secureServer *http.Server
	certificates *tlsconfig.Manager
	sessions     *session.Service
}

// newApplication prepares everything the server needs before it accepts
// traffic: secrets, database connection, migrations and the HTTP handler.
func newApplication(ctx context.Context, configuration config.Config, logger *slog.Logger) (*application, error) {
	instanceSecrets, err := secrets.LoadOrCreate(secrets.DirectoryIn(configuration.DataDirectory))
	if err != nil {
		return nil, fmt.Errorf("loading instance secrets: %w", err)
	}
	if err := voice.WriteMediaConfig(configuration.DataDirectory, voice.MediaSecrets{
		LiveKitAPIKey: instanceSecrets.LiveKitAPIKey, LiveKitAPISecret: instanceSecrets.LiveKitAPISecret,
		TURNSecret: instanceSecrets.TURNSecret,
	}); err != nil {
		return nil, fmt.Errorf("writing media server configuration: %w", err)
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
	handler := httpserver.NewHandler(features, httpserver.Options{
		Logger: logger, TrustedProxies: configuration.TrustedProxies, Authenticator: app.sessions,
	})
	app.server = httpserver.NewServer("", handler, logger)
	app.secureServer = httpserver.NewServer("", handler, logger)
	app.certificates = app.newCertificateManager(configuration)
	app.secureServer.TLSConfig = app.certificates.TLSConfig()
	if err := app.certificates.Reload(ctx); err != nil {
		// A bad certificate must not keep the instance offline: plain HTTP
		// keeps working and the admin sees the reason in the log.
		logger.ErrorContext(ctx, "HTTPS is off: the installed certificate cannot be used", slog.Any("error", err))
	}
	return app, nil
}

// newCertificateManager watches DATA_DIR/tls for the HTTPS certificate.
func (app *application) newCertificateManager(configuration config.Config) *tlsconfig.Manager {
	settings := setupstore.New(app.pool)
	return tlsconfig.NewManager(tlsconfig.ManagerOptions{
		Directory: filepath.Join(configuration.DataDirectory, "tls"),
		Domain: func(ctx context.Context) (string, error) {
			if configuration.Domain != "" {
				return configuration.Domain, nil
			}
			return settings.StoredString(ctx, instanceconfig.KeyInstanceDomain)
		},
		Clock:  clock.System{},
		Logger: app.logger,
	})
}

// buildFeatures creates every feature's service and handler.
func (app *application) buildFeatures(ctx context.Context, configuration config.Config) (httpserver.Features, error) {
	// One hash per CPU at a time keeps memory bounded under a login flood
	// (each hash uses 64 MiB).
	hasher, err := password.NewHasher(password.DefaultParams(), runtime.NumCPU())
	if err != nil {
		return httpserver.Features{}, err
	}
	breachedPasswords, err := breached.Load()
	if err != nil {
		return httpserver.Features{}, err
	}
	validator := accounts.NewValidator(breachedPasswords)
	setupService := setup.NewService(setup.Dependencies{
		Store:             setupstore.New(app.pool),
		Validator:         validator,
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
	app.sessions = session.NewService(sessionstore.New(app.pool), clock.System{})
	// Authenticator apps show "Nexus Chat" next to the code. The instance
	// name can change later, which would not update apps already set up.
	secondFactors, err := mfa.NewService(mfastore.New(app.pool), app.secrets.TOTPKey, clock.System{}, "Nexus Chat")
	if err != nil {
		return httpserver.Features{}, err
	}
	authFeatures := app.buildAuthFeatures(authInputs{hasher: hasher, validator: validator, domain: configuration.Domain, secondFactors: secondFactors})
	authFeatures.Health = health.NewHandler(healthService)
	authFeatures.Setup = setup.NewHandler(setupService)
	return authFeatures, nil
}

// authInputs are the shared pieces the account features are built from.
type authInputs struct {
	hasher        *password.Hasher
	validator     *accounts.Validator
	domain        string
	secondFactors *mfa.Service
}

// buildAuthFeatures creates login, registration, verification and
// password reset. Each limiter is per client IP.
func (app *application) buildAuthFeatures(inputs authInputs) httpserver.Features {
	system := clock.System{}
	accountQueries := accountsstore.New(app.pool)
	settings := instanceconfig.NewService(instanceconfigstore.New(app.pool), system, inputs.domain)
	accountTokens := tokens.NewService(tokensstore.New(app.pool), system)
	mailer := mail.NewConfigured(func(ctx context.Context) (mail.SMTPConfig, bool, error) {
		var smtp mail.SMTPConfig
		found, err := settings.Load(ctx, instanceconfig.KeySMTP, &smtp)
		return smtp, found, err
	}, system)
	accountEmails := emails.NewService(accountTokens, mailer, settings)
	logins := login.NewService(login.Dependencies{
		Accounts: accountQueries, Hasher: inputs.hasher, Sessions: app.sessions,
		Limiter: ratelimit.New(ratelimit.PerMinute(20, 10), system), Clock: system,
		SecondFactors: inputs.secondFactors, Challenges: accountTokens,
	})
	resets := passwordreset.NewService(passwordreset.Dependencies{
		Accounts: accountQueries, Sessions: sessionstore.New(app.pool), Tokens: accountTokens, Mailer: accountEmails,
		Validator: inputs.validator, Hasher: inputs.hasher, Limiter: ratelimit.New(ratelimit.PerMinute(5, 3), system),
		Clock: system, Logger: app.logger,
	})
	registrations := registration.NewService(registration.Dependencies{
		Accounts: registrationstore.NewRegistrar(app.pool), Validator: inputs.validator, Hasher: inputs.hasher,
		Sessions: app.sessions, Emails: accountEmails, Settings: settings,
		Limiter: ratelimit.New(ratelimit.PerMinute(5, 5), system), Clock: system, Logger: app.logger,
	})
	return httpserver.Features{
		Accounts: accounts.NewHandler(accountQueries),
		Auth: auth.NewHandler(auth.HandlerDependencies{
			Logins: logins, Sessions: app.sessions, Resets: resets,
			Verifier: emails.NewVerifier(accountTokens, accountQueries, system), Emails: accountEmails, Users: accountQueries,
			MFA: inputs.secondFactors,
		}),
		Registration: registration.NewHandler(registrations, registration.NewInvites(registrationstore.New(app.pool), system), accountQueries),
	}
}

func (app *application) prepareDatabase(ctx context.Context) error {
	if err := database.Migrate(ctx, app.pool, app.logger); err != nil {
		return err
	}
	settings := instanceconfig.NewService(instanceconfigstore.New(app.pool), clock.System{}, "")
	firstBoot, err := settings.RecordFirstBoot(ctx)
	if err != nil {
		return err
	}
	app.logger.InfoContext(ctx, "instance ready", slog.Time("first_boot_at", firstBoot))
	return nil
}

// listeners are the open sockets the application serves on.
type listeners struct {
	plain  net.Listener
	secure net.Listener
}

// serve handles requests on both listeners until ctx is cancelled or one of
// the servers fails, then shuts both down gracefully.
func (app *application) serve(ctx context.Context, sockets listeners) error {
	serveErrors := make(chan error, 2)
	// Each goroutine ends when its Serve call returns, which happens when
	// shutdown below runs or the listener fails.
	go func() { serveErrors <- app.server.Serve(sockets.plain) }()
	go func() { serveErrors <- app.secureServer.ServeTLS(sockets.secure, "", "") }()
	// Watch stops when ctx is cancelled.
	go app.certificates.Watch(ctx, certificateCheckInterval)

	var failure error
	select {
	case failure = <-serveErrors:
		failure = fmt.Errorf("serving HTTP: %w", failure)
	case <-ctx.Done():
		app.logger.Info("shutting down")
	}
	return errors.Join(failure, app.shutdown(ctx, serveErrors, failure != nil))
}

// shutdown stops both servers and waits for their Serve calls to return.
// One of them has already returned when a server failed.
func (app *application) shutdown(ctx context.Context, serveErrors chan error, oneAlreadyStopped bool) error {
	// ctx may already be cancelled, so shutdown gets a fresh deadline that
	// keeps ctx's values but not its cancellation.
	shutdownContext, cancel := context.WithTimeout(context.WithoutCancel(ctx), shutdownTimeout)
	defer cancel()
	result := errors.Join(app.server.Shutdown(shutdownContext), app.secureServer.Shutdown(shutdownContext))
	remaining := 2
	if oneAlreadyStopped {
		remaining = 1
	}
	for range remaining {
		if err := <-serveErrors; !errors.Is(err, http.ErrServerClosed) {
			result = errors.Join(result, fmt.Errorf("serving HTTP: %w", err))
		}
	}
	if result != nil {
		return fmt.Errorf("shutting down: %w", result)
	}
	return nil
}

// close releases resources. It is safe to call on a partly built application.
func (app *application) close() {
	app.pool.Close()
}
