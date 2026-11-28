package reports

import (
	"context"
	"crypto/ed25519"
	"encoding/binary"
	"log/slog"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	accountsstore "github.com/aazam476/nexus-chat/apps/server/internal/accounts/store"
	"github.com/aazam476/nexus-chat/apps/server/internal/delivery"
	"github.com/aazam476/nexus-chat/apps/server/internal/platform/clock"
	"github.com/aazam476/nexus-chat/apps/server/internal/platform/database"
	"github.com/aazam476/nexus-chat/apps/server/internal/platform/databasetest"
	"github.com/aazam476/nexus-chat/apps/server/internal/reports/store"
)

var now = time.Date(2026, 11, 20, 12, 0, 0, 0, time.UTC) //nolint:gochecknoglobals // test fixture

// copies answers with the stored copies a test put there.
type copies struct {
	byMessage map[uuid.UUID]delivery.StoredCopy
	err       error
	asked     []uuid.UUID
}

func (fake *copies) ChannelCopy(_ context.Context, ref delivery.ChannelRef, messageID uuid.UUID) (delivery.StoredCopy, error) {
	fake.asked = append(fake.asked, ref.UserID)
	found, ok := fake.byMessage[messageID]
	if fake.err != nil {
		return delivery.StoredCopy{}, fake.err
	}
	if !ok || found.ChannelID == nil || *found.ChannelID != ref.ChannelID {
		return delivery.StoredCopy{}, delivery.ErrNoMessage
	}
	return found, nil
}

func (fake *copies) ConversationCopy(_ context.Context, me uuid.UUID, conversationID uuid.UUID, messageID uuid.UUID) (delivery.StoredCopy, error) {
	fake.asked = append(fake.asked, me)
	found, ok := fake.byMessage[messageID]
	if fake.err != nil {
		return delivery.StoredCopy{}, fake.err
	}
	if !ok || found.ConversationID != conversationID || found.ChannelID != nil {
		return delivery.StoredCopy{}, delivery.ErrNotFound
	}
	return found, nil
}

// moderation lets the users in `moderators` handle reports, knows only `members`.
type moderation struct {
	moderators map[uuid.UUID]bool
	members    map[uuid.UUID]bool
	handled    []Handled
	err        error
}

func (fake *moderation) MayHandleReports(_ context.Context, _ uuid.UUID, userID uuid.UUID) (bool, error) {
	if fake.err != nil {
		return false, fake.err
	}
	if !fake.members[userID] {
		return false, ErrNoServer
	}
	return fake.moderators[userID], nil
}

func (fake *moderation) ReportHandled(_ context.Context, handled Handled) error {
	fake.handled = append(fake.handled, handled)
	return fake.err
}

type settings struct {
	everything bool
	err        error
}

func (fake *settings) ReportsToAdmins(context.Context) (bool, error) {
	return fake.everything, fake.err
}

// fixture is a server with a channel in its audience, and a DM, with a
// stored copy of a message from Sam in each.
type fixture struct {
	service                         *Service
	copies                          *copies
	moderation                      *moderation
	settings                        *settings
	key                             ed25519.PublicKey
	pool                            *pgxpool.Pool
	accounts                        *accountsstore.Queries
	ada, grace, sam                 uuid.UUID
	serverID, channelID, audienceID uuid.UUID
	dmID, channelMessage, dmMessage uuid.UUID
}

func newFixture(t *testing.T) fixture {
	t.Helper()
	pool := databasetest.NewPool(t)
	require.NoError(t, database.Migrate(context.Background(), pool, slog.New(slog.DiscardHandler)))
	public, private, err := ed25519.GenerateKey(nil)
	require.NoError(t, err)
	f := fixture{pool: pool, key: public, accounts: accountsstore.New(pool), copies: &copies{byMessage: map[uuid.UUID]delivery.StoredCopy{}}, settings: &settings{}}
	f.ada, f.grace, f.sam = f.user(t, "ada"), f.user(t, "grace"), f.user(t, "sam")
	f.moderation = &moderation{moderators: map[uuid.UUID]bool{f.ada: true}, members: map[uuid.UUID]bool{f.ada: true, f.grace: true, f.sam: true}}
	f.serverID, f.channelID, f.audienceID, f.dmID = uuid.New(), uuid.New(), uuid.New(), uuid.New()
	f.exec(t, "INSERT INTO servers (id, name, owner_id, created_at) VALUES ($1, 'Club', $2, $3)", f.serverID, f.ada, now)
	f.exec(t, "INSERT INTO conversations (id, kind, mls_group_id, server_id, created_at) VALUES ($1, 'audience', $2, $3, $4)", f.audienceID, []byte("audience-group-1"), f.serverID, now)
	f.exec(t, "INSERT INTO server_channels (id, server_id, kind, name, position, created_at, audience_id) VALUES ($1, $2, 'text', 'general', 0, $3, $4)", f.channelID, f.serverID, now, f.audienceID)
	f.exec(t, "INSERT INTO conversations (id, kind, mls_group_id, created_at) VALUES ($1, 'group', $2, $3)", f.dmID, []byte("group-0000000001"), now)
	f.channelMessage, f.dmMessage = uuid.New(), uuid.New()
	f.copies.byMessage[f.channelMessage] = delivery.StoredCopy{
		ConversationID: f.audienceID, ChannelID: &f.channelID, MessageID: f.channelMessage, Epoch: 3, SenderID: &f.sam, Sealed: []byte("sealed"), CreatedAt: now.Add(-time.Hour),
	}
	f.copies.byMessage[f.dmMessage] = delivery.StoredCopy{
		ConversationID: f.dmID, MessageID: f.dmMessage, Epoch: 1, SenderID: &f.sam, Sealed: []byte("sealed dm"), CreatedAt: now.Add(-time.Minute),
	}
	f.service = NewService(Dependencies{Store: store.New(pool), Copies: f.copies, Moderation: f.moderation, Settings: f.settings, Users: f.accounts, Key: private, Clock: clock.Fixed{Time: now}})
	return f
}

func (f fixture) exec(t *testing.T, sql string, args ...any) {
	t.Helper()
	_, err := f.pool.Exec(t.Context(), sql, args...)
	require.NoError(t, err)
}

func (f fixture) user(t *testing.T, name string) uuid.UUID {
	t.Helper()
	id := uuid.New()
	require.NoError(t, f.accounts.InsertUser(t.Context(), accountsstore.InsertUserParams{ID: id, Username: name, DisplayName: name, Email: name + "@example.com", CreatedAt: now}))
	return id
}

// prefixed is length-prefixed fields, as the crypto package writes them.
func prefixed(parts ...[]byte) []byte {
	out := []byte{}
	for _, part := range parts {
		out = binary.BigEndian.AppendUint32(out, uint32(len(part))) //nolint:gosec // test data is small
		out = append(out, part...)
	}
	return out
}

// evidenceFor is well-formed evidence about a message at a place and epoch.
func evidenceFor(place uuid.UUID, message uuid.UUID, epoch uint64) []byte {
	return prefixed([]byte(evidenceLabel), []byte(place.String()), []byte(message.String()), binary.BigEndian.AppendUint64(nil, epoch),
		[]byte("you are all fools"), []byte("@sam@example.com/d1"), make([]byte, 32), make([]byte, 64))
}

func (f fixture) channelReport(comment string) NewReport {
	return NewReport{
		Target:   Target{ServerID: &f.serverID, ChannelID: &f.channelID, MessageID: f.channelMessage},
		Evidence: evidenceFor(f.channelID, f.channelMessage, 3), Comment: comment,
	}
}

func (f fixture) dmReport() NewReport {
	return NewReport{Target: Target{ConversationID: &f.dmID, MessageID: f.dmMessage}, Evidence: evidenceFor(f.dmID, f.dmMessage, 1)}
}
