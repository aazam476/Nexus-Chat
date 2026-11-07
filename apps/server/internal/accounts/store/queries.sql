-- name: InsertUser :exec
INSERT INTO users (id, username, display_name, email, email_verified_at, password_hash, is_instance_admin, created_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8);

-- name: GetUserByUsername :one
SELECT * FROM users WHERE username = $1;

-- name: CountInstanceAdmins :one
SELECT count(*) FROM users WHERE is_instance_admin;
