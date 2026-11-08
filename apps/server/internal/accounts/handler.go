package accounts

import (
	"context"
	"errors"
	"fmt"

	"github.com/aazam476/nexus-chat/apps/server/internal/accounts/store"
	"github.com/aazam476/nexus-chat/apps/server/internal/api"
	"github.com/aazam476/nexus-chat/apps/server/internal/auth/session"
)

// Handler serves the account endpoints.
type Handler struct {
	users UserReader
}

// NewHandler returns a Handler reading accounts from users.
func NewHandler(users UserReader) *Handler {
	return &Handler{users: users}
}

// errNoPrincipal means a protected handler ran without authentication,
// which the session middleware makes impossible; it is a programming error.
var errNoPrincipal = errors.New("handler requires an authenticated request")

// GetCurrentUser answers GET /api/v1/users/me.
func (handler *Handler) GetCurrentUser(ctx context.Context, _ api.GetCurrentUserRequestObject) (api.GetCurrentUserResponseObject, error) {
	principal, ok := session.PrincipalFrom(ctx)
	if !ok {
		return nil, errNoPrincipal
	}
	user, err := handler.users.GetUserByID(ctx, principal.UserID)
	if err != nil {
		return nil, fmt.Errorf("loading current user: %w", err)
	}
	return api.GetCurrentUser200JSONResponse(CurrentUserResponse(user)), nil
}

// CurrentUserResponse converts an account row to its API form. Only the
// account's owner may see this (it includes the email address).
func CurrentUserResponse(user store.User) api.CurrentUser {
	response := api.CurrentUser{
		Id: user.ID, Username: user.Username, DisplayName: user.DisplayName, Email: user.Email,
		EmailVerified: user.EmailVerifiedAt != nil, IsInstanceAdmin: user.IsInstanceAdmin,
		Bio: user.Bio, AvatarUrl: AvatarURL(user), Status: api.UserStatus(user.Status),
	}
	if user.CustomStatusText != nil {
		response.CustomStatus = &api.CustomStatus{Text: *user.CustomStatusText, ExpiresAt: user.CustomStatusExpiresAt}
	}
	return response
}

// AvatarURL is the path of the user's profile picture, or nil without one.
// Each upload has a new ID, so the address changes whenever the picture
// does and browsers may cache it forever.
func AvatarURL(user store.User) *string {
	if user.AvatarID == nil {
		return nil
	}
	path := "/api/v1/avatars/" + user.AvatarID.String()
	return &path
}
