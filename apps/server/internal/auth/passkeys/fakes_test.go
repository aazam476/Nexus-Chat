package passkeys

import (
	"bytes"
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	accountsstore "github.com/aazam476/nexus-chat/apps/server/internal/accounts/store"
	"github.com/aazam476/nexus-chat/apps/server/internal/auth/passkeys/store"
)

type fakeStore struct {
	credentials map[uuid.UUID]store.WebauthnCredential
	ceremonies  map[uuid.UUID]store.WebauthnCeremony
	fail        error
	failOn      string
}

func newFakeStore() *fakeStore {
	return &fakeStore{credentials: map[uuid.UUID]store.WebauthnCredential{}, ceremonies: map[uuid.UUID]store.WebauthnCeremony{}}
}

func (fake *fakeStore) failure(method string) error {
	if fake.failOn == "" || fake.failOn == method {
		return fake.fail
	}
	return nil
}

func (fake *fakeStore) InsertCredential(_ context.Context, arg store.InsertCredentialParams) error {
	if err := fake.failure("InsertCredential"); err != nil {
		return err
	}
	fake.credentials[arg.ID] = store.WebauthnCredential{
		ID: arg.ID, UserID: arg.UserID, CredentialID: arg.CredentialID, Credential: arg.Credential,
		Kind: arg.Kind, Name: arg.Name, CreatedAt: arg.CreatedAt,
	}
	return nil
}

func (fake *fakeStore) ListCredentials(_ context.Context, userID uuid.UUID) ([]store.WebauthnCredential, error) {
	if err := fake.failure("ListCredentials"); err != nil {
		return nil, err
	}
	var rows []store.WebauthnCredential
	for _, row := range fake.credentials {
		if row.UserID == userID {
			rows = append(rows, row)
		}
	}
	return rows, nil
}

func (fake *fakeStore) UpdateCredentialAfterUse(_ context.Context, arg store.UpdateCredentialAfterUseParams) error {
	if err := fake.failure("UpdateCredentialAfterUse"); err != nil {
		return err
	}
	for id, row := range fake.credentials {
		if bytes.Equal(row.CredentialID, arg.CredentialID) {
			row.Credential, row.LastUsedAt = arg.Credential, arg.LastUsedAt
			fake.credentials[id] = row
		}
	}
	return nil
}

func (fake *fakeStore) DeleteCredential(_ context.Context, arg store.DeleteCredentialParams) (int64, error) {
	if err := fake.failure("DeleteCredential"); err != nil {
		return 0, err
	}
	row, ok := fake.credentials[arg.ID]
	if !ok || row.UserID != arg.UserID {
		return 0, nil
	}
	delete(fake.credentials, arg.ID)
	return 1, nil
}

func (fake *fakeStore) InsertCeremony(_ context.Context, arg store.InsertCeremonyParams) error {
	if err := fake.failure("InsertCeremony"); err != nil {
		return err
	}
	fake.ceremonies[arg.ID] = store.WebauthnCeremony(arg)
	return nil
}

func (fake *fakeStore) TakeCeremony(_ context.Context, arg store.TakeCeremonyParams) (store.WebauthnCeremony, error) {
	if err := fake.failure("TakeCeremony"); err != nil {
		return store.WebauthnCeremony{}, err
	}
	row, ok := fake.ceremonies[arg.ID]
	if !ok || row.Purpose != arg.Purpose || !row.ExpiresAt.After(arg.Now) {
		return store.WebauthnCeremony{}, pgx.ErrNoRows
	}
	delete(fake.ceremonies, arg.ID)
	return row, nil
}

type fakeUsers struct {
	users map[uuid.UUID]accountsstore.User
	err   error
}

func (fake *fakeUsers) GetUserByID(_ context.Context, id uuid.UUID) (accountsstore.User, error) {
	user, ok := fake.users[id]
	if !ok {
		return accountsstore.User{}, pgx.ErrNoRows
	}
	return user, fake.err
}

type fixedDomain struct {
	domain string
	err    error
}

func (fake fixedDomain) Domain(context.Context) (string, error) { return fake.domain, fake.err }

// switchableDomain lets a test break the domain between a ceremony's start
// and finish.
type switchableDomain struct{ fixedDomain }

func (domain *switchableDomain) Domain(context.Context) (string, error) {
	return domain.domain, domain.err
}

type movableClock struct{ now time.Time }

func (clock *movableClock) Now() time.Time { return clock.now }
