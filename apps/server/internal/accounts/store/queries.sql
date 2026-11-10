-- name: InsertUser :exec
INSERT INTO users (id, username, display_name, email, email_verified_at, password_hash, is_instance_admin, created_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8);

-- name: GetUserByUsername :one
SELECT * FROM users WHERE username = $1;

-- name: CountInstanceAdmins :one
SELECT count(*) FROM users WHERE is_instance_admin;

-- name: GetUserByLogin :one
-- Finds an account by username or email; both are stored in lower case.
SELECT * FROM users WHERE username = lower(sqlc.arg(login)) OR email = lower(sqlc.arg(login));

-- name: RecordFailedLogin :exec
UPDATE users SET failed_login_count = failed_login_count + 1, login_blocked_until = $2 WHERE id = $1;

-- name: ResetFailedLogins :exec
UPDATE users SET failed_login_count = 0, login_blocked_until = NULL WHERE id = $1;

-- name: UpdatePasswordHash :exec
UPDATE users SET password_hash = $2 WHERE id = $1;

-- name: GetUserByID :one
SELECT * FROM users WHERE id = $1;

-- name: MarkEmailVerified :exec
UPDATE users SET email_verified_at = $2 WHERE id = $1 AND email_verified_at IS NULL;

-- name: GetUserByEmail :one
SELECT * FROM users WHERE email = lower(sqlc.arg(email));

-- name: SetPassword :exec
-- Sets a new password after an email reset and clears any login lockout.
-- It also deletes the backup key's password-wrapped copy: the client never
-- learned the old password, so the copy can't be re-wrapped, and the user
-- restores keys another way (docs/e2ee-design.md §5).
WITH dropped AS (DELETE FROM key_backup_wrapped_keys WHERE user_id = $1 AND method = 'password')
UPDATE users SET password_hash = $2, failed_login_count = 0, login_blocked_until = NULL WHERE id = $1;

-- name: UpdateProfile :one
UPDATE users SET display_name = $2, bio = $3 WHERE id = $1 RETURNING *;

-- name: SetAvatar :one
UPDATE users SET avatar_id = $2, avatar_format = $3 WHERE id = $1 RETURNING *;

-- name: GetAvatarFormat :one
SELECT avatar_format::text FROM users WHERE avatar_id = sqlc.arg(avatar_id)::uuid;

-- name: SetStatus :one
UPDATE users
SET status = $2, custom_status_text = $3, custom_status_expires_at = $4
WHERE id = $1
RETURNING *;

-- name: ClearExpiredCustomStatuses :many
-- Removes custom statuses whose expiry has passed and returns whose.
UPDATE users SET custom_status_text = NULL, custom_status_expires_at = NULL
WHERE custom_status_expires_at <= sqlc.arg(now)::timestamptz
RETURNING id;
