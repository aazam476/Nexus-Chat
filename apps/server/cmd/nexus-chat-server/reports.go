package main

import (
	accountsstore "github.com/aazam476/nexus-chat/apps/server/internal/accounts/store"
	"github.com/aazam476/nexus-chat/apps/server/internal/config"
	"github.com/aazam476/nexus-chat/apps/server/internal/delivery"
	"github.com/aazam476/nexus-chat/apps/server/internal/instanceadmin"
	instanceadminstore "github.com/aazam476/nexus-chat/apps/server/internal/instanceadmin/store"
	"github.com/aazam476/nexus-chat/apps/server/internal/instanceconfig"
	instanceconfigstore "github.com/aazam476/nexus-chat/apps/server/internal/instanceconfig/store"
	"github.com/aazam476/nexus-chat/apps/server/internal/platform/clock"
	"github.com/aazam476/nexus-chat/apps/server/internal/reports"
	reportsstore "github.com/aazam476/nexus-chat/apps/server/internal/reports/store"
	"github.com/aazam476/nexus-chat/apps/server/internal/servers"
)

// newReports builds user reports (spec §4.7). Receipts are signed with the
// instance key that signs membership directives, which devices already trust.
func (app *application) newReports(copies *delivery.Service, moderation *servers.Service, configuration config.Config) *reports.Handler {
	system := clock.System{}
	service := reports.NewService(reports.Dependencies{
		Store: reportsstore.New(app.pool), Copies: copies, Moderation: moderation,
		Settings: instanceconfig.NewService(instanceconfigstore.New(app.pool), system, configuration.Domain),
		Users:    accountsstore.New(app.pool), Key: app.secrets.FederationKey, Clock: system,
	})
	return reports.NewHandler(service, accountsstore.New(app.pool))
}

// newInstanceAdmin builds the instance admin panel's accounts, servers,
// settings and numbers.
func (app *application) newInstanceAdmin(serverService *servers.Service, configuration config.Config) *instanceadmin.Handler {
	system := clock.System{}
	service := instanceadmin.NewService(instanceadmin.Dependencies{
		Store: instanceadminstore.New(app.pool), Servers: serverService, Clock: system,
		Settings: instanceconfig.NewService(instanceconfigstore.New(app.pool), system, configuration.Domain),
	})
	return instanceadmin.NewHandler(service, accountsstore.New(app.pool))
}
