package passkeys

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/google/uuid"

	"github.com/aazam476/nexus-chat/apps/server/internal/auth/passkeys/store"
)

// Kinds of credential.
const (
	KindPasskey     = "passkey"
	KindSecurityKey = "security_key"
)

// ErrUnknownKind means the kind is neither passkey nor security_key.
var ErrUnknownKind = errors.New("kind must be passkey or security_key")

// Started is a ceremony waiting for the browser: Options goes to
// navigator.credentials.create() or .get(), and CeremonyID comes back with
// the answer.
type Started struct {
	CeremonyID uuid.UUID
	Options    json.RawMessage
}

// BeginRegistration starts adding a credential of kind ("passkey" or
// "security_key") named name to the user's account.
func (service *Service) BeginRegistration(ctx context.Context, userID uuid.UUID, kind string, name string) (Started, error) {
	options, purpose, err := registrationOptions(kind)
	if err != nil {
		return Started{}, err
	}
	party, err := service.relyingParty(ctx)
	if err != nil {
		return Started{}, err
	}
	user, err := service.loadAccount(ctx, userID)
	if err != nil {
		return Started{}, err
	}
	// Excluding existing credentials stops the same key from being
	// registered twice.
	options = append(options, webauthn.WithExclusions(webauthn.Credentials(user.credentials).CredentialDescriptors()))
	creation, session, err := party.BeginRegistration(user, options...)
	if err != nil {
		return Started{}, fmt.Errorf("starting WebAuthn registration: %w", err)
	}
	return service.start(ctx, ceremony{purpose: purpose, userID: &userID, name: name, session: session}, creation)
}

// registrationOptions: a passkey must be discoverable and user-verified so
// it can replace the password; a security key only proves possession.
func registrationOptions(kind string) ([]webauthn.RegistrationOption, string, error) {
	switch kind {
	case KindPasskey:
		return []webauthn.RegistrationOption{webauthn.WithAuthenticatorSelection(protocol.AuthenticatorSelection{
			ResidentKey: protocol.ResidentKeyRequirementRequired, UserVerification: protocol.VerificationRequired,
		})}, purposeRegisterPasskey, nil
	case KindSecurityKey:
		return []webauthn.RegistrationOption{webauthn.WithAuthenticatorSelection(protocol.AuthenticatorSelection{
			ResidentKey: protocol.ResidentKeyRequirementDiscouraged, UserVerification: protocol.VerificationDiscouraged,
		})}, purposeRegisterSecurityKey, nil
	default:
		return nil, "", ErrUnknownKind
	}
}

// Answer is the browser's reply to a ceremony started for UserID.
type Answer struct {
	UserID     uuid.UUID
	CeremonyID uuid.UUID
	Kind       string
	// Response is the PublicKeyCredential the browser produced, as JSON.
	Response []byte
}

// FinishRegistration checks the browser's answer and stores the credential.
func (service *Service) FinishRegistration(ctx context.Context, answer Answer) error {
	_, purpose, err := registrationOptions(answer.Kind)
	if err != nil {
		return err
	}
	row, session, err := service.takeCeremony(ctx, answer.CeremonyID, purpose)
	if err != nil {
		return err
	}
	userID := answer.UserID
	if row.UserID == nil || *row.UserID != userID {
		return ErrCeremonyNotFound
	}
	credential, err := service.verifyRegistration(ctx, userID, session, answer.Response)
	if err != nil {
		return err
	}
	err = service.dependencies.Store.InsertCredential(ctx, store.InsertCredentialParams{
		ID: uuid.Must(uuid.NewV7()), UserID: userID, CredentialID: credential.ID, Credential: encodeJSON(credential),
		Kind: answer.Kind, Name: row.Name, CreatedAt: service.dependencies.Clock.Now(),
	})
	if err != nil {
		return fmt.Errorf("saving credential: %w", err)
	}
	return nil
}

func (service *Service) verifyRegistration(ctx context.Context, userID uuid.UUID, session webauthn.SessionData, response []byte) (*webauthn.Credential, error) {
	parsed, err := protocol.ParseCredentialCreationResponseBytes(response)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrVerificationFails, err)
	}
	party, err := service.relyingParty(ctx)
	if err != nil {
		return nil, err
	}
	user, err := service.loadAccount(ctx, userID)
	if err != nil {
		return nil, err
	}
	credential, err := party.CreateCredential(user, session, parsed)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrVerificationFails, err)
	}
	return credential, nil
}

// start saves the ceremony and returns the options for the browser.
func (service *Service) start(ctx context.Context, pending ceremony, options any) (Started, error) {
	id, err := service.saveCeremony(ctx, pending)
	if err != nil {
		return Started{}, err
	}
	return Started{CeremonyID: id, Options: encodeJSON(options)}, nil
}
