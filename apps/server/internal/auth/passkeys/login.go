package passkeys

import (
	"context"
	"fmt"
	"time"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/google/uuid"

	"github.com/aazam476/nexus-chat/apps/server/internal/auth/passkeys/store"
)

// Credential is a registered security key or passkey, as listed in
// account settings.
type Credential struct {
	ID         uuid.UUID
	Kind       string
	Name       string
	CreatedAt  time.Time
	LastUsedAt *time.Time
}

// BeginSecondFactor starts a security-key check for a user who already
// passed the password step. Any of the user's credentials is accepted.
func (service *Service) BeginSecondFactor(ctx context.Context, userID uuid.UUID) (Started, error) {
	party, err := service.relyingParty(ctx)
	if err != nil {
		return Started{}, err
	}
	user, err := service.loadAccount(ctx, userID)
	if err != nil {
		return Started{}, err
	}
	if len(user.credentials) == 0 {
		return Started{}, ErrNoCredentials
	}
	assertion, session, err := party.BeginLogin(user)
	if err != nil {
		return Started{}, fmt.Errorf("starting WebAuthn login: %w", err)
	}
	return service.start(ctx, ceremony{purpose: purposeSecondFactor, userID: &userID, session: session}, assertion)
}

// FinishSecondFactor checks the security key's answer and returns the user.
func (service *Service) FinishSecondFactor(ctx context.Context, ceremonyID uuid.UUID, response []byte) (uuid.UUID, error) {
	row, session, err := service.takeCeremony(ctx, ceremonyID, purposeSecondFactor)
	if err != nil {
		return uuid.Nil, err
	}
	parsed, party, err := service.parseAssertion(ctx, response)
	if err != nil {
		return uuid.Nil, err
	}
	user, err := service.loadAccount(ctx, *row.UserID)
	if err != nil {
		return uuid.Nil, err
	}
	credential, err := party.ValidateLogin(user, session, parsed)
	if err != nil {
		return uuid.Nil, fmt.Errorf("%w: %w", ErrVerificationFails, err)
	}
	return user.id, service.recordUse(ctx, credential)
}

// BeginPasskeyLogin starts a passwordless login. The browser offers the
// passkeys it holds for this domain; the server does not know the user yet.
func (service *Service) BeginPasskeyLogin(ctx context.Context) (Started, error) {
	party, err := service.relyingParty(ctx)
	if err != nil {
		return Started{}, err
	}
	assertion, session, err := party.BeginDiscoverableLogin(webauthn.WithUserVerification(protocol.VerificationRequired))
	if err != nil {
		return Started{}, fmt.Errorf("starting passkey login: %w", err)
	}
	return service.start(ctx, ceremony{purpose: purposePasskeyLogin, session: session}, assertion)
}

// FinishPasskeyLogin checks the passkey's answer and returns its owner.
func (service *Service) FinishPasskeyLogin(ctx context.Context, ceremonyID uuid.UUID, response []byte) (uuid.UUID, error) {
	_, session, err := service.takeCeremony(ctx, ceremonyID, purposePasskeyLogin)
	if err != nil {
		return uuid.Nil, err
	}
	parsed, party, err := service.parseAssertion(ctx, response)
	if err != nil {
		return uuid.Nil, err
	}
	// The authenticator says whose passkey it is (the user handle, which is
	// the account's UUID); the library then checks the signature against
	// that account's stored credentials.
	findOwner := func(_ []byte, userHandle []byte) (webauthn.User, error) {
		userID, err := uuid.FromBytes(userHandle)
		if err != nil {
			return nil, fmt.Errorf("unknown user handle: %w", err)
		}
		return service.loadAccount(ctx, userID)
	}
	owner, credential, err := party.ValidatePasskeyLogin(findOwner, session, parsed)
	if err != nil {
		return uuid.Nil, fmt.Errorf("%w: %w", ErrVerificationFails, err)
	}
	userID, _ := uuid.FromBytes(owner.WebAuthnID()) // findOwner already parsed it
	return userID, service.recordUse(ctx, credential)
}

func (service *Service) parseAssertion(ctx context.Context, response []byte) (*protocol.ParsedCredentialAssertionData, relyingParty, error) {
	parsed, err := protocol.ParseCredentialRequestResponseBytes(response)
	if err != nil {
		return nil, nil, fmt.Errorf("%w: %w", ErrVerificationFails, err)
	}
	party, err := service.relyingParty(ctx)
	if err != nil {
		return nil, nil, err
	}
	return parsed, party, nil
}

// recordUse saves the credential's new signature counter, which lets the
// library detect a cloned authenticator next time (W3C WebAuthn §6.1.1).
func (service *Service) recordUse(ctx context.Context, credential *webauthn.Credential) error {
	now := service.dependencies.Clock.Now()
	err := service.dependencies.Store.UpdateCredentialAfterUse(ctx, store.UpdateCredentialAfterUseParams{
		CredentialID: credential.ID, Credential: encodeJSON(credential), LastUsedAt: &now,
	})
	if err != nil {
		return fmt.Errorf("recording credential use: %w", err)
	}
	return nil
}

// List returns the user's security keys and passkeys, oldest first.
func (service *Service) List(ctx context.Context, userID uuid.UUID) ([]Credential, error) {
	rows, err := service.dependencies.Store.ListCredentials(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("listing credentials: %w", err)
	}
	credentials := make([]Credential, 0, len(rows))
	for _, row := range rows {
		credentials = append(credentials, Credential{ID: row.ID, Kind: row.Kind, Name: row.Name, CreatedAt: row.CreatedAt, LastUsedAt: row.LastUsedAt})
	}
	return credentials, nil
}

// Remove deletes one of the user's credentials.
func (service *Service) Remove(ctx context.Context, userID uuid.UUID, id uuid.UUID) error {
	deleted, err := service.dependencies.Store.DeleteCredential(ctx, store.DeleteCredentialParams{ID: id, UserID: userID})
	if err != nil {
		return fmt.Errorf("removing credential: %w", err)
	}
	if deleted == 0 {
		return ErrNotFound
	}
	return nil
}
