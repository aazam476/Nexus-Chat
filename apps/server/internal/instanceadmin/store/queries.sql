-- name: ListUsers :many
-- Accounts by username, after `after`, matching `pattern` (an ILIKE pattern) in username, display name or email.
SELECT id, username, display_name, email, is_instance_admin, created_at, suspended_at, suspension_reason FROM users
WHERE (username ILIKE @pattern::text ESCAPE '\' OR display_name ILIKE @pattern::text ESCAPE '\' OR email ILIKE @pattern::text ESCAPE '\')
  AND username > @after::text
ORDER BY username LIMIT @row_limit;

-- name: GetAdminUser :one
SELECT id, username, is_instance_admin, suspended_at FROM users WHERE id = @id;

-- name: SuspendUser :exec
UPDATE users SET suspended_at = @suspended_at, suspension_reason = @reason WHERE id = @id;

-- name: ReinstateUser :execrows
UPDATE users SET suspended_at = NULL, suspension_reason = '' WHERE id = @id AND suspended_at IS NOT NULL;

-- name: EndUserSessions :exec
DELETE FROM sessions WHERE user_id = @user_id;

-- name: SetInstanceAdmin :exec
UPDATE users SET is_instance_admin = @admin WHERE id = @id;

-- name: CountInstanceAdmins :one
SELECT count(*) FROM users WHERE is_instance_admin AND suspended_at IS NULL;

-- name: ListServers :many
-- Servers by name, after `after`, matching `pattern`, with their owner and size.
SELECT s.id, s.name, s.owner_id, u.username AS owner_username, s.created_at,
       (SELECT count(*) FROM server_members m WHERE m.server_id = s.id) AS members
FROM servers s LEFT JOIN users u ON u.id = s.owner_id
WHERE s.name ILIKE @pattern::text ESCAPE '\' AND (s.name, s.id::text) > (@after_name::text, @after_id::text)
ORDER BY s.name, s.id LIMIT @row_limit;

-- name: InstanceStats :one
SELECT
    (SELECT count(*) FROM users) AS users,
    (SELECT count(*) FROM users WHERE suspended_at IS NOT NULL) AS suspended_users,
    (SELECT count(DISTINCT ss.user_id) FROM sessions ss WHERE ss.last_seen_at > @active_since::timestamptz) AS active_users,
    (SELECT count(*) FROM servers) AS servers,
    (SELECT count(*) FROM conversations WHERE kind IN ('dm', 'group')) AS conversations,
    (SELECT count(*) FROM mls_messages mm WHERE mm.kind = 'application' AND mm.created_at > @messages_since::timestamptz) AS recent_messages,
    (SELECT count(*) FROM reports WHERE status = 'open') AS open_reports,
    (SELECT coalesce(sum(a.size), 0) FROM attachments a)::bigint AS attachment_bytes;
