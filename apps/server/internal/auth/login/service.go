package login

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	accountsstore "github.com/aazam476/nexus-chat/apps/server/internal/accounts/store"
	"github.com/aazam476/nexus-chat/apps/server/internal/accounts/tokens"
	"github.com/aazam476/nexus-chat/apps/server/internal/auth/session"
	sessionstore "github.com/aazam476/nexus-chat/apps/server/internal/auth/session/store"
	"github.com/aazam476/nexus-chat/apps/server/internal/platform/clock"
)

// Progressive delay settings.
const (
	freeAttempts = 4
	firstDelay   = time.Second
	maximumDelay = 15 * time.Minute
)

// dummyHash is verified when the account does not exist, so a wrong
// username costs the same time as a wrong password. It is a valid Argon2id
// hash with production parameters of a random password nobody knows.
const dummyHash = "$argon2id$v=19$m=65536,t=3,p=1$QwMRQhvEnkmyFLkAD38mOA$4Lg4v6dJpEjBsm5LGZcTZ3XhmO0r2r4e7c2C6M1PZmo"

// Errors returned by Login.
var (
	ErrInvalidCredentials = errors.New("the username or password is not correct")
	ErrRateLimited        = errors.New("too many login attempts from this address; wait a minute")
)

// DelayedError means the account is temporarily locked after repeated
// wrong passwords.
type DelayedError struct {
	RetryAfter time.Duration
}

func (delayed *DelayedError) Error() string {
	return fmt.Sprintf("too many wrong passwords; try again in %s", delayed.RetryAfter.Round(time.Second))
}

type accountStore interface {
	GetUserByLogin(ctx context.Context, login string) (accountsstore.User, error)
	GetUserByID(ctx context.Context, id uuid.UUID) (accountsstore.User, error)
	RecordFailedLogin(ctx context.Context, arg accountsstore.RecordFailedLoginParams) error
	ResetFailedLogins(ctx context.Context, id uuid.UUID) error
	UpdatePasswordHash(ctx context.Context, arg accountsstore.UpdatePasswordHashParams) error
}

type passwordHasher interface {
	Hash(ctx context.Context, password string) (string, error)
	Verify(ctx context.Context, password string, encoded string) (bool, error)
	NeedsRehash(encoded string) bool
}

type sessionCreator interface {
	Create(ctx context.Context, userID uuid.UUID, device session.Device) (string, sessionstore.Session, error)
}

type attemptLimiter interface {
	Allow(key string) bool
}

// secondFactors is satisfied by *mfa.Service.
type secondFactors interface {
	HasSecondFactor(ctx context.Context, userID uuid.UUID) (bool, error)
	VerifyTOTP(ctx context.Context, userID uuid.UUID, code string) (bool, error)
	UseBackupCode(ctx context.Context, userID uuid.UUID, code string) (bool, error)
	Methods(ctx context.Context, userID uuid.UUID) ([]string, error)
}

// challengeTokens is satisfied by *tokens.Service.
type challengeTokens interface {
	Issue(ctx context.Context, userID uuid.UUID, purpose tokens.Purpose) (string, error)
	Redeem(ctx context.Context, purpose tokens.Purpose, token string) (uuid.UUID, error)
}

// Dependencies are the collaborators a Service needs.
type Dependencies struct {
	Accounts      accountStore
	Hasher        passwordHasher
	Sessions      sessionCreator
	Limiter       attemptLimiter
	Clock         clock.Clock
	SecondFactors secondFactors
	Challenges    challengeTokens
}

// Request is one login attempt.
type Request struct {
	Login     string
	Password  string
	Device    session.Device
	ClientKey string // the client's IP address, for rate limiting
}

// Result is a successful login. If MFAChallenge is set, the password was
// right but the account has a second factor: no session exists yet, and the
// client must finish with one of MFAMethods.
type Result struct {
	Token        string
	Session      sessionstore.Session
	User         accountsstore.User
	MFAChallenge string
	MFAMethods   []string
}

// Service performs logins.
type Service struct {
	dependencies Dependencies
}

// NewService returns a Service.
func NewService(dependencies Dependencies) *Service {
	return &Service{dependencies: dependencies}
}

// Login checks the credentials and starts a session, or returns an MFA
// challenge when the account has a second factor.
func (service *Service) Login(ctx context.Context, request Request) (Result, error) {
	user, err := service.authenticate(ctx, request)
	if err != nil {
		return Result{}, err
	}
	needsSecondFactor, err := service.dependencies.SecondFactors.HasSecondFactor(ctx, user.ID)
	if err != nil {
		return Result{}, err
	}
	if !needsSecondFactor {
		return service.startSession(ctx, user, request.Device)
	}
	methods, err := service.dependencies.SecondFactors.Methods(ctx, user.ID)
	if err != nil {
		return Result{}, err
	}
	challenge, err := service.dependencies.Challenges.Issue(ctx, user.ID, tokens.MFALogin)
	if err != nil {
		return Result{}, err
	}
	return Result{MFAChallenge: challenge, MFAMethods: methods}, nil
}

// authenticate applies the rate limit and lockout and checks the password.
func (service *Service) authenticate(ctx context.Context, request Request) (accountsstore.User, error) {
	if !service.dependencies.Limiter.Allow(request.ClientKey) {
		return accountsstore.User{}, ErrRateLimited
	}
	user, err := service.dependencies.Accounts.GetUserByLogin(ctx, request.Login)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && user.PasswordHash == nil) {
		_, _ = service.dependencies.Hasher.Verify(ctx, request.Password, dummyHash) // equalize timing only
		return accountsstore.User{}, ErrInvalidCredentials
	}
	if err != nil {
		return accountsstore.User{}, fmt.Errorf("looking up account: %w", err)
	}
	now := service.dependencies.Clock.Now()
	if user.LoginBlockedUntil != nil && now.Before(*user.LoginBlockedUntil) {
		return accountsstore.User{}, &DelayedError{RetryAfter: user.LoginBlockedUntil.Sub(now)}
	}
	if err := service.checkPassword(ctx, user, request.Password, now); err != nil {
		return accountsstore.User{}, err
	}
	return user, nil
}

func (service *Service) startSession(ctx context.Context, user accountsstore.User, device session.Device) (Result, error) {
	token, created, err := service.dependencies.Sessions.Create(ctx, user.ID, device)
	if err != nil {
		return Result{}, err
	}
	return Result{Token: token, Session: created, User: user}, nil
}

// checkPassword verifies the password, records failures, and on success
// clears the failure count and upgrades an outdated hash.
func (service *Service) checkPassword(ctx context.Context, user accountsstore.User, password string, now time.Time) error {
	accounts := service.dependencies.Accounts
	match, err := service.dependencies.Hasher.Verify(ctx, password, *user.PasswordHash)
	if err != nil {
		return fmt.Errorf("verifying password: %w", err)
	}
	if !match {
		blockedUntil := blockAfterFailure(user.FailedLoginCount+1, now)
		if err := accounts.RecordFailedLogin(ctx, accountsstore.RecordFailedLoginParams{ID: user.ID, LoginBlockedUntil: blockedUntil}); err != nil {
			return fmt.Errorf("recording failed login: %w", err)
		}
		return ErrInvalidCredentials
	}
	if user.FailedLoginCount > 0 {
		if err := accounts.ResetFailedLogins(ctx, user.ID); err != nil {
			return fmt.Errorf("resetting failed logins: %w", err)
		}
	}
	return service.upgradeHash(ctx, user, password)
}

func (service *Service) upgradeHash(ctx context.Context, user accountsstore.User, password string) error {
	if !service.dependencies.Hasher.NeedsRehash(*user.PasswordHash) {
		return nil
	}
	hash, err := service.dependencies.Hasher.Hash(ctx, password)
	if err != nil {
		return fmt.Errorf("upgrading password hash: %w", err)
	}
	if err := service.dependencies.Accounts.UpdatePasswordHash(ctx, accountsstore.UpdatePasswordHashParams{ID: user.ID, PasswordHash: &hash}); err != nil {
		return fmt.Errorf("saving upgraded password hash: %w", err)
	}
	return nil
}

// blockAfterFailure returns until when the account is locked after its
// failures-th consecutive wrong password, or nil if it is not locked yet.
// Failures 1–4 are free; the 5th locks for 1 s, the 6th 2 s, then 4 s, 8 s,
// and so on up to 15 minutes.
func blockAfterFailure(failures int32, now time.Time) *time.Time {
	if failures <= freeAttempts {
		return nil
	}
	delay := maximumDelay
	if exponent := failures - freeAttempts - 1; exponent < 20 {
		delay = min(firstDelay<<exponent, maximumDelay)
	}
	until := now.Add(delay)
	return &until
}
