package accounts

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/aazam476/nexus-chat/apps/server/internal/accounts/store"
	"github.com/aazam476/nexus-chat/apps/server/internal/api"
	"github.com/aazam476/nexus-chat/apps/server/internal/auth/session"
)

type fakeUsers struct {
	user store.User
	err  error
}

func (fake fakeUsers) GetUserByID(context.Context, uuid.UUID) (store.User, error) {
	return fake.user, fake.err
}

func TestGetCurrentUser(t *testing.T) {
	verified := time.Date(2026, 11, 7, 12, 0, 0, 0, time.UTC)
	user := store.User{ID: uuid.New(), Username: "alice", DisplayName: "Alice", Email: "alice@example.com", EmailVerifiedAt: &verified}
	ctx := session.WithPrincipal(t.Context(), session.Principal{UserID: user.ID})

	response, err := NewHandler(fakeUsers{user: user}).GetCurrentUser(ctx, api.GetCurrentUserRequestObject{})

	require.NoError(t, err)
	assert.Equal(t, api.GetCurrentUser200JSONResponse{
		Id: user.ID, Username: "alice", DisplayName: "Alice", Email: "alice@example.com", EmailVerified: true,
	}, response)
}

func TestGetCurrentUserErrors(t *testing.T) {
	_, err := NewHandler(fakeUsers{}).GetCurrentUser(t.Context(), api.GetCurrentUserRequestObject{})
	require.ErrorIs(t, err, errNoPrincipal)

	ctx := session.WithPrincipal(t.Context(), session.Principal{UserID: uuid.New()})
	_, err = NewHandler(fakeUsers{err: errors.New("gone")}).GetCurrentUser(ctx, api.GetCurrentUserRequestObject{})
	assert.ErrorContains(t, err, "gone")
}
