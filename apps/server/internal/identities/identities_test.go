package identities

import (
	"context"
	"crypto/ed25519"
	"encoding/hex"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/aazam476/nexus-chat/apps/server/internal/api"
	"github.com/aazam476/nexus-chat/apps/server/internal/auth/session"
	"github.com/aazam476/nexus-chat/apps/server/internal/identities/store"
	"github.com/aazam476/nexus-chat/apps/server/internal/platform/clock"
)

var errDown = errors.New("database down")

var now = time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC) //nolint:gochecknoglobals // test fixture

type fakeStore struct {
	rows   map[uuid.UUID]store.UserIdentity
	getErr error
	putErr error
}

func (fake *fakeStore) GetIdentity(_ context.Context, userID uuid.UUID) (store.UserIdentity, error) {
	if fake.getErr != nil {
		return store.UserIdentity{}, fake.getErr
	}
	row, ok := fake.rows[userID]
	if !ok {
		return store.UserIdentity{}, pgx.ErrNoRows
	}
	return row, nil
}

func (fake *fakeStore) PutIdentity(_ context.Context, params store.PutIdentityParams) error {
	if fake.putErr != nil {
		return fake.putErr
	}
	fake.rows[params.UserID] = store.UserIdentity(params)
	return nil
}

func newIdentity(t *testing.T) Identity {
	t.Helper()
	masterPublic, masterPrivate, err := ed25519.GenerateKey(nil)
	require.NoError(t, err)
	selfSigning, _, err := ed25519.GenerateKey(nil)
	require.NoError(t, err)
	return Identity{
		MasterPublicKey: masterPublic, SelfSigningPublicKey: selfSigning,
		SelfSigningSignature: ed25519.Sign(masterPrivate, statement(selfSigning)),
	}
}

func setUp() (*Handler, *fakeStore) {
	fake := &fakeStore{rows: map[uuid.UUID]store.UserIdentity{}}
	return NewHandler(NewService(fake, clock.Fixed{Time: now})), fake
}

// caller is a user whose last login was `ago` before now.
func caller(t *testing.T, userID uuid.UUID, ago time.Duration) context.Context {
	return session.WithPrincipal(t.Context(), session.Principal{UserID: userID, SessionID: uuid.New(), AuthenticatedAt: now.Add(-ago)})
}

func publish(identity Identity) api.PublishIdentityRequestObject {
	return api.PublishIdentityRequestObject{Body: &api.PublishIdentityJSONRequestBody{
		MasterPublicKey: identity.MasterPublicKey, SelfSigningPublicKey: identity.SelfSigningPublicKey,
		SelfSigningSignature: identity.SelfSigningSignature,
	}}
}

func TestAClientSignatureVerifies(t *testing.T) {
	// Made by createIdentity in packages/crypto: both sides sign the same statement.
	decode := func(value string) []byte {
		bytes, err := hex.DecodeString(value)
		require.NoError(t, err)
		return bytes
	}
	master := decode("6fdf721637f3f84f5def4fa1a0f136c1be070f8e27212b6286cbedce2776032f")
	selfSigning := decode("d9d839629575b1ca4169110e13465bc966a3abed289630781face86ec9aa92f1")
	signature := decode("db9ff6e1d10880de39eb97dd6f1c489fd7b343be55003e588bf655173efd5d1e1b7c52895da20cd1d1b8c962ddd9594fca440060ea51e505db1b64b3eff1de00")

	assert.True(t, ed25519.Verify(master, statement(selfSigning), signature))
}

func TestPublishingAndReadingAnIdentity(t *testing.T) {
	handler, _ := setUp()
	userID := uuid.New()
	identity := newIdentity(t)

	response, err := handler.PublishIdentity(caller(t, userID, time.Hour), publish(identity))
	require.NoError(t, err)
	assert.Equal(t, api.PublishIdentity204Response{}, response)

	read, err := handler.GetIdentity(t.Context(), api.GetIdentityRequestObject{UserId: userID})
	require.NoError(t, err)
	assert.Equal(t, api.GetIdentity200JSONResponse{
		MasterPublicKey: identity.MasterPublicKey, SelfSigningPublicKey: identity.SelfSigningPublicKey,
		SelfSigningSignature: identity.SelfSigningSignature,
	}, read)

	missing, err := handler.GetIdentity(t.Context(), api.GetIdentityRequestObject{UserId: uuid.New()})
	require.NoError(t, err)
	assert.IsType(t, api.GetIdentity404JSONResponse{}, missing)
	assert.Equal(t, "identity_not_found", missing.(api.GetIdentity404JSONResponse).Code) //nolint:forcetypeassert // checked above
}

func TestReplacingTheMasterKeyNeedsARecentLogin(t *testing.T) {
	handler, fake := setUp()
	userID := uuid.New()
	first := newIdentity(t)
	_, err := handler.PublishIdentity(caller(t, userID, time.Hour), publish(first))
	require.NoError(t, err)

	// A new identity: another master key, as when a user starts fresh.
	rotated := newIdentity(t)

	stale, err := handler.PublishIdentity(caller(t, userID, time.Hour), publish(rotated))
	require.NoError(t, err)
	assert.IsType(t, api.PublishIdentity403JSONResponse{}, stale)
	assert.Equal(t, first.MasterPublicKey, fake.rows[userID].MasterPublicKey, "unchanged")

	fresh, err := handler.PublishIdentity(caller(t, userID, time.Minute), publish(rotated))
	require.NoError(t, err)
	assert.Equal(t, api.PublishIdentity204Response{}, fresh)
	assert.Equal(t, rotated.MasterPublicKey, fake.rows[userID].MasterPublicKey)
}

func TestARotatedSelfSigningKeyNeedsNoNewLogin(t *testing.T) {
	handler, fake := setUp()
	userID := uuid.New()
	masterPublic, masterPrivate, err := ed25519.GenerateKey(nil)
	require.NoError(t, err)
	signed := func() Identity {
		selfSigning, _, err := ed25519.GenerateKey(nil)
		require.NoError(t, err)
		return Identity{MasterPublicKey: masterPublic, SelfSigningPublicKey: selfSigning, SelfSigningSignature: ed25519.Sign(masterPrivate, statement(selfSigning))}
	}
	_, err = handler.PublishIdentity(caller(t, userID, time.Hour), publish(signed()))
	require.NoError(t, err)
	next := signed()

	response, err := handler.PublishIdentity(caller(t, userID, time.Hour), publish(next))
	require.NoError(t, err)
	assert.Equal(t, api.PublishIdentity204Response{}, response)
	assert.Equal(t, []byte(next.SelfSigningPublicKey), fake.rows[userID].SelfSigningPublicKey)
}

func TestInvalidIdentitiesAreRefused(t *testing.T) {
	handler, _ := setUp()
	good := newIdentity(t)
	other := newIdentity(t)
	cases := map[string]struct {
		identity Identity
		code     string
	}{
		"short master key":     {Identity{good.MasterPublicKey[:31], good.SelfSigningPublicKey, good.SelfSigningSignature}, "invalid_identity"},
		"short self-signing":   {Identity{good.MasterPublicKey, good.SelfSigningPublicKey[:31], good.SelfSigningSignature}, "invalid_identity"},
		"short signature":      {Identity{good.MasterPublicKey, good.SelfSigningPublicKey, good.SelfSigningSignature[:63]}, "invalid_identity"},
		"another key's signer": {Identity{good.MasterPublicKey, other.SelfSigningPublicKey, good.SelfSigningSignature}, "invalid_signature"},
		"another master":       {Identity{other.MasterPublicKey, good.SelfSigningPublicKey, good.SelfSigningSignature}, "invalid_signature"},
	}
	for name, test := range cases {
		t.Run(name, func(t *testing.T) {
			response, err := handler.PublishIdentity(caller(t, uuid.New(), 0), publish(test.identity))
			require.NoError(t, err)
			require.IsType(t, api.PublishIdentity422JSONResponse{}, response)
			assert.Equal(t, test.code, response.(api.PublishIdentity422JSONResponse).Code) //nolint:forcetypeassert // checked above
		})
	}
}

func TestStoreFailuresAreReported(t *testing.T) {
	handler, fake := setUp()
	identity := newIdentity(t)

	fake.getErr = errDown
	_, err := handler.PublishIdentity(caller(t, uuid.New(), 0), publish(identity))
	require.ErrorIs(t, err, errDown)
	_, err = handler.GetIdentity(t.Context(), api.GetIdentityRequestObject{UserId: uuid.New()})
	require.ErrorIs(t, err, errDown)

	fake.getErr = nil
	fake.putErr = errDown
	_, err = handler.PublishIdentity(caller(t, uuid.New(), 0), publish(identity))
	require.ErrorIs(t, err, errDown)
}

func TestHandlersNeedASession(t *testing.T) {
	handler, _ := setUp()

	_, err := handler.PublishIdentity(t.Context(), publish(newIdentity(t)))
	require.ErrorIs(t, err, errNoPrincipal)
}
