package accounts

import (
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/aazam476/nexus-chat/apps/server/internal/accounts/store"
	"github.com/aazam476/nexus-chat/apps/server/internal/auth/session"
)

func TestRequireInstanceAdmin(t *testing.T) {
	ctx := session.WithPrincipal(t.Context(), session.Principal{UserID: uuid.New()})

	admin, err := RequireInstanceAdmin(ctx, fakeUsers{user: store.User{Username: "root", IsInstanceAdmin: true}})
	require.NoError(t, err)
	assert.Equal(t, "root", admin.Username)

	_, err = RequireInstanceAdmin(ctx, fakeUsers{user: store.User{}})
	require.ErrorIs(t, err, ErrForbidden)

	_, err = RequireInstanceAdmin(ctx, fakeUsers{err: errors.New("down")})
	require.ErrorContains(t, err, "down")

	_, err = RequireInstanceAdmin(t.Context(), fakeUsers{})
	assert.ErrorIs(t, err, errNoPrincipal)
}
