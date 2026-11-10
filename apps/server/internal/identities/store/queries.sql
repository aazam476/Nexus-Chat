-- name: GetIdentity :one
SELECT * FROM user_identities WHERE user_id = $1;

-- name: PutIdentity :exec
INSERT INTO user_identities (user_id, master_public_key, self_signing_public_key, self_signing_signature, updated_at)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (user_id) DO UPDATE SET
    master_public_key = EXCLUDED.master_public_key,
    self_signing_public_key = EXCLUDED.self_signing_public_key,
    self_signing_signature = EXCLUDED.self_signing_signature,
    updated_at = EXCLUDED.updated_at;
