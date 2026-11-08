package oidc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/aazam476/nexus-chat/apps/server/internal/auth/login"
	"github.com/aazam476/nexus-chat/apps/server/internal/auth/oidc/store"
	"github.com/aazam476/nexus-chat/apps/server/internal/auth/session"
)

// CompleteRequest finishes a sign-in. FlowID comes from the client's own
// Begin call and Code from the callback's redirect: both are needed, which
// is what stops login CSRF (ADR 0016).
type CompleteRequest struct {
	FlowID    uuid.UUID
	Code      string
	Device    session.Device
	ClientKey string
}

// Complete logs in the account linked to the provider identity, creating
// it first if the provider allows sign-up. Like a password login, it
// returns an MFA challenge when the account has a second factor.
func (service *SignIn) Complete(ctx context.Context, request CompleteRequest) (login.Result, error) {
	if !service.dependencies.Limiter.Allow(request.ClientKey) {
		return login.Result{}, ErrRateLimited
	}
	flow, claims, err := service.consume(ctx, request.FlowID, request.Code)
	if err != nil {
		return login.Result{}, err
	}
	if flow.UserID != nil {
		return login.Result{}, ErrInvalidFlow // a link flow cannot log anyone in
	}
	var userID uuid.UUID
	identity, err := service.dependencies.Store.GetIdentity(ctx, store.GetIdentityParams{ProviderID: flow.ProviderID, Subject: claims.Subject})
	switch {
	case err == nil:
		userID = identity.UserID
	case errors.Is(err, pgx.ErrNoRows):
		if userID, err = service.signUp(ctx, flow, claims); err != nil {
			return login.Result{}, err
		}
	default:
		return login.Result{}, fmt.Errorf("finding linked account: %w", err)
	}
	return service.dependencies.Logins.ContinueLogin(ctx, userID, request.Device)
}

// CompleteLink links the provider identity to userID, who must be the
// user who began the link.
func (service *SignIn) CompleteLink(ctx context.Context, userID uuid.UUID, flowID uuid.UUID, code string) error {
	flow, claims, err := service.consume(ctx, flowID, code)
	if err != nil {
		return err
	}
	if flow.UserID == nil || *flow.UserID != userID {
		return ErrInvalidFlow
	}
	existing, err := service.dependencies.Store.GetIdentity(ctx, store.GetIdentityParams{ProviderID: flow.ProviderID, Subject: claims.Subject})
	switch {
	case err == nil && existing.UserID == userID:
		return nil // already linked to this very account
	case err == nil:
		return ErrIdentityInUse
	case !errors.Is(err, pgx.ErrNoRows):
		return fmt.Errorf("checking provider login: %w", err)
	}
	err = service.dependencies.Store.InsertIdentity(ctx, store.InsertIdentityParams{
		ProviderID: flow.ProviderID, Subject: claims.Subject, UserID: userID,
		Email: normalizeEmail(claims.Email), CreatedAt: service.now(),
	})
	return linkError(err)
}

func linkError(err error) error {
	switch {
	case err == nil:
		return nil
	case store.IsDuplicate(err, store.IdentityPerUserKey):
		return ErrAlreadyLinked
	case store.IsDuplicate(err, store.IdentityKey):
		return ErrIdentityInUse
	default:
		return fmt.Errorf("linking provider login: %w", err)
	}
}

// consume uses up a flow whose callback has run and returns its claims.
func (service *SignIn) consume(ctx context.Context, flowID uuid.UUID, code string) (store.OidcFlow, Claims, error) {
	flow, err := service.dependencies.Store.ConsumeFlow(ctx, store.ConsumeFlowParams{
		ID: flowID, CompletionHash: hash(code), Now: service.dependencies.Clock.Now(),
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return store.OidcFlow{}, Claims{}, ErrInvalidFlow
	}
	if err != nil {
		return store.OidcFlow{}, Claims{}, fmt.Errorf("finishing sign-in: %w", err)
	}
	var claims Claims
	if err := json.Unmarshal(flow.Claims, &claims); err != nil {
		return store.OidcFlow{}, Claims{}, fmt.Errorf("reading sign-in result: %w", err)
	}
	return flow, claims, nil
}
