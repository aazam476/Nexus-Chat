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
-- Sets a new password and clears any login lockout.
UPDATE users SET password_hash = $2, failed_login_count = 0, login_blocked_until = NULL WHERE id = $1;
