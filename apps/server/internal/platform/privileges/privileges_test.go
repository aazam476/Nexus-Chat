package privileges

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseTarget(t *testing.T) {
	target, enabled, err := ParseTarget("65532:65533")
	require.NoError(t, err)
	assert.True(t, enabled)
	assert.Equal(t, Target{UID: 65532, GID: 65533}, target)

	_, enabled, err = ParseTarget("")
	require.NoError(t, err)
	assert.False(t, enabled)

	for _, bad := range []string{"65532", "a:b", "0:0", "65532:0", "-1:5", "1:x"} {
		_, _, err := ParseTarget(bad)
		assert.Error(t, err, bad) //nolint:testifylint // each value is checked independently
	}
}

// recorder fakes the system calls and records what Drop asked for.
type recorder struct {
	uid     int
	chowned []string
	calls   []string
	fail    string
}

func (fake *recorder) system() system {
	failIf := func(name string) error {
		fake.calls = append(fake.calls, name)
		if fake.fail == name {
			return errors.New(name + " failed")
		}
		return nil
	}
	return system{
		getuid: func() int { return fake.uid },
		lchown: func(path string, _ int, _ int) error {
			fake.chowned = append(fake.chowned, filepath.Base(path))
			return failIf("lchown")
		},
		setgroups: func([]int) error { return failIf("setgroups") },
		setgid:    func(int) error { return failIf("setgid") },
		setuid:    func(int) error { return failIf("setuid") },
	}
}

func TestDropAsRootChownsThenSwitches(t *testing.T) {
	directory := filepath.Join(t.TempDir(), "data")
	require.NoError(t, os.MkdirAll(filepath.Join(directory, "secrets"), 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(directory, "secrets", "key.pem"), nil, 0o600))
	fake := &recorder{uid: 0}

	require.NoError(t, drop(fake.system(), directory, Target{UID: 65532, GID: 65532}))

	assert.Equal(t, []string{"data", "secrets", "key.pem"}, fake.chowned)
	assert.Equal(t, []string{"lchown", "lchown", "lchown", "setgroups", "setgid", "setuid"}, fake.calls)
}

func TestDropCreatesMissingDataDirectory(t *testing.T) {
	directory := filepath.Join(t.TempDir(), "new")
	fake := &recorder{uid: 0}

	require.NoError(t, drop(fake.system(), directory, Target{UID: 1, GID: 1}))

	assert.DirExists(t, directory)
}

func TestDropDoesNothingWhenNotRoot(t *testing.T) {
	fake := &recorder{uid: 1000}

	require.NoError(t, drop(fake.system(), "/does/not/matter", Target{UID: 1, GID: 1}))

	assert.Empty(t, fake.calls)
}

func TestDropReportsFailures(t *testing.T) {
	for _, step := range []string{"lchown", "setgroups", "setgid", "setuid"} {
		t.Run(step, func(t *testing.T) {
			fake := &recorder{uid: 0, fail: step}

			err := drop(fake.system(), t.TempDir(), Target{UID: 1, GID: 1})

			assert.ErrorContains(t, err, step+" failed")
		})
	}
}

func TestDropReportsUnusableDirectory(t *testing.T) {
	blocker := filepath.Join(t.TempDir(), "file")
	require.NoError(t, os.WriteFile(blocker, nil, 0o600))
	fake := &recorder{uid: 0}

	err := drop(fake.system(), filepath.Join(blocker, "data"), Target{UID: 1, GID: 1})

	assert.ErrorContains(t, err, "creating")
}

func TestDropReportsWalkErrors(t *testing.T) {
	directory := t.TempDir()
	locked := filepath.Join(directory, "locked")
	require.NoError(t, os.Mkdir(locked, 0o700))
	fake := &recorder{uid: 0}
	calls := fake.system()
	// Simulate a directory disappearing mid-walk: WalkDir visits it, then
	// fails to read its entries.
	calls.lchown = func(path string, _ int, _ int) error {
		if path == locked {
			return os.Remove(locked)
		}
		return nil
	}

	err := drop(calls, directory, Target{UID: 1, GID: 1})

	assert.ErrorContains(t, err, "taking ownership")
}

func TestRealSystemIsWired(t *testing.T) {
	calls := realSystem()

	assert.Equal(t, os.Getuid(), calls.getuid())
	assert.NotNil(t, calls.lchown)
	assert.NotNil(t, calls.setgroups)
	assert.NotNil(t, calls.setgid)
	assert.NotNil(t, calls.setuid)
}
