package main

import (
	"net/http"

	"github.com/aazam476/nexus-chat/apps/server/internal/httpserver"
	"github.com/aazam476/nexus-chat/apps/server/internal/identities"
	identitiesstore "github.com/aazam476/nexus-chat/apps/server/internal/identities/store"
	"github.com/aazam476/nexus-chat/apps/server/internal/keybackup"
	keybackupstore "github.com/aazam476/nexus-chat/apps/server/internal/keybackup/store"
	"github.com/aazam476/nexus-chat/apps/server/internal/keypackages"
	keypackagesstore "github.com/aazam476/nexus-chat/apps/server/internal/keypackages/store"
	"github.com/aazam476/nexus-chat/apps/server/internal/platform/clock"
	"github.com/aazam476/nexus-chat/apps/server/internal/platform/ratelimit"
	"github.com/aazam476/nexus-chat/apps/server/internal/social"
	socialstore "github.com/aazam476/nexus-chat/apps/server/internal/social/store"
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

// newKeyBackupHandler builds the encrypted key-backup store.
func (app *application) newKeyBackupHandler() *keybackup.Handler {
	return keybackup.NewHandler(keybackup.NewService(keybackupstore.New(app.pool), clock.System{}))
}

// addEncryptionFeatures adds what end-to-end encryption needs from the
// server: the key-package directory, published identities and the key backup.
func (app *application) addEncryptionFeatures(features *httpserver.Features) {
	features.KeyPackages = app.newKeyPackagesHandler()
	features.Identities = app.newIdentitiesHandler()
	features.KeyBackup = app.newKeyBackupHandler()
}

// addSocialFeatures adds friends, blocks and privacy. Friend requests are
// limited per sender (30 a minute, bursts of 10); changes reach both
// users' devices through the realtime hub, so it must exist already.
func (app *application) addSocialFeatures(features *httpserver.Features) {
	system := clock.System{}
	features.Social = social.NewHandler(social.NewService(social.Dependencies{
		Store: socialstore.New(app.pool), Notifier: app.hub, Servers: social.NoServers{},
		Limiter: ratelimit.New(ratelimit.PerMinute(30, 10), system), Clock: system,
	}))
}
