-- name: InsertCredential :exec
INSERT INTO webauthn_credentials (id, user_id, credential_id, credential, kind, name, created_at)
VALUES ($1, $2, $3, $4, $5, $6, $7);

-- name: ListCredentials :many
SELECT * FROM webauthn_credentials WHERE user_id = $1 ORDER BY created_at;

-- name: UpdateCredentialAfterUse :exec
-- Saves the new signature counter and flags after a successful login.
UPDATE webauthn_credentials SET credential = $2, last_used_at = $3 WHERE credential_id = $1;

-- name: DeleteCredential :execrows
DELETE FROM webauthn_credentials WHERE id = $1 AND user_id = $2;

-- name: InsertCeremony :exec
INSERT INTO webauthn_ceremonies (id, user_id, purpose, session, name, expires_at)
VALUES ($1, $2, $3, $4, $5, $6);

-- name: TakeCeremony :one
-- Removes and returns a ceremony, so each challenge is answered once.
DELETE FROM webauthn_ceremonies
WHERE id = $1 AND purpose = $2 AND expires_at > sqlc.arg(now)
RETURNING *;

-- name: DeleteExpiredCeremonies :exec
DELETE FROM webauthn_ceremonies WHERE expires_at <= sqlc.arg(now);
