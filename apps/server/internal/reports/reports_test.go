package reports

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/aazam476/nexus-chat/apps/server/internal/delivery"
)

func TestAReportCarriesTheInstancesReceipt(t *testing.T) {
	f := newFixture(t)
	id, err := f.service.Submit(t.Context(), f.grace, f.channelReport("abusive"))
	require.NoError(t, err)

	list, err := f.service.ServerReports(t.Context(), f.serverID, f.ada, StatusOpen)
	require.NoError(t, err)
	require.Len(t, list, 1)
	report := list[0]
	assert.Equal(t, id, report.ID)
	assert.Equal(t, &f.grace, report.ReporterID)
	assert.Equal(t, &f.sam, report.SenderID)
	assert.Equal(t, &f.audienceID, report.ConversationID)
	assert.Equal(t, "abusive", report.Comment)

	signed := SignedReceipt{Receipt: report.Receipt, Signature: report.ReceiptSig}
	require.True(t, VerifyReceipt(f.key, signed))
	var receipt Receipt
	require.NoError(t, json.Unmarshal(report.Receipt, &receipt))
	assert.Equal(t, f.channelMessage, receipt.MessageID)
	assert.Equal(t, f.sam, receipt.SenderID)
	assert.Equal(t, "@sam@chat.example.com", receipt.Sender)
	assert.Equal(t, &f.channelID, receipt.ChannelID)
	assert.Equal(t, int64(3), receipt.Epoch)
	assert.Equal(t, archiveHash([]byte("sealed")), receipt.ArchiveHash)
	assert.True(t, now.Add(-time.Hour).Equal(receipt.SentAt), "when the server stored it, not when it was reported")

	tampered := signed
	tampered.Receipt = []byte(strings.Replace(string(report.Receipt), f.sam.String(), f.grace.String(), 1))
	assert.False(t, VerifyReceipt(f.key, tampered))

	_, err = f.service.Submit(t.Context(), f.grace, f.channelReport(""))
	require.ErrorIs(t, err, ErrReported, "once per reporter")
	_, err = f.service.Submit(t.Context(), f.ada, f.channelReport(""))
	require.NoError(t, err, "others may report it too")
}

func TestReportsAreRefusedWhenTheyDoNotFit(t *testing.T) {
	f := newFixture(t)
	ctx := t.Context()
	_, err := f.service.Submit(ctx, f.sam, f.channelReport(""))
	require.ErrorIs(t, err, ErrOwn)
	_, err = f.service.Submit(ctx, f.grace, f.channelReport(strings.Repeat("x", 1001)))
	require.ErrorIs(t, err, ErrInvalid)

	wrong := map[string][]byte{
		"another message": evidenceFor(f.channelID, uuid.New(), 3),
		"another place":   evidenceFor(f.audienceID, f.channelMessage, 3),
		"another epoch":   evidenceFor(f.channelID, f.channelMessage, 4),
		"another label":   prefixed([]byte("x"), []byte(f.channelID.String()), []byte(f.channelMessage.String()), make([]byte, 8), nil, nil, nil, nil),
		"a short epoch":   prefixed([]byte(evidenceLabel), []byte(f.channelID.String()), []byte(f.channelMessage.String()), make([]byte, 4), nil, nil, nil, nil),
		"too few fields":  prefixed([]byte(evidenceLabel)),
		"a broken length": {0, 0, 0, 9, 1},
		"trailing bytes":  append(evidenceFor(f.channelID, f.channelMessage, 3), 0),
		"too large":       make([]byte, MaximumEvidence+1),
	}
	for name, evidence := range wrong {
		report := f.channelReport("")
		report.Evidence = evidence
		_, err := f.service.Submit(ctx, f.grace, report)
		require.ErrorIs(t, err, ErrBadEvidence, name)
	}

	unknown := f.channelReport("")
	unknown.Target.MessageID = uuid.New()
	_, err = f.service.Submit(ctx, f.grace, unknown)
	require.ErrorIs(t, err, ErrNotFound)
	both := f.channelReport("")
	both.Target.ConversationID = &f.dmID
	_, err = f.service.Submit(ctx, f.grace, both)
	require.ErrorIs(t, err, ErrNotFound, "a message is in a channel or a conversation, not both")

	gone := f.copies.byMessage[f.dmMessage]
	gone.SenderID = nil
	f.copies.byMessage[f.dmMessage] = gone
	_, err = f.service.Submit(ctx, f.grace, f.dmReport())
	require.ErrorIs(t, err, ErrNotFound, "the sender's account is gone")

	f.settings.domainErr = errors.New("settings are down")
	_, err = f.service.Submit(ctx, f.ada, f.channelReport(""))
	require.ErrorIs(t, err, f.settings.domainErr)
	f.copies.byMessage[f.channelMessage] = withSender(f.copies.byMessage[f.channelMessage], uuid.New())
	_, err = f.service.Submit(ctx, f.ada, f.channelReport(""))
	require.Error(t, err, "a sender with no account")

	f.copies.err = errors.New("database is down")
	_, err = f.service.Submit(ctx, f.grace, f.channelReport(""))
	require.ErrorIs(t, err, f.copies.err)
}

func TestModeratorsHandleTheirServersReports(t *testing.T) {
	f := newFixture(t)
	ctx := t.Context()
	id, err := f.service.Submit(ctx, f.grace, f.channelReport(""))
	require.NoError(t, err)
	action := Action{ServerID: f.serverID, UserID: f.ada, ReportID: id, Resolution: "Warned them"}

	_, err = f.service.ServerReports(ctx, f.serverID, f.grace, StatusOpen)
	require.ErrorIs(t, err, ErrForbidden, "only with MANAGE_MESSAGES")
	_, err = f.service.ServerReports(ctx, f.serverID, uuid.New(), StatusOpen)
	require.ErrorIs(t, err, ErrNoServer)
	require.ErrorIs(t, f.service.ResolveServerReport(ctx, Action{ServerID: f.serverID, UserID: f.ada, ReportID: uuid.New()}), ErrNotFound)
	require.ErrorIs(t, f.service.ResolveServerReport(ctx, Action{ServerID: uuid.New(), UserID: f.ada, ReportID: id}), ErrNotFound, "another server's")
	require.ErrorIs(t, f.service.ResolveServerReport(ctx, Action{ServerID: f.serverID, UserID: f.ada, ReportID: id, Resolution: strings.Repeat("x", 1001)}), ErrInvalid)

	require.NoError(t, f.service.EscalateServerReport(ctx, action))
	require.ErrorIs(t, f.service.EscalateServerReport(ctx, action), ErrHandled)
	require.NoError(t, f.service.ResolveServerReport(ctx, action))
	require.ErrorIs(t, f.service.ResolveServerReport(ctx, action), ErrHandled)
	assert.Equal(t, []Handled{
		{ServerID: f.serverID, ActorID: f.ada, ReportID: id, Escalated: true},
		{ServerID: f.serverID, ActorID: f.ada, ReportID: id},
	}, f.moderation.handled)

	open, err := f.service.ServerReports(ctx, f.serverID, f.ada, StatusOpen)
	require.NoError(t, err)
	assert.Empty(t, open)
	resolved, err := f.service.ServerReports(ctx, f.serverID, f.ada, StatusResolved)
	require.NoError(t, err)
	require.Len(t, resolved, 1)
	assert.Equal(t, "Warned them", resolved[0].Resolution)
	assert.Equal(t, &f.ada, resolved[0].ResolvedBy)
	assert.True(t, resolved[0].Escalated)

	f.moderation.err = errors.New("audit log is down")
	require.ErrorIs(t, f.service.ResolveServerReport(ctx, action), f.moderation.err)
}

func TestInstanceAdminsSeeDirectMessageAndPassedOnReports(t *testing.T) {
	f := newFixture(t)
	ctx := t.Context()
	direct, err := f.service.Submit(ctx, f.grace, f.dmReport())
	require.NoError(t, err)
	channel, err := f.service.Submit(ctx, f.grace, f.channelReport(""))
	require.NoError(t, err)

	list, err := f.service.AdminReports(ctx, StatusOpen)
	require.NoError(t, err)
	require.Len(t, list, 1, "a server's own reports stay with its moderators")
	assert.Equal(t, direct, list[0].ID)
	assert.Nil(t, list[0].ServerID)
	moderated, err := f.service.ServerReports(ctx, f.serverID, f.ada, StatusOpen)
	require.NoError(t, err)
	assert.Len(t, moderated, 1, "and direct messages' reports are not theirs")
	require.ErrorIs(t, f.service.ResolveAdminReport(ctx, f.ada, channel, ""), ErrNotFound)

	f.settings.everything = true
	list, err = f.service.AdminReports(ctx, StatusOpen)
	require.NoError(t, err)
	assert.Len(t, list, 2, "unless the instance sends them every report")
	require.NoError(t, f.service.ResolveAdminReport(ctx, f.ada, channel, "Removed"))
	require.NoError(t, f.service.ResolveAdminReport(ctx, f.ada, direct, ""))
	require.ErrorIs(t, f.service.ResolveAdminReport(ctx, f.ada, direct, ""), ErrHandled)
	require.ErrorIs(t, f.service.ResolveAdminReport(ctx, f.ada, uuid.New(), ""), ErrNotFound)

	f.settings.err = errors.New("settings are down")
	_, err = f.service.AdminReports(ctx, StatusOpen)
	require.Error(t, err)
	require.Error(t, f.service.ResolveAdminReport(ctx, f.ada, direct, ""))
}

func TestReportsReportDatabaseFailures(t *testing.T) {
	f := newFixture(t)
	f.pool.Close()
	ctx := t.Context()
	_, err := f.service.Submit(ctx, f.grace, f.dmReport())
	require.Error(t, err)
	_, err = f.service.ServerReports(ctx, f.serverID, f.ada, StatusOpen)
	require.Error(t, err)
	require.Error(t, f.service.ResolveServerReport(ctx, Action{ServerID: f.serverID, UserID: f.ada, ReportID: uuid.New()}))
	require.Error(t, f.service.EscalateServerReport(ctx, Action{ServerID: f.serverID, UserID: f.ada, ReportID: uuid.New()}))
	f.moderation.err = errors.New("servers are down")
	_, err = f.service.ServerReports(ctx, f.serverID, f.ada, StatusOpen)
	require.Error(t, err)
	assert.NotErrorIs(t, delivery.ErrNotFound, err)
}

func withSender(found delivery.StoredCopy, sender uuid.UUID) delivery.StoredCopy {
	found.SenderID = &sender
	return found
}
