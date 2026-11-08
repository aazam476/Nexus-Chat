package passkeys

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	accountsstore "github.com/aazam476/nexus-chat/apps/server/internal/accounts/store"
	"github.com/aazam476/nexus-chat/apps/server/internal/auth/passkeys/store"
	"github.com/aazam476/nexus-chat/apps/server/internal/platform/clock"
)

// ceremonyLifetime is how long the browser has to answer a challenge.
const ceremonyLifetime = 5 * time.Minute

// Ceremony purposes. A challenge issued for one never completes another.
const (
	purposeRegisterPasskey     = "register_passkey"
	purposeRegisterSecurityKey = "register_security_key"
	purposeSecondFactor        = "second_factor"
	purposePasskeyLogin        = "passkey_login"
)

// Errors returned by the ceremonies.
var (
	ErrNoDomain          = errors.New("passkeys and security keys need the instance's domain to be set, and HTTPS")
	ErrCeremonyNotFound  = errors.New("this request has expired or was already used; start again")
	ErrVerificationFails = errors.New("the passkey or security key could not be verified")
	ErrNoCredentials     = errors.New("this account has no security key or passkey")
	ErrNotFound          = errors.New("no passkey or security key with this ID")
)

type credentialStore interface {
	InsertCredential(ctx context.Context, arg store.InsertCredentialParams) error
	ListCredentials(ctx context.Context, userID uuid.UUID) ([]store.WebauthnCredential, error)
	UpdateCredentialAfterUse(ctx context.Context, arg store.UpdateCredentialAfterUseParams) error
	DeleteCredential(ctx context.Context, arg store.DeleteCredentialParams) (int64, error)
	InsertCeremony(ctx context.Context, arg store.InsertCeremonyParams) error
	TakeCeremony(ctx context.Context, arg store.TakeCeremonyParams) (store.WebauthnCeremony, error)
}

type userReader interface {
	GetUserByID(ctx context.Context, id uuid.UUID) (accountsstore.User, error)
}

type domainSource interface {
	Domain(ctx context.Context) (string, error)
}

// relyingParty is the part of *webauthn.WebAuthn the ceremonies use. The
// interface lets tests replace the library when they need it to fail.
type relyingParty interface {
	BeginRegistration(user webauthn.User, opts ...webauthn.RegistrationOption) (*protocol.CredentialCreation, *webauthn.SessionData, error)
	CreateCredential(user webauthn.User, session webauthn.SessionData, parsed *protocol.ParsedCredentialCreationData) (*webauthn.Credential, error)
	BeginLogin(user webauthn.User, opts ...webauthn.LoginOption) (*protocol.CredentialAssertion, *webauthn.SessionData, error)
	ValidateLogin(user webauthn.User, session webauthn.SessionData, parsed *protocol.ParsedCredentialAssertionData) (*webauthn.Credential, error)
	BeginDiscoverableLogin(opts ...webauthn.LoginOption) (*protocol.CredentialAssertion, *webauthn.SessionData, error)
	ValidatePasskeyLogin(handler webauthn.DiscoverableUserHandler, session webauthn.SessionData, parsed *protocol.ParsedCredentialAssertionData) (webauthn.User, *webauthn.Credential, error)
}

// newLibraryParty is the production relyingParty: the go-webauthn library.
func newLibraryParty(config *webauthn.Config) (relyingParty, error) {
	party, err := webauthn.New(config)
	if err != nil {
		return nil, fmt.Errorf("invalid WebAuthn configuration: %w", err)
	}
	return party, nil
}

// Dependencies are the collaborators a Service needs.
type Dependencies struct {
	Store  credentialStore
	Users  userReader
	Domain domainSource
	Clock  clock.Clock
	// newParty builds the relying party; nil means the go-webauthn library.
	// Only tests set it.
	newParty func(config *webauthn.Config) (relyingParty, error)
}

// Service runs WebAuthn ceremonies.
type Service struct {
	dependencies Dependencies
}

// NewService returns a Service.
func NewService(dependencies Dependencies) *Service {
	if dependencies.newParty == nil {
		dependencies.newParty = newLibraryParty
	}
	return &Service{dependencies: dependencies}
}

// relyingParty configures WebAuthn for the instance's current domain. It
// is built per ceremony, so a domain change applies immediately.
func (service *Service) relyingParty(ctx context.Context) (relyingParty, error) {
	domain, err := service.dependencies.Domain.Domain(ctx)
	if err != nil {
		return nil, err
	}
	if domain == "" {
		return nil, ErrNoDomain
	}
	party, err := service.dependencies.newParty(&webauthn.Config{
		RPID: domain, RPDisplayName: "Nexus Chat", RPOrigins: []string{"https://" + domain},
	})
	if err != nil {
		return nil, fmt.Errorf("configuring WebAuthn for %s: %w", domain, err)
	}
	return party, nil
}

// loadAccount returns the user with their credentials.
func (service *Service) loadAccount(ctx context.Context, userID uuid.UUID) (account, error) {
	user, err := service.dependencies.Users.GetUserByID(ctx, userID)
	if err != nil {
		return account{}, fmt.Errorf("loading account: %w", err)
	}
	rows, err := service.dependencies.Store.ListCredentials(ctx, userID)
	if err != nil {
		return account{}, fmt.Errorf("loading credentials: %w", err)
	}
	credentials, err := decodeCredentials(rows)
	if err != nil {
		return account{}, err
	}
	return account{id: user.ID, name: user.Username, displayName: user.DisplayName, credentials: credentials}, nil
}

// ceremony is a stored challenge.
type ceremony struct {
	purpose string
	userID  *uuid.UUID
	name    string
	session *webauthn.SessionData
}

// encodeJSON encodes the library's own structs (session data, options,
// credentials). They contain only strings, numbers, byte slices and
// nested structs, which encoding/json always handles, so Marshal cannot
// fail for them and its error is not checked.
func encodeJSON(value any) []byte {
	encoded, _ := json.Marshal(value)
	return encoded
}

func (service *Service) saveCeremony(ctx context.Context, pending ceremony) (uuid.UUID, error) {
	id := uuid.Must(uuid.NewV7())
	err := service.dependencies.Store.InsertCeremony(ctx, store.InsertCeremonyParams{
		ID: id, UserID: pending.userID, Purpose: pending.purpose, Session: encodeJSON(pending.session), Name: pending.name,
		ExpiresAt: service.dependencies.Clock.Now().Add(ceremonyLifetime),
	})
	if err != nil {
		return uuid.Nil, fmt.Errorf("saving WebAuthn session: %w", err)
	}
	return id, nil
}

func (service *Service) takeCeremony(ctx context.Context, id uuid.UUID, purpose string) (store.WebauthnCeremony, webauthn.SessionData, error) {
	row, err := service.dependencies.Store.TakeCeremony(ctx, store.TakeCeremonyParams{ID: id, Purpose: purpose, Now: service.dependencies.Clock.Now()})
	if errors.Is(err, pgx.ErrNoRows) {
		return store.WebauthnCeremony{}, webauthn.SessionData{}, ErrCeremonyNotFound
	}
	if err != nil {
		return store.WebauthnCeremony{}, webauthn.SessionData{}, fmt.Errorf("loading WebAuthn session: %w", err)
	}
	var session webauthn.SessionData
	if err := json.Unmarshal(row.Session, &session); err != nil {
		return store.WebauthnCeremony{}, webauthn.SessionData{}, fmt.Errorf("decoding WebAuthn session: %w", err)
	}
	return row, session, nil
}
