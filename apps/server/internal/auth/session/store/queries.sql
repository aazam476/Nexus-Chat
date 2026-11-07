-- name: InsertSession :exec
INSERT INTO sessions (id, user_id, token_hash, device_name, user_agent, created_at, last_seen_at, expires_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8);

-- name: GetSessionByTokenHash :one
SELECT * FROM sessions WHERE token_hash = $1;

-- name: TouchSession :exec
-- Records activity and extends the session (sliding expiry).
UPDATE sessions SET last_seen_at = $2, expires_at = $3 WHERE id = $1;

-- name: ListSessionsForUser :many
SELECT * FROM sessions WHERE user_id = $1 AND expires_at > sqlc.arg(now) ORDER BY last_seen_at DESC;

-- name: DeleteSession :execrows
DELETE FROM sessions WHERE id = $1 AND user_id = $2;

-- name: DeleteExpiredSessions :execrows
DELETE FROM sessions WHERE expires_at <= $1;

-- name: DeleteSessionsForUser :exec
DELETE FROM sessions WHERE user_id = $1;
