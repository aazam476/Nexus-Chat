package keypackages

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/aazam476/nexus-chat/apps/server/internal/api"
	"github.com/aazam476/nexus-chat/apps/server/internal/auth/session"
	"github.com/aazam476/nexus-chat/apps/server/internal/keypackages/store"
	"github.com/aazam476/nexus-chat/apps/server/internal/platform/clock"
)

var errDown = errors.New("database down")

type fakeStore struct {
	count   store.CountKeyPackagesRow
	added   []store.Upload
	claimed []store.Claimed
	err     error
}

func (fake *fakeStore) Count(context.Context, uuid.UUID) (store.CountKeyPackagesRow, error) {
	return fake.count, fake.err
}

func (fake *fakeStore) Add(_ context.Context, upload store.Upload) (store.CountKeyPackagesRow, error) {
	fake.added = append(fake.added, upload)
	return fake.count, fake.err
}

func (fake *fakeStore) Claim(context.Context, uuid.UUID) ([]store.Claimed, error) {
	return fake.claimed, fake.err
}

type fakeLimiter struct{ deny bool }

func (fake fakeLimiter) Allow(string) bool { return !fake.deny }

func keyPackage(extra ...byte) []byte {
	return append([]byte{0, 1, 0, 5, 0, 1, 0, 1}, append([]byte{42}, extra...)...)
}

func newHandler(fake *fakeStore, deny bool) *Handler {
	return NewHandler(NewService(fake, fakeLimiter{deny: deny}, clock.Fixed{Time: time.Date(2026, 11, 8, 12, 0, 0, 0, time.UTC)}))
}

func signedIn(t *testing.T) context.Context {
	return session.WithPrincipal(t.Context(), session.Principal{UserID: uuid.New(), SessionID: uuid.New()})
}

func upload(packages [][]byte, lastResort []byte) api.UploadKeyPackagesRequestObject {
	body := api.UploadKeyPackagesJSONRequestBody{KeyPackages: packages}
	if lastResort != nil {
		body.LastResort = &lastResort
	}
	return api.UploadKeyPackagesRequestObject{Body: &body}
}

func TestUploadStoresValidPackagesAndReportsTheStock(t *testing.T) {
	fake := &fakeStore{count: store.CountKeyPackagesRow{OneTime: 12, HasLastResort: true}}
	response, err := newHandler(fake, false).UploadKeyPackages(signedIn(t), upload([][]byte{keyPackage(1), keyPackage(2)}, keyPackage(3)))

	require.NoError(t, err)
	assert.Equal(t, api.UploadKeyPackages200JSONResponse{OneTime: 12, HasLastResort: true, Refill: true}, response)
	require.Len(t, fake.added, 1)
	assert.Equal(t, int64(MaximumOneTime), fake.added[0].MaximumOneTime)
	assert.Equal(t, keyPackage(3), fake.added[0].LastResort)
}

func TestUploadRejectsWhatIsNotAKeyPackage(t *testing.T) {
	tooLarge := keyPackage(make([]byte, maximumSize)...)
	wrongSuite := []byte{0, 1, 0, 5, 0, 1, 0, 2, 9}
	cases := map[string]api.UploadKeyPackagesRequestObject{
		"wrong ciphersuite":     upload([][]byte{wrongSuite}, nil),
		"a welcome":             upload([][]byte{{0, 1, 0, 3, 0, 1, 0, 1, 9}}, nil),
		"only a header":         upload([][]byte{keyPackage()[:8]}, nil),
		"too large":             upload([][]byte{tooLarge}, nil),
		"invalid last resort":   upload(nil, wrongSuite),
		"empty":                 upload(nil, nil),
		"too many in one batch": upload(make([][]byte, MaximumBatch+1), nil),
	}
	for name, request := range cases {
		fake := &fakeStore{}
		response, err := newHandler(fake, false).UploadKeyPackages(signedIn(t), request)

		require.NoError(t, err, name)
		assert.IsType(t, api.UploadKeyPackages422JSONResponse{}, response, name)
		assert.Empty(t, fake.added, name)
	}
}

func TestUploadReportsAFullStockAndFailures(t *testing.T) {
	full, err := newHandler(&fakeStore{err: store.ErrStockFull}, false).UploadKeyPackages(signedIn(t), upload([][]byte{keyPackage()}, nil))
	require.NoError(t, err)
	assert.Equal(t, "key_package_stock_full", full.(api.UploadKeyPackages409JSONResponse).Code)

	_, err = newHandler(&fakeStore{err: errDown}, false).UploadKeyPackages(signedIn(t), upload([][]byte{keyPackage()}, nil))
	require.ErrorIs(t, err, errDown)
}

func TestStockAndClaims(t *testing.T) {
	device := uuid.New()
	fake := &fakeStore{
		count:   store.CountKeyPackagesRow{OneTime: 50, HasLastResort: true},
		claimed: []store.Claimed{{SessionID: device, Data: keyPackage(), LastResort: true}},
	}
	handler := newHandler(fake, false)

	stock, err := handler.GetKeyPackageStock(signedIn(t), api.GetKeyPackageStockRequestObject{})
	require.NoError(t, err)
	assert.Equal(t, api.GetKeyPackageStock200JSONResponse{OneTime: 50, HasLastResort: true, Refill: false}, stock)

	claimed, err := handler.ClaimKeyPackages(signedIn(t), api.ClaimKeyPackagesRequestObject{UserId: uuid.New()})
	require.NoError(t, err)
	assert.Equal(t, api.ClaimKeyPackages200JSONResponse{{DeviceId: device, KeyPackage: keyPackage(), LastResort: true}}, claimed)

	limited, err := newHandler(fake, true).ClaimKeyPackages(signedIn(t), api.ClaimKeyPackagesRequestObject{})
	require.NoError(t, err)
	assert.IsType(t, api.ClaimKeyPackages429JSONResponse{}, limited)
}

func TestFailuresAndMissingPrincipalsArePassedOn(t *testing.T) {
	broken := newHandler(&fakeStore{err: errDown}, false)
	_, err := broken.GetKeyPackageStock(signedIn(t), api.GetKeyPackageStockRequestObject{})
	require.ErrorIs(t, err, errDown)
	_, err = broken.ClaimKeyPackages(signedIn(t), api.ClaimKeyPackagesRequestObject{})
	require.ErrorIs(t, err, errDown)

	_, err = broken.GetKeyPackageStock(t.Context(), api.GetKeyPackageStockRequestObject{})
	require.ErrorIs(t, err, errNoPrincipal)
	_, err = broken.UploadKeyPackages(t.Context(), upload(nil, nil))
	require.ErrorIs(t, err, errNoPrincipal)
	_, err = broken.ClaimKeyPackages(t.Context(), api.ClaimKeyPackagesRequestObject{})
	require.ErrorIs(t, err, errNoPrincipal)
}
