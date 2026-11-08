package registration

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/aazam476/nexus-chat/apps/server/internal/accounts"
	accountsstore "github.com/aazam476/nexus-chat/apps/server/internal/accounts/store"
	"github.com/aazam476/nexus-chat/apps/server/internal/auth/session"
	sessionstore "github.com/aazam476/nexus-chat/apps/server/internal/auth/session/store"
	"github.com/aazam476/nexus-chat/apps/server/internal/platform/clock"
	"github.com/aazam476/nexus-chat/apps/server/internal/platform/validation"
	"github.com/aazam476/nexus-chat/apps/server/internal/registration/store"
)

// Errors returned by Register.
var (
	ErrRegistrationClosed = errors.New("this instance is not accepting new accounts")
	ErrInviteRequired     = errors.New("this instance requires an invite code to register")
	ErrInvalidInvite      = errors.New("the invite code is invalid, used up or expired")
	ErrRateLimited        = errors.New("too many registrations from this address; try again later")
)

type accountCreator interface {
	CreateAccount(ctx context.Context, account store.NewAccount) error
}

type accountValidator interface {
	ValidateNewAccount(account accounts.NewAccount) (accounts.NewAccount, error)
}

type passwordHasher interface {
	Hash(ctx context.Context, password string) (string, error)
}

type sessionCreator interface {
	Create(ctx context.Context, userID uuid.UUID, device session.Device) (string, sessionstore.Session, error)
}

type verificationSender interface {
	SendVerification(ctx context.Context, user accountsstore.User) error
}

type modeReader interface {
	RegistrationMode(ctx context.Context) (string, error)
}

type attemptLimiter interface {
	Allow(key string) bool
}

// Dependencies are the collaborators a Service needs.
type Dependencies struct {
	Accounts  accountCreator
	Validator accountValidator
	Hasher    passwordHasher
	Sessions  sessionCreator
	Emails    verificationSender
	Settings  modeReader
	Limiter   attemptLimiter
	Clock     clock.Clock
	Logger    *slog.Logger
}

// Request is one registration.
type Request struct {
	Account    accounts.NewAccount
	InviteCode string
	Device     session.Device
	ClientKey  string
}

// Result is a new, logged-in account.
type Result struct {
	Token   string
	Session sessionstore.Session
	User    accountsstore.User
}

// Service registers accounts.
type Service struct {
	dependencies Dependencies
}

// NewService returns a Service.
func NewService(dependencies Dependencies) *Service {
	return &Service{dependencies: dependencies}
}

// Register creates the account, logs it in, and sends a verification email
// (best effort: an email problem never fails the registration).
func (service *Service) Register(ctx context.Context, request Request) (Result, error) {
	if !service.dependencies.Limiter.Allow(request.ClientKey) {
		return Result{}, ErrRateLimited
	}
	inviteHash, err := service.checkMode(ctx, request.InviteCode)
	if err != nil {
		return Result{}, err
	}
	account, err := service.dependencies.Validator.ValidateNewAccount(request.Account)
	if err != nil {
		return Result{}, err
	}
	user, err := service.create(ctx, account, inviteHash)
	if err != nil {
		return Result{}, err
	}
	token, created, err := service.dependencies.Sessions.Create(ctx, user.ID, request.Device)
	if err != nil {
		return Result{}, err
	}
	if err := service.dependencies.Emails.SendVerification(ctx, user); err != nil {
		service.dependencies.Logger.WarnContext(ctx, "could not send verification email", slog.Any("error", err))
	}
	return Result{Token: token, Session: created, User: user}, nil
}

// checkMode applies the registration mode and returns the invite hash to
// consume (nil when no invite is used).
func (service *Service) checkMode(ctx context.Context, inviteCode string) ([]byte, error) {
	mode, err := service.dependencies.Settings.RegistrationMode(ctx)
	if err != nil {
		return nil, err
	}
	switch {
	case mode == "closed":
		return nil, ErrRegistrationClosed
	case inviteCode != "":
		// An invite is honoured in every open mode, so invited people can
		// join even after an admin switches from open to invite-only.
		hash := HashInviteCode(inviteCode)
		return hash[:], nil
	case mode == "invite_only":
		return nil, ErrInviteRequired
	default:
		return nil, nil
	}
}

func (service *Service) create(ctx context.Context, account accounts.NewAccount, inviteHash []byte) (accountsstore.User, error) {
	hash, err := service.dependencies.Hasher.Hash(ctx, account.Password)
	if err != nil {
		return accountsstore.User{}, err
	}
	now := service.dependencies.Clock.Now().Truncate(time.Microsecond)
	params := accountsstore.InsertUserParams{
		ID: uuid.Must(uuid.NewV7()), Username: account.Username, DisplayName: account.DisplayName,
		Email: account.Email, PasswordHash: &hash, CreatedAt: now,
	}
	err = service.dependencies.Accounts.CreateAccount(ctx, store.NewAccount{User: params, InviteHash: inviteHash, Now: now})
	if err != nil {
		return accountsstore.User{}, translateCreateError(err)
	}
	return accountsstore.User{
		ID: params.ID, Username: params.Username, DisplayName: params.DisplayName,
		Email: params.Email, PasswordHash: params.PasswordHash, CreatedAt: params.CreatedAt,
	}, nil
}

func translateCreateError(err error) error {
	var problems validation.Errors
	switch {
	case errors.Is(err, store.ErrInvalidInvite):
		return ErrInvalidInvite
	case errors.Is(err, store.ErrUsernameTaken):
		problems.Add("username", errors.New("is already taken"))
	case errors.Is(err, store.ErrEmailTaken):
		problems.Add("email", errors.New("already has an account; log in or reset your password"))
	default:
		return err
	}
	return problems.Err()
}
