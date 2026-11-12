-- name: InsertConversation :exec
INSERT INTO conversations (id, kind, name, dm_pair, mls_group_id, created_by, created_at)
VALUES ($1, $2, $3, $4, $5, $6, $7);

-- name: FindDirectMessage :one
SELECT id FROM conversations WHERE dm_pair = $1;

-- name: AddMember :exec
INSERT INTO conversation_members (conversation_id, user_id, joined_at) VALUES ($1, $2, $3)
ON CONFLICT DO NOTHING;

-- name: RemoveMember :execrows
DELETE FROM conversation_members WHERE conversation_id = $1 AND user_id = $2;

-- name: GetConversation :one
SELECT * FROM conversations WHERE id = $1;

-- name: IsMember :one
SELECT EXISTS (SELECT 1 FROM conversation_members WHERE conversation_id = $1 AND user_id = $2);

-- name: CountMembers :one
SELECT count(*) FROM conversation_members WHERE conversation_id = $1;

-- name: MemberIDs :many
SELECT user_id FROM conversation_members WHERE conversation_id = $1 ORDER BY joined_at, user_id;

-- name: ListMembers :many
SELECT m.conversation_id, u.id, u.username, u.display_name, u.avatar_id
FROM conversation_members m JOIN users u ON u.id = m.user_id
WHERE m.conversation_id = ANY (sqlc.arg(conversation_ids)::uuid[])
ORDER BY m.joined_at, u.id;

-- name: ListConversations :many
SELECT c.* FROM conversations c
JOIN conversation_members m ON m.conversation_id = c.id
WHERE m.user_id = $1
ORDER BY c.created_at DESC, c.id;

-- name: RenameConversation :exec
UPDATE conversations SET name = $2 WHERE id = $1;

-- name: UserExists :one
SELECT EXISTS (SELECT 1 FROM users WHERE id = $1);

-- name: SetIcon :exec
UPDATE conversations SET icon_id = sqlc.narg(icon_id), icon_format = sqlc.narg(icon_format) WHERE id = sqlc.arg(id);

-- name: GetIconFormat :one
-- A row with an icon ID always has its format (a table CHECK).
SELECT COALESCE(icon_format, '')::text FROM conversations WHERE icon_id = sqlc.arg(icon_id)::uuid;
