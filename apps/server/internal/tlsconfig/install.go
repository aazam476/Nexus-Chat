package tlsconfig

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// RejectedError means an uploaded certificate cannot be used. Its message
// says why, in words an admin can act on.
type RejectedError struct {
	Err error
}

func (rejected *RejectedError) Error() string { return rejected.Err.Error() }

// Unwrap returns the reason, such as ErrWrongDomain.
func (rejected *RejectedError) Unwrap() error { return rejected.Err }

// Status describes the certificate in use.
type Status struct {
	Active bool
	Info   Info
}

// Status returns the certificate currently served, if any.
func (manager *Manager) Status() Status {
	manager.mutex.Lock()
	defer manager.mutex.Unlock()
	return Status{Active: manager.current.Load() != nil, Info: manager.info}
}

// Install validates a certificate chain and key uploaded in the Admin panel
// and, only if they are usable, saves them to the TLS directory and starts
// serving them immediately. A rejected upload returns a *RejectedError and
// changes nothing.
func (manager *Manager) Install(ctx context.Context, certificatePEM []byte, keyPEM []byte) (Info, error) {
	domain, err := manager.options.Domain(ctx)
	if err != nil {
		return Info{}, fmt.Errorf("reading instance domain: %w", err)
	}
	checks := Checks{Domain: domain, Now: manager.options.Clock.Now(), Roots: manager.options.Roots}
	if _, _, err := Validate(certificatePEM, keyPEM, checks); err != nil {
		return Info{}, &RejectedError{Err: err}
	}
	if err := os.MkdirAll(manager.options.Directory, 0o700); err != nil {
		return Info{}, fmt.Errorf("creating TLS directory: %w", err)
	}
	// The key is written first: a half-finished install then leaves the
	// old certificate file in place, and Reload still matches old files.
	if err := writeAtomically(filepath.Join(manager.options.Directory, KeyFile), keyPEM); err != nil {
		return Info{}, err
	}
	if err := writeAtomically(filepath.Join(manager.options.Directory, CertificateFile), certificatePEM); err != nil {
		return Info{}, err
	}
	if err := manager.Reload(ctx); err != nil {
		return Info{}, err
	}
	return manager.Status().Info, nil
}

// Remove deletes the installed certificate and stops serving HTTPS.
func (manager *Manager) Remove() error {
	for _, name := range []string{CertificateFile, KeyFile} {
		err := os.Remove(filepath.Join(manager.options.Directory, name))
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("removing %s: %w", name, err)
		}
	}
	manager.current.Store(nil)
	manager.mutex.Lock()
	manager.info, manager.modified, manager.warned = Info{}, manager.modified.Add(0), 0
	manager.mutex.Unlock()
	return nil
}

// writeAtomically writes a private file via a temporary file and rename.
func writeAtomically(path string, contents []byte) error {
	temporary := path + ".tmp"
	if err := os.WriteFile(temporary, contents, 0o600); err != nil {
		return fmt.Errorf("writing %s: %w", filepath.Base(path), err)
	}
	if err := os.Rename(temporary, path); err != nil {
		return fmt.Errorf("saving %s: %w", filepath.Base(path), err)
	}
	return nil
}
