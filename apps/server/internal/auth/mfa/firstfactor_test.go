package mfa

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBackupCodesIfFirstFactor(t *testing.T) {
	service, fake, clock := newService(t)
	codes, err := service.BackupCodesIfFirstFactor(t.Context(), alice)
	require.NoError(t, err)
	assert.Nil(t, codes, "no second factor yet")

	enrolled(t, service, clock) // first factor: TOTP, which created codes
	codes, err = service.BackupCodesIfFirstFactor(t.Context(), alice)
	require.NoError(t, err)
	assert.Nil(t, codes, "codes already exist")

	require.NoError(t, service.store.DeleteBackupCodes(t.Context(), alice))
	codes, err = service.BackupCodesIfFirstFactor(t.Context(), alice)
	require.NoError(t, err)
	assert.Len(t, codes, 10)

	for _, method := range []string{"CountSecondFactors", "CountBackupCodes"} {
		fake.fail, fake.failOn = errors.New("down"), method
		_, err = service.BackupCodesIfFirstFactor(t.Context(), alice)
		require.Error(t, err, method)
	}
}
