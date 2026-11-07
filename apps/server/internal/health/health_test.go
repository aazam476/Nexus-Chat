package health

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/aazam476/nexus-chat/apps/server/internal/api"
)

// fakeChecker returns err from Ping, optionally after waiting for the
// context to be cancelled (to prove the timeout is applied).
type fakeChecker struct {
	err           error
	waitForCancel bool
}

func (fake fakeChecker) Ping(ctx context.Context) error {
	if fake.waitForCancel {
		<-ctx.Done()
		return ctx.Err()
	}
	return fake.err
}

func TestReadinessIsOKWhenEveryCheckPasses(t *testing.T) {
	service := NewService([]NamedChecker{{Name: "database", Checker: fakeChecker{}}})

	response, err := NewHandler(service).GetReadiness(context.Background(), api.GetReadinessRequestObject{})

	require.NoError(t, err)
	assert.Equal(t, api.GetReadiness200JSONResponse{
		Status: api.HealthStateOk,
		Checks: []api.HealthCheck{{Name: "database", Status: api.HealthStateOk}},
	}, response)
}

func TestReadinessIsUnavailableWhenAnyCheckFails(t *testing.T) {
	service := NewService([]NamedChecker{
		{Name: "database", Checker: fakeChecker{err: errors.New("down")}},
		{Name: "other", Checker: fakeChecker{}},
	})

	response, err := NewHandler(service).GetReadiness(context.Background(), api.GetReadinessRequestObject{})

	require.NoError(t, err)
	assert.Equal(t, api.GetReadiness503JSONResponse{
		Status: api.HealthStateUnavailable,
		Checks: []api.HealthCheck{
			{Name: "database", Status: api.HealthStateUnavailable},
			{Name: "other", Status: api.HealthStateOk},
		},
	}, response)
}

func TestReadinessTimesOutAHungCheck(t *testing.T) {
	service := NewService([]NamedChecker{{Name: "database", Checker: fakeChecker{waitForCancel: true}}})
	start := time.Now()

	allOK, results := service.Readiness(context.Background())

	assert.False(t, allOK)
	assert.Equal(t, []Result{{Name: "database", OK: false}}, results)
	assert.Less(t, time.Since(start), checkTimeout+time.Second)
}

func TestLivenessIsAlwaysOK(t *testing.T) {
	response, err := NewHandler(NewService(nil)).GetLiveness(context.Background(), api.GetLivenessRequestObject{})

	require.NoError(t, err)
	assert.Equal(t, api.GetLiveness200JSONResponse{Status: api.HealthStateOk, Checks: []api.HealthCheck{}}, response)
}
