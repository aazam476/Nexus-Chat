package breached

import (
	"bytes"
	"compress/gzip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBundledListCatchesCommonPasswords(t *testing.T) {
	list, err := Load()
	require.NoError(t, err)

	assert.Greater(t, list.Size(), 100_000)
	for _, common := range []string{"password123", "1234567890", "PASSWORD123", "qwertyuiop"} {
		assert.ErrorIs(t, list.Check(common), ErrBreached, common) //nolint:testifylint // each password is checked independently
	}
	assert.NoError(t, list.Check("violet tractor umbrella sunrise"))
}

func gzipped(t *testing.T, text string) []byte {
	t.Helper()
	var buffer bytes.Buffer
	writer := gzip.NewWriter(&buffer)
	_, err := writer.Write([]byte(text))
	require.NoError(t, err)
	require.NoError(t, writer.Close())
	return buffer.Bytes()
}

func TestLoadRejectsBrokenLists(t *testing.T) {
	_, err := load([]byte("not gzip"))
	require.ErrorContains(t, err, "opening")

	truncated := gzipped(t, "aaaaaaaaaa\nbbbbbbbbbb\n")
	_, err = load(truncated[:len(truncated)-4])
	require.ErrorContains(t, err, "reading")

	_, err = load(gzipped(t, "bbbbbbbbbb\naaaaaaaaaa\n"))
	assert.ErrorContains(t, err, "not sorted")
}

func TestCheckBoundaries(t *testing.T) {
	list, err := load(gzipped(t, "bbbbbbbbbb\ndddddddddd\n"))
	require.NoError(t, err)

	require.ErrorIs(t, list.Check("bbbbbbbbbb"), ErrBreached)
	require.NoError(t, list.Check("aaaaaaaaaa"), "before the first entry")
	require.NoError(t, list.Check("cccccccccc"), "between entries")
	assert.NoError(t, list.Check("zzzzzzzzzz"), "after the last entry")
}
