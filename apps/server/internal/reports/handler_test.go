package reports

import (
	"context"
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

func TestReportEndpointsNeedASession(t *testing.T) {
	handler := NewHandler(NewService(Dependencies{}), nil)
	ctx, id := t.Context(), uuid.New()
	for _, err := range []error{
		second(handler.SubmitReport(ctx, api.SubmitReportRequestObject{Body: &api.NewReport{}})),
		second(handler.ListServerReports(ctx, api.ListServerReportsRequestObject{ServerId: id})),
		second(handler.ResolveServerReport(ctx, api.ResolveServerReportRequestObject{ServerId: id, ReportId: id})),
		second(handler.EscalateServerReport(ctx, api.EscalateServerReportRequestObject{ServerId: id, ReportId: id})),
		second(handler.ListAdminReports(ctx, api.ListAdminReportsRequestObject{})),
		second(handler.ResolveAdminReport(ctx, api.ResolveAdminReportRequestObject{ReportId: id})),
	} {
		require.Error(t, err)
	}
}

func second[T any](_ T, err error) error { return err }

func TestSubmittingAReport(t *testing.T) {
	f := newFixture(t)
	handler := NewHandler(f.service, f.accounts)
	submit := func(user uuid.UUID, report NewReport, comment *string) api.SubmitReportResponseObject {
		body := api.NewReport{ServerId: report.Target.ServerID, ChannelId: report.Target.ChannelID, ConversationId: report.Target.ConversationID, MessageId: report.Target.MessageID, Evidence: report.Evidence, Comment: comment}
		return must(handler.SubmitReport(as(t, user), api.SubmitReportRequestObject{Body: &body}))
	}
	why, long := "spam", strings.Repeat("x", 1001)
	assert.IsType(t, api.SubmitReport201JSONResponse{}, submit(f.grace, f.channelReport(""), &why))
	assert.IsType(t, api.SubmitReport409JSONResponse{}, submit(f.grace, f.channelReport(""), nil))
	assert.IsType(t, api.SubmitReport422JSONResponse{}, submit(f.sam, f.channelReport(""), nil))
	assert.IsType(t, api.SubmitReport422JSONResponse{}, submit(f.ada, f.channelReport(""), &long))
	bad := f.dmReport()
	bad.Evidence = []byte{1}
	assert.IsType(t, api.SubmitReport422JSONResponse{}, submit(f.grace, bad, nil))
	missing := f.dmReport()
	missing.Target.MessageID = uuid.New()
	assert.IsType(t, api.SubmitReport404JSONResponse{}, submit(f.grace, missing, nil))
	f.pool.Close()
	require.Error(t, second(handler.SubmitReport(as(t, f.ada), api.SubmitReportRequestObject{Body: &api.NewReport{ConversationId: &f.dmID, MessageId: f.dmMessage, Evidence: f.dmReport().Evidence}})))
}

func TestModeratorReportEndpoints(t *testing.T) {
	f := newFixture(t)
	handler := NewHandler(f.service, f.accounts)
	id := must(f.service.Submit(t.Context(), f.grace, f.channelReport("abuse")))
	list := func(user uuid.UUID, status *api.ListServerReportsParamsStatus) api.ListServerReportsResponseObject {
		return must(handler.ListServerReports(as(t, user), api.ListServerReportsRequestObject{ServerId: f.serverID, Params: api.ListServerReportsParams{Status: status}}))
	}
	open := list(f.ada, nil).(api.ListServerReports200JSONResponse) //nolint:forcetypeassert // checked by the test
	require.Len(t, open, 1)
	assert.Equal(t, "abuse", open[0].Comment)
	assert.Equal(t, api.ReportStatus("open"), open[0].Status)
	assert.Len(t, open[0].ReceiptSignature, 64)
	assert.IsType(t, api.ListServerReports403JSONResponse{}, list(f.grace, nil))
	assert.IsType(t, api.ListServerReports404JSONResponse{}, list(uuid.New(), nil))

	escalate := func(user uuid.UUID, report uuid.UUID) api.EscalateServerReportResponseObject {
		return must(handler.EscalateServerReport(as(t, user), api.EscalateServerReportRequestObject{ServerId: f.serverID, ReportId: report}))
	}
	assert.IsType(t, api.EscalateServerReport403JSONResponse{}, escalate(f.grace, id))
	assert.IsType(t, api.EscalateServerReport404JSONResponse{}, escalate(f.ada, uuid.New()))
	assert.IsType(t, api.EscalateServerReport204Response{}, escalate(f.ada, id))
	assert.IsType(t, api.EscalateServerReport409JSONResponse{}, escalate(f.ada, id))

	resolve := func(user uuid.UUID, report uuid.UUID, text *string) api.ResolveServerReportResponseObject {
		return must(handler.ResolveServerReport(as(t, user), api.ResolveServerReportRequestObject{ServerId: f.serverID, ReportId: report, Body: &api.ReportResolution{Resolution: text}}))
	}
	long, done := strings.Repeat("x", 1001), "Banned"
	assert.IsType(t, api.ResolveServerReport403JSONResponse{}, resolve(f.grace, id, nil))
	assert.IsType(t, api.ResolveServerReport404JSONResponse{}, resolve(f.ada, uuid.New(), nil))
	assert.IsType(t, api.ResolveServerReport422JSONResponse{}, resolve(f.ada, id, &long))
	assert.IsType(t, api.ResolveServerReport204Response{}, resolve(f.ada, id, &done))
	assert.IsType(t, api.ResolveServerReport409JSONResponse{}, resolve(f.ada, id, nil))
	resolved := api.ListServerReportsParamsStatus("resolved")
	closed := list(f.ada, &resolved).(api.ListServerReports200JSONResponse) //nolint:forcetypeassert // checked by the test
	assert.Equal(t, "Banned", closed[0].Resolution)

	f.pool.Close()
	require.Error(t, second(handler.ListServerReports(as(t, f.ada), api.ListServerReportsRequestObject{ServerId: f.serverID})))
	require.Error(t, second(handler.ResolveServerReport(as(t, f.ada), api.ResolveServerReportRequestObject{ServerId: f.serverID, ReportId: id})))
	require.Error(t, second(handler.EscalateServerReport(as(t, f.ada), api.EscalateServerReportRequestObject{ServerId: f.serverID, ReportId: id})))
}

func TestAdminReportEndpoints(t *testing.T) {
	f := newFixture(t)
	handler := NewHandler(f.service, f.accounts)
	f.exec(t, "UPDATE users SET is_instance_admin = true WHERE id = $1", f.ada)
	id := must(f.service.Submit(t.Context(), f.grace, f.dmReport()))

	assert.IsType(t, api.ListAdminReports403JSONResponse{}, must(handler.ListAdminReports(as(t, f.grace), api.ListAdminReportsRequestObject{})))
	listed := must(handler.ListAdminReports(as(t, f.ada), api.ListAdminReportsRequestObject{})).(api.ListAdminReports200JSONResponse) //nolint:forcetypeassert // checked by the test
	require.Len(t, listed, 1)
	assert.Equal(t, &f.dmID, listed[0].ConversationId)

	resolve := func(user uuid.UUID, report uuid.UUID, text *string) api.ResolveAdminReportResponseObject {
		return must(handler.ResolveAdminReport(as(t, user), api.ResolveAdminReportRequestObject{ReportId: report, Body: &api.ReportResolution{Resolution: text}}))
	}
	long := strings.Repeat("x", 1001)
	assert.IsType(t, api.ResolveAdminReport403JSONResponse{}, resolve(f.grace, id, nil))
	assert.IsType(t, api.ResolveAdminReport404JSONResponse{}, resolve(f.ada, uuid.New(), nil))
	assert.IsType(t, api.ResolveAdminReport422JSONResponse{}, resolve(f.ada, id, &long))
	assert.IsType(t, api.ResolveAdminReport204Response{}, must(handler.ResolveAdminReport(as(t, f.ada), api.ResolveAdminReportRequestObject{ReportId: id})))
	assert.IsType(t, api.ResolveAdminReport409JSONResponse{}, resolve(f.ada, id, nil))

	f.settings.err = assert.AnError
	require.Error(t, second(handler.ListAdminReports(as(t, f.ada), api.ListAdminReportsRequestObject{})))
	require.Error(t, second(handler.ResolveAdminReport(as(t, f.ada), api.ResolveAdminReportRequestObject{ReportId: id})))
	f.pool.Close()
	require.Error(t, second(handler.ListAdminReports(as(t, f.ada), api.ListAdminReportsRequestObject{})))
	require.Error(t, second(handler.ResolveAdminReport(as(t, f.ada), api.ResolveAdminReportRequestObject{ReportId: id})))
}
