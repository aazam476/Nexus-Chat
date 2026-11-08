// Package webclient serves the web build of the Nexus Chat app (spec §2:
// the server also serves the web client) with the security headers a page
// that handles end-to-end encrypted content needs.
//
// The build is a single-page app: every address that is not a file is
// answered with index.html, and the app's router takes it from there.
//
// Start reading at [Handler].
package webclient

import (
	"errors"
	"io/fs"
	"net/http"
	"os"
	"path"
	"strings"
)

// contentSecurityPolicy allows only the instance's own scripts, styles
// and connections, so injected markup cannot load or run code from
// elsewhere or send data to another site. 'self' covers the gateway's
// ws:// and wss:// addresses on this host. React Native Web sets styles
// inline, hence 'unsafe-inline' for styles only. Images may be data: and
// blob: URLs, which the app creates for previews.
const contentSecurityPolicy = "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; " +
	"img-src 'self' data: blob:; connect-src 'self'; font-src 'self' data:; " +
	"object-src 'none'; base-uri 'none'; form-action 'self'; frame-ancestors 'none'"

// permissionsPolicy turns off browser features the app never uses. Camera,
// microphone and screen capture stay available to the instance's own pages
// for calls (M8, M9); the browser still asks the user first.
const permissionsPolicy = "camera=(self), microphone=(self), display-capture=(self), " +
	"geolocation=(), payment=(), usb=(), serial=(), bluetooth=(), hid=(), midi=(), " +
	"accelerometer=(), gyroscope=(), magnetometer=(), browsing-topics=()"

// immutablePrefix holds files whose names contain a content hash, which a
// browser may keep forever.
const immutablePrefix = "/_expo/static/"

// Handler serves the build in directory.
type Handler struct {
	files fs.FS
}

// NewHandler returns a Handler for the build in directory.
func NewHandler(directory string) *Handler {
	return &Handler{files: os.DirFS(directory)}
}

// NewHandlerFS returns a Handler for files; tests use an in-memory build.
func NewHandlerFS(files fs.FS) *Handler {
	return &Handler{files: files}
}

// ServeHTTP answers with the requested file, or index.html for app routes.
func (handler *Handler) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	name := strings.TrimPrefix(path.Clean("/"+request.URL.Path), "/")
	headers := writer.Header()
	headers.Set("Content-Security-Policy", contentSecurityPolicy)
	headers.Set("Cross-Origin-Opener-Policy", "same-origin")
	// Pages load only same-origin files, so cross-origin isolation costs
	// nothing and keeps other sites' resources out of the process.
	headers.Set("Cross-Origin-Embedder-Policy", "require-corp")
	headers.Set("Permissions-Policy", permissionsPolicy)
	if name == "" || !handler.isFile(name) {
		name = "index.html"
		// The page names the current hashed bundles, so it must be
		// fetched again after every upgrade.
		headers.Set("Cache-Control", "no-cache")
	} else if strings.HasPrefix(request.URL.Path, immutablePrefix) {
		headers.Set("Cache-Control", "public, max-age=31536000, immutable")
	}
	// name was cleaned against "/", and an fs.FS rejects any name that is
	// not a plain relative path, so no request can leave the build folder;
	// TestPathsThatClimbOutAreRefused checks it.
	http.ServeFileFS(writer, request, handler.files, name) //nolint:gosec // G703: see above
}

func (handler *Handler) isFile(name string) bool {
	info, err := fs.Stat(handler.files, name)
	return err == nil && !info.IsDir() && !errors.Is(err, fs.ErrNotExist)
}
