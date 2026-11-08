package tlsconfig

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/aazam476/nexus-chat/apps/server/internal/tlsconfig/tlsconfigtest"
)

type movableClock struct {
	mutex sync.Mutex
	now   time.Time
}

func (clock *movableClock) Now() time.Time {
	clock.mutex.Lock()
	defer clock.mutex.Unlock()
	return clock.now
}

func (clock *movableClock) set(now time.Time) {
	clock.mutex.Lock()
	defer clock.mutex.Unlock()
	clock.now = now
}

type safeBuffer struct {
	mutex  sync.Mutex
	buffer bytes.Buffer
}

func (safe *safeBuffer) Write(data []byte) (int, error) {
	safe.mutex.Lock()
	defer safe.mutex.Unlock()
	return safe.buffer.Write(data)
}

func (safe *safeBuffer) String() string {
	safe.mutex.Lock()
	defer safe.mutex.Unlock()
	return safe.buffer.String()
}

type managerFixture struct {
	manager   *Manager
	directory string
	authority tlsconfigtest.Authority
	clock     *movableClock
	logs      *safeBuffer
}

func newFixture(t *testing.T, domainError error) managerFixture {
	t.Helper()
	fixture := managerFixture{directory: t.TempDir(), authority: tlsconfigtest.NewAuthority(t), clock: &movableClock{now: tlsconfigtest.Now()}, logs: &safeBuffer{}}
	fixture.manager = NewManager(ManagerOptions{
		Directory: fixture.directory,
		Domain:    func(context.Context) (string, error) { return "chat.example.com", domainError },
		Clock:     fixture.clock,
		Logger:    slog.New(slog.NewTextHandler(fixture.logs, nil)),
		Roots:     fixture.authority.Roots,
	})
	return fixture
}

// install writes a certificate pair and gives it a distinct modification
// time, so the manager notices the change even within one clock tick.
func (fixture managerFixture) install(t *testing.T, certificate []byte, key []byte, modified time.Time) {
	t.Helper()
	certificatePath := filepath.Join(fixture.directory, CertificateFile)
	require.NoError(t, os.WriteFile(certificatePath, certificate, 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(fixture.directory, KeyFile), key, 0o600))
	require.NoError(t, os.Chtimes(certificatePath, modified, modified))
}

func (fixture managerFixture) issueValid(t *testing.T, options tlsconfigtest.LeafOptions) ([]byte, []byte) {
	leaf, key := fixture.authority.Issue(t, options)
	return fixture.authority.FullChain(leaf), key
}

func TestNoFilesMeansHTTPSIsOff(t *testing.T) {
	fixture := newFixture(t, nil)

	require.NoError(t, fixture.manager.Reload(t.Context()))

	assert.False(t, fixture.manager.Active())
	_, err := fixture.manager.TLSConfig().GetCertificate(&tls.ClientHelloInfo{})
	require.ErrorIs(t, err, ErrNoCertificate)
	assert.Zero(t, fixture.manager.WarnIfExpiring(t.Context()))
}

func TestReloadLoadsAndSwapsCertificates(t *testing.T) {
	fixture := newFixture(t, nil)
	first, firstKey := fixture.issueValid(t, tlsconfigtest.DefaultLeaf())
	fixture.install(t, first, firstKey, tlsconfigtest.Now())
	require.NoError(t, fixture.manager.Reload(t.Context()))
	firstServed, err := fixture.manager.TLSConfig().GetCertificate(&tls.ClientHelloInfo{})
	require.NoError(t, err)

	second, secondKey := fixture.issueValid(t, tlsconfigtest.DefaultLeaf())
	fixture.install(t, second, secondKey, tlsconfigtest.Now().Add(time.Minute))
	require.NoError(t, fixture.manager.Reload(t.Context()))
	secondServed, err := fixture.manager.TLSConfig().GetCertificate(&tls.ClientHelloInfo{})
	require.NoError(t, err)

	assert.True(t, fixture.manager.Active())
	assert.NotEqual(t, firstServed.Certificate[0], secondServed.Certificate[0])
	assert.Equal(t, 2, strings.Count(fixture.logs.String(), "HTTPS certificate loaded"))
}

func TestReloadSkipsUnchangedFiles(t *testing.T) {
	fixture := newFixture(t, nil)
	certificate, key := fixture.issueValid(t, tlsconfigtest.DefaultLeaf())
	fixture.install(t, certificate, key, tlsconfigtest.Now())
	require.NoError(t, fixture.manager.Reload(t.Context()))

	require.NoError(t, fixture.manager.Reload(t.Context()))

	assert.Equal(t, 1, strings.Count(fixture.logs.String(), "HTTPS certificate loaded"))
}

func TestBadReplacementKeepsTheOldCertificate(t *testing.T) {
	fixture := newFixture(t, nil)
	certificate, key := fixture.issueValid(t, tlsconfigtest.DefaultLeaf())
	fixture.install(t, certificate, key, tlsconfigtest.Now())
	require.NoError(t, fixture.manager.Reload(t.Context()))
	served, err := fixture.manager.TLSConfig().GetCertificate(&tls.ClientHelloInfo{})
	require.NoError(t, err)

	fixture.install(t, []byte("garbage"), key, tlsconfigtest.Now().Add(time.Minute))
	err = fixture.manager.Reload(t.Context())

	require.ErrorIs(t, err, ErrKeyMismatch)
	stillServed, err := fixture.manager.TLSConfig().GetCertificate(&tls.ClientHelloInfo{})
	require.NoError(t, err)
	assert.Equal(t, served, stillServed)
}

func TestReloadReportsMissingKeyAndDomainErrors(t *testing.T) {
	fixture := newFixture(t, nil)
	certificate, _ := fixture.issueValid(t, tlsconfigtest.DefaultLeaf())
	require.NoError(t, os.WriteFile(filepath.Join(fixture.directory, CertificateFile), certificate, 0o600))
	require.ErrorContains(t, fixture.manager.Reload(t.Context()), "reading private key")

	broken := newFixture(t, errors.New("database down"))
	certificate, key := broken.issueValid(t, tlsconfigtest.DefaultLeaf())
	broken.install(t, certificate, key, tlsconfigtest.Now())
	assert.ErrorContains(t, broken.manager.Reload(t.Context()), "reading instance domain")
}

func TestReloadReportsUnreadableFiles(t *testing.T) {
	fixture := newFixture(t, nil)
	require.NoError(t, os.Mkdir(filepath.Join(fixture.directory, CertificateFile), 0o700))
	require.ErrorContains(t, fixture.manager.Reload(t.Context()), "reading certificate")

	notADirectory := filepath.Join(t.TempDir(), "file")
	require.NoError(t, os.WriteFile(notADirectory, nil, 0o600))
	fixture.manager.options.Directory = notADirectory
	assert.ErrorContains(t, fixture.manager.Reload(t.Context()), "checking")
}

func TestExpiryWarningsFireOncePerThreshold(t *testing.T) {
	fixture := newFixture(t, nil)
	certificate, key := fixture.issueValid(t, tlsconfigtest.DefaultLeaf()) // expires tlsconfigtest.Now() + 90 days
	fixture.install(t, certificate, key, tlsconfigtest.Now())
	require.NoError(t, fixture.manager.Reload(t.Context()))
	expires := tlsconfigtest.Now().Add(90 * 24 * time.Hour)
	warnings := func() int { return strings.Count(fixture.logs.String(), "expires soon") }
	var notified []Expiry
	fixture.manager.OnExpiring(func(_ context.Context, expiry Expiry) { notified = append(notified, expiry) })

	fixture.manager.WarnIfExpiring(t.Context())
	assert.Equal(t, 0, warnings(), "90 days left: no warning")

	fixture.clock.set(expires.Add(-20 * 24 * time.Hour))
	fixture.manager.WarnIfExpiring(t.Context())
	fixture.manager.WarnIfExpiring(t.Context())
	assert.Equal(t, 1, warnings(), "30-day warning once")

	fixture.clock.set(expires.Add(-5 * 24 * time.Hour))
	fixture.manager.WarnIfExpiring(t.Context())
	assert.Equal(t, 2, warnings(), "7-day warning")

	fixture.clock.set(expires.Add(-time.Hour))
	remaining := fixture.manager.WarnIfExpiring(t.Context())
	assert.Equal(t, 3, warnings(), "1-day warning")
	assert.Equal(t, time.Hour, remaining)

	require.Len(t, notified, 3, "the function hears each warning once")
	day := 24 * time.Hour
	assert.Equal(t, []time.Duration{30 * day, 7 * day, day}, []time.Duration{notified[0].Threshold, notified[1].Threshold, notified[2].Threshold})
	assert.Equal(t, time.Hour, notified[2].Remaining)
	assert.True(t, notified[2].Certificate.NotAfter.Equal(expires))
}

func TestStartingCloseToExpiryWarnsOnlyOnce(t *testing.T) {
	fixture := newFixture(t, nil)
	options := tlsconfigtest.DefaultLeaf()
	options.NotAfter = tlsconfigtest.Now().Add(5 * 24 * time.Hour)
	certificate, key := fixture.issueValid(t, options)
	fixture.install(t, certificate, key, tlsconfigtest.Now())
	require.NoError(t, fixture.manager.Reload(t.Context()))

	fixture.manager.WarnIfExpiring(t.Context())

	assert.Equal(t, 1, strings.Count(fixture.logs.String(), "expires soon"))
}

func TestWatchReloadsUntilCancelled(t *testing.T) {
	fixture := newFixture(t, nil)
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan struct{})
	go func() {
		fixture.manager.Watch(ctx, 10*time.Millisecond)
		close(done)
	}()
	require.NoError(t, os.WriteFile(filepath.Join(fixture.directory, CertificateFile), []byte("bad"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(fixture.directory, KeyFile), []byte("bad"), 0o600))
	require.Eventually(t, func() bool { return strings.Contains(fixture.logs.String(), "reload failed") }, 5*time.Second, 10*time.Millisecond)

	certificate, key := fixture.issueValid(t, tlsconfigtest.DefaultLeaf())
	fixture.install(t, certificate, key, tlsconfigtest.Now().Add(time.Hour))
	require.Eventually(t, fixture.manager.Active, 5*time.Second, 10*time.Millisecond)

	cancel()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Watch did not stop")
	}
}
