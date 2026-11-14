-- Servers

-- name: InsertServer :exec
INSERT INTO servers (id, name, description, owner_id, created_at) VALUES ($1, $2, $3, $4, $5);

-- name: GetServer :one
SELECT * FROM servers WHERE id = $1;

-- name: ListServersOf :many
SELECT s.* FROM servers s JOIN server_members m ON m.server_id = s.id
WHERE m.user_id = $1 ORDER BY m.joined_at, s.id;

-- name: UpdateServer :exec
UPDATE servers SET name = $2, description = $3, attachment_max_bytes = $4, compress_media = $5 WHERE id = $1;

-- name: DeleteServer :exec
DELETE FROM servers WHERE id = $1;

-- Members

-- name: AddServerMember :execrows
INSERT INTO server_members (server_id, user_id, joined_at) VALUES ($1, $2, $3) ON CONFLICT DO NOTHING;

-- name: RemoveServerMember :execrows
DELETE FROM server_members WHERE server_id = $1 AND user_id = $2;

-- name: GetServerMember :one
SELECT * FROM server_members WHERE server_id = $1 AND user_id = $2;

-- name: ListServerMembers :many
SELECT m.user_id, m.joined_at, m.timed_out_until, u.username, u.display_name, u.avatar_id
FROM server_members m JOIN users u ON u.id = m.user_id
WHERE m.server_id = $1 ORDER BY m.joined_at, m.user_id;

-- name: CountServerMembers :one
SELECT count(*) FROM server_members WHERE server_id = $1;

-- name: ServerMemberIDs :many
SELECT user_id FROM server_members WHERE server_id = $1 ORDER BY user_id;

-- name: SharesServer :one
SELECT EXISTS (
    SELECT 1 FROM server_members a JOIN server_members b ON a.server_id = b.server_id
    WHERE a.user_id = $1 AND b.user_id = $2
);

-- Roles

-- name: InsertRole :exec
INSERT INTO server_roles (id, server_id, name, color, permissions, position, everyone)
VALUES ($1, $2, $3, $4, $5, $6, $7);

-- name: GetRole :one
SELECT * FROM server_roles WHERE id = $1;

-- name: ListRoles :many
SELECT * FROM server_roles WHERE server_id = $1 ORDER BY position, id;

-- name: UpdateRole :exec
UPDATE server_roles SET name = $2, color = $3, permissions = $4 WHERE id = $1;

-- name: SetRolePosition :exec
UPDATE server_roles SET position = $2 WHERE id = $1 AND NOT everyone;

-- name: DeleteRole :exec
DELETE FROM server_roles WHERE id = $1 AND NOT everyone;

-- name: TopRolePosition :one
SELECT COALESCE(max(position), 0)::integer FROM server_roles WHERE server_id = $1;

-- name: AddMemberRole :exec
INSERT INTO server_member_roles (server_id, user_id, role_id) VALUES ($1, $2, $3) ON CONFLICT DO NOTHING;

-- name: RemoveMemberRole :exec
DELETE FROM server_member_roles WHERE server_id = $1 AND user_id = $2 AND role_id = $3;

-- name: ListMemberRoles :many
SELECT user_id, role_id FROM server_member_roles WHERE server_id = $1;

-- name: MemberRoleIDs :many
SELECT role_id FROM server_member_roles WHERE server_id = $1 AND user_id = $2;

-- Channels

-- name: InsertChannel :exec
INSERT INTO server_channels (id, server_id, kind, name, topic, parent_id, position, created_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8);

-- name: GetChannel :one
SELECT * FROM server_channels WHERE id = $1;

-- name: ListChannels :many
SELECT * FROM server_channels WHERE server_id = $1 ORDER BY position, id;

-- name: UpdateChannel :exec
UPDATE server_channels SET name = $2, topic = $3 WHERE id = $1;

-- name: MoveChannel :exec
UPDATE server_channels SET parent_id = $2, position = $3 WHERE id = $1;

-- name: DeleteChannel :exec
DELETE FROM server_channels WHERE id = $1;

-- name: NextChannelPosition :one
SELECT COALESCE(max(position) + 1, 0)::integer FROM server_channels
WHERE server_id = $1 AND parent_id IS NOT DISTINCT FROM sqlc.narg(parent_id)::uuid;

-- Overrides

-- name: UpsertOverride :exec
INSERT INTO server_channel_overrides (channel_id, target_kind, target_id, allow_bits, deny_bits)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (channel_id, target_kind, target_id) DO UPDATE SET allow_bits = $4, deny_bits = $5;

-- name: DeleteOverride :exec
DELETE FROM server_channel_overrides WHERE channel_id = $1 AND target_kind = $2 AND target_id = $3;

-- name: ListOverrides :many
SELECT o.* FROM server_channel_overrides o JOIN server_channels c ON c.id = o.channel_id
WHERE c.server_id = $1;

-- Invites

-- name: InsertInvite :exec
INSERT INTO server_invites (code, server_id, created_by, max_uses, expires_at, created_at)
VALUES ($1, $2, $3, $4, $5, $6);

-- name: GetInvite :one
SELECT * FROM server_invites WHERE code = $1;

-- name: ListInvites :many
SELECT * FROM server_invites WHERE server_id = $1 ORDER BY created_at, code;

-- name: DeleteInvite :execrows
DELETE FROM server_invites WHERE code = $1 AND server_id = $2;

-- name: UseInvite :one
UPDATE server_invites SET uses = uses + 1
WHERE code = $1 AND (max_uses IS NULL OR uses < max_uses) AND (expires_at IS NULL OR expires_at > sqlc.arg(now)::timestamptz)
RETURNING server_id;

-- Audiences (docs/e2ee-design.md §3.1)

-- name: LockServer :one
-- Taken before audiences are worked out, so two changes to one server do so
-- one after the other.
SELECT id FROM servers WHERE id = $1 FOR UPDATE;

-- name: ListAudiences :many
SELECT id, mls_group_id FROM conversations WHERE server_id = $1 AND kind = 'audience' ORDER BY id;

-- name: ListAudienceMembers :many
SELECT m.conversation_id, m.user_id FROM conversation_members m
JOIN conversations c ON c.id = m.conversation_id
WHERE c.server_id = $1 AND c.kind = 'audience';

-- name: InsertAudience :exec
INSERT INTO conversations (id, kind, mls_group_id, server_id, created_at) VALUES ($1, 'audience', $2, $3, $4);

-- name: AddAudienceMember :exec
INSERT INTO conversation_members (conversation_id, user_id, joined_at) VALUES ($1, $2, $3) ON CONFLICT DO NOTHING;

-- name: RemoveAudienceMember :exec
DELETE FROM conversation_members WHERE conversation_id = $1 AND user_id = $2;

-- name: SetChannelAudience :exec
UPDATE server_channels SET audience_id = $2 WHERE id = $1;
