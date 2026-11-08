package oidc

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/aazam476/nexus-chat/apps/server/internal/auth/oidc/store"
	"github.com/aazam476/nexus-chat/apps/server/internal/platform/clock"
	"github.com/aazam476/nexus-chat/apps/server/internal/platform/secretbox"
)

// Provider is a configured sign-in provider with its client secret
// decrypted, ready to talk to.
type Provider struct {
	ID           string
	DisplayName  string
	Issuer       string
	ClientID     string
	ClientSecret string
	Scopes       []string
	AllowSignup  bool
}

// ProviderView is a provider as the Admin panel shows it: no secret.
type ProviderView struct {
	ID              string
	DisplayName     string
	Issuer          string
	ClientID        string
	ClientSecretSet bool
	Scopes          []string
	AllowSignup     bool
}

// ProviderInput is an admin's new or changed provider. ID is used only
// when creating. A nil ClientSecret keeps the stored one; an empty one
// removes it (a public client).
type ProviderInput struct {
	ID           string
	DisplayName  string
	Issuer       string
	ClientID     string
	ClientSecret *string
	Scopes       []string
	AllowSignup  bool
}

type providerStore interface {
	ListProviders(ctx context.Context) ([]store.OidcProvider, error)
	GetProvider(ctx context.Context, id string) (store.OidcProvider, error)
	InsertProvider(ctx context.Context, arg store.InsertProviderParams) error
	UpdateProvider(ctx context.Context, arg store.UpdateProviderParams) (int64, error)
	DeleteProvider(ctx context.Context, id string) (int64, error)
}

// issuerChecker confirms an issuer publishes a usable OIDC configuration.
type issuerChecker interface {
	Check(ctx context.Context, issuer string) error
}

// Providers manages the configured sign-in providers.
type Providers struct {
	store   providerStore
	box     *secretbox.Box
	checker issuerChecker
	clock   clock.Clock
}

// NewProviders returns a Providers. box encrypts client secrets; checker
// is consulted before a provider is saved.
func NewProviders(providers providerStore, box *secretbox.Box, checker issuerChecker, clock clock.Clock) *Providers {
	return &Providers{store: providers, box: box, checker: checker, clock: clock}
}

// secretContext binds a sealed client secret to its provider.
func secretContext(providerID string) string {
	return "nexus-chat oidc client secret " + providerID
}

// List returns every provider, sorted by display name.
func (service *Providers) List(ctx context.Context) ([]ProviderView, error) {
	rows, err := service.store.ListProviders(ctx)
	if err != nil {
		return nil, fmt.Errorf("listing sign-in providers: %w", err)
	}
	views := make([]ProviderView, 0, len(rows))
	for _, row := range rows {
		views = append(views, view(row))
	}
	return views, nil
}

// Get returns the provider with its secret decrypted, or
// ErrProviderNotFound.
func (service *Providers) Get(ctx context.Context, id string) (Provider, error) {
	row, err := service.store.GetProvider(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return Provider{}, ErrProviderNotFound
	}
	if err != nil {
		return Provider{}, fmt.Errorf("loading sign-in provider: %w", err)
	}
	provider := Provider{
		ID: row.ID, DisplayName: row.DisplayName, Issuer: row.Issuer, ClientID: row.ClientID,
		Scopes: row.Scopes, AllowSignup: row.AllowSignup,
	}
	if row.ClientSecretSealed != nil {
		secret, err := service.box.Open(secretContext(row.ID), row.ClientSecretSealed)
		if err != nil {
			return Provider{}, fmt.Errorf("client secret of %s: %w", row.ID, err)
		}
		provider.ClientSecret = string(secret)
	}
	return provider, nil
}

// Create validates and saves a new provider.
func (service *Providers) Create(ctx context.Context, input ProviderInput) (ProviderView, error) {
	checked, err := service.validate(ctx, input, true)
	if err != nil {
		return ProviderView{}, err
	}
	var sealed []byte
	if checked.ClientSecret != nil && *checked.ClientSecret != "" {
		sealed = service.box.Seal(secretContext(checked.ID), []byte(*checked.ClientSecret))
	}
	now := service.now()
	err = service.store.InsertProvider(ctx, store.InsertProviderParams{
		ID: checked.ID, DisplayName: checked.DisplayName, Issuer: checked.Issuer, ClientID: checked.ClientID,
		ClientSecretSealed: sealed, Scopes: checked.Scopes, AllowSignup: checked.AllowSignup, CreatedAt: now,
	})
	if store.IsDuplicate(err, store.ProviderKey) {
		return ProviderView{}, ErrProviderExists
	}
	if err != nil {
		return ProviderView{}, fmt.Errorf("saving sign-in provider: %w", err)
	}
	return view(store.OidcProvider{
		ID: checked.ID, DisplayName: checked.DisplayName, Issuer: checked.Issuer, ClientID: checked.ClientID,
		ClientSecretSealed: sealed, Scopes: checked.Scopes, AllowSignup: checked.AllowSignup,
	}), nil
}

// Update validates and saves changes to provider id.
func (service *Providers) Update(ctx context.Context, id string, input ProviderInput) (ProviderView, error) {
	current, err := service.store.GetProvider(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return ProviderView{}, ErrProviderNotFound
	}
	if err != nil {
		return ProviderView{}, fmt.Errorf("loading sign-in provider: %w", err)
	}
	input.ID = id
	checked, err := service.validate(ctx, input, false)
	if err != nil {
		return ProviderView{}, err
	}
	sealed := current.ClientSecretSealed
	if checked.ClientSecret != nil {
		sealed = nil
		if *checked.ClientSecret != "" {
			sealed = service.box.Seal(secretContext(id), []byte(*checked.ClientSecret))
		}
	}
	updated := store.UpdateProviderParams{
		ID: id, DisplayName: checked.DisplayName, Issuer: checked.Issuer, ClientID: checked.ClientID,
		ClientSecretSealed: sealed, Scopes: checked.Scopes, AllowSignup: checked.AllowSignup, UpdatedAt: service.now(),
	}
	rows, err := service.store.UpdateProvider(ctx, updated)
	if err != nil {
		return ProviderView{}, fmt.Errorf("saving sign-in provider: %w", err)
	}
	if rows == 0 {
		return ProviderView{}, ErrProviderNotFound
	}
	return view(store.OidcProvider{
		ID: id, DisplayName: updated.DisplayName, Issuer: updated.Issuer, ClientID: updated.ClientID,
		ClientSecretSealed: sealed, Scopes: updated.Scopes, AllowSignup: updated.AllowSignup,
	}), nil
}

// Delete removes provider id and every identity linked through it.
func (service *Providers) Delete(ctx context.Context, id string) error {
	rows, err := service.store.DeleteProvider(ctx, id)
	if err != nil {
		return fmt.Errorf("deleting sign-in provider: %w", err)
	}
	if rows == 0 {
		return ErrProviderNotFound
	}
	return nil
}

// now is truncated to the database's microsecond precision.
func (service *Providers) now() time.Time {
	return service.clock.Now().Truncate(time.Microsecond)
}

func view(row store.OidcProvider) ProviderView {
	return ProviderView{
		ID: row.ID, DisplayName: row.DisplayName, Issuer: row.Issuer, ClientID: row.ClientID,
		ClientSecretSet: row.ClientSecretSealed != nil, Scopes: row.Scopes, AllowSignup: row.AllowSignup,
	}
}
