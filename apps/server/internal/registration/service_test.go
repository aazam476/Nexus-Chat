package registration

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/aazam476/nexus-chat/apps/server/internal/accounts"
	accountsstore "github.com/aazam476/nexus-chat/apps/server/internal/accounts/store"
	"github.com/aazam476/nexus-chat/apps/server/internal/auth/session"
	sessionstore "github.com/aazam476/nexus-chat/apps/server/internal/auth/session/store"
	"github.com/aazam476/nexus-chat/apps/server/internal/platform/clock"
	"github.com/aazam476/nexus-chat/apps/server/internal/platform/validation"
	"github.com/aazam476/nexus-chat/apps/server/internal/registration/store"
)

var now = time.Date(2026, 11, 7, 12, 0, 0, 123456789, time.UTC)

type fakeCreator struct {
	saved []store.NewAccount
	err   error
}

func (fake *fakeCreator) CreateAccount(_ context.Context, account store.NewAccount) error {
	fake.saved = append(fake.saved, account)
	return fake.err
}

type acceptAll struct{}

func (acceptAll) Check(string) error { return nil }

type fakeHasher struct{ err error }

func (fake fakeHasher) Hash(_ context.Context, password string) (string, error) {
	return "hashed:" + password, fake.err
}

type fakeSessions struct{ err error }

func (fake fakeSessions) Create(_ context.Context, userID uuid.UUID, _ session.Device) (string, sessionstore.Session, error) {
	return "nxs_token", sessionstore.Session{UserID: userID}, fake.err
}

type fakeEmails struct{ err error }

func (fake fakeEmails) SendVerification(context.Context, accountsstore.User) error { return fake.err }

type fakeMode struct {
	mode string
	err  error
}

func (fake fakeMode) RegistrationMode(context.Context) (string, error) { return fake.mode, fake.err }

type fakeLimiter struct{ deny bool }

func (fake fakeLimiter) Allow(string) bool { return !fake.deny }

type fixture struct {
	creator *fakeCreator
	logs    *bytes.Buffer
	service *Service
}

func newFixture(mode string, change func(*Dependencies)) fixture {
	setup := fixture{creator: &fakeCreator{}, logs: &bytes.Buffer{}}
	dependencies := Dependencies{
		Accounts: setup.creator, Validator: accounts.NewValidator(acceptAll{}), Hasher: fakeHasher{},
		Sessions: fakeSessions{}, Emails: fakeEmails{}, Settings: fakeMode{mode: mode}, Limiter: fakeLimiter{},
		Clock: clock.Fixed{Time: now}, Logger: slog.New(slog.NewTextHandler(setup.logs, nil)),
	}
	if change != nil {
		change(&dependencies)
	}
	setup.service = NewService(dependencies)
	return setup
}

func request(invite string) Request {
	return Request{
		Account:    accounts.NewAccount{Username: "bob", DisplayName: "Bob", Email: "Bob@Example.com", Password: "a long password"},
		InviteCode: invite,
		Device:     session.Device{Name: "Phone"},
		ClientKey:  "198.51.100.2",
	}
}

func TestOpenRegistration(t *testing.T) {
	setup := newFixture("open", nil)

	result, err := setup.service.Register(t.Context(), request(""))

	require.NoError(t, err)
	assert.Equal(t, "nxs_token", result.Token)
	assert.Equal(t, "bob@example.com", result.User.Email)
	assert.False(t, result.User.IsInstanceAdmin)
	require.Len(t, setup.creator.saved, 1)
	saved := setup.creator.saved[0]
	assert.Nil(t, saved.InviteHash)
	assert.Equal(t, "hashed:a long password", *saved.User.PasswordHash)
	assert.Equal(t, now.Truncate(time.Microsecond), saved.User.CreatedAt)
}

func TestModes(t *testing.T) {
	tests := []struct {
		mode    string
		invite  string
		want    error
		invited bool
	}{
		{mode: "closed", invite: "ABCD", want: ErrRegistrationClosed},
		{mode: "invite_only", want: ErrInviteRequired},
		{mode: "invite_only", invite: "abcd-efgh", invited: true},
		{mode: "open", invite: "abcd-efgh", invited: true},
	}
	for _, test := range tests {
		t.Run(test.mode+"/"+test.invite, func(t *testing.T) {
			setup := newFixture(test.mode, nil)

			_, err := setup.service.Register(t.Context(), request(test.invite))

			require.ErrorIs(t, err, test.want)
			if test.invited {
				expected := hashInviteCode("ABCDEFGH")
				assert.Equal(t, expected[:], setup.creator.saved[0].InviteHash, "codes are normalized before hashing")
			}
		})
	}
}

func TestStoreOutcomes(t *testing.T) {
	failure := errors.New("database down")
	tests := map[string]struct {
		storeErr  error
		want      error
		wantField string
	}{
		"invalid invite": {storeErr: store.ErrInvalidInvite, want: ErrInvalidInvite},
		"username taken": {storeErr: store.ErrUsernameTaken, wantField: "username"},
		"email taken":    {storeErr: store.ErrEmailTaken, wantField: "email"},
		"other failure":  {storeErr: failure, want: failure},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			setup := newFixture("open", func(d *Dependencies) { d.Accounts = &fakeCreator{err: test.storeErr} })

			_, err := setup.service.Register(t.Context(), request(""))

			if test.wantField != "" {
				var problems *validation.Error
				require.ErrorAs(t, err, &problems)
				assert.Equal(t, test.wantField, problems.Fields[0].Field)
				return
			}
			assert.ErrorIs(t, err, test.want)
		})
	}
}

func TestEmailFailureDoesNotFailRegistration(t *testing.T) {
	setup := newFixture("open", func(d *Dependencies) { d.Emails = fakeEmails{err: errors.New("SMTP not set up")} })

	_, err := setup.service.Register(t.Context(), request(""))

	require.NoError(t, err)
	assert.Contains(t, setup.logs.String(), "SMTP not set up")
}

func TestOtherFailures(t *testing.T) {
	failure := errors.New("failure")
	tests := map[string]func(*Dependencies){
		"rate limited": func(d *Dependencies) { d.Limiter = fakeLimiter{deny: true} },
		"mode unknown": func(d *Dependencies) { d.Settings = fakeMode{err: failure} },
		"hashing":      func(d *Dependencies) { d.Hasher = fakeHasher{err: failure} },
		"session":      func(d *Dependencies) { d.Sessions = fakeSessions{err: failure} },
	}
	for name, change := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := newFixture("open", change).service.Register(t.Context(), request(""))

			assert.Error(t, err)
		})
	}
}

func TestInvalidAccountIsRejected(t *testing.T) {
	setup := newFixture("open", nil)
	bad := request("")
	bad.Account.Username = "X"

	_, err := setup.service.Register(t.Context(), bad)

	var problems *validation.Error
	require.ErrorAs(t, err, &problems)
	assert.Empty(t, setup.creator.saved)
}
