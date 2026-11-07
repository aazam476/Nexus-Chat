package accounts

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/aazam476/nexus-chat/apps/server/internal/accounts/store"
	"github.com/aazam476/nexus-chat/apps/server/internal/auth/session"
)

// ErrForbidden means the caller is logged in but is not an instance admin.
var ErrForbidden = errors.New("only instance admins may do this")

// UserReader looks up accounts by ID; the generated store satisfies it.
type UserReader interface {
	GetUserByID(ctx context.Context, id uuid.UUID) (store.User, error)
}

// RequireInstanceAdmin returns the caller if they are an instance admin,
// ErrForbidden if they are not. Every admin endpoint calls it first.
func RequireInstanceAdmin(ctx context.Context, users UserReader) (store.User, error) {
	principal, ok := session.PrincipalFrom(ctx)
	if !ok {
		return store.User{}, errNoPrincipal
	}
	user, err := users.GetUserByID(ctx, principal.UserID)
	if err != nil {
		return store.User{}, fmt.Errorf("loading caller: %w", err)
	}
	if !user.IsInstanceAdmin {
		return store.User{}, ErrForbidden
	}
	return user, nil
}
