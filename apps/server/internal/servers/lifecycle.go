package servers

import (
	"context"
	"crypto/rand"
	"errors"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/aazam476/nexus-chat/apps/server/internal/permissions"
	"github.com/aazam476/nexus-chat/apps/server/internal/servers/store"
)

// Limits on what members type (spec §4.4).
const (
	maximumName        = 100
	maximumDescription = 1000
	maximumTopic       = 1024
)

// cleanName trims a name and checks its length.
func cleanName(name string) (string, error) {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" || utf8.RuneCountInString(trimmed) > maximumName {
		return "", ErrInvalid
	}
	return trimmed, nil
}

func newMLSGroupID() []byte {
	id := make([]byte, 16)
	_, _ = rand.Read(id) // crypto/rand never fails on supported platforms
	return id
}

// Create makes a server owned by ownerID, with @everyone and a text and a
// voice channel to start with.
func (service *Service) Create(ctx context.Context, ownerID uuid.UUID, name string, description string) (Details, error) {
	name, err := cleanName(name)
	if err != nil || utf8.RuneCountInString(description) > maximumDescription {
		return Details{}, ErrInvalid
	}
	now := service.clock.Now()
	id := uuid.Must(uuid.NewV7())
	channel := func(kind string, name string, parent *uuid.UUID, position int32) store.InsertChannelParams {
		return store.InsertChannelParams{ID: uuid.Must(uuid.NewV7()), ServerID: id, Kind: kind, Name: name, ParentID: parent, Position: position, CreatedAt: now}
	}
	textCategory, voiceCategory := channel(KindCategory, "Text channels", nil, 0), channel(KindCategory, "Voice channels", nil, 1)
	err = service.store.CreateServer(ctx, store.NewServer{
		Server:   store.InsertServerParams{ID: id, Name: name, Description: description, OwnerID: ownerID, CreatedAt: now},
		Everyone: store.InsertRoleParams{ID: uuid.Must(uuid.NewV7()), ServerID: id, Name: "@everyone", Permissions: int64(permissions.DefaultEveryone), Everyone: true},
		Owner:    store.AddServerMemberParams{ServerID: id, UserID: ownerID, JoinedAt: now},
		Channels: []store.InsertChannelParams{
			textCategory, voiceCategory,
			channel(KindText, "general", &textCategory.ID, 0), channel(KindVoice, "General", &voiceCategory.ID, 0),
		},
	})
	if err != nil {
		return Details{}, err
	}
	if err := service.reconcile(ctx, id); err != nil {
		return Details{}, err
	}
	return service.Details(ctx, Scope{ServerID: id, UserID: ownerID})
}

// List returns the servers scope.UserID is a member of.
func (service *Service) List(ctx context.Context, userID uuid.UUID) ([]Server, error) {
	rows, err := service.store.ListServersOf(ctx, userID)
	if err != nil {
		return nil, err
	}
	servers := make([]Server, 0, len(rows))
	for _, row := range rows {
		servers = append(servers, serverOf(row))
	}
	return servers, nil
}

func serverOf(row store.Server) Server {
	return Server{
		ID: row.ID, Name: row.Name, Description: row.Description, OwnerID: row.OwnerID,
		AttachmentMaxBytes: row.AttachmentMaxBytes, CompressMedia: row.CompressMedia, CreatedAt: row.CreatedAt,
	}
}

// Details returns the server as scope.UserID sees it.
func (service *Service) Details(ctx context.Context, scope Scope) (Details, error) {
	a, err := service.load(ctx, scope)
	if err != nil {
		return Details{}, err
	}
	channels, err := service.store.ListChannels(ctx, scope.ServerID)
	if err != nil {
		return Details{}, err
	}
	groups, err := service.audienceGroups(ctx, scope.ServerID)
	if err != nil {
		return Details{}, err
	}
	details := Details{Server: serverOf(a.server), Permissions: a.inServer()}
	for _, role := range a.roles {
		details.Roles = append(details.Roles, roleOf(role))
		if a.myRoles[role.ID] {
			details.MyRoleIDs = append(details.MyRoleIDs, role.ID)
		}
	}
	for _, row := range channels {
		if mine := a.inChannel(row.ID); mine.Has(permissions.ViewChannel) {
			details.Channels = append(details.Channels, channelOf(row, a.overrides, mine, groups))
		}
	}
	return details, nil
}

func roleOf(row store.ServerRole) Role {
	return Role{
		ID: row.ID, Name: row.Name, Color: row.Color, Permissions: permissions.Permission(row.Permissions), //nolint:gosec // stored non-negative
		Position: row.Position, Everyone: row.Everyone,
	}
}

// audienceGroups maps the server's audiences to their MLS group IDs.
func (service *Service) audienceGroups(ctx context.Context, serverID uuid.UUID) (map[uuid.UUID][]byte, error) {
	rows, err := service.store.ListAudiences(ctx, &serverID)
	groups := map[uuid.UUID][]byte{}
	for _, row := range rows {
		groups[row.ID] = row.MlsGroupID
	}
	return groups, err
}

func channelOf(row store.ServerChannel, overrides []store.ServerChannelOverride, mine permissions.Permission, groups map[uuid.UUID][]byte) Channel {
	channel := Channel{
		ID: row.ID, Kind: row.Kind, Name: row.Name, Topic: row.Topic, ParentID: row.ParentID,
		Position: row.Position, AudienceID: row.AudienceID, Permissions: mine, Overrides: []Override{},
	}
	if row.AudienceID != nil {
		channel.MLSGroupID = groups[*row.AudienceID]
	}
	for _, o := range overrides {
		if o.ChannelID == row.ID {
			channel.Overrides = append(channel.Overrides, Override{
				TargetKind: o.TargetKind, TargetID: o.TargetID,
				Allow: permissions.Permission(o.AllowBits), Deny: permissions.Permission(o.DenyBits), //nolint:gosec // stored non-negative
			})
		}
	}
	return channel
}

// Settings are the server's changeable settings.
type Settings struct {
	Name               string
	Description        string
	AttachmentMaxBytes *int64
	CompressMedia      bool
}

// Update changes the server's settings (MANAGE_SERVER).
func (service *Service) Update(ctx context.Context, scope Scope, settings Settings) error {
	a, err := service.load(ctx, scope)
	if err != nil {
		return err
	}
	if err := a.require(permissions.ManageServer); err != nil {
		return err
	}
	name, err := cleanName(settings.Name)
	if err != nil || utf8.RuneCountInString(settings.Description) > maximumDescription ||
		(settings.AttachmentMaxBytes != nil && *settings.AttachmentMaxBytes <= 0) {
		return ErrInvalid
	}
	if err := service.store.UpdateServer(ctx, store.UpdateServerParams{
		ID: scope.ServerID, Name: name, Description: settings.Description,
		AttachmentMaxBytes: settings.AttachmentMaxBytes, CompressMedia: settings.CompressMedia,
	}); err != nil {
		return err
	}
	return service.notify(ctx, scope.ServerID)
}

// Delete removes the server and everything in it; only its owner may.
func (service *Service) Delete(ctx context.Context, scope Scope) error {
	a, err := service.load(ctx, scope)
	if err != nil {
		return err
	}
	if !a.owner() {
		return ErrForbidden
	}
	members, err := service.store.ServerMemberIDs(ctx, scope.ServerID)
	if err != nil {
		return err
	}
	if err := service.store.DeleteServer(ctx, scope.ServerID); err != nil {
		return err
	}
	service.notifier.ServerChanged(ctx, members, scope.ServerID)
	return nil
}

// Leave takes scope.UserID out of the server; the owner cannot leave.
func (service *Service) Leave(ctx context.Context, scope Scope) error {
	a, err := service.load(ctx, scope)
	if err != nil {
		return err
	}
	if a.owner() {
		return ErrOwnerLeaving
	}
	if _, err := service.store.RemoveServerMember(ctx, store.RemoveServerMemberParams{ServerID: scope.ServerID, UserID: scope.UserID}); err != nil {
		return err
	}
	service.notifier.ServerChanged(ctx, []uuid.UUID{scope.UserID}, scope.ServerID)
	return service.notify(ctx, scope.ServerID)
}

// ShareAServer reports whether two users are in a server together, for the
// "server members" privacy choices (it implements social.ServerMembership).
func (service *Service) ShareAServer(ctx context.Context, first uuid.UUID, second uuid.UUID) (bool, error) {
	shares, err := service.store.SharesServer(ctx, store.SharesServerParams{UserID: first, UserID_2: second})
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	return shares, err
}
