package store

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/aazam476/nexus-chat/apps/server/internal/platform/database"
)

// AudienceSnapshot is everything that decides a server's audiences, read
// in one transaction.
type AudienceSnapshot struct {
	Server          Server
	Roles           []ServerRole
	Members         []ListServerMembersRow
	MemberRoles     []ListMemberRolesRow
	Overrides       []ServerChannelOverride
	Channels        []ServerChannel
	Audiences       []ListAudiencesRow
	AudienceMembers []ListAudienceMembersRow
}

// AudiencePlan is how a server's audiences change.
type AudiencePlan struct {
	Create []InsertAudienceParams
	Assign []SetChannelAudienceParams
	Add    []AddAudienceMemberParams
	Remove []RemoveAudienceMemberParams
}

// ReconcileAudiences locks the server, reads its snapshot, and applies the
// plan that plan works out from it, all in one transaction. A server that
// no longer exists has nothing to reconcile.
func (writer *Writer) ReconcileAudiences(ctx context.Context, serverID uuid.UUID, plan func(AudienceSnapshot) AudiencePlan) (AudiencePlan, error) {
	var applied AudiencePlan
	err := database.InTransaction(ctx, writer.pool, func(transaction pgx.Tx) error {
		queries := writer.WithTx(transaction)
		if _, err := queries.LockServer(ctx, serverID); err != nil {
			return err
		}
		snapshot, err := snapshotOf(ctx, queries, serverID)
		if err != nil {
			return err
		}
		applied = plan(snapshot)
		return apply(ctx, queries, applied)
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return AudiencePlan{}, nil
	}
	return applied, err
}

func snapshotOf(ctx context.Context, queries *Queries, serverID uuid.UUID) (AudienceSnapshot, error) {
	var s AudienceSnapshot
	var err error
	if s.Server, err = queries.GetServer(ctx, serverID); err != nil {
		return s, err
	}
	if s.Roles, err = queries.ListRoles(ctx, serverID); err != nil {
		return s, err
	}
	if s.Members, err = queries.ListServerMembers(ctx, serverID); err != nil {
		return s, err
	}
	if s.MemberRoles, err = queries.ListMemberRoles(ctx, serverID); err != nil {
		return s, err
	}
	if s.Overrides, err = queries.ListOverrides(ctx, serverID); err != nil {
		return s, err
	}
	if s.Channels, err = queries.ListChannels(ctx, serverID); err != nil {
		return s, err
	}
	if s.Audiences, err = queries.ListAudiences(ctx, &serverID); err != nil {
		return s, err
	}
	s.AudienceMembers, err = queries.ListAudienceMembers(ctx, &serverID)
	return s, err
}

func apply(ctx context.Context, queries *Queries, plan AudiencePlan) error {
	for _, audience := range plan.Create {
		if err := queries.InsertAudience(ctx, audience); err != nil {
			return err
		}
	}
	for _, assign := range plan.Assign {
		if err := queries.SetChannelAudience(ctx, assign); err != nil {
			return err
		}
	}
	for _, add := range plan.Add {
		if err := queries.AddAudienceMember(ctx, add); err != nil {
			return err
		}
	}
	for _, remove := range plan.Remove {
		if err := queries.RemoveAudienceMember(ctx, remove); err != nil {
			return err
		}
	}
	return nil
}
