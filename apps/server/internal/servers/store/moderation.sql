-- Moderation (spec §4.7)

-- name: InsertAuditEntry :exec
INSERT INTO server_audit_log (server_id, actor_id, action, target_kind, target_id, details, created_at)
VALUES (@server_id, @actor_id, @action, sqlc.narg(target_kind), sqlc.narg(target_id), @details, @created_at);

-- name: ListAuditEntries :many
-- Newest first, before the cursor (0 for the newest), with optional filters.
SELECT id, actor_id, action, target_kind, target_id, details, created_at FROM server_audit_log
WHERE server_id = @server_id
  AND (@before::bigint = 0 OR id < @before::bigint)
  AND (sqlc.narg(actor_id)::uuid IS NULL OR actor_id = sqlc.narg(actor_id)::uuid)
  AND (sqlc.narg(action_prefix)::text IS NULL OR action = sqlc.narg(action_prefix)::text OR action LIKE sqlc.narg(action_prefix)::text || '.%')
  AND (sqlc.narg(target_id)::text IS NULL OR target_id = sqlc.narg(target_id)::text)
ORDER BY id DESC LIMIT @row_limit;

-- name: PruneAuditLog :execrows
DELETE FROM server_audit_log WHERE created_at < @older_than;

-- name: InsertBan :exec
INSERT INTO server_bans (server_id, user_id, banned_by, reason, created_at)
VALUES (@server_id, @user_id, @banned_by, @reason, @created_at)
ON CONFLICT (server_id, user_id) DO UPDATE SET banned_by = EXCLUDED.banned_by, reason = EXCLUDED.reason, created_at = EXCLUDED.created_at;

-- name: DeleteBan :execrows
DELETE FROM server_bans WHERE server_id = @server_id AND user_id = @user_id;

-- name: IsBanned :one
SELECT EXISTS (SELECT 1 FROM server_bans WHERE server_id = @server_id AND user_id = @user_id);

-- name: ListBans :many
SELECT b.user_id, u.username, u.display_name, b.banned_by, b.reason, b.created_at FROM server_bans b
JOIN users u ON u.id = b.user_id
WHERE b.server_id = @server_id ORDER BY b.created_at DESC, b.user_id;

-- name: SetTimeout :execrows
UPDATE server_members SET timed_out_until = sqlc.narg(until) WHERE server_id = @server_id AND user_id = @user_id;

-- name: UsernameOf :one
SELECT username FROM users WHERE id = @id;
