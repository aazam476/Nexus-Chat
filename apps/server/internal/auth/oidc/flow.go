package oidc

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"golang.org/x/oauth2"

	"github.com/aazam476/nexus-chat/apps/server/internal/auth/login"
	"github.com/aazam476/nexus-chat/apps/server/internal/auth/oidc/store"
	"github.com/aazam476/nexus-chat/apps/server/internal/auth/session"
	"github.com/aazam476/nexus-chat/apps/server/internal/platform/clock"
	"github.com/aazam476/nexus-chat/apps/server/internal/registration"
	registrationstore "github.com/aazam476/nexus-chat/apps/server/internal/registration/store"
)

// CallbackPath is where providers send the browser back. Admins register
// the instance's base URL plus this path at the provider.
const CallbackPath = "/api/v1/auth/oidc/callback"

// flowLifetime is how long a user has to sign in at the provider.
const flowLifetime = 10 * time.Minute

// Error codes put in the return address when the provider step fails.
const (
	redirectProviderError = "provider_error"
	redirectSignInFailed  = "sign_in_failed"
)

type flowStore interface {
	InsertFlow(ctx context.Context, arg store.InsertFlowParams) error
	DeleteExpiredFlows(ctx context.Context, now time.Time) error
	GetFlowByState(ctx context.Context, arg store.GetFlowByStateParams) (store.OidcFlow, error)
	RecordCallback(ctx context.Context, arg store.RecordCallbackParams) (int64, error)
	ConsumeFlow(ctx context.Context, arg store.ConsumeFlowParams) (store.OidcFlow, error)
	GetIdentity(ctx context.Context, arg store.GetIdentityParams) (store.OidcIdentity, error)
	InsertIdentity(ctx context.Context, arg store.InsertIdentityParams) error
	ListIdentitiesForUser(ctx context.Context, userID uuid.UUID) ([]store.ListIdentitiesForUserRow, error)
	DeleteIdentity(ctx context.Context, arg store.DeleteIdentityParams) (int64, error)
	CountOtherLoginMethods(ctx context.Context, arg store.CountOtherLoginMethodsParams) (int32, error)
	UsernameExists(ctx context.Context, username string) (bool, error)
}

type providerSource interface {
	Get(ctx context.Context, id string) (Provider, error)
}

type partyFactory interface {
	Connect(ctx context.Context, provider Provider, redirectURL string) (relyingParty, error)
}

type instanceSettings interface {
	BaseURL(ctx context.Context) (string, error)
	RegistrationMode(ctx context.Context) (string, error)
}

type accountCreator interface {
	CreateWithIdentity(ctx context.Context, account registrationstore.NewAccount, identity store.InsertIdentityParams) error
}

type loginStarter interface {
	ContinueLogin(ctx context.Context, userID uuid.UUID, device session.Device) (login.Result, error)
}

type attemptLimiter interface {
	Allow(key string) bool
}

// Dependencies are what SignIn needs.
type Dependencies struct {
	Store     flowStore
	Providers providerSource
	Connector partyFactory
	Settings  instanceSettings
	Accounts  accountCreator
	Logins    loginStarter
	// Limiter bounds starts and completions per client IP.
	Limiter attemptLimiter
	Clock   clock.Clock
	Logger  *slog.Logger
}

// SignIn runs provider sign-ins and links.
type SignIn struct {
	dependencies Dependencies
}

// NewSignIn returns a SignIn.
func NewSignIn(dependencies Dependencies) *SignIn {
	return &SignIn{dependencies: dependencies}
}

// BeginRequest starts a sign-in, or a link when LinkUserID is set.
type BeginRequest struct {
	ProviderID string
	ReturnTo   string
	InviteCode string
	LinkUserID *uuid.UUID
	ClientKey  string
}

// Started is a flow the client continues in a browser.
type Started struct {
	FlowID           uuid.UUID
	AuthorizationURL string
}

// Begin stores a new flow and returns the provider address to open.
func (service *SignIn) Begin(ctx context.Context, request BeginRequest) (Started, error) {
	if !service.dependencies.Limiter.Allow(request.ClientKey) {
		return Started{}, ErrRateLimited
	}
	provider, err := service.dependencies.Providers.Get(ctx, request.ProviderID)
	if err != nil {
		return Started{}, err
	}
	base, err := service.dependencies.Settings.BaseURL(ctx)
	if err != nil {
		return Started{}, err
	}
	returnTo, err := resolveReturnTo(base, request.ReturnTo)
	if err != nil {
		return Started{}, err
	}
	party, err := service.dependencies.Connector.Connect(ctx, provider, base+CallbackPath)
	if err != nil {
		return Started{}, &UnavailableError{Err: err}
	}
	now := service.dependencies.Clock.Now()
	if err := service.dependencies.Store.DeleteExpiredFlows(ctx, now); err != nil {
		return Started{}, fmt.Errorf("removing expired sign-ins: %w", err)
	}
	state, nonce, verifier := randomToken(), randomToken(), oauth2.GenerateVerifier()
	flow := store.InsertFlowParams{
		ID: uuid.Must(uuid.NewV7()), ProviderID: provider.ID, UserID: request.LinkUserID, StateHash: hash(state),
		Nonce: nonce, CodeVerifier: verifier, ReturnTo: returnTo, ExpiresAt: now.Add(flowLifetime),
	}
	if request.InviteCode != "" {
		inviteHash := registration.HashInviteCode(request.InviteCode)
		flow.InviteHash = inviteHash[:]
	}
	if err := service.dependencies.Store.InsertFlow(ctx, flow); err != nil {
		return Started{}, fmt.Errorf("saving sign-in: %w", err)
	}
	return Started{FlowID: flow.ID, AuthorizationURL: party.AuthorizationURL(state, nonce, verifier)}, nil
}

// CallbackRequest is the provider's redirect: a code, or an error.
type CallbackRequest struct {
	State string
	Code  string
	Error string
}

// Callback verifies the provider's answer and returns the address to send
// the browser to: the flow's return address with either the completion
// code or an error code in the fragment.
func (service *SignIn) Callback(ctx context.Context, request CallbackRequest) (string, error) {
	flow, err := service.dependencies.Store.GetFlowByState(ctx, store.GetFlowByStateParams{
		StateHash: hash(request.State), Now: service.dependencies.Clock.Now(),
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrInvalidState
	}
	if err != nil {
		return "", fmt.Errorf("finding sign-in: %w", err)
	}
	if request.Error != "" || request.Code == "" {
		return withFragment(flow.ReturnTo, url.Values{"error": {redirectProviderError}}), nil
	}
	claims, err := service.exchange(ctx, flow, request.Code)
	if err != nil {
		service.dependencies.Logger.WarnContext(ctx, "provider sign-in failed",
			slog.String("provider", flow.ProviderID), slog.Any("error", err))
		return withFragment(flow.ReturnTo, url.Values{"error": {redirectSignInFailed}}), nil
	}
	completion := randomToken()
	encoded, _ := json.Marshal(claims) // Claims holds only strings and a bool
	recorded, err := service.dependencies.Store.RecordCallback(ctx, store.RecordCallbackParams{
		ID: flow.ID, CompletionHash: hash(completion), Claims: encoded,
	})
	if err != nil {
		return "", fmt.Errorf("saving sign-in result: %w", err)
	}
	if recorded == 0 {
		return "", ErrInvalidState // a concurrent callback for the same flow won
	}
	return withFragment(flow.ReturnTo, url.Values{"flow": {flow.ID.String()}, "code": {completion}}), nil
}

// exchange redeems the code and checks the nonce, which ties the ID token
// to this flow (OIDC Core §3.1.3.7 step 11).
func (service *SignIn) exchange(ctx context.Context, flow store.OidcFlow, code string) (Claims, error) {
	provider, err := service.dependencies.Providers.Get(ctx, flow.ProviderID)
	if err != nil {
		return Claims{}, err
	}
	base, err := service.dependencies.Settings.BaseURL(ctx)
	if err != nil {
		return Claims{}, err
	}
	party, err := service.dependencies.Connector.Connect(ctx, provider, base+CallbackPath)
	if err != nil {
		return Claims{}, err
	}
	claims, err := party.Exchange(ctx, code, flow.CodeVerifier)
	if err != nil {
		return Claims{}, err
	}
	if subtle.ConstantTimeCompare([]byte(claims.Nonce), []byte(flow.Nonce)) != 1 {
		return Claims{}, errors.New("the ID token's nonce does not match this sign-in")
	}
	if claims.Subject == "" {
		return Claims{}, errors.New("the ID token has no subject")
	}
	return claims, nil
}

// randomToken returns 256 random bits, URL-safe.
func randomToken() string {
	secret := make([]byte, 32)
	_, _ = rand.Read(secret) // crypto/rand.Read never fails (Go 1.24+)
	return base64.RawURLEncoding.EncodeToString(secret)
}

func hash(token string) []byte {
	sum := sha256.Sum256([]byte(token))
	return sum[:]
}
