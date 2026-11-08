-- +goose Up
-- When the person using a session last proved who they are: at login, or by
-- confirming their password. Changes to how an account logs in need this to
-- be recent, so a stolen session cannot add its own way back in.
ALTER TABLE sessions ADD COLUMN authenticated_at timestamptz;
UPDATE sessions SET authenticated_at = created_at;
ALTER TABLE sessions ALTER COLUMN authenticated_at SET NOT NULL;

-- +goose Down
ALTER TABLE sessions DROP COLUMN authenticated_at;
