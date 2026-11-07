package setup

import (
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/aazam476/nexus-chat/apps/server/internal/instanceconfig"
	"github.com/aazam476/nexus-chat/apps/server/internal/platform/validation"
	"github.com/aazam476/nexus-chat/apps/server/internal/setup/store"
)

func TestStartLogsAGroupedToken(t *testing.T) {
	_, token := started(t, &fakeStore{}, nil)

	assert.Regexp(t, `^([A-Z2-7]{4}-){6}[A-Z2-7]{2}$`, token)
}

func TestStartDoesNothingWhenComplete(t *testing.T) {
	_, token := started(t, &fakeStore{complete: true}, nil)

	assert.Empty(t, token)
}

func TestStartReportsStoreErrors(t *testing.T) {
	service := NewService(Dependencies{Store: &fakeStore{readError: errBoom}})

	assert.ErrorIs(t, service.Start(t.Context()), errBoom)
}

func TestCompleteSavesAdminAndSettings(t *testing.T) {
	fake := &fakeStore{}
	service, token := started(t, fake, nil)

	// The token is accepted in lower case and without dashes.
	adminID, err := service.Complete(t.Context(), validRequest(strings.ToLower(strings.ReplaceAll(token, "-", ""))), "198.51.100.1")

	require.NoError(t, err)
	require.Len(t, fake.saved, 1)
	saved := fake.saved[0]
	assert.Equal(t, adminID, saved.Admin.ID)
	assert.Equal(t, uuid.Version(7), saved.Admin.ID.Version())
	assert.Equal(t, "admin@example.com", saved.Admin.Email)
	assert.True(t, saved.Admin.IsInstanceAdmin)
	assert.Equal(t, "hashed:a long password", *saved.Admin.PasswordHash)
	assert.Equal(t, testTime, saved.Admin.CreatedAt)
	assert.Equal(t, testTime.Truncate(1000), saved.CompletedAt)
	assert.Equal(t, map[string]string{
		instanceconfig.KeyInstanceName:     "Example Chat",
		instanceconfig.KeyRegistrationMode: "invite_only",
		instanceconfig.KeyInstanceDomain:   "chat.example.com",
	}, saved.Settings)
}

func TestCompleteWorksOnlyOnce(t *testing.T) {
	service, token := started(t, &fakeStore{}, nil)
	_, err := service.Complete(t.Context(), validRequest(token), "client")
	require.NoError(t, err)

	_, err = service.Complete(t.Context(), validRequest(token), "client")

	assert.ErrorIs(t, err, ErrAlreadyComplete)
}

func TestCompleteRejectsWrongToken(t *testing.T) {
	fake := &fakeStore{}
	service, _ := started(t, fake, nil)

	_, err := service.Complete(t.Context(), validRequest("AAAA-BBBB"), "client")

	require.ErrorIs(t, err, ErrInvalidToken)
	assert.Empty(t, fake.saved)
}

func TestCompleteIsRateLimitedBeforeTheTokenIsChecked(t *testing.T) {
	service, token := started(t, &fakeStore{}, func(d *Dependencies) { d.Limiter = fakeLimiter{deny: true} })

	_, err := service.Complete(t.Context(), validRequest(token), "client")

	assert.ErrorIs(t, err, ErrRateLimited)
}

func TestCompleteReportsEveryInvalidField(t *testing.T) {
	service, token := started(t, &fakeStore{}, nil)
	request := Request{Token: token, InstanceName: " padded", Domain: "https://x", RegistrationMode: "everyone"}

	_, err := service.Complete(t.Context(), request, "client")

	var problems *validation.Error
	require.ErrorAs(t, err, &problems)
	var fields []string
	for _, field := range problems.Fields {
		fields = append(fields, field.Field)
	}
	assert.Equal(t, []string{
		"admin.username", "admin.displayName", "admin.email", "admin.password",
		"instance.name", "instance.domain", "instance.registrationMode",
	}, fields)
}

func TestInstanceNameRules(t *testing.T) {
	assert.NoError(t, validateInstanceName("A"))
	assert.NoError(t, validateInstanceName(strings.Repeat("é", 64)))
	for _, name := range []string{"", strings.Repeat("é", 65), "trailing ", "bell\u0007"} {
		assert.Error(t, validateInstanceName(name), name) //nolint:testifylint // each name is checked independently
	}
}

func TestEnvironmentDomainWins(t *testing.T) {
	fake := &fakeStore{}
	service, token := started(t, fake, func(d *Dependencies) { d.EnvironmentDomain = "env.example.com" })
	request := validRequest(token)
	request.Domain = "not a domain, ignored"

	_, err := service.Complete(t.Context(), request, "client")

	require.NoError(t, err)
	assert.NotContains(t, fake.saved[0].Settings, instanceconfig.KeyInstanceDomain)
}

func TestEmptyDomainIsNotSaved(t *testing.T) {
	fake := &fakeStore{}
	service, token := started(t, fake, nil)
	request := validRequest(token)
	request.Domain = ""

	_, err := service.Complete(t.Context(), request, "client")

	require.NoError(t, err)
	assert.NotContains(t, fake.saved[0].Settings, instanceconfig.KeyInstanceDomain)
}

func TestCompletePassesOnFailures(t *testing.T) {
	tests := map[string]struct {
		change func(*Dependencies)
		store  *fakeStore
		want   error
	}{
		"hashing fails":       {change: func(d *Dependencies) { d.Hasher = fakeHasher{err: errBoom} }, store: &fakeStore{}, want: errBoom},
		"store fails":         {store: &fakeStore{completeError: errBoom}, want: errBoom},
		"completed elsewhere": {store: &fakeStore{completeError: store.ErrAlreadyComplete}, want: ErrAlreadyComplete},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			service, token := started(t, test.store, test.change)

			_, err := service.Complete(t.Context(), validRequest(token), "client")

			assert.ErrorIs(t, err, test.want)
		})
	}
}

func TestStatus(t *testing.T) {
	tests := map[string]struct {
		store       *fakeStore
		environment string
		want        Status
	}{
		"fresh": {store: &fakeStore{}, want: Status{Required: true}},
		"done with stored domain": {store: &fakeStore{complete: true, stored: map[string]string{instanceconfig.KeyInstanceDomain: "chat.example.com"}},
			want: Status{Domain: "chat.example.com"}},
		"environment domain": {store: &fakeStore{stored: map[string]string{instanceconfig.KeyInstanceDomain: "ignored.example.com"}},
			environment: "env.example.com", want: Status{Required: true, Domain: "env.example.com", DomainFromEnvironment: true}},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			service := NewService(Dependencies{Store: test.store, EnvironmentDomain: test.environment})

			status, err := service.Status(t.Context())

			require.NoError(t, err)
			assert.Equal(t, test.want, status)
		})
	}
}

func TestStatusReportsStoreErrors(t *testing.T) {
	_, err := NewService(Dependencies{Store: &fakeStore{readError: errBoom}}).Status(t.Context())

	assert.ErrorIs(t, err, errBoom)
}
