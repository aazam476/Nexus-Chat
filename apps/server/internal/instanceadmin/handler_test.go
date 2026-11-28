package instanceadmin

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/aazam476/nexus-chat/apps/server/internal/api"
	"github.com/aazam476/nexus-chat/apps/server/internal/auth/session"
)

func as(t *testing.T, userID uuid.UUID) context.Context {
	t.Helper()
	return session.WithPrincipal(t.Context(), session.Principal{UserID: userID, SessionID: uuid.New()})
}

func must[T any](value T, err error) T {
	if err != nil {
		panic(err)
	}
	return value
}

// calls runs every operation once as ctx's user, returning each answer and error.
func calls(ctx context.Context, handler *Handler, id uuid.UUID) ([]any, []error) {
	answers, errs := []any{}, []error{}
	add := func(answer any, err error) {
		answers = append(answers, answer)
		errs = append(errs, err)
	}
	add(handler.ListAdminUsers(ctx, api.ListAdminUsersRequestObject{}))
	add(handler.SuspendUser(ctx, api.SuspendUserRequestObject{UserId: id, Body: &api.Suspension{}}))
	add(handler.ReinstateUser(ctx, api.ReinstateUserRequestObject{UserId: id}))
	add(handler.GrantInstanceAdmin(ctx, api.GrantInstanceAdminRequestObject{UserId: id}))
	add(handler.RevokeInstanceAdmin(ctx, api.RevokeInstanceAdminRequestObject{UserId: id}))
	add(handler.ListAdminServers(ctx, api.ListAdminServersRequestObject{}))
	add(handler.DeleteAdminServer(ctx, api.DeleteAdminServerRequestObject{ServerId: id}))
	add(handler.GetInstanceStats(ctx, api.GetInstanceStatsRequestObject{}))
	add(handler.GetRegistrationSettings(ctx, api.GetRegistrationSettingsRequestObject{}))
	add(handler.UpdateRegistrationSettings(ctx, api.UpdateRegistrationSettingsRequestObject{Body: &api.RegistrationSettings{Mode: "open"}}))
	add(handler.GetFederationSettings(ctx, api.GetFederationSettingsRequestObject{}))
	add(handler.UpdateFederationSettings(ctx, api.UpdateFederationSettingsRequestObject{Body: &api.FederationPolicy{Mode: "open"}}))
	add(handler.GetModerationSettings(ctx, api.GetModerationSettingsRequestObject{}))
	add(handler.UpdateModerationSettings(ctx, api.UpdateModerationSettingsRequestObject{Body: &api.ModerationSettings{}}))
	return answers, errs
}

func TestOnlyAdminsUseThePanel(t *testing.T) {
	f := newFixture(t)
	handler := NewHandler(f.service, f.accounts)
	plain := f.user(t, "ada", false)
	answers, errs := calls(as(t, plain), handler, uuid.New())
	for index, answer := range answers {
		require.NoError(t, errs[index])
		assert.Contains(t, typeName(answer), "403", "call %d", index)
	}
	_, errs = calls(t.Context(), handler, uuid.New())
	for _, err := range errs {
		require.Error(t, err, "no session")
	}
	f.pool.Close()
	_, errs = calls(as(t, f.admin), handler, uuid.New())
	for _, err := range errs {
		require.Error(t, err, "the database is down")
	}
}

func typeName(value any) string {
	return fmt.Sprintf("%T", value)
}

func TestThePanelAnswersAdmins(t *testing.T) {
	f := newFixture(t)
	handler := NewHandler(f.service, f.accounts)
	admin := as(t, f.admin)
	troll := f.user(t, "troll", false)
	reason, long := "spam", strings.Repeat("x", 600)

	users := must(handler.ListAdminUsers(admin, api.ListAdminUsersRequestObject{Params: api.ListAdminUsersParams{Q: &[]string{"tro"}[0]}}))
	require.Len(t, users.(api.ListAdminUsers200JSONResponse), 1) //nolint:forcetypeassert // checked by the test
	assert.IsType(t, api.ListAdminUsers422JSONResponse{}, must(handler.ListAdminUsers(admin, api.ListAdminUsersRequestObject{Params: api.ListAdminUsersParams{Q: &long}})))

	assert.IsType(t, api.SuspendUser204Response{}, must(handler.SuspendUser(admin, api.SuspendUserRequestObject{UserId: troll, Body: &api.Suspension{Reason: &reason}})))
	assert.IsType(t, api.SuspendUser422JSONResponse{}, must(handler.SuspendUser(admin, api.SuspendUserRequestObject{UserId: troll, Body: &api.Suspension{Reason: &long}})))
	assert.IsType(t, api.SuspendUser404JSONResponse{}, must(handler.SuspendUser(admin, api.SuspendUserRequestObject{UserId: uuid.New()})))
	assert.IsType(t, api.ReinstateUser204Response{}, must(handler.ReinstateUser(admin, api.ReinstateUserRequestObject{UserId: troll})))
	assert.IsType(t, api.ReinstateUser404JSONResponse{}, must(handler.ReinstateUser(admin, api.ReinstateUserRequestObject{UserId: troll})))
	assert.IsType(t, api.GrantInstanceAdmin204Response{}, must(handler.GrantInstanceAdmin(admin, api.GrantInstanceAdminRequestObject{UserId: troll})))
	assert.IsType(t, api.RevokeInstanceAdmin204Response{}, must(handler.RevokeInstanceAdmin(admin, api.RevokeInstanceAdminRequestObject{UserId: troll})))
	assert.IsType(t, api.RevokeInstanceAdmin422JSONResponse{}, must(handler.RevokeInstanceAdmin(admin, api.RevokeInstanceAdminRequestObject{UserId: f.admin})))

	assert.IsType(t, api.ListAdminServers200JSONResponse{}, must(handler.ListAdminServers(admin, api.ListAdminServersRequestObject{})))
	assert.IsType(t, api.ListAdminServers422JSONResponse{}, must(handler.ListAdminServers(admin, api.ListAdminServersRequestObject{Params: api.ListAdminServersParams{Q: &long}})))
	assert.IsType(t, api.DeleteAdminServer204Response{}, must(handler.DeleteAdminServer(admin, api.DeleteAdminServerRequestObject{ServerId: uuid.New()})))
	assert.IsType(t, api.DeleteAdminServer404JSONResponse{}, must(handler.DeleteAdminServer(admin, api.DeleteAdminServerRequestObject{ServerId: uuid.Nil})))
	stats := must(handler.GetInstanceStats(admin, api.GetInstanceStatsRequestObject{})).(api.GetInstanceStats200JSONResponse) //nolint:forcetypeassert // checked by the test
	assert.Equal(t, int64(2), stats.Users)
}

func TestThePanelChangesSettings(t *testing.T) {
	f := newFixture(t)
	handler := NewHandler(f.service, f.accounts)
	admin := as(t, f.admin)

	registration := must(handler.GetRegistrationSettings(admin, api.GetRegistrationSettingsRequestObject{}))
	assert.Equal(t, api.GetRegistrationSettings200JSONResponse{Mode: "invite_only"}, registration)
	assert.IsType(t, api.UpdateRegistrationSettings204Response{}, must(handler.UpdateRegistrationSettings(admin, api.UpdateRegistrationSettingsRequestObject{Body: &api.RegistrationSettings{Mode: "closed"}})))
	assert.IsType(t, api.UpdateRegistrationSettings422JSONResponse{}, must(handler.UpdateRegistrationSettings(admin, api.UpdateRegistrationSettingsRequestObject{Body: &api.RegistrationSettings{Mode: "maybe"}})))
	assert.Equal(t, api.GetRegistrationSettings200JSONResponse{Mode: "closed"}, must(handler.GetRegistrationSettings(admin, api.GetRegistrationSettingsRequestObject{})))

	policy := api.FederationPolicy{Mode: "allowlist", Allowed: []string{"Friends.Example"}, Blocked: []string{}}
	assert.IsType(t, api.UpdateFederationSettings204Response{}, must(handler.UpdateFederationSettings(admin, api.UpdateFederationSettingsRequestObject{Body: &policy})))
	assert.Equal(t, api.GetFederationSettings200JSONResponse{Mode: "allowlist", Allowed: []string{"friends.example"}, Blocked: []string{}},
		must(handler.GetFederationSettings(admin, api.GetFederationSettingsRequestObject{})))
	policy.Blocked = []string{"not a domain"}
	assert.IsType(t, api.UpdateFederationSettings422JSONResponse{}, must(handler.UpdateFederationSettings(admin, api.UpdateFederationSettingsRequestObject{Body: &policy})))

	assert.IsType(t, api.UpdateModerationSettings204Response{}, must(handler.UpdateModerationSettings(admin, api.UpdateModerationSettingsRequestObject{Body: &api.ModerationSettings{ReportsToAdmins: true}})))
	assert.Equal(t, api.GetModerationSettings200JSONResponse{ReportsToAdmins: true}, must(handler.GetModerationSettings(admin, api.GetModerationSettingsRequestObject{})))
}
