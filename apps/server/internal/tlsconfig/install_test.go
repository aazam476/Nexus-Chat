package tlsconfig

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/aazam476/nexus-chat/apps/server/internal/tlsconfig/tlsconfigtest"
)

func TestInstallValidSavesAndActivates(t *testing.T) {
	fixture := newFixture(t, nil)
	certificate, key := fixture.issueValid(t, tlsconfigtest.DefaultLeaf())

	info, err := fixture.manager.Install(t.Context(), certificate, key)

	require.NoError(t, err)
	assert.Equal(t, []string{"chat.example.com"}, info.Names)
	assert.True(t, fixture.manager.Status().Active)
	saved, err := os.ReadFile(filepath.Join(fixture.directory, CertificateFile))
	require.NoError(t, err)
	assert.Equal(t, certificate, saved)
	assertMode(t, filepath.Join(fixture.directory, KeyFile), 0o600)
}

func TestInstallRejectsBadUploadsWithoutChangingAnything(t *testing.T) {
	fixture := newFixture(t, nil)
	good, goodKey := fixture.issueValid(t, tlsconfigtest.DefaultLeaf())
	_, err := fixture.manager.Install(t.Context(), good, goodKey)
	require.NoError(t, err)
	options := tlsconfigtest.DefaultLeaf()
	options.Names = []string{"other.example.org"}
	wrong, wrongKey := fixture.issueValid(t, options)

	_, err = fixture.manager.Install(t.Context(), wrong, wrongKey)

	require.ErrorIs(t, err, ErrWrongDomain)
	var rejected *RejectedError
	require.ErrorAs(t, err, &rejected)
	assert.Equal(t, rejected.Err.Error(), err.Error())
	saved, err := os.ReadFile(filepath.Join(fixture.directory, CertificateFile))
	require.NoError(t, err)
	assert.Equal(t, good, saved, "the working certificate stays")
	assert.True(t, fixture.manager.Status().Active)
}

func TestInstallFailures(t *testing.T) {
	broken := newFixture(t, errors.New("database down"))
	certificate, key := broken.issueValid(t, tlsconfigtest.DefaultLeaf())
	_, err := broken.manager.Install(t.Context(), certificate, key)
	require.ErrorContains(t, err, "reading instance domain")

	blocked := newFixture(t, nil)
	file := filepath.Join(t.TempDir(), "file")
	require.NoError(t, os.WriteFile(file, nil, 0o600))
	blocked.manager.options.Directory = filepath.Join(file, "tls")
	certificate, key = blocked.issueValid(t, tlsconfigtest.DefaultLeaf())
	_, err = blocked.manager.Install(t.Context(), certificate, key)
	require.ErrorContains(t, err, "creating TLS directory")

	for _, name := range []string{KeyFile + ".tmp", CertificateFile + ".tmp", CertificateFile} {
		t.Run(name, func(t *testing.T) {
			fixture := newFixture(t, nil)
			// A directory where a file must go makes that step fail.
			require.NoError(t, os.MkdirAll(filepath.Join(fixture.directory, name), 0o700))
			require.NoError(t, os.WriteFile(filepath.Join(fixture.directory, name, "x"), nil, 0o600))
			certificate, key := fixture.issueValid(t, tlsconfigtest.DefaultLeaf())

			_, err := fixture.manager.Install(t.Context(), certificate, key)

			assert.Error(t, err)
		})
	}
}

func TestInstallReportsReloadFailure(t *testing.T) {
	fixture := newFixture(t, nil)
	certificate, key := fixture.issueValid(t, tlsconfigtest.DefaultLeaf())
	lookups := 0
	// The domain changes between validating the upload and loading it.
	fixture.manager.options.Domain = func(context.Context) (string, error) {
		lookups++
		if lookups > 1 {
			return "", errors.New("database down")
		}
		return "chat.example.com", nil
	}

	_, err := fixture.manager.Install(t.Context(), certificate, key)

	require.ErrorContains(t, err, "database down")
	assert.False(t, fixture.manager.Status().Active)
}

func TestRemoveTurnsHTTPSOff(t *testing.T) {
	fixture := newFixture(t, nil)
	certificate, key := fixture.issueValid(t, tlsconfigtest.DefaultLeaf())
	_, err := fixture.manager.Install(t.Context(), certificate, key)
	require.NoError(t, err)

	require.NoError(t, fixture.manager.Remove())
	require.NoError(t, fixture.manager.Remove(), "removing twice is fine")

	assert.False(t, fixture.manager.Status().Active)
	assert.NoFileExists(t, filepath.Join(fixture.directory, CertificateFile))
	require.NoError(t, fixture.manager.Reload(t.Context()))
	assert.False(t, fixture.manager.Active())
}

func TestRemoveReportsFailures(t *testing.T) {
	fixture := newFixture(t, nil)
	require.NoError(t, os.MkdirAll(filepath.Join(fixture.directory, CertificateFile, "child"), 0o700))

	assert.ErrorContains(t, fixture.manager.Remove(), "removing")
}

func assertMode(t *testing.T, path string, want os.FileMode) {
	t.Helper()
	info, err := os.Stat(path)
	require.NoError(t, err)
	assert.Equal(t, want, info.Mode().Perm())
}
