package main

import (
	"net/http"

	"github.com/aazam476/nexus-chat/apps/server/internal/identities"
	identitiesstore "github.com/aazam476/nexus-chat/apps/server/internal/identities/store"
	"github.com/aazam476/nexus-chat/apps/server/internal/keypackages"
	keypackagesstore "github.com/aazam476/nexus-chat/apps/server/internal/keypackages/store"
	"github.com/aazam476/nexus-chat/apps/server/internal/platform/clock"
	"github.com/aazam476/nexus-chat/apps/server/internal/platform/ratelimit"
	"github.com/aazam476/nexus-chat/apps/server/internal/webclient"
)

// webClient serves the web build from directory, or nothing when the
// server runs without one (development, API-only deployments).
func webClient(directory string) http.Handler {
	if directory == "" {
		return nil
	}
	return webclient.NewHandler(directory)
}

// newKeyPackagesHandler builds the MLS key-package directory. Claims are
// limited per claiming user (60 a minute, bursts of 20), so nobody can
// drain another user's packages.
func (app *application) newKeyPackagesHandler() *keypackages.Handler {
	system := clock.System{}
	return keypackages.NewHandler(keypackages.NewService(
		keypackagesstore.NewStock(app.pool), ratelimit.New(ratelimit.PerMinute(60, 20), system), system,
	))
}

// newIdentitiesHandler builds the published-identity directory.
func (app *application) newIdentitiesHandler() *identities.Handler {
	return identities.NewHandler(identities.NewService(identitiesstore.New(app.pool), clock.System{}))
}
