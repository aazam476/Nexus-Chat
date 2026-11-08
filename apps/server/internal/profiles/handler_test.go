package profiles

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/aazam476/nexus-chat/apps/server/internal/api"
	"github.com/aazam476/nexus-chat/apps/server/internal/auth/session"
	"github.com/aazam476/nexus-chat/apps/server/internal/profiles/avatar"
)

func (fixture fixture) ctx(t *testing.T) context.Context {
	return session.WithPrincipal(t.Context(), session.Principal{UserID: fixture.user.ID})
}

func intPointer(value int) *int { return &value }

func TestUpdateCurrentUserHandler(t *testing.T) {
	fixture := newFixture(t)

	response, err := fixture.handler.UpdateCurrentUser(fixture.ctx(t), api.UpdateCurrentUserRequestObject{Body: &api.UpdateProfileRequest{Bio: pointer("Hi")}})
	require.NoError(t, err)
	assert.Equal(t, "Hi", response.(api.UpdateCurrentUser200JSONResponse).Bio) //nolint:forcetypeassert // the test fails otherwise

	invalid, err := fixture.handler.UpdateCurrentUser(fixture.ctx(t), api.UpdateCurrentUserRequestObject{Body: &api.UpdateProfileRequest{DisplayName: pointer("")}})
	require.NoError(t, err)
	assert.IsType(t, api.UpdateCurrentUser422JSONResponse{}, invalid)

	fixture.users.failOn = "UpdateProfile"
	_, err = fixture.handler.UpdateCurrentUser(fixture.ctx(t), api.UpdateCurrentUserRequestObject{Body: &api.UpdateProfileRequest{}})
	assert.ErrorIs(t, err, errFailure)
}

func TestSetStatusHandler(t *testing.T) {
	fixture := newFixture(t)
	later := testNow.Add(time.Hour)
	body := &api.SetStatusRequest{Status: api.UserStatusDnd, CustomStatus: &api.CustomStatus{Text: "Busy", ExpiresAt: &later}}

	response, err := fixture.handler.SetStatus(fixture.ctx(t), api.SetStatusRequestObject{Body: body})
	require.NoError(t, err)
	user := response.(api.SetStatus200JSONResponse) //nolint:forcetypeassert // the test fails otherwise
	assert.Equal(t, api.UserStatusDnd, user.Status)
	assert.Equal(t, &api.CustomStatus{Text: "Busy", ExpiresAt: &later}, user.CustomStatus)

	invalid, err := fixture.handler.SetStatus(fixture.ctx(t), api.SetStatusRequestObject{Body: &api.SetStatusRequest{Status: "away"}})
	require.NoError(t, err)
	assert.IsType(t, api.SetStatus422JSONResponse{}, invalid)

	fixture.users.failOn = "SetStatus"
	_, err = fixture.handler.SetStatus(fixture.ctx(t), api.SetStatusRequestObject{Body: &api.SetStatusRequest{Status: api.UserStatusOnline}})
	assert.ErrorIs(t, err, errFailure)
}

func TestSetAvatarHandler(t *testing.T) {
	fixture := newFixture(t)
	request := api.SetAvatarRequestObject{
		Params: api.SetAvatarParams{CropX: intPointer(0), CropY: intPointer(0), CropSize: intPointer(10)},
		Body:   bytes.NewReader(testPNG(t)),
	}

	response, err := fixture.handler.SetAvatar(fixture.ctx(t), request)

	require.NoError(t, err)
	user := response.(api.SetAvatar200JSONResponse) //nolint:forcetypeassert // the test fails otherwise
	require.NotNil(t, user.AvatarUrl)
	assert.True(t, strings.HasPrefix(*user.AvatarUrl, "/api/v1/avatars/"))
}

func TestSetAvatarHandlerErrors(t *testing.T) {
	tests := []struct {
		name   string
		params api.SetAvatarParams
		body   []byte
		change func(*fixture)
		want   any
	}{
		{"partial crop", api.SetAvatarParams{CropX: intPointer(1)}, nil, nil,
			api.SetAvatar422JSONResponse{Code: "invalid_crop", Message: errPartialCrop.Error()}},
		{"crop outside", api.SetAvatarParams{CropX: intPointer(60), CropY: intPointer(0), CropSize: intPointer(10)}, nil, nil,
			api.SetAvatar422JSONResponse{Code: "invalid_crop", Message: avatar.ErrBadCrop.Error()}},
		{"not an image", api.SetAvatarParams{}, []byte("text"), nil,
			api.SetAvatar422JSONResponse{Code: "unsupported_image", Message: avatar.ErrUnsupported.Error()}},
		{"too large", api.SetAvatarParams{}, make([]byte, avatar.MaximumUploadBytes+10), nil,
			api.SetAvatar413JSONResponse{Code: "image_too_large", Message: avatar.ErrTooLarge.Error()}},
		{"rate limited", api.SetAvatarParams{}, nil, func(f *fixture) { f.limiter.denied = true },
			api.SetAvatar429JSONResponse{RateLimitedJSONResponse: api.RateLimitedJSONResponse{Code: "rate_limited", Message: ErrRateLimited.Error()}}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fixture := newFixture(t)
			if test.change != nil {
				test.change(&fixture)
			}
			body := test.body
			if body == nil {
				body = testPNG(t)
			}

			response, err := fixture.handler.SetAvatar(fixture.ctx(t), api.SetAvatarRequestObject{Params: test.params, Body: bytes.NewReader(body)})

			require.NoError(t, err)
			assert.Equal(t, test.want, response)
		})
	}
	fixture := newFixture(t)
	fixture.files.failOn = "Save"
	_, err := fixture.handler.SetAvatar(fixture.ctx(t), api.SetAvatarRequestObject{Body: bytes.NewReader(testPNG(t))})
	require.ErrorIs(t, err, errFailure)
	_, err = fixture.handler.SetAvatar(fixture.ctx(t), api.SetAvatarRequestObject{Body: failingReader{}})
	assert.ErrorContains(t, err, "reading picture")
}

type failingReader struct{}

func (failingReader) Read([]byte) (int, error) { return 0, errFailure }

func TestRemoveAvatarHandler(t *testing.T) {
	fixture := newFixture(t)
	response, err := fixture.handler.RemoveAvatar(fixture.ctx(t), api.RemoveAvatarRequestObject{})
	require.NoError(t, err)
	assert.Equal(t, api.RemoveAvatar204Response{}, response)

	fixture.users.failOn = "GetUserByID"
	_, err = fixture.handler.RemoveAvatar(fixture.ctx(t), api.RemoveAvatarRequestObject{})
	assert.ErrorIs(t, err, errFailure)
}

func TestGetUserProfileShowsPresenceNotEmail(t *testing.T) {
	fixture := newFixture(t)

	response, err := fixture.handler.GetUserProfile(t.Context(), api.GetUserProfileRequestObject{UserId: fixture.user.ID})

	require.NoError(t, err)
	assert.Equal(t, api.GetUserProfile200JSONResponse{
		Id: fixture.user.ID, Username: "ada", DisplayName: "Ada", Status: api.PresenceStatusIdle,
	}, response)
	missing, err := fixture.handler.GetUserProfile(t.Context(), api.GetUserProfileRequestObject{UserId: uuid.New()})
	require.NoError(t, err)
	assert.IsType(t, api.GetUserProfile404JSONResponse{}, missing)
	fixture.users.failOn = "GetUserByID"
	_, err = fixture.handler.GetUserProfile(t.Context(), api.GetUserProfileRequestObject{UserId: fixture.user.ID})
	assert.ErrorIs(t, err, errFailure)
}

func TestGetAvatarHandlerServesEachFormat(t *testing.T) {
	fixture := newFixture(t)
	user, err := fixture.service.SetAvatar(t.Context(), fixture.user.ID, testPNG(t), nil)
	require.NoError(t, err)

	for format, want := range map[string]any{
		"png": api.GetAvatar200ImagepngResponse{}, "jpeg": api.GetAvatar200ImagejpegResponse{}, "gif": api.GetAvatar200ImagegifResponse{},
	} {
		stored := fixture.users.users[fixture.user.ID]
		stored.AvatarFormat = &format
		fixture.users.users[fixture.user.ID] = stored

		response, err := fixture.handler.GetAvatar(t.Context(), api.GetAvatarRequestObject{AvatarId: *user.AvatarID})

		require.NoError(t, err)
		assert.IsType(t, want, response, format)
	}
	missing, err := fixture.handler.GetAvatar(t.Context(), api.GetAvatarRequestObject{AvatarId: uuid.New()})
	require.NoError(t, err)
	assert.IsType(t, api.GetAvatar404JSONResponse{}, missing)
	fixture.users.failOn = "GetAvatarFormat"
	_, err = fixture.handler.GetAvatar(t.Context(), api.GetAvatarRequestObject{AvatarId: *user.AvatarID})
	assert.ErrorIs(t, err, errFailure)
}

func TestHandlersNeedAPrincipal(t *testing.T) {
	handler := newFixture(t).handler
	ctx := t.Context()
	_, err := handler.UpdateCurrentUser(ctx, api.UpdateCurrentUserRequestObject{})
	require.ErrorIs(t, err, errNoPrincipal)
	_, err = handler.SetStatus(ctx, api.SetStatusRequestObject{})
	require.ErrorIs(t, err, errNoPrincipal)
	_, err = handler.SetAvatar(ctx, api.SetAvatarRequestObject{})
	require.ErrorIs(t, err, errNoPrincipal)
	_, err = handler.RemoveAvatar(ctx, api.RemoveAvatarRequestObject{})
	assert.ErrorIs(t, err, errNoPrincipal)
}
