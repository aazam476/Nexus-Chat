-- +goose Up
-- users holds local accounts (spec §4.1). Remote users are added with
-- federation in a later migration. Validation rules are repeated here as
-- CHECK constraints so no code path can store an invalid row.
CREATE TABLE users (
    id                uuid        PRIMARY KEY,
    username          text        NOT NULL UNIQUE CHECK (username ~ '^[a-z0-9_.]{3,32}$'),
    display_name      text        NOT NULL CHECK (char_length(display_name) BETWEEN 1 AND 32),
    email             text        NOT NULL CHECK (email = lower(email) AND position('@' IN email) > 1),
    email_verified_at timestamptz,
    -- NULL for accounts that log in only with passkeys or OAuth.
    password_hash     text,
    is_instance_admin boolean     NOT NULL DEFAULT false,
    created_at        timestamptz NOT NULL
);

CREATE UNIQUE INDEX users_email_key ON users (email);

-- +goose Down
DROP TABLE users;
