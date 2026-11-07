-- name: ConsumeInvite :one
-- Uses up one use of an invite if it is still valid, in a single statement
-- so concurrent registrations cannot exceed max_uses.
UPDATE instance_invites
SET uses = uses + 1
WHERE code_hash = $1
  AND (max_uses IS NULL OR uses < max_uses)
  AND (expires_at IS NULL OR expires_at > sqlc.arg(now)::timestamptz)
RETURNING id;

-- name: InsertInvite :exec
INSERT INTO instance_invites (id, code_hash, created_by, max_uses, expires_at, created_at)
VALUES ($1, $2, $3, $4, $5, $6);

-- name: ListInvites :many
SELECT id, created_by, max_uses, uses, expires_at, created_at FROM instance_invites ORDER BY created_at DESC;

-- name: DeleteInvite :execrows
DELETE FROM instance_invites WHERE id = $1;
