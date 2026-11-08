package oidc

import (
	"bytes"
	"context"
	"errors"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/aazam476/nexus-chat/apps/server/internal/auth/oidc/store"
)

// movableClock is a clock tests can move forward.
type movableClock struct {
	mutex sync.Mutex
	now   time.Time
}

func (clock *movableClock) Now() time.Time {
	clock.mutex.Lock()
	defer clock.mutex.Unlock()
	return clock.now
}

// errFailure is what failing fakes return.
var errFailure = errors.New("database down")

// fakeStore keeps providers, flows and identities in memory and behaves
// like the SQL in store/queries.sql. failOn makes the named method fail.
type fakeStore struct {
	mutex      sync.Mutex
	providers  map[string]store.OidcProvider
	flows      map[uuid.UUID]store.OidcFlow
	identities map[[2]string]store.OidcIdentity
	usernames  map[string]bool
	// otherMethods is what CountOtherLoginMethods returns.
	otherMethods int32
	failOn       map[string]error
	// duplicateOn makes InsertProvider or InsertIdentity fail with a
	// unique violation of this constraint.
	duplicateOn string
	// recordNothing makes RecordCallback update no rows.
	recordNothing bool
	// deleteNothing makes UpdateProvider and DeleteIdentity touch no rows.
	deleteNothing bool
}

func newFakeStore() *fakeStore {
	return &fakeStore{
		providers: map[string]store.OidcProvider{}, flows: map[uuid.UUID]store.OidcFlow{},
		identities: map[[2]string]store.OidcIdentity{}, usernames: map[string]bool{}, failOn: map[string]error{},
	}
}

func (fake *fakeStore) fail(method string) error {
	return fake.failOn[method]
}

func duplicate(constraint string) error {
	return &pgconn.PgError{Code: "23505", ConstraintName: constraint}
}

func (fake *fakeStore) ListProviders(context.Context) ([]store.OidcProvider, error) {
	if err := fake.fail("ListProviders"); err != nil {
		return nil, err
	}
	fake.mutex.Lock()
	defer fake.mutex.Unlock()
	rows := make([]store.OidcProvider, 0, len(fake.providers))
	for _, row := range fake.providers {
		rows = append(rows, row)
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].DisplayName < rows[j].DisplayName })
	return rows, nil
}

func (fake *fakeStore) GetProvider(_ context.Context, id string) (store.OidcProvider, error) {
	if err := fake.fail("GetProvider"); err != nil {
		return store.OidcProvider{}, err
	}
	fake.mutex.Lock()
	defer fake.mutex.Unlock()
	row, found := fake.providers[id]
	if !found {
		return store.OidcProvider{}, pgx.ErrNoRows
	}
	return row, nil
}

func (fake *fakeStore) InsertProvider(_ context.Context, arg store.InsertProviderParams) error {
	if err := fake.fail("InsertProvider"); err != nil {
		return err
	}
	if fake.duplicateOn == store.ProviderKey {
		return duplicate(store.ProviderKey)
	}
	fake.mutex.Lock()
	defer fake.mutex.Unlock()
	fake.providers[arg.ID] = store.OidcProvider{
		ID: arg.ID, DisplayName: arg.DisplayName, Issuer: arg.Issuer, ClientID: arg.ClientID,
		ClientSecretSealed: arg.ClientSecretSealed, Scopes: arg.Scopes, AllowSignup: arg.AllowSignup,
		CreatedAt: arg.CreatedAt, UpdatedAt: arg.CreatedAt,
	}
	return nil
}

func (fake *fakeStore) UpdateProvider(_ context.Context, arg store.UpdateProviderParams) (int64, error) {
	if err := fake.fail("UpdateProvider"); err != nil {
		return 0, err
	}
	fake.mutex.Lock()
	defer fake.mutex.Unlock()
	row, found := fake.providers[arg.ID]
	if !found || fake.deleteNothing {
		return 0, nil
	}
	row.DisplayName, row.Issuer, row.ClientID, row.ClientSecretSealed = arg.DisplayName, arg.Issuer, arg.ClientID, arg.ClientSecretSealed
	row.Scopes, row.AllowSignup, row.UpdatedAt = arg.Scopes, arg.AllowSignup, arg.UpdatedAt
	fake.providers[arg.ID] = row
	return 1, nil
}

func (fake *fakeStore) DeleteProvider(_ context.Context, id string) (int64, error) {
	if err := fake.fail("DeleteProvider"); err != nil {
		return 0, err
	}
	fake.mutex.Lock()
	defer fake.mutex.Unlock()
	if _, found := fake.providers[id]; !found {
		return 0, nil
	}
	delete(fake.providers, id)
	return 1, nil
}

func (fake *fakeStore) InsertFlow(_ context.Context, arg store.InsertFlowParams) error {
	if err := fake.fail("InsertFlow"); err != nil {
		return err
	}
	fake.mutex.Lock()
	defer fake.mutex.Unlock()
	fake.flows[arg.ID] = store.OidcFlow{
		ID: arg.ID, ProviderID: arg.ProviderID, UserID: arg.UserID, StateHash: arg.StateHash, Nonce: arg.Nonce,
		CodeVerifier: arg.CodeVerifier, ReturnTo: arg.ReturnTo, InviteHash: arg.InviteHash, ExpiresAt: arg.ExpiresAt,
	}
	return nil
}

func (fake *fakeStore) DeleteExpiredFlows(_ context.Context, now time.Time) error {
	if err := fake.fail("DeleteExpiredFlows"); err != nil {
		return err
	}
	fake.mutex.Lock()
	defer fake.mutex.Unlock()
	for id, flow := range fake.flows {
		if !flow.ExpiresAt.After(now) {
			delete(fake.flows, id)
		}
	}
	return nil
}

func (fake *fakeStore) GetFlowByState(_ context.Context, arg store.GetFlowByStateParams) (store.OidcFlow, error) {
	if err := fake.fail("GetFlowByState"); err != nil {
		return store.OidcFlow{}, err
	}
	fake.mutex.Lock()
	defer fake.mutex.Unlock()
	for _, flow := range fake.flows {
		if bytes.Equal(flow.StateHash, arg.StateHash) && flow.CompletionHash == nil && flow.ExpiresAt.After(arg.Now) {
			return flow, nil
		}
	}
	return store.OidcFlow{}, pgx.ErrNoRows
}

func (fake *fakeStore) RecordCallback(_ context.Context, arg store.RecordCallbackParams) (int64, error) {
	if err := fake.fail("RecordCallback"); err != nil {
		return 0, err
	}
	fake.mutex.Lock()
	defer fake.mutex.Unlock()
	flow, found := fake.flows[arg.ID]
	if !found || flow.CompletionHash != nil || fake.recordNothing {
		return 0, nil
	}
	flow.CompletionHash, flow.Claims = arg.CompletionHash, arg.Claims
	fake.flows[arg.ID] = flow
	return 1, nil
}

func (fake *fakeStore) ConsumeFlow(_ context.Context, arg store.ConsumeFlowParams) (store.OidcFlow, error) {
	if err := fake.fail("ConsumeFlow"); err != nil {
		return store.OidcFlow{}, err
	}
	fake.mutex.Lock()
	defer fake.mutex.Unlock()
	flow, found := fake.flows[arg.ID]
	if !found || flow.CompletionHash == nil || !bytes.Equal(flow.CompletionHash, arg.CompletionHash) || !flow.ExpiresAt.After(arg.Now) {
		return store.OidcFlow{}, pgx.ErrNoRows
	}
	delete(fake.flows, arg.ID)
	return flow, nil
}

func (fake *fakeStore) GetIdentity(_ context.Context, arg store.GetIdentityParams) (store.OidcIdentity, error) {
	if err := fake.fail("GetIdentity"); err != nil {
		return store.OidcIdentity{}, err
	}
	fake.mutex.Lock()
	defer fake.mutex.Unlock()
	identity, found := fake.identities[[2]string{arg.ProviderID, arg.Subject}]
	if !found {
		return store.OidcIdentity{}, pgx.ErrNoRows
	}
	return identity, nil
}

func (fake *fakeStore) InsertIdentity(_ context.Context, arg store.InsertIdentityParams) error {
	if err := fake.fail("InsertIdentity"); err != nil {
		return err
	}
	if fake.duplicateOn != "" {
		return duplicate(fake.duplicateOn)
	}
	fake.mutex.Lock()
	defer fake.mutex.Unlock()
	fake.identities[[2]string{arg.ProviderID, arg.Subject}] = store.OidcIdentity(arg)
	return nil
}

func (fake *fakeStore) ListIdentitiesForUser(_ context.Context, userID uuid.UUID) ([]store.ListIdentitiesForUserRow, error) {
	if err := fake.fail("ListIdentitiesForUser"); err != nil {
		return nil, err
	}
	fake.mutex.Lock()
	defer fake.mutex.Unlock()
	var rows []store.ListIdentitiesForUserRow
	for _, identity := range fake.identities {
		if identity.UserID == userID {
			rows = append(rows, store.ListIdentitiesForUserRow{
				ProviderID: identity.ProviderID, DisplayName: fake.providers[identity.ProviderID].DisplayName,
				Email: identity.Email, CreatedAt: identity.CreatedAt,
			})
		}
	}
	return rows, nil
}

func (fake *fakeStore) DeleteIdentity(_ context.Context, arg store.DeleteIdentityParams) (int64, error) {
	if err := fake.fail("DeleteIdentity"); err != nil {
		return 0, err
	}
	fake.mutex.Lock()
	defer fake.mutex.Unlock()
	for key, identity := range fake.identities {
		if identity.ProviderID == arg.ProviderID && identity.UserID == arg.UserID && !fake.deleteNothing {
			delete(fake.identities, key)
			return 1, nil
		}
	}
	return 0, nil
}

func (fake *fakeStore) CountOtherLoginMethods(context.Context, store.CountOtherLoginMethodsParams) (int32, error) {
	if err := fake.fail("CountOtherLoginMethods"); err != nil {
		return 0, err
	}
	return fake.otherMethods, nil
}

func (fake *fakeStore) UsernameExists(_ context.Context, username string) (bool, error) {
	if err := fake.fail("UsernameExists"); err != nil {
		return false, err
	}
	fake.mutex.Lock()
	defer fake.mutex.Unlock()
	return fake.usernames[username], nil
}

// fakeChecker accepts every issuer unless err is set.
type fakeChecker struct {
	err     error
	checked []string
}

func (fake *fakeChecker) Check(_ context.Context, issuer string) error {
	fake.checked = append(fake.checked, issuer)
	return fake.err
}

func stringsCut(text string, separator string) (string, string, bool) {
	return strings.Cut(text, separator)
}
