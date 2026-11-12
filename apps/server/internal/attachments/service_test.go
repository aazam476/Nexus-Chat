package attachments

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	accountsstore "github.com/aazam476/nexus-chat/apps/server/internal/accounts/store"
	"github.com/aazam476/nexus-chat/apps/server/internal/attachments/store"
	"github.com/aazam476/nexus-chat/apps/server/internal/conversations"
	conversationsstore "github.com/aazam476/nexus-chat/apps/server/internal/conversations/store"
	"github.com/aazam476/nexus-chat/apps/server/internal/platform/clock"
	"github.com/aazam476/nexus-chat/apps/server/internal/platform/database"
	"github.com/aazam476/nexus-chat/apps/server/internal/platform/databasetest"
	"github.com/aazam476/nexus-chat/apps/server/internal/platform/filestore"
	"github.com/aazam476/nexus-chat/apps/server/internal/social"
	socialstore "github.com/aazam476/nexus-chat/apps/server/internal/social/store"
)

var now = time.Date(2026, 11, 12, 12, 0, 0, 0, time.UTC) //nolint:gochecknoglobals // test fixture

type quiet struct{}

func (quiet) RelationshipChanged(context.Context, uuid.UUID, uuid.UUID, string) {}
func (quiet) ConversationChanged(context.Context, []uuid.UUID, uuid.UUID)       {}
func (quiet) MemberLeft(context.Context, uuid.UUID, uuid.UUID) error            { return nil }

// limiter allows while allow is true.
type limiter struct{ allow bool }

func (fake limiter) Allow(string) bool { return fake.allow }

// fixedLimit is the instance's attachment limit, or a failure.
type fixedLimit struct {
	limit int64
	err   error
}

func (fake fixedLimit) AttachmentMaxBytes(context.Context) (int64, error) {
	return fake.limit, fake.err
}

type fixture struct {
	service       *Service
	dependencies  Dependencies
	conversations *conversations.Service
	social        *social.Service
	accounts      *accountsstore.Queries
}

func newFixture(t *testing.T) fixture {
	t.Helper()
	pool := databasetest.NewPool(t)
	require.NoError(t, database.Migrate(context.Background(), pool, slog.New(slog.DiscardHandler)))
	system := clock.Fixed{Time: now}
	socialService := social.NewService(social.Dependencies{Store: socialstore.New(pool), Notifier: quiet{}, Servers: social.NoServers{}, Limiter: limiter{true}, Clock: system})
	conversationService := conversations.NewService(conversations.Dependencies{
		Store: conversationsstore.NewWriter(pool), Policy: socialService, Notifier: quiet{}, Members: quiet{}, Clock: system,
	})
	dependencies := Dependencies{
		Store: store.New(pool), Files: filestore.NewLocal(t.TempDir()), Members: conversationService,
		Limits: fixedLimit{limit: 1 << 20}, Limiter: limiter{true}, Clock: system,
	}
	return fixture{service: NewService(dependencies), dependencies: dependencies, conversations: conversationService, social: socialService, accounts: accountsstore.New(pool)}
}

func (f fixture) user(t *testing.T, name string) uuid.UUID {
	t.Helper()
	id := uuid.New()
	require.NoError(t, f.accounts.InsertUser(t.Context(), accountsstore.InsertUserParams{ID: id, Username: name, DisplayName: name, Email: name + "@example.com", CreatedAt: now}))
	require.NoError(t, f.social.SetPrivacy(t.Context(), id, social.Privacy{WhoCanDirectMessage: social.DirectMessagesFromEveryone, WhoCanFriendRequest: social.FriendRequestsFromEveryone}))
	return id
}

func (f fixture) with(change func(*Dependencies)) *Service {
	dependencies := f.dependencies
	change(&dependencies)
	return NewService(dependencies)
}

func TestMembersUploadAndDownloadEncryptedFiles(t *testing.T) {
	f := newFixture(t)
	ada, grace, mallory := f.user(t, "ada"), f.user(t, "grace"), f.user(t, "mallory")
	dm, _, err := f.conversations.OpenDirectMessage(t.Context(), ada, grace)
	require.NoError(t, err)

	stored, err := f.service.Upload(t.Context(), ada, Upload{ConversationID: dm.ID, Size: 10, Content: strings.NewReader("ciphertext")})
	require.NoError(t, err)
	assert.Equal(t, int64(10), stored.Size)
	download, err := f.service.Open(t.Context(), grace, dm.ID, stored.ID)
	require.NoError(t, err)
	content, err := io.ReadAll(download.Content)
	require.NoError(t, err)
	require.NoError(t, download.Content.Close())
	assert.Equal(t, "ciphertext", string(content))
	assert.Equal(t, int64(10), download.Size)

	_, err = f.service.Open(t.Context(), mallory, dm.ID, stored.ID)
	require.ErrorIs(t, err, ErrNotFound, "outsiders cannot download")
	_, err = f.service.Open(t.Context(), ada, dm.ID, uuid.New())
	require.ErrorIs(t, err, ErrNotFound)
	_, err = f.service.Upload(t.Context(), mallory, Upload{ConversationID: dm.ID, Size: 10, Content: strings.NewReader("ciphertext")})
	require.ErrorIs(t, err, ErrNotAllowed)
}

func TestUploadsAreCheckedAgainstTheLimitAndTheirSize(t *testing.T) {
	f := newFixture(t)
	ada, grace := f.user(t, "ada"), f.user(t, "grace")
	dm, _, err := f.conversations.OpenDirectMessage(t.Context(), ada, grace)
	require.NoError(t, err)
	upload := func(service *Service, size int64, content string) error {
		_, err := service.Upload(t.Context(), ada, Upload{ConversationID: dm.ID, Size: size, Content: strings.NewReader(content)})
		return err
	}

	require.ErrorIs(t, upload(f.service, 0, ""), ErrInvalidSize)
	require.ErrorIs(t, upload(f.service, CiphertextAllowance(1<<20)+1, "x"), ErrTooLarge)
	require.ErrorIs(t, upload(f.service, 5, "shorter"), ErrInvalidSize, "longer than declared")
	require.ErrorIs(t, upload(f.service, 8, "short"), ErrInvalidSize, "shorter than declared")
	require.ErrorIs(t, upload(f.with(func(d *Dependencies) { d.Limiter = limiter{false} }), 1, "x"), ErrRateLimited)
	require.ErrorIs(t, upload(f.with(func(d *Dependencies) { d.Limits = fixedLimit{err: errFailure} }), 1, "x"), errFailure)
	assert.Equal(t, int64((1<<20)+24+17*2), CiphertextAllowance(1<<20))
}

var errFailure = errors.New("down")

// failingFiles fails the named file operation.
type failingFiles struct {
	fileStore
	failOn string
}

func (files failingFiles) SaveStream(ctx context.Context, key string, content io.Reader, limit int64) (int64, error) {
	if files.failOn == "SaveStream" {
		return 0, errFailure
	}
	return files.fileStore.SaveStream(ctx, key, content, limit)
}

func (files failingFiles) Open(ctx context.Context, key string) (io.ReadCloser, error) {
	if files.failOn == "Open" {
		return nil, errFailure
	}
	return files.fileStore.Open(ctx, key)
}

// failingStore fails every query.
type failingStore struct{}

func (failingStore) InsertAttachment(context.Context, store.InsertAttachmentParams) error {
	return errFailure
}
func (failingStore) AttachmentSize(context.Context, store.AttachmentSizeParams) (int64, error) {
	return 0, errFailure
}

// failingMembers fails every membership question.
type failingMembers struct{}

func (failingMembers) IsMember(context.Context, uuid.UUID, uuid.UUID) (bool, error) {
	return false, errFailure
}
func (failingMembers) MaySend(context.Context, uuid.UUID, uuid.UUID) (bool, error) {
	return false, errFailure
}

func TestFailuresAreReported(t *testing.T) {
	f := newFixture(t)
	ada, grace := f.user(t, "ada"), f.user(t, "grace")
	dm, _, err := f.conversations.OpenDirectMessage(t.Context(), ada, grace)
	require.NoError(t, err)
	stored, err := f.service.Upload(t.Context(), ada, Upload{ConversationID: dm.ID, Size: 1, Content: strings.NewReader("x")})
	require.NoError(t, err)

	for name, service := range map[string]*Service{
		"save":    f.with(func(d *Dependencies) { d.Files = failingFiles{d.Files, "SaveStream"} }),
		"store":   f.with(func(d *Dependencies) { d.Store = failingStore{} }),
		"members": f.with(func(d *Dependencies) { d.Members = failingMembers{} }),
	} {
		_, err := service.Upload(t.Context(), ada, Upload{ConversationID: dm.ID, Size: 1, Content: strings.NewReader("x")})
		require.ErrorIs(t, err, errFailure, name)
	}
	for name, service := range map[string]*Service{
		"open":    f.with(func(d *Dependencies) { d.Files = failingFiles{d.Files, "Open"} }),
		"store":   f.with(func(d *Dependencies) { d.Store = failingStore{} }),
		"members": f.with(func(d *Dependencies) { d.Members = failingMembers{} }),
	} {
		_, err := service.Open(t.Context(), ada, dm.ID, stored.ID)
		require.ErrorIs(t, err, errFailure, name)
	}
}
