package servers

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/aazam476/nexus-chat/apps/server/internal/delivery"
	"github.com/aazam476/nexus-chat/apps/server/internal/permissions"
	"github.com/aazam476/nexus-chat/apps/server/internal/reports"
	"github.com/aazam476/nexus-chat/apps/server/internal/servers/store"
)

// Moderation (spec §4.7): kicking, banning and timing out members. Each
// needs its permission and a highest role above the target's; nobody
// moderates the owner or themselves.

// Limits on moderation.
const (
	MinimumTimeout = time.Minute
	MaximumTimeout = 28 * 24 * time.Hour
	// MaximumPurge is how far back a ban may delete the member's messages.
	MaximumPurge      = 7 * 24 * time.Hour
	maximumReason     = 512
	foreignKeyViolate = "23503"
)

// Errors from moderation.
var (
	ErrBanned       = errors.New("you are banned from this server")
	ErrNotBanned    = errors.New("that user is not banned from this server")
	ErrTimeoutRange = errors.New("a timeout lasts 60 seconds to 28 days")
	ErrBadSanction  = errors.New("a reason is at most 512 characters, and a ban deletes at most 7 days of messages")
)

// Sanction is who a kick is for, and why.
type Sanction struct {
	UserID uuid.UUID
	Reason string
}

// BanRequest is who to ban, why, and how far back to delete their messages.
type BanRequest struct {
	UserID uuid.UUID
	Reason string
	// DeleteMessages is how far back to delete what they sent; 0 keeps it.
	DeleteMessages time.Duration
}

func (ban BanRequest) valid() bool {
	return len([]rune(ban.Reason)) <= maximumReason && ban.DeleteMessages >= 0 && ban.DeleteMessages <= MaximumPurge
}

// Banned is someone banned from a server.
type Banned struct {
	UserID      uuid.UUID
	Username    string
	DisplayName string
	BannedBy    *uuid.UUID
	Reason      string
	CreatedAt   time.Time
}

// over checks that scope may use want on targetID: they hold it, the target
// is someone else, not the owner, and below the actor's highest role.
// Someone not in the server counts as having no roles; ok says whether
// they are a member.
func (service *Service) over(ctx context.Context, scope Scope, want permissions.Permission, targetID uuid.UUID) (bool, error) {
	a, err := service.load(ctx, scope)
	if err != nil {
		return false, err
	}
	if err := a.require(want); err != nil {
		return false, err
	}
	if targetID == scope.UserID {
		return false, ErrForbidden
	}
	target, err := service.load(ctx, Scope{ServerID: scope.ServerID, UserID: targetID})
	if errors.Is(err, ErrNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if target.owner() || !permissions.Outranks(a.top(), target.top(), a.owner()) {
		return true, ErrForbidden
	}
	return true, nil
}

// Kick takes a member out of the server; they may join again (KICK_MEMBERS).
func (service *Service) Kick(ctx context.Context, scope Scope, sanction Sanction) error {
	if len([]rune(sanction.Reason)) > maximumReason {
		return ErrBadSanction
	}
	member, err := service.over(ctx, scope, permissions.KickMembers, sanction.UserID)
	if err != nil {
		return err
	}
	if !member {
		return ErrNoMember
	}
	if err := service.remove(ctx, scope.ServerID, sanction.UserID); err != nil {
		return err
	}
	return service.recordOnMember(ctx, scope, event(ActionMemberKick, TargetKindMember, sanction.UserID, map[string]any{"reason": sanction.Reason}))
}

// remove takes userID out of the server and tells everyone.
func (service *Service) remove(ctx context.Context, serverID uuid.UUID, userID uuid.UUID) error {
	if _, err := service.store.RemoveServerMember(ctx, store.RemoveServerMemberParams{ServerID: serverID, UserID: userID}); err != nil {
		return err
	}
	service.notifier.ServerChanged(ctx, []uuid.UUID{userID}, serverID)
	return service.notify(ctx, serverID)
}

// Ban keeps someone out of the server, taking them out if they are in it
// and deleting what they sent recently if asked (BAN_MEMBERS). Someone who
// is not a member can be banned too.
func (service *Service) Ban(ctx context.Context, scope Scope, ban BanRequest) error {
	if !ban.valid() {
		return ErrBadSanction
	}
	member, err := service.over(ctx, scope, permissions.BanMembers, ban.UserID)
	if err != nil {
		return err
	}
	err = service.store.InsertBan(ctx, store.InsertBanParams{
		ServerID: scope.ServerID, UserID: ban.UserID, BannedBy: &scope.UserID, Reason: ban.Reason, CreatedAt: service.clock.Now(),
	})
	if pgErr := (*pgconn.PgError)(nil); errors.As(err, &pgErr) && pgErr.Code == foreignKeyViolate {
		return ErrNoMember
	}
	if err != nil {
		return err
	}
	purged, err := service.purge(ctx, scope.ServerID, ban)
	if err != nil {
		return err
	}
	if member {
		if err := service.remove(ctx, scope.ServerID, ban.UserID); err != nil {
			return err
		}
	}
	details := map[string]any{"reason": ban.Reason, "deletedMessages": purged, "deleteSeconds": int64(ban.DeleteMessages / time.Second)}
	return service.recordOnMember(ctx, scope, event(ActionMemberBan, TargetKindMember, ban.UserID, details))
}

// purge deletes what a banned user sent to the server's channels recently.
func (service *Service) purge(ctx context.Context, serverID uuid.UUID, ban BanRequest) (int64, error) {
	if ban.DeleteMessages == 0 {
		return 0, nil
	}
	audiences, err := service.store.ListAudiences(ctx, &serverID)
	if err != nil || len(audiences) == 0 {
		return 0, err
	}
	ids := make([]uuid.UUID, 0, len(audiences))
	for _, audience := range audiences {
		ids = append(ids, audience.ID)
	}
	return service.audiences.PurgeMessages(ctx, ids, ban.UserID, service.clock.Now().Add(-ban.DeleteMessages))
}

// Unban lets someone join the server again (BAN_MEMBERS).
func (service *Service) Unban(ctx context.Context, scope Scope, userID uuid.UUID) error {
	if err := service.requiring(ctx, scope, permissions.BanMembers); err != nil {
		return err
	}
	removed, err := service.store.DeleteBan(ctx, store.DeleteBanParams{ServerID: scope.ServerID, UserID: userID})
	if err != nil {
		return err
	}
	if removed == 0 {
		return ErrNotBanned
	}
	return service.recordOnMember(ctx, scope, event(ActionMemberUnban, TargetKindMember, userID, map[string]any{}))
}

// Bans lists who is banned, most recent first (BAN_MEMBERS).
func (service *Service) Bans(ctx context.Context, scope Scope) ([]Banned, error) {
	if err := service.requiring(ctx, scope, permissions.BanMembers); err != nil {
		return nil, err
	}
	rows, err := service.store.ListBans(ctx, scope.ServerID)
	if err != nil {
		return nil, err
	}
	bans := make([]Banned, 0, len(rows))
	for _, row := range rows {
		bans = append(bans, Banned(row))
	}
	return bans, nil
}

// requiring loads scope's access and requires want in the server.
func (service *Service) requiring(ctx context.Context, scope Scope, want permissions.Permission) error {
	a, err := service.load(ctx, scope)
	if err != nil {
		return err
	}
	return a.require(want)
}

// Timeout stops a member from sending, reacting, speaking and joining
// calls for a while; nil (or zero) lifts it (TIMEOUT_MEMBERS).
func (service *Service) Timeout(ctx context.Context, scope Scope, userID uuid.UUID, length time.Duration) error {
	if length != 0 && (length < MinimumTimeout || length > MaximumTimeout) {
		return ErrTimeoutRange
	}
	member, err := service.over(ctx, scope, permissions.TimeoutMembers, userID)
	if err != nil {
		return err
	}
	if !member {
		return ErrNoMember
	}
	params := store.SetTimeoutParams{ServerID: scope.ServerID, UserID: userID}
	details := map[string]any{"cleared": true}
	if length != 0 {
		until := service.clock.Now().Add(length)
		params.Until, details = &until, map[string]any{"until": until, "seconds": int64(length / time.Second)}
	}
	if _, err := service.store.SetTimeout(ctx, params); err != nil {
		return err
	}
	service.notifier.ServerChanged(ctx, []uuid.UUID{userID}, scope.ServerID)
	return service.recordOnMember(ctx, scope, event(ActionMemberTimeout, TargetKindMember, userID, details))
}

// MessageDeleted records a moderator's deletion of a channel message (it
// implements the audit half of delivery.ChannelAccess).
func (service *Service) MessageDeleted(ctx context.Context, ref delivery.ChannelRef, deleted delivery.DeletedMessage) error {
	details := map[string]any{"channelId": ref.ChannelID}
	if deleted.SenderID != nil {
		details["senderId"] = *deleted.SenderID
	}
	return service.record(ctx, Scope{ServerID: ref.ServerID, UserID: ref.UserID}, event(ActionMessageDelete, TargetKindMessage, deleted.MessageID, details))
}

// MayHandleReports reports whether the member handles the server's
// reports: MANAGE_MESSAGES (it implements reports.Moderation).
func (service *Service) MayHandleReports(ctx context.Context, serverID uuid.UUID, userID uuid.UUID) (bool, error) {
	a, err := service.load(ctx, Scope{ServerID: serverID, UserID: userID})
	if errors.Is(err, ErrNotFound) {
		return false, reports.ErrNoServer
	}
	if err != nil {
		return false, err
	}
	return a.inServer().Has(permissions.ManageMessages), nil
}

// ReportHandled records a moderator resolving a report or passing it on
// to the instance admins (it implements reports.Moderation).
func (service *Service) ReportHandled(ctx context.Context, handled reports.Handled) error {
	action := ActionReportResolve
	if handled.Escalated {
		action = ActionReportEscalate
	}
	return service.record(ctx, Scope{ServerID: handled.ServerID, UserID: handled.ActorID}, event(action, TargetKindReport, handled.ReportID, nil))
}

// recordOnMember records an action on a member with their username, so the
// log still names them once they have left.
func (service *Service) recordOnMember(ctx context.Context, scope Scope, happened auditEvent) error {
	username, err := service.store.UsernameOf(ctx, uuid.MustParse(happened.targetID))
	if err != nil {
		return err
	}
	happened.details["username"] = username
	return service.record(ctx, scope, happened)
}
