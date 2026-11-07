-- name: InsertToken :exec
INSERT INTO account_tokens (id, user_id, purpose, token_hash, created_at, expires_at)
VALUES ($1, $2, $3, $4, $5, $6);

-- name: ConsumeToken :one
-- Deletes the token and returns its owner, but only if it is unexpired and
-- for the right purpose. Deleting makes it single-use, even under
-- concurrent requests.
DELETE FROM account_tokens
WHERE token_hash = $1 AND purpose = $2 AND expires_at > sqlc.arg(now)
RETURNING user_id;

-- name: DeleteTokensForUser :exec
DELETE FROM account_tokens WHERE user_id = $1 AND purpose = $2;
