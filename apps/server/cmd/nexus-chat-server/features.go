package main

import (
	"context"
	"errors"
	"net/http"
	"path/filepath"

	"github.com/google/uuid"

	"github.com/aazam476/nexus-chat/apps/server/internal/admin"
	"github.com/aazam476/nexus-chat/apps/server/internal/attachments"
	attachmentsstore "github.com/aazam476/nexus-chat/apps/server/internal/attachments/store"
	"github.com/aazam476/nexus-chat/apps/server/internal/config"
	"github.com/aazam476/nexus-chat/apps/server/internal/conversations"
	conversationsstore "github.com/aazam476/nexus-chat/apps/server/internal/conversations/store"
	"github.com/aazam476/nexus-chat/apps/server/internal/delivery"
	deliverystore "github.com/aazam476/nexus-chat/apps/server/internal/delivery/store"
	"github.com/aazam476/nexus-chat/apps/server/internal/httpserver"
	"github.com/aazam476/nexus-chat/apps/server/internal/identities"
	identitiesstore "github.com/aazam476/nexus-chat/apps/server/internal/identities/store"
	"github.com/aazam476/nexus-chat/apps/server/internal/instanceconfig"
	instanceconfigstore "github.com/aazam476/nexus-chat/apps/server/internal/instanceconfig/store"
	"github.com/aazam476/nexus-chat/apps/server/internal/keybackup"
	keybackupstore "github.com/aazam476/nexus-chat/apps/server/internal/keybackup/store"
	"github.com/aazam476/nexus-chat/apps/server/internal/keypackages"
	keypackagesstore "github.com/aazam476/nexus-chat/apps/server/internal/keypackages/store"
	"github.com/aazam476/nexus-chat/apps/server/internal/linkpreview"
	"github.com/aazam476/nexus-chat/apps/server/internal/platform/clock"
	"github.com/aazam476/nexus-chat/apps/server/internal/platform/filestore"
	"github.com/aazam476/nexus-chat/apps/server/internal/platform/ratelimit"
	"github.com/aazam476/nexus-chat/apps/server/internal/platform/secretbox"
	"github.com/aazam476/nexus-chat/apps/server/internal/servers"
	serversstore "github.com/aazam476/nexus-chat/apps/server/internal/servers/store"
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

// addSocialFeatures adds servers, friends, blocks and privacy, DMs and
// group DMs, and the MLS delivery service. Friend requests are limited per sender (30
// a minute, bursts of 10). Changes reach users' devices through the
// realtime hub, so it must exist already.
func (app *application) addSocialFeatures(features *httpserver.Features, configuration config.Config) {
	system := clock.System{}
	serverService := servers.NewService(servers.Dependencies{Store: serversstore.NewWriter(app.pool), Notifier: app.hub, Clock: system})
	features.Servers = servers.NewHandler(serverService)
	socialService := social.NewService(social.Dependencies{
		Store: socialstore.New(app.pool), Notifier: app.hub, Servers: serverService,
		Limiter: ratelimit.New(ratelimit.PerMinute(30, 10), system), Clock: system,
	})
	features.Social = social.NewHandler(socialService)
	// Conversations tell the delivery service when someone leaves, and the
	// delivery service asks conversations who is in them; the forwarder
	// lets each be built with the other.
	leaves := &leaveForwarder{}
	conversationService := conversations.NewService(conversations.Dependencies{
		Store: conversationsstore.NewWriter(app.pool), Policy: socialService, Notifier: app.hub, Members: leaves, Clock: system,
		MaximumGroupMembers: configuration.GroupDMMaxMembers,
		// Icon changes are limited per user, like profile pictures.
		Icons: conversations.Icons{
			Files:   filestore.NewLocal(filepath.Join(configuration.DataDirectory, "files")),
			Limiter: ratelimit.New(ratelimit.PerMinute(10, 5), system), Logger: app.logger,
		},
	})
	deliveryService := delivery.NewService(delivery.Dependencies{
		Store: deliverystore.NewWriter(app.pool), Members: conversationService, Notifier: app.hub,
		DirectiveKey: app.secrets.FederationKey, Clock: system,
	})
	leaves.delivery = deliveryService
	app.typing.conversations = conversationService
	features.Conversations = conversations.NewHandler(conversationService)
	features.LinkPreviews = linkpreview.NewHandler(linkpreview.NewService(linkpreview.Options{
		Switch: instanceconfig.NewService(instanceconfigstore.New(app.pool), system, configuration.Domain),
		// Previews are limited per user: 30 a minute, bursts of 10.
		Limiter: ratelimit.New(ratelimit.PerMinute(30, 10), system),
	}))
	features.Attachments = attachments.NewHandler(attachments.NewService(attachments.Dependencies{
		Store: attachmentsstore.New(app.pool), Files: filestore.NewLocal(filepath.Join(configuration.DataDirectory, "files")),
		Members: conversationService, Limits: instanceconfig.NewService(instanceconfigstore.New(app.pool), system, configuration.Domain),
		// Uploads are limited per user: 60 a minute, bursts of 20.
		Limiter: ratelimit.New(ratelimit.PerMinute(60, 20), system), Clock: system,
	}))
	features.Delivery = delivery.NewHandler(deliveryService)
}

// leaveForwarder passes conversations' leave events to the delivery service.
type leaveForwarder struct {
	delivery *delivery.Service
}

// MemberLeft implements conversations.MembershipListener.
func (forwarder *leaveForwarder) MemberLeft(ctx context.Context, conversationID uuid.UUID, userID uuid.UUID) error {
	return forwarder.delivery.MemberLeft(ctx, conversationID, userID)
}

// typingForwarder answers the gateway's typing questions from the
// conversations feature. Someone who may not send in a conversation is
// told nothing went wrong, and nobody hears them.
type typingForwarder struct {
	conversations *conversations.Service
}

// TypingRecipients implements realtime.TypingRelay.
func (forwarder *typingForwarder) TypingRecipients(ctx context.Context, conversationID uuid.UUID, userID uuid.UUID) ([]uuid.UUID, error) {
	recipients, err := forwarder.conversations.TypingRecipients(ctx, conversationID, userID)
	if errors.Is(err, conversations.ErrNotFound) {
		return nil, nil
	}
	return recipients, err
}

// newMailSettings manages SMTP settings; the limit on test emails stops a
// script from flooding the mail server.
func newMailSettings(settings *instanceconfig.Service, box *secretbox.Box) *admin.MailSettings {
	return admin.NewMailSettings(admin.MailDependencies{
		Settings: settings, Box: box, NewSender: newMailer,
		TestLimiter: ratelimit.New(ratelimit.PerMinute(3, 3), clock.System{}),
	})
}
