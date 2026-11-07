-- name: GetSetting :one
SELECT key, value, updated_at
FROM instance_settings
WHERE key = $1;

-- name: InsertSettingIfMissing :one
-- Inserts the value only when the key has no value yet, and returns whatever
-- value is stored afterwards. Safe when two processes start at once.
INSERT INTO instance_settings (key, value)
VALUES ($1, $2)
ON CONFLICT (key) DO UPDATE SET key = instance_settings.key
RETURNING key, value, updated_at;

-- name: InsertSettingIfAbsent :execrows
-- Inserts the value only if the key has none. Returns 1 if it was inserted
-- and 0 if a value already existed.
INSERT INTO instance_settings (key, value)
VALUES ($1, $2)
ON CONFLICT (key) DO NOTHING;

-- name: UpsertSetting :exec
INSERT INTO instance_settings (key, value, updated_at)
VALUES ($1, $2, now())
ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, updated_at = now();
