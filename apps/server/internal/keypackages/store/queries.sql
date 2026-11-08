-- name: InsertKeyPackage :exec
INSERT INTO key_packages (session_id, user_id, data, last_resort, created_at)
VALUES ($1, $2, $3, false, $4);

-- name: ReplaceLastResort :exec
-- A device has one last-resort package; uploading another replaces it.
INSERT INTO key_packages (session_id, user_id, data, last_resort, created_at)
VALUES ($1, $2, $3, true, $4)
ON CONFLICT (session_id) WHERE last_resort DO UPDATE SET data = EXCLUDED.data, created_at = EXCLUDED.created_at;

-- name: CountKeyPackages :one
SELECT count(*) FILTER (WHERE NOT last_resort) AS one_time,
       count(*) FILTER (WHERE last_resort) > 0 AS has_last_resort
FROM key_packages WHERE session_id = $1;

-- name: ClaimOneTime :many
-- One one-time package per device of the user, removed as it is handed
-- out: the oldest of each device, locked so concurrent claims take
-- different packages (SKIP LOCKED).
DELETE FROM key_packages AS claimed
WHERE claimed.id IN (
    SELECT picked.id
    FROM (SELECT DISTINCT owned.session_id FROM key_packages AS owned
          WHERE owned.user_id = sqlc.arg(user_id) AND NOT owned.last_resort) AS devices
    CROSS JOIN LATERAL (
        SELECT candidate.id FROM key_packages AS candidate
        WHERE candidate.session_id = devices.session_id AND NOT candidate.last_resort
        ORDER BY candidate.id
        LIMIT 1
        FOR UPDATE SKIP LOCKED
    ) AS picked
)
RETURNING claimed.session_id, claimed.data;

-- name: LastResortPackages :many
-- Devices of the user with no one-time package left get their last-resort
-- one, except devices already served a one-time package in this claim.
SELECT k.session_id, k.data FROM key_packages AS k
WHERE k.user_id = sqlc.arg(user_id) AND k.last_resort
  AND NOT (k.session_id = ANY (sqlc.arg(served)::uuid[]))
  AND NOT EXISTS (SELECT 1 FROM key_packages AS o WHERE o.session_id = k.session_id AND NOT o.last_resort);
