package oidc

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/aazam476/nexus-chat/apps/server/internal/auth/oidc/store"
)

// LinkedIdentity is one provider a user can sign in with.
type LinkedIdentity struct {
	ProviderID  string
	DisplayName string
	Email       string
	LinkedAt    time.Time
}

// Identities lists the providers linked to userID.
func (service *SignIn) Identities(ctx context.Context, userID uuid.UUID) ([]LinkedIdentity, error) {
	rows, err := service.dependencies.Store.ListIdentitiesForUser(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("listing provider logins: %w", err)
	}
	identities := make([]LinkedIdentity, 0, len(rows))
	for _, row := range rows {
		identities = append(identities, LinkedIdentity{ProviderID: row.ProviderID, DisplayName: row.DisplayName, Email: row.Email, LinkedAt: row.CreatedAt})
	}
	return identities, nil
}

// Unlink removes userID's link to providerID, unless it is the account's
// only way to log in.
func (service *SignIn) Unlink(ctx context.Context, userID uuid.UUID, providerID string) error {
	others, err := service.dependencies.Store.CountOtherLoginMethods(ctx, store.CountOtherLoginMethodsParams{UserID: userID, ProviderID: providerID})
	if err != nil {
		return fmt.Errorf("counting login methods: %w", err)
	}
	if others == 0 {
		linked, err := service.isLinked(ctx, userID, providerID)
		if err != nil {
			return err
		}
		if linked {
			return ErrLastLoginMethod
		}
		return ErrNotLinked
	}
	deleted, err := service.dependencies.Store.DeleteIdentity(ctx, store.DeleteIdentityParams{ProviderID: providerID, UserID: userID})
	if err != nil {
		return fmt.Errorf("unlinking provider login: %w", err)
	}
	if deleted == 0 {
		return ErrNotLinked
	}
	return nil
}

func (service *SignIn) isLinked(ctx context.Context, userID uuid.UUID, providerID string) (bool, error) {
	identities, err := service.Identities(ctx, userID)
	if err != nil {
		return false, err
	}
	for _, identity := range identities {
		if identity.ProviderID == providerID {
			return true, nil
		}
	}
	return false, nil
}
