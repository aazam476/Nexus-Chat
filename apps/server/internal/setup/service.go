package setup

import (
	"context"
	"crypto/sha256"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/aazam476/nexus-chat/apps/server/internal/accounts"
	accountsstore "github.com/aazam476/nexus-chat/apps/server/internal/accounts/store"
	"github.com/aazam476/nexus-chat/apps/server/internal/config"
	"github.com/aazam476/nexus-chat/apps/server/internal/instanceconfig"
	"github.com/aazam476/nexus-chat/apps/server/internal/platform/clock"
	"github.com/aazam476/nexus-chat/apps/server/internal/platform/validation"
	"github.com/aazam476/nexus-chat/apps/server/internal/setup/store"
)

// Errors returned by [Service.Complete].
var (
	ErrInvalidToken    = errors.New("the setup token is not correct")
	ErrAlreadyComplete = errors.New("setup is already complete")
	ErrRateLimited     = errors.New("too many setup attempts; wait a minute and try again")
)

// Registration modes accepted by setup.
var registrationModes = map[string]bool{"open": true, "invite_only": true, "closed": true} //nolint:gochecknoglobals // constant lookup table

const maximumInstanceNameLength = 64

// setupStore is what the service needs from the database.
type setupStore interface {
	IsComplete(ctx context.Context) (bool, error)
	StoredString(ctx context.Context, key string) (string, error)
	Complete(ctx context.Context, completion store.Completion) error
}

// passwordHasher is satisfied by *password.Hasher.
type passwordHasher interface {
	Hash(ctx context.Context, password string) (string, error)
}

// attemptLimiter is satisfied by *ratelimit.Limiter.
type attemptLimiter interface {
	Allow(key string) bool
}

// Dependencies are the collaborators a Service needs.
type Dependencies struct {
	Store   setupStore
	Hasher  passwordHasher
	Limiter attemptLimiter
	Clock   clock.Clock
	Logger  *slog.Logger
	// EnvironmentDomain is DOMAIN from .env; when set, setup cannot change it.
	EnvironmentDomain string
}

// Request is the data the setup wizard submits.
type Request struct {
	Token            string
	Admin            accounts.NewAccount
	InstanceName     string
	Domain           string
	RegistrationMode string
}

// Status describes whether setup is still needed.
type Status struct {
	Required              bool
	Domain                string
	DomainFromEnvironment bool
}

// Service runs first-run setup.
type Service struct {
	dependencies Dependencies
	// mutex guards tokenHash. hasToken is false once setup is complete.
	mutex     sync.Mutex
	tokenHash [sha256.Size]byte
	hasToken  bool
}

// NewService returns a Service. Call Start before serving requests.
func NewService(dependencies Dependencies) *Service {
	return &Service{dependencies: dependencies}
}

// Start creates a setup token and prints it to the log if setup has not
// been completed. Each start creates a new token; earlier ones stop working.
func (service *Service) Start(ctx context.Context) error {
	complete, err := service.dependencies.Store.IsComplete(ctx)
	if err != nil || complete {
		return err
	}
	token, hash := newToken()
	service.mutex.Lock()
	service.tokenHash, service.hasToken = hash, true
	service.mutex.Unlock()
	// Printing the token is the point (spec §3.5): only someone with access
	// to the server's logs can finish setup.
	service.dependencies.Logger.WarnContext(ctx,
		"setup required: open /setup in a browser and enter this one-time setup token",
		slog.String("setup_token", token))
	return nil
}

// Status reports whether setup is still required and which domain is set.
func (service *Service) Status(ctx context.Context) (Status, error) {
	complete, err := service.dependencies.Store.IsComplete(ctx)
	if err != nil {
		return Status{}, err
	}
	status := Status{Required: !complete, Domain: service.dependencies.EnvironmentDomain}
	status.DomainFromEnvironment = status.Domain != ""
	if !status.DomainFromEnvironment {
		status.Domain, err = service.dependencies.Store.StoredString(ctx, instanceconfig.KeyInstanceDomain)
	}
	return status, err
}

// Complete creates the first admin and saves the instance settings.
// clientKey identifies the caller for rate limiting (its IP address).
func (service *Service) Complete(ctx context.Context, request Request, clientKey string) (uuid.UUID, error) {
	if !service.dependencies.Limiter.Allow(clientKey) {
		return uuid.Nil, ErrRateLimited
	}
	if err := service.checkToken(request.Token); err != nil {
		return uuid.Nil, err
	}
	admin, err := service.validate(request)
	if err != nil {
		return uuid.Nil, err
	}
	completion, err := service.completion(ctx, request, admin)
	if err != nil {
		return uuid.Nil, err
	}
	err = service.dependencies.Store.Complete(ctx, completion)
	if errors.Is(err, store.ErrAlreadyComplete) {
		return uuid.Nil, ErrAlreadyComplete
	}
	if err != nil {
		return uuid.Nil, err
	}
	service.mutex.Lock()
	service.hasToken = false
	service.mutex.Unlock()
	service.dependencies.Logger.InfoContext(ctx, "setup complete", slog.String("admin_username", admin.Username))
	return completion.Admin.ID, nil
}

func (service *Service) checkToken(token string) error {
	service.mutex.Lock()
	defer service.mutex.Unlock()
	if !service.hasToken {
		return ErrAlreadyComplete
	}
	if !tokenMatches(token, service.tokenHash) {
		return ErrInvalidToken
	}
	return nil
}

func (service *Service) validate(request Request) (accounts.NewAccount, error) {
	var problems validation.Errors
	admin, err := accounts.ValidateNewAccount(request.Admin)
	var accountProblems *validation.Error
	if errors.As(err, &accountProblems) {
		problems.Merge("admin.", accountProblems)
	}
	problems.Add("instance.name", validateInstanceName(request.InstanceName))
	if service.dependencies.EnvironmentDomain == "" {
		problems.Add("instance.domain", config.ValidateDomain(request.Domain))
	}
	if !registrationModes[request.RegistrationMode] {
		problems.Add("instance.registrationMode", errors.New("must be open, invite_only or closed"))
	}
	return admin, problems.Err()
}

func (service *Service) completion(ctx context.Context, request Request, admin accounts.NewAccount) (store.Completion, error) {
	hash, err := service.dependencies.Hasher.Hash(ctx, admin.Password)
	if err != nil {
		return store.Completion{}, err
	}
	now := service.dependencies.Clock.Now()
	settings := map[string]string{
		instanceconfig.KeyInstanceName:     request.InstanceName,
		instanceconfig.KeyRegistrationMode: request.RegistrationMode,
	}
	if service.dependencies.EnvironmentDomain == "" && request.Domain != "" {
		settings[instanceconfig.KeyInstanceDomain] = request.Domain
	}
	return store.Completion{
		Admin: accountsstore.InsertUserParams{
			ID: uuid.Must(uuid.NewV7()), Username: admin.Username, DisplayName: admin.DisplayName,
			Email: admin.Email, PasswordHash: &hash, IsInstanceAdmin: true, CreatedAt: now,
		},
		Settings:    settings,
		CompletedAt: now.Truncate(time.Microsecond), // PostgreSQL precision
	}, nil
}

func validateInstanceName(name string) error {
	length := utf8.RuneCountInString(name)
	if strings.TrimSpace(name) != name || length < 1 || length > maximumInstanceNameLength ||
		strings.ContainsFunc(name, unicode.IsControl) {
		return errors.New("must be 1–64 characters, without leading or trailing spaces")
	}
	return nil
}
