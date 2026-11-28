package instanceadmin

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/aazam476/nexus-chat/apps/server/internal/instanceadmin/store"
	"github.com/aazam476/nexus-chat/apps/server/internal/instanceconfig"
	"github.com/aazam476/nexus-chat/apps/server/internal/platform/clock"
)

// Limits on what the panel lists and takes.
const (
	pageSize      = 50
	maximumQuery  = 100
	maximumReason = 512
	// activeWindow is how recently a user must have used a session to count as active.
	activeWindow = 30 * 24 * time.Hour
)

// Errors returned to the caller.
var (
	ErrNoUser       = errors.New("there is no such account")
	ErrNoServer     = errors.New("there is no such server")
	ErrSelf         = errors.New("you cannot do this to your own account")
	ErrLastAdmin    = errors.New("the instance needs at least one admin who is not suspended")
	ErrNotSuspended = errors.New("that account is not suspended")
	ErrInvalid      = errors.New("a search is at most 100 characters and a reason at most 512")
)

// Settings are the instance settings the panel changes; *instanceconfig.Service implements them.
type Settings interface {
	RegistrationMode(ctx context.Context) (string, error)
	SetRegistrationMode(ctx context.Context, mode string) error
	Federation(ctx context.Context) (instanceconfig.FederationPolicy, error)
	SetFederation(ctx context.Context, policy instanceconfig.FederationPolicy) error
	ReportsToAdmins(ctx context.Context) (bool, error)
	SetReportsToAdmins(ctx context.Context, enabled bool) error
}

// Servers deletes servers for admins; *servers.Service implements it.
type Servers interface {
	DeleteAsAdmin(ctx context.Context, serverID uuid.UUID) error
}

// Dependencies are what the service needs.
type Dependencies struct {
	Store    *store.Queries
	Settings Settings
	Servers  Servers
	Clock    clock.Clock
}

// Service runs the admin panel.
type Service struct {
	dependencies Dependencies
}

// NewService returns a Service.
func NewService(dependencies Dependencies) *Service {
	return &Service{dependencies: dependencies}
}

// User is an account as the panel lists it.
type User = store.ListUsersRow

// pattern is an ILIKE pattern for text containing query, wildcards taken literally.
func pattern(query string) string {
	return "%" + strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(strings.TrimSpace(query)) + "%"
}

// Users lists accounts by username after `after`, matching query in
// username, display name or email, a page at a time.
func (service *Service) Users(ctx context.Context, query string, after string) ([]User, error) {
	if len([]rune(query)) > maximumQuery {
		return nil, ErrInvalid
	}
	return service.dependencies.Store.ListUsers(ctx, store.ListUsersParams{Pattern: pattern(query), After: after, RowLimit: pageSize})
}

// target loads an account an admin acts on, never their own.
func (service *Service) target(ctx context.Context, adminID uuid.UUID, userID uuid.UUID) (store.GetAdminUserRow, error) {
	if adminID == userID {
		return store.GetAdminUserRow{}, ErrSelf
	}
	user, err := service.dependencies.Store.GetAdminUser(ctx, userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return store.GetAdminUserRow{}, ErrNoUser
	}
	return user, err
}

// Suspend stops an account from signing in and ends its sessions; its
// devices are disconnected at their next heartbeat. Suspending again
// replaces the reason.
func (service *Service) Suspend(ctx context.Context, adminID uuid.UUID, userID uuid.UUID, reason string) error {
	if len([]rune(reason)) > maximumReason {
		return ErrInvalid
	}
	user, err := service.target(ctx, adminID, userID)
	if err != nil {
		return err
	}
	if user.IsInstanceAdmin && user.SuspendedAt == nil {
		if err := service.keepsAnAdmin(ctx); err != nil {
			return err
		}
	}
	now := service.dependencies.Clock.Now()
	if err := service.dependencies.Store.SuspendUser(ctx, store.SuspendUserParams{ID: userID, SuspendedAt: &now, Reason: reason}); err != nil {
		return err
	}
	return service.dependencies.Store.EndUserSessions(ctx, userID)
}

// keepsAnAdmin fails unless another active admin would remain after one goes.
func (service *Service) keepsAnAdmin(ctx context.Context) error {
	admins, err := service.dependencies.Store.CountInstanceAdmins(ctx)
	if err != nil {
		return err
	}
	if admins <= 1 {
		return ErrLastAdmin
	}
	return nil
}

// Reinstate lets a suspended account sign in again.
func (service *Service) Reinstate(ctx context.Context, adminID uuid.UUID, userID uuid.UUID) error {
	if _, err := service.target(ctx, adminID, userID); err != nil {
		return err
	}
	changed, err := service.dependencies.Store.ReinstateUser(ctx, userID)
	if err != nil {
		return err
	}
	if changed == 0 {
		return ErrNotSuspended
	}
	return nil
}

// SetAdmin makes an account an instance admin, or not. Admins cannot
// change their own standing, so the instance always keeps one.
func (service *Service) SetAdmin(ctx context.Context, adminID uuid.UUID, userID uuid.UUID, admin bool) error {
	user, err := service.target(ctx, adminID, userID)
	if err != nil {
		return err
	}
	if !admin && user.IsInstanceAdmin && user.SuspendedAt == nil {
		if err := service.keepsAnAdmin(ctx); err != nil {
			return err
		}
	}
	return service.dependencies.Store.SetInstanceAdmin(ctx, store.SetInstanceAdminParams{ID: userID, Admin: admin})
}

// Server is a server as the panel lists it.
type Server = store.ListServersRow

// ServerCursor is where the previous page of servers ended.
type ServerCursor struct {
	Name string
	ID   string
}

// Servers lists servers by name after the cursor, matching query.
func (service *Service) Servers(ctx context.Context, query string, after ServerCursor) ([]Server, error) {
	if len([]rune(query)) > maximumQuery {
		return nil, ErrInvalid
	}
	return service.dependencies.Store.ListServers(ctx, store.ListServersParams{
		Pattern: pattern(query), AfterName: after.Name, AfterID: after.ID, RowLimit: pageSize,
	})
}

// DeleteServer removes a server and everything in it.
func (service *Service) DeleteServer(ctx context.Context, serverID uuid.UUID) error {
	return service.dependencies.Servers.DeleteAsAdmin(ctx, serverID)
}

// Stats are the instance's numbers.
type Stats = store.InstanceStatsRow

// Stats counts accounts, active users (a session used in the last 30
// days), servers, conversations, messages in the last day, open reports
// and stored attachments.
func (service *Service) Stats(ctx context.Context) (Stats, error) {
	now := service.dependencies.Clock.Now()
	return service.dependencies.Store.InstanceStats(ctx, store.InstanceStatsParams{ActiveSince: now.Add(-activeWindow), MessagesSince: now.Add(-24 * time.Hour)})
}

// Settings returns the settings the panel shows and changes.
func (service *Service) Settings() Settings {
	return service.dependencies.Settings
}
