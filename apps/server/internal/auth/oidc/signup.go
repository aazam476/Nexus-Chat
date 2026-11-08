package oidc

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	accountsstore "github.com/aazam476/nexus-chat/apps/server/internal/accounts/store"
	"github.com/aazam476/nexus-chat/apps/server/internal/auth/oidc/store"
	"github.com/aazam476/nexus-chat/apps/server/internal/platform/mail"
	registrationstore "github.com/aazam476/nexus-chat/apps/server/internal/registration/store"
)

// Account field limits, matching the users table (spec §4.1).
const (
	minimumUsername    = 3
	maximumUsername    = 32
	maximumDisplayName = 32
	usernameAttempts   = 10
	// usernameSuffix is "_" plus 4 random characters added on a clash.
	usernameSuffix = 5
)

var errNoFreeUsername = errors.New("could not find a free username")

// signUp creates an account for a provider identity seen for the first
// time, following the instance's registration mode.
func (service *SignIn) signUp(ctx context.Context, flow store.OidcFlow, claims Claims) (uuid.UUID, error) {
	provider, err := service.dependencies.Providers.Get(ctx, flow.ProviderID)
	if err != nil {
		return uuid.Nil, err
	}
	if !provider.AllowSignup {
		return uuid.Nil, ErrNoAccount
	}
	if err := service.checkMode(ctx, flow.InviteHash != nil); err != nil {
		return uuid.Nil, err
	}
	email := normalizeEmail(claims.Email)
	if mail.CheckAddress(email) != nil {
		return uuid.Nil, ErrEmailRequired
	}
	username, err := service.pickUsername(ctx, claims)
	if err != nil {
		return uuid.Nil, err
	}
	now := service.now()
	user := accountsstore.InsertUserParams{
		ID: uuid.Must(uuid.NewV7()), Username: username, DisplayName: displayName(claims.Name, username),
		Email: email, CreatedAt: now,
	}
	if claims.EmailVerified {
		user.EmailVerifiedAt = &now
	}
	err = service.dependencies.Accounts.CreateWithIdentity(ctx,
		registrationstore.NewAccount{User: user, InviteHash: flow.InviteHash, Now: now},
		store.InsertIdentityParams{ProviderID: flow.ProviderID, Subject: claims.Subject, UserID: user.ID, Email: email, CreatedAt: now})
	return user.ID, signUpError(err)
}

func signUpError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, registrationstore.ErrInvalidInvite):
		return ErrInvalidInvite
	case errors.Is(err, registrationstore.ErrEmailTaken):
		return ErrEmailInUse
	case store.IsDuplicate(err, store.IdentityKey):
		return ErrIdentityInUse // the same login signed up twice at once
	default:
		return fmt.Errorf("creating account: %w", err)
	}
}

// checkMode applies the registration mode. An invite is honoured in every
// mode except closed, as for password registration.
func (service *SignIn) checkMode(ctx context.Context, hasInvite bool) error {
	mode, err := service.dependencies.Settings.RegistrationMode(ctx)
	if err != nil {
		return err
	}
	switch {
	case mode == "closed":
		return ErrRegistrationClosed
	case mode == "invite_only" && !hasInvite:
		return ErrInviteRequired
	default:
		return nil
	}
}

// pickUsername derives a valid username from the claims and adds a random
// suffix until it is free.
func (service *SignIn) pickUsername(ctx context.Context, claims Claims) (string, error) {
	base := usernameBase(claims)
	candidate := base
	for range usernameAttempts {
		taken, err := service.dependencies.Store.UsernameExists(ctx, candidate)
		if err != nil {
			return "", fmt.Errorf("checking username: %w", err)
		}
		if !taken {
			return candidate, nil
		}
		candidate = truncate(base, maximumUsername-usernameSuffix) + "_" + strings.ToLower(rand.Text()[:usernameSuffix-1])
	}
	return "", errNoFreeUsername
}

// usernameBase uses preferred_username, or else the email's local part,
// keeping only the characters usernames allow.
func usernameBase(claims Claims) string {
	source := claims.PreferredUsername
	if source == "" {
		source, _, _ = strings.Cut(claims.Email, "@")
	}
	var builder strings.Builder
	for _, character := range strings.ToLower(source) {
		switch {
		case character >= 'a' && character <= 'z', character >= '0' && character <= '9', character == '_', character == '.':
			builder.WriteRune(character)
		default:
			builder.WriteRune('_')
		}
	}
	base := truncate(strings.Trim(builder.String(), "_."), maximumUsername)
	if len(base) < minimumUsername {
		return "user"
	}
	return base
}

// truncate shortens an ASCII string to at most length bytes.
func truncate(text string, length int) string {
	if len(text) > length {
		return text[:length]
	}
	return text
}

// displayName tidies the provider's name, falling back to the username.
func displayName(name string, username string) string {
	name = strings.Join(strings.Fields(name), " ")
	if name == "" || !utf8.ValidString(name) {
		return username
	}
	if utf8.RuneCountInString(name) > maximumDisplayName {
		name = string([]rune(name)[:maximumDisplayName])
	}
	return name
}

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

// now is truncated to the database's microsecond precision.
func (service *SignIn) now() time.Time {
	return service.dependencies.Clock.Now().Truncate(time.Microsecond)
}
