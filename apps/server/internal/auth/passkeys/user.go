package passkeys

import (
	"encoding/json"
	"fmt"

	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/google/uuid"

	"github.com/aazam476/nexus-chat/apps/server/internal/auth/passkeys/store"
)

// account adapts a user and their stored credentials to webauthn.User.
type account struct {
	id          uuid.UUID
	name        string
	displayName string
	credentials []webauthn.Credential
}

// WebAuthnID is the user handle: the account's UUID. It identifies the
// account without revealing anything about it (W3C WebAuthn §14.6.1).
func (user account) WebAuthnID() []byte { return user.id[:] }

// WebAuthnName is shown by the authenticator, for example in a passkey list.
func (user account) WebAuthnName() string { return user.name }

// WebAuthnDisplayName is the friendly name shown next to it.
func (user account) WebAuthnDisplayName() string { return user.displayName }

// WebAuthnCredentials are the account's registered credentials.
func (user account) WebAuthnCredentials() []webauthn.Credential { return user.credentials }

func decodeCredentials(rows []store.WebauthnCredential) ([]webauthn.Credential, error) {
	credentials := make([]webauthn.Credential, 0, len(rows))
	for _, row := range rows {
		var credential webauthn.Credential
		if err := json.Unmarshal(row.Credential, &credential); err != nil {
			return nil, fmt.Errorf("decoding stored credential %s: %w", row.ID, err)
		}
		credentials = append(credentials, credential)
	}
	return credentials, nil
}
