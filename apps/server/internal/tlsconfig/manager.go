package tlsconfig

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/aazam476/nexus-chat/apps/server/internal/platform/clock"
)

// File names inside the TLS directory.
const (
	CertificateFile = "cert.pem"
	KeyFile         = "key.pem"
)

// expiryWarnings are the days-before-expiry at which admins are warned
// (spec §3.3), from earliest to latest.
var expiryWarnings = []time.Duration{30 * 24 * time.Hour, 7 * 24 * time.Hour, 24 * time.Hour} //nolint:gochecknoglobals // constant list

// ErrNoCertificate is returned during a TLS handshake when no valid
// certificate is installed.
var ErrNoCertificate = errors.New("no HTTPS certificate is installed")

// ManagerOptions configures a Manager.
type ManagerOptions struct {
	// Directory holds cert.pem (full chain) and key.pem.
	Directory string
	// Domain returns the instance's current domain.
	Domain func(ctx context.Context) (string, error)
	Clock  clock.Clock
	Logger *slog.Logger
	// Roots are the trusted roots; nil means the system's.
	Roots *x509.CertPool
}

// Manager holds the certificate currently used for HTTPS.
type Manager struct {
	options  ManagerOptions
	current  atomic.Pointer[tls.Certificate]
	mutex    sync.Mutex // guards the fields below
	info     Info
	modified time.Time
	warned   time.Duration // smallest expiry threshold already warned about
}

// NewManager returns a Manager with no certificate loaded yet.
func NewManager(options ManagerOptions) *Manager {
	return &Manager{options: options}
}

// TLSConfig returns the configuration for the HTTPS listener. It asks the
// Manager for the certificate on every handshake, which is what makes
// reloading work without a restart.
func (manager *Manager) TLSConfig() *tls.Config {
	return &tls.Config{
		MinVersion: tls.VersionTLS12,
		GetCertificate: func(*tls.ClientHelloInfo) (*tls.Certificate, error) {
			certificate := manager.current.Load()
			if certificate == nil {
				return nil, ErrNoCertificate
			}
			return certificate, nil
		},
	}
}

// Active reports whether a valid certificate is installed.
func (manager *Manager) Active() bool {
	return manager.current.Load() != nil
}

// Reload reads the certificate files if they changed since the last
// successful load, validates them, and switches to them. If the new files
// are invalid, the previous certificate stays in use and the error is
// returned. Missing files are not an error: HTTPS is simply off.
func (manager *Manager) Reload(ctx context.Context) error {
	certificatePath := filepath.Join(manager.options.Directory, CertificateFile)
	stat, err := os.Stat(certificatePath)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("checking %s: %w", certificatePath, err)
	}
	manager.mutex.Lock()
	unchanged := stat.ModTime().Equal(manager.modified)
	manager.mutex.Unlock()
	if unchanged {
		return nil
	}
	return manager.load(ctx, stat.ModTime())
}

func (manager *Manager) load(ctx context.Context, modified time.Time) error {
	certificatePEM, keyPEM, err := readPair(manager.options.Directory)
	if err != nil {
		return err
	}
	domain, err := manager.options.Domain(ctx)
	if err != nil {
		return fmt.Errorf("reading instance domain: %w", err)
	}
	checks := Checks{Domain: domain, Now: manager.options.Clock.Now(), Roots: manager.options.Roots}
	certificate, info, err := Validate(certificatePEM, keyPEM, checks)
	if err != nil {
		return fmt.Errorf("certificate in %s not used: %w", manager.options.Directory, err)
	}
	manager.current.Store(certificate)
	manager.mutex.Lock()
	manager.info, manager.modified, manager.warned = info, modified, 0
	manager.mutex.Unlock()
	manager.options.Logger.InfoContext(ctx, "HTTPS certificate loaded",
		slog.Any("names", info.Names), slog.Time("expires", info.NotAfter))
	return nil
}

func readPair(directory string) ([]byte, []byte, error) {
	// G304: both paths are our TLS directory joined with constant names.
	certificatePEM, err := os.ReadFile(filepath.Join(directory, CertificateFile)) //nolint:gosec // see above
	if err != nil {
		return nil, nil, fmt.Errorf("reading certificate: %w", err)
	}
	keyPEM, err := os.ReadFile(filepath.Join(directory, KeyFile)) //nolint:gosec // see above
	if err != nil {
		return nil, nil, fmt.Errorf("reading private key: %w", err)
	}
	return certificatePEM, keyPEM, nil
}

// WarnIfExpiring logs a warning the first time the active certificate comes
// within 30, 7 and 1 days of expiry. It returns the time left, or 0 when no
// certificate is active.
func (manager *Manager) WarnIfExpiring(ctx context.Context) time.Duration {
	if !manager.Active() {
		return 0
	}
	manager.mutex.Lock()
	defer manager.mutex.Unlock()
	remaining := manager.info.NotAfter.Sub(manager.options.Clock.Now())
	// Find the closest threshold already crossed, so a server that starts
	// 5 days before expiry warns once (for 7 days), not twice.
	var crossed time.Duration
	for _, threshold := range expiryWarnings {
		if remaining <= threshold {
			crossed = threshold
		}
	}
	if crossed != 0 && (manager.warned == 0 || crossed < manager.warned) {
		manager.warned = crossed
		manager.options.Logger.WarnContext(ctx, "HTTPS certificate expires soon; replace it",
			slog.Duration("remaining", remaining.Round(time.Hour)), slog.Time("expires", manager.info.NotAfter))
	}
	return remaining
}

// Watch reloads the certificate and checks its expiry every interval until
// ctx is cancelled. The caller runs it in a goroutine and stops it by
// cancelling ctx (the server's root context, on shutdown).
func (manager *Manager) Watch(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := manager.Reload(ctx); err != nil {
				manager.options.Logger.ErrorContext(ctx, "HTTPS certificate reload failed", slog.Any("error", err))
			}
			manager.WarnIfExpiring(ctx)
		}
	}
}
