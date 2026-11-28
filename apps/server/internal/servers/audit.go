package servers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/aazam476/nexus-chat/apps/server/internal/permissions"
	"github.com/aazam476/nexus-chat/apps/server/internal/servers/store"
)

// The audit log (spec §4.7) records every moderation and configuration
// action in a server: who did it, to what, and what changed. Members with
// VIEW_AUDIT_LOG read it. Entries are kept for AuditRetention.

// AuditRetention is how long audit entries are kept.
const AuditRetention = 90 * 24 * time.Hour

// Audited actions, as stored. A filter by "role" matches every "role.*".
const (
	ActionServerUpdate    = "server.update"
	ActionServerIcon      = "server.icon"
	ActionChannelCreate   = "channel.create"
	ActionChannelUpdate   = "channel.update"
	ActionChannelDelete   = "channel.delete"
	ActionChannelOrder    = "channel.order"
	ActionOverrideSet     = "channel.override_set"
	ActionOverrideClear   = "channel.override_clear"
	ActionRoleCreate      = "role.create"
	ActionRoleUpdate      = "role.update"
	ActionRoleDelete      = "role.delete"
	ActionRoleOrder       = "role.order"
	ActionMemberRoleGive  = "member.role_give"
	ActionMemberRoleTake  = "member.role_take"
	ActionMemberKick      = "member.kick"
	ActionMemberBan       = "member.ban"
	ActionMemberUnban     = "member.unban"
	ActionMemberTimeout   = "member.timeout"
	ActionInviteCreate    = "invite.create"
	ActionInviteRevoke    = "invite.revoke"
	ActionEmojiAdd        = "emoji.add"
	ActionEmojiRemove     = "emoji.remove"
	ActionMessageDelete   = "message.delete"
	ActionReportResolve   = "report.resolve"
	ActionReportEscalate  = "report.escalate"
	auditTargetNone       = ""
	maximumAuditPageItems = 100
)

// What an audit entry's target is.
const (
	TargetKindServer  = "server"
	TargetKindChannel = "channel"
	TargetKindRole    = "role"
	TargetKindMember  = "member"
	TargetKindInvite  = "invite"
	TargetKindEmoji   = "emoji"
	TargetKindMessage = "message"
	TargetKindReport  = "report"
)

// AuditEntry is one recorded action.
type AuditEntry struct {
	ID int64
	// ActorID is nil once the actor's account is gone.
	ActorID    *uuid.UUID
	Action     string
	TargetKind string
	TargetID   string
	Details    map[string]any
	CreatedAt  time.Time
}

// AuditFilter narrows the audit log. Zero values match everything.
type AuditFilter struct {
	// Before is the ID of the oldest entry already shown, for the next page.
	Before  int64
	ActorID *uuid.UUID
	// Action is an action, or its first part ("role") for all of its kind.
	Action   string
	TargetID string
	Limit    int
}

// auditEvent is an action to record.
type auditEvent struct {
	action     string
	targetKind string
	targetID   string
	details    map[string]any
}

// event names an action on a target with an ID.
func event(action string, targetKind string, targetID fmt.Stringer, details map[string]any) auditEvent {
	return auditEvent{action: action, targetKind: targetKind, targetID: targetID.String(), details: details}
}

// record writes the event to the server's audit log, as done by scope.UserID.
func (service *Service) record(ctx context.Context, scope Scope, happened auditEvent) error {
	details := happened.details
	if details == nil {
		details = map[string]any{}
	}
	encoded, err := json.Marshal(details)
	if err != nil {
		return fmt.Errorf("encoding audit details: %w", err)
	}
	params := store.InsertAuditEntryParams{
		ServerID: scope.ServerID, ActorID: &scope.UserID, Action: happened.action, Details: encoded, CreatedAt: service.clock.Now(),
	}
	if happened.targetKind != auditTargetNone {
		params.TargetKind, params.TargetID = &happened.targetKind, &happened.targetID
	}
	return service.store.InsertAuditEntry(ctx, params)
}

// logged records a completed action, then returns the outcome of what
// followed it (telling members), joined with any failure to record.
func (service *Service) logged(ctx context.Context, scope Scope, happened auditEvent, then error) error {
	return errors.Join(then, service.record(ctx, scope, happened))
}

// AuditLog returns the server's audit entries, newest first (VIEW_AUDIT_LOG).
func (service *Service) AuditLog(ctx context.Context, scope Scope, filter AuditFilter) ([]AuditEntry, error) {
	a, err := service.load(ctx, scope)
	if err != nil {
		return nil, err
	}
	if err := a.require(permissions.ViewAuditLog); err != nil {
		return nil, err
	}
	rows, err := service.store.ListAuditEntries(ctx, auditQuery(scope.ServerID, filter))
	if err != nil {
		return nil, err
	}
	entries := make([]AuditEntry, 0, len(rows))
	for _, row := range rows {
		entry := AuditEntry{ID: row.ID, ActorID: row.ActorID, Action: row.Action, CreatedAt: row.CreatedAt, Details: map[string]any{}}
		if row.TargetKind != nil && row.TargetID != nil {
			entry.TargetKind, entry.TargetID = *row.TargetKind, *row.TargetID
		}
		if err := json.Unmarshal(row.Details, &entry.Details); err != nil {
			return nil, fmt.Errorf("reading audit details: %w", err)
		}
		entries = append(entries, entry)
	}
	return entries, nil
}

func auditQuery(serverID uuid.UUID, filter AuditFilter) store.ListAuditEntriesParams {
	limit := filter.Limit
	if limit <= 0 || limit > maximumAuditPageItems {
		limit = maximumAuditPageItems
	}
	params := store.ListAuditEntriesParams{ServerID: serverID, Before: filter.Before, ActorID: filter.ActorID, RowLimit: int32(limit)} //nolint:gosec // at most 100
	if filter.Action != "" {
		params.ActionPrefix = &filter.Action
	}
	if filter.TargetID != "" {
		params.TargetID = &filter.TargetID
	}
	return params
}

// PruneAuditLog forgets entries older than AuditRetention.
func (service *Service) PruneAuditLog(ctx context.Context) (int64, error) {
	return service.store.PruneAuditLog(ctx, service.clock.Now().Add(-AuditRetention))
}

// AuditPruner prunes the audit log on a schedule.
type AuditPruner struct {
	Service *Service
	Logger  *slog.Logger
}

// Run prunes once now and then every interval, until ctx is cancelled.
func (pruner AuditPruner) Run(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		if _, err := pruner.Service.PruneAuditLog(ctx); err != nil && ctx.Err() == nil {
			pruner.Logger.WarnContext(ctx, "could not prune the audit log", slog.Any("error", err))
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
