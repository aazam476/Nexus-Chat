-- name: EnsureBackup :one
-- The user's backup salt, created on first use. ON CONFLICT DO UPDATE with
-- no change returns the existing row.
INSERT INTO key_backups (user_id, salt, created_at) VALUES ($1, $2, $3)
ON CONFLICT (user_id) DO UPDATE SET user_id = EXCLUDED.user_id
RETURNING salt;

-- name: ListWrappedKeys :many
SELECT method, wrapped_key, parameters FROM key_backup_wrapped_keys WHERE user_id = $1 ORDER BY method;

-- name: PutWrappedKey :exec
INSERT INTO key_backup_wrapped_keys (user_id, method, wrapped_key, parameters, updated_at)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (user_id, method) DO UPDATE SET
    wrapped_key = EXCLUDED.wrapped_key, parameters = EXCLUDED.parameters, updated_at = EXCLUDED.updated_at;

-- name: DeleteWrappedKey :execrows
DELETE FROM key_backup_wrapped_keys WHERE user_id = $1 AND method = $2;

-- name: ListBlobs :many
SELECT name, octet_length(data)::int AS size, updated_at FROM key_backup_blobs WHERE user_id = $1 ORDER BY name;

-- name: CountBlobs :one
SELECT count(*) FROM key_backup_blobs WHERE user_id = $1;

-- name: GetBlob :one
SELECT data FROM key_backup_blobs WHERE user_id = $1 AND name = $2;

-- name: PutBlob :exec
INSERT INTO key_backup_blobs (user_id, name, data, updated_at) VALUES ($1, $2, $3, $4)
ON CONFLICT (user_id, name) DO UPDATE SET data = EXCLUDED.data, updated_at = EXCLUDED.updated_at;

-- name: DeleteBlob :execrows
DELETE FROM key_backup_blobs WHERE user_id = $1 AND name = $2;
