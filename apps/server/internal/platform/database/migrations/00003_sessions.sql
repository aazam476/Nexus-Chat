-- +goose Up
-- sessions are logged-in devices (spec §4.1: list and revoke sessions).
-- Only a SHA-256 hash of each token is stored, so a database leak does not
-- let anyone log in.
CREATE TABLE sessions (
    id           uuid        PRIMARY KEY,
    user_id      uuid        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    token_hash   bytea       NOT NULL UNIQUE CHECK (octet_length(token_hash) = 32),
    device_name  text        NOT NULL CHECK (char_length(device_name) BETWEEN 1 AND 64),
    user_agent   text        NOT NULL DEFAULT '' CHECK (char_length(user_agent) <= 512),
    created_at   timestamptz NOT NULL,
    last_seen_at timestamptz NOT NULL,
    expires_at   timestamptz NOT NULL
);

CREATE INDEX sessions_user_id_idx ON sessions (user_id);

-- Failed password attempts, for progressive delays (spec §4.1).
ALTER TABLE users
    ADD COLUMN failed_login_count integer     NOT NULL DEFAULT 0,
    ADD COLUMN login_blocked_until timestamptz;

-- +goose Down
ALTER TABLE users DROP COLUMN login_blocked_until, DROP COLUMN failed_login_count;
DROP TABLE sessions;
