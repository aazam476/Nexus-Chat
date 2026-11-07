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
