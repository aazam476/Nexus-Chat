-- +goose Up
-- MLS key packages (RFC 9420 §10): what lets others add a device to a
-- group while it is offline. Each device (for now, each session) keeps a
-- stock of one-time packages and one last-resort package, which is handed
-- out only when the one-time ones run out and is never used up
-- (docs/e2ee-design.md §3.2). Packages go with the session.
CREATE TABLE key_packages (
    id           bigint      GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    session_id   uuid        NOT NULL REFERENCES sessions (id) ON DELETE CASCADE,
    user_id      uuid        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    data         bytea       NOT NULL CHECK (octet_length(data) BETWEEN 8 AND 16384),
    last_resort  boolean     NOT NULL DEFAULT false,
    created_at   timestamptz NOT NULL
);

CREATE INDEX key_packages_user_idx ON key_packages (user_id, session_id, last_resort, id);
CREATE UNIQUE INDEX key_packages_one_last_resort_idx ON key_packages (session_id) WHERE last_resort;

-- +goose Down
DROP TABLE key_packages;
