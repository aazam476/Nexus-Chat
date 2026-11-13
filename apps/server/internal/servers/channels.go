package servers

import (
	"context"
	"errors"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/aazam476/nexus-chat/apps/server/internal/permissions"
	"github.com/aazam476/nexus-chat/apps/server/internal/servers/store"
)

// Override targets.
const (
	TargetRole   = "role"
	TargetMember = "member"
)

// managing loads the member's access and requires MANAGE_CHANNELS.
func (service *Service) managing(ctx context.Context, scope Scope) (access, error) {
	a, err := service.load(ctx, scope)
	if err != nil {
		return access{}, err
	}
	return a, a.require(permissions.ManageChannels)
}

// channelIn returns a channel of the server, or ErrNoChannel.
func (service *Service) channelIn(ctx context.Context, serverID uuid.UUID, channelID uuid.UUID) (store.ServerChannel, error) {
	channel, err := service.store.GetChannel(ctx, channelID)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && channel.ServerID != serverID) {
		return store.ServerChannel{}, ErrNoChannel
	}
	return channel, err
}

// NewChannel describes a channel to create; ParentID is its category.
type NewChannel struct {
	Kind     string
	Name     string
	Topic    string
	ParentID *uuid.UUID
}

// CreateChannel adds a category, text or voice channel at the end of its
// category (MANAGE_CHANNELS).
func (service *Service) CreateChannel(ctx context.Context, scope Scope, spec NewChannel) (uuid.UUID, error) {
	if _, err := service.managing(ctx, scope); err != nil {
		return uuid.Nil, err
	}
	name, err := service.checkNewChannel(ctx, scope, spec)
	if err != nil {
		return uuid.Nil, err
	}
	position, err := service.store.NextChannelPosition(ctx, store.NextChannelPositionParams{ServerID: scope.ServerID, ParentID: spec.ParentID})
	if err != nil {
		return uuid.Nil, err
	}
	params := store.InsertChannelParams{
		ID: uuid.Must(uuid.NewV7()), ServerID: scope.ServerID, Kind: spec.Kind, Name: name, Topic: spec.Topic,
		ParentID: spec.ParentID, Position: position, CreatedAt: service.clock.Now(),
	}
	if spec.Kind == KindText {
		params.MlsGroupID = newMLSGroupID()
	}
	if err := service.store.InsertChannel(ctx, params); err != nil {
		return uuid.Nil, err
	}
	return params.ID, service.notify(ctx, scope.ServerID)
}

// checkNewChannel validates a channel to create and returns its clean name.
func (service *Service) checkNewChannel(ctx context.Context, scope Scope, spec NewChannel) (string, error) {
	name, err := cleanName(spec.Name)
	known := spec.Kind == KindCategory || spec.Kind == KindText || spec.Kind == KindVoice
	if err != nil || !known || utf8.RuneCountInString(spec.Topic) > maximumTopic {
		return "", ErrInvalid
	}
	if spec.ParentID == nil {
		return name, nil
	}
	parent, err := service.channelIn(ctx, scope.ServerID, *spec.ParentID)
	if err != nil || parent.Kind != KindCategory || spec.Kind == KindCategory {
		return "", ErrInvalid
	}
	return name, nil
}

// ChannelText is a channel's name and topic.
type ChannelText struct {
	Name  string
	Topic string
}

// UpdateChannel renames a channel and sets its topic (MANAGE_CHANNELS).
func (service *Service) UpdateChannel(ctx context.Context, scope Scope, channelID uuid.UUID, text ChannelText) error {
	name, topic := text.Name, text.Topic
	if _, err := service.managing(ctx, scope); err != nil {
		return err
	}
	if _, err := service.channelIn(ctx, scope.ServerID, channelID); err != nil {
		return err
	}
	name, err := cleanName(name)
	if err != nil || utf8.RuneCountInString(topic) > maximumTopic {
		return ErrInvalid
	}
	if err := service.store.UpdateChannel(ctx, store.UpdateChannelParams{ID: channelID, Name: name, Topic: topic}); err != nil {
		return err
	}
	return service.notify(ctx, scope.ServerID)
}

// DeleteChannel removes a channel; a category's channels stay, uncategorized (MANAGE_CHANNELS).
func (service *Service) DeleteChannel(ctx context.Context, scope Scope, channelID uuid.UUID) error {
	if _, err := service.managing(ctx, scope); err != nil {
		return err
	}
	if _, err := service.channelIn(ctx, scope.ServerID, channelID); err != nil {
		return err
	}
	if err := service.store.DeleteChannel(ctx, channelID); err != nil {
		return err
	}
	return service.notify(ctx, scope.ServerID)
}

// Placement is where one channel goes: in which category, if any.
type Placement struct {
	ID       uuid.UUID
	ParentID *uuid.UUID
}

// OrderChannels places every channel of the server, in the order given
// within each category and among categories (drag and drop; MANAGE_CHANNELS).
// Listing every channel needs seeing every channel, so someone a channel
// is hidden from cannot rearrange them.
func (service *Service) OrderChannels(ctx context.Context, scope Scope, order []Placement) error {
	a, err := service.managing(ctx, scope)
	if err != nil {
		return err
	}
	channels, err := service.store.ListChannels(ctx, scope.ServerID)
	if err != nil {
		return err
	}
	for _, channel := range channels {
		if !a.inChannel(channel.ID).Has(permissions.ViewChannel) {
			return ErrForbidden
		}
	}
	moves, err := placements(channels, order)
	if err != nil {
		return err
	}
	if err := service.store.MoveChannels(ctx, moves); err != nil {
		return err
	}
	return service.notify(ctx, scope.ServerID)
}

// placements checks that order lists every channel once, in categories
// that exist, and numbers positions within each category.
func placements(channels []store.ServerChannel, order []Placement) ([]store.MoveChannelParams, error) {
	kinds := map[uuid.UUID]string{}
	for _, channel := range channels {
		kinds[channel.ID] = channel.Kind
	}
	if len(order) != len(channels) {
		return nil, ErrBadOrder
	}
	seen := map[uuid.UUID]bool{}
	next := map[uuid.UUID]int32{}
	moves := make([]store.MoveChannelParams, 0, len(order))
	for _, place := range order {
		kind, known := kinds[place.ID]
		if !known || seen[place.ID] || !validParent(kinds, kind, place.ParentID) {
			return nil, ErrBadOrder
		}
		seen[place.ID] = true
		parent := uuid.Nil
		if place.ParentID != nil {
			parent = *place.ParentID
		}
		moves = append(moves, store.MoveChannelParams{ID: place.ID, ParentID: place.ParentID, Position: next[parent]})
		next[parent]++
	}
	return moves, nil
}

// validParent: categories have no parent; other channels sit in a category or none.
func validParent(kinds map[uuid.UUID]string, kind string, parent *uuid.UUID) bool {
	if parent == nil {
		return true
	}
	return kind != KindCategory && kinds[*parent] == KindCategory
}

// SetOverride sets a channel's allow and deny for a role or member
// (MANAGE_CHANNELS). Nobody can allow or deny what they cannot do
// themselves, unless they are an administrator.
func (service *Service) SetOverride(ctx context.Context, scope Scope, channelID uuid.UUID, override Override) error {
	a, err := service.managing(ctx, scope)
	if err != nil {
		return err
	}
	if _, err := service.channelIn(ctx, scope.ServerID, channelID); err != nil {
		return err
	}
	if err := service.validTarget(ctx, a, override.TargetKind, override.TargetID); err != nil {
		return err
	}
	if override.Allow&override.Deny != 0 || !a.mayGrant(override.Allow|override.Deny) {
		return ErrForbidden
	}
	if err := service.store.UpsertOverride(ctx, store.UpsertOverrideParams{
		ChannelID: channelID, TargetKind: override.TargetKind, TargetID: override.TargetID,
		AllowBits: int64(override.Allow), DenyBits: int64(override.Deny), //nolint:gosec // permissions use 27 bits
	}); err != nil {
		return err
	}
	return service.notify(ctx, scope.ServerID)
}

// ClearOverride removes a channel's override for a role or member (MANAGE_CHANNELS).
func (service *Service) ClearOverride(ctx context.Context, scope Scope, channelID uuid.UUID, target Target) error {
	if _, err := service.managing(ctx, scope); err != nil {
		return err
	}
	if _, err := service.channelIn(ctx, scope.ServerID, channelID); err != nil {
		return err
	}
	if err := service.store.DeleteOverride(ctx, store.DeleteOverrideParams{ChannelID: channelID, TargetKind: target.Kind, TargetID: target.ID}); err != nil {
		return err
	}
	return service.notify(ctx, scope.ServerID)
}

// validTarget checks that an override's role or member belongs to the server.
func (service *Service) validTarget(ctx context.Context, a access, kind string, id uuid.UUID) error {
	switch kind {
	case TargetRole:
		if _, ok := a.role(id); !ok {
			return ErrNoRole
		}
		return nil
	case TargetMember:
		_, err := service.store.GetServerMember(ctx, store.GetServerMemberParams{ServerID: a.server.ID, UserID: id})
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNoMember
		}
		return err
	default:
		return ErrInvalid
	}
}
