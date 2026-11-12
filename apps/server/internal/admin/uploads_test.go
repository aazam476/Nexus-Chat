package admin

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/aazam476/nexus-chat/apps/server/internal/api"
	"github.com/aazam476/nexus-chat/apps/server/internal/instanceconfig"
)

// fakeUploads keeps the upload limit and preview setting in memory,
// checking the limit like the real service.
type fakeUploads struct {
	limit    int64
	previews bool
	err      error
}

func (fake *fakeUploads) LinkPreviewProxy(context.Context) (bool, error) {
	return fake.previews, fake.err
}

func (fake *fakeUploads) SetLinkPreviewProxy(_ context.Context, enabled bool) error {
	if fake.err != nil {
		return fake.err
	}
	fake.previews = enabled
	return nil
}

func (fake *fakeUploads) AttachmentMaxBytes(context.Context) (int64, error) {
	return fake.limit, fake.err
}

func (fake *fakeUploads) SetAttachmentMaxBytes(_ context.Context, limit int64) error {
	if fake.err != nil {
		return fake.err
	}
	if limit < instanceconfig.MinimumAttachmentMaxBytes || limit > instanceconfig.MaximumAttachmentMaxBytes {
		return instanceconfig.ErrInvalidAttachmentLimit
	}
	fake.limit = limit
	return nil
}

func TestUploadSettingsRoundTrip(t *testing.T) {
	fixture := newHandlerFixture(t, fakeUsers{admin: true})
	ctx := adminContext(t)

	got, err := fixture.handler.GetUploadSettings(ctx, api.GetUploadSettingsRequestObject{})
	require.NoError(t, err)
	assert.Equal(t, api.GetUploadSettings200JSONResponse{AttachmentMaxBytes: 25 << 20}, got)
	updated, err := fixture.handler.UpdateUploadSettings(ctx, api.UpdateUploadSettingsRequestObject{Body: &api.UploadSettings{AttachmentMaxBytes: 100 << 20}})
	require.NoError(t, err)
	assert.Equal(t, api.UpdateUploadSettings200JSONResponse{AttachmentMaxBytes: 100 << 20}, updated)

	invalid, err := fixture.handler.UpdateUploadSettings(ctx, api.UpdateUploadSettingsRequestObject{Body: &api.UploadSettings{AttachmentMaxBytes: 1}})
	require.NoError(t, err)
	require.IsType(t, api.UpdateUploadSettings422JSONResponse{}, invalid)
	assert.Equal(t, int64(100<<20), fixture.uploads.limit)
}

func TestUploadSettingsReportStoreFailures(t *testing.T) {
	fixture := newHandlerFixture(t, fakeUsers{admin: true})
	fixture.uploads.err = errors.New("down")
	ctx := adminContext(t)

	_, err := fixture.handler.GetUploadSettings(ctx, api.GetUploadSettingsRequestObject{})
	require.Error(t, err)
	_, err = fixture.handler.UpdateUploadSettings(ctx, api.UpdateUploadSettingsRequestObject{Body: &api.UploadSettings{AttachmentMaxBytes: 2 << 20}})
	require.Error(t, err)
}

func TestPreviewSettingsRoundTrip(t *testing.T) {
	fixture := newHandlerFixture(t, fakeUsers{admin: true})
	ctx := adminContext(t)

	got, err := fixture.handler.GetPreviewSettings(ctx, api.GetPreviewSettingsRequestObject{})
	require.NoError(t, err)
	assert.Equal(t, api.GetPreviewSettings200JSONResponse{ProxyEnabled: false}, got)
	updated, err := fixture.handler.UpdatePreviewSettings(ctx, api.UpdatePreviewSettingsRequestObject{Body: &api.PreviewSettings{ProxyEnabled: true}})
	require.NoError(t, err)
	assert.Equal(t, api.UpdatePreviewSettings200JSONResponse{ProxyEnabled: true}, updated)
	assert.True(t, fixture.uploads.previews)

	fixture.uploads.err = errors.New("down")
	_, err = fixture.handler.GetPreviewSettings(ctx, api.GetPreviewSettingsRequestObject{})
	require.Error(t, err)
	_, err = fixture.handler.UpdatePreviewSettings(ctx, api.UpdatePreviewSettingsRequestObject{Body: &api.PreviewSettings{}})
	require.Error(t, err)
}
