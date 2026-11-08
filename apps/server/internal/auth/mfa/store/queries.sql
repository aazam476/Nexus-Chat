-- name: InsertTOTP :exec
INSERT INTO totp_credentials (id, user_id, name, encrypted_secret, created_at)
VALUES ($1, $2, $3, $4, $5);

-- name: GetTOTP :one
SELECT * FROM totp_credentials WHERE id = $1 AND user_id = $2;

-- name: ConfirmTOTP :execrows
UPDATE totp_credentials SET confirmed_at = $3 WHERE id = $1 AND user_id = $2 AND confirmed_at IS NULL;

-- name: ListConfirmedTOTP :many
SELECT * FROM totp_credentials WHERE user_id = $1 AND confirmed_at IS NOT NULL ORDER BY created_at;

-- name: UseTOTPStep :execrows
-- Records that a code from this 30-second step was used. Fails (0 rows)
-- if this or a later step was already used, which stops replays even when
-- two requests race.
UPDATE totp_credentials SET last_used_step = $2 WHERE id = $1 AND last_used_step < $2;

-- name: DeleteTOTP :execrows
DELETE FROM totp_credentials WHERE id = $1 AND user_id = $2;

-- name: DeleteUnconfirmedTOTP :exec
-- Enrollments abandoned before confirmation are cleaned up when a new one starts.
DELETE FROM totp_credentials WHERE user_id = $1 AND confirmed_at IS NULL;

-- name: CountSecondFactors :one
SELECT count(*) FROM totp_credentials WHERE user_id = $1 AND confirmed_at IS NOT NULL;

-- name: DeleteBackupCodes :exec
DELETE FROM backup_codes WHERE user_id = $1;

-- name: InsertBackupCode :exec
INSERT INTO backup_codes (user_id, code_hash) VALUES ($1, $2);

-- name: UseBackupCode :execrows
DELETE FROM backup_codes WHERE user_id = $1 AND code_hash = $2;

-- name: CountBackupCodes :one
SELECT count(*) FROM backup_codes WHERE user_id = $1;
