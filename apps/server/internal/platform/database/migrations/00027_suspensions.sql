-- +goose Up
-- Instance admins can suspend an account (spec §4.7): it cannot sign in,
-- and its sessions are ended, until an admin reinstates it.
ALTER TABLE users ADD COLUMN suspended_at timestamptz;
ALTER TABLE users ADD COLUMN suspension_reason text NOT NULL DEFAULT '' CHECK (char_length(suspension_reason) <= 512);

-- +goose Down
ALTER TABLE users DROP COLUMN suspension_reason;
ALTER TABLE users DROP COLUMN suspended_at;
