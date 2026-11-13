package servers

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/aazam476/nexus-chat/apps/server/internal/permissions"
	"github.com/aazam476/nexus-chat/apps/server/internal/platform/clock"
	"github.com/aazam476/nexus-chat/apps/server/internal/servers/store"
)

// Kinds of channel.
const (
	KindCategory = "category"
	KindText     = "text"
	KindVoice    = "voice"
)

// Errors returned to the caller.
var (
	ErrNotFound      = errors.New("there is no such server, or you are not in it")
	ErrNoChannel     = errors.New("there is no such channel in this server")
	ErrNoRole        = errors.New("there is no such role in this server")
	ErrNoMember      = errors.New("there is no such member in this server")
	ErrForbidden     = errors.New("you do not have the permission for this")
	ErrInvalid       = errors.New("a name is 1 to 100 characters, a description at most 1000, a topic at most 1024")
	ErrOwnerLeaving  = errors.New("the owner cannot leave the server; delete it instead")
	ErrBadOrder      = errors.New("the order must list every channel or role exactly once")
	ErrBadPermission = errors.New("unknown permission name")
)

// Scope is who is acting, and in which server.
type Scope struct {
	ServerID uuid.UUID
	UserID   uuid.UUID
}

// Target is a role or member an override is for.
type Target struct {
	Kind string
	ID   uuid.UUID
}

// Server is a server's settings.
type Server struct {
	ID                 uuid.UUID
	Name               string
	Description        string
	OwnerID            uuid.UUID
	AttachmentMaxBytes *int64
	CompressMedia      bool
	CreatedAt          time.Time
}

// Role is one of a server's roles; Position 0 is @everyone.
type Role struct {
	ID          uuid.UUID
	Name        string
	Color       *int32
	Permissions permissions.Permission
	Position    int32
	Everyone    bool
}

// Override is a channel's allow and deny for one role or member.
type Override struct {
	TargetKind string
	TargetID   uuid.UUID
	Allow      permissions.Permission
	Deny       permissions.Permission
}

// Channel is a category, text or voice channel, with what the caller may do in it.
type Channel struct {
	ID          uuid.UUID
	Kind        string
	Name        string
	Topic       string
	ParentID    *uuid.UUID
	Position    int32
	MLSGroupID  []byte
	Overrides   []Override
	Permissions permissions.Permission
}

// Details is a server as one member sees it: the channels they can see,
// every role, and what they may do in the server.
type Details struct {
	Server      Server
	Roles       []Role
	Channels    []Channel
	MyRoleIDs   []uuid.UUID
	Permissions permissions.Permission
}

// Notifier tells members' devices that a server changed; they fetch it again.
type Notifier interface {
	ServerChanged(ctx context.Context, userIDs []uuid.UUID, serverID uuid.UUID)
}

// Dependencies are what the service needs.
type Dependencies struct {
	Store    *store.Writer
	Notifier Notifier
	Clock    clock.Clock
}

// Service manages servers.
type Service struct {
	store    *store.Writer
	notifier Notifier
	clock    clock.Clock
}

// NewService returns a Service.
func NewService(dependencies Dependencies) *Service {
	return &Service{store: dependencies.Store, notifier: dependencies.Notifier, clock: dependencies.Clock}
}

// notify tells every member of the server that it changed.
func (service *Service) notify(ctx context.Context, serverID uuid.UUID) error {
	members, err := service.store.ServerMemberIDs(ctx, serverID)
	if err != nil {
		return err
	}
	service.notifier.ServerChanged(ctx, members, serverID)
	return nil
}
