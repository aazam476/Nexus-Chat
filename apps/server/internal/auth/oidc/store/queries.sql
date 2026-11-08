-- name: ListProviders :many
SELECT * FROM oidc_providers ORDER BY display_name, id;

-- name: GetProvider :one
SELECT * FROM oidc_providers WHERE id = $1;

-- name: InsertProvider :exec
INSERT INTO oidc_providers (id, display_name, issuer, client_id, client_secret_sealed, scopes, allow_signup, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $8);

-- name: UpdateProvider :execrows
UPDATE oidc_providers
SET display_name = $2, issuer = $3, client_id = $4, client_secret_sealed = $5, scopes = $6, allow_signup = $7, updated_at = $8
WHERE id = $1;

-- name: DeleteProvider :execrows
DELETE FROM oidc_providers WHERE id = $1;

-- name: InsertFlow :exec
INSERT INTO oidc_flows (id, provider_id, user_id, state_hash, nonce, code_verifier, return_to, invite_hash, expires_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9);

-- name: DeleteExpiredFlows :exec
DELETE FROM oidc_flows WHERE expires_at <= sqlc.arg(now);

-- name: GetFlowByState :one
-- Only a flow whose callback has not run yet can be found by its state, so
-- a callback URL works once.
SELECT * FROM oidc_flows
WHERE state_hash = sqlc.arg(state_hash) AND completion_hash IS NULL AND expires_at > sqlc.arg(now);

-- name: RecordCallback :execrows
UPDATE oidc_flows SET completion_hash = sqlc.arg(completion_hash), claims = sqlc.arg(claims)
WHERE id = sqlc.arg(id) AND completion_hash IS NULL;

-- name: ConsumeFlow :one
-- Uses up a flow whose callback has run. Wrong codes, used flows and
-- expired flows all find nothing.
DELETE FROM oidc_flows
WHERE id = sqlc.arg(id) AND completion_hash = sqlc.arg(completion_hash) AND expires_at > sqlc.arg(now)
RETURNING *;

-- name: GetIdentity :one
SELECT * FROM oidc_identities WHERE provider_id = $1 AND subject = $2;

-- name: InsertIdentity :exec
INSERT INTO oidc_identities (provider_id, subject, user_id, email, created_at)
VALUES ($1, $2, $3, $4, $5);

-- name: ListIdentitiesForUser :many
SELECT i.provider_id, p.display_name, i.email, i.created_at
FROM oidc_identities i JOIN oidc_providers p ON p.id = i.provider_id
WHERE i.user_id = $1
ORDER BY p.display_name;

-- name: DeleteIdentity :execrows
DELETE FROM oidc_identities WHERE provider_id = $1 AND user_id = $2;

-- name: CountOtherLoginMethods :one
-- How many ways besides this provider the account can log in with: a
-- password, a passkey, or another provider.
SELECT
    (SELECT count(*) FROM users u WHERE u.id = sqlc.arg(user_id) AND u.password_hash IS NOT NULL)
  + (SELECT count(*) FROM webauthn_credentials w WHERE w.user_id = sqlc.arg(user_id) AND w.kind = 'passkey')
  + (SELECT count(*) FROM oidc_identities o WHERE o.user_id = sqlc.arg(user_id) AND o.provider_id <> sqlc.arg(provider_id))
  AS methods;

-- name: UsernameExists :one
SELECT EXISTS (SELECT 1 FROM users WHERE username = $1);
