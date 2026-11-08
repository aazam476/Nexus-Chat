-- +goose Up
-- FIDO2 security keys and passkeys (spec §4.1). "credential" holds the
-- library's serialized credential (public key, sign count, flags); none of
-- it is secret.
CREATE TABLE webauthn_credentials (
    id            uuid        PRIMARY KEY,
    user_id       uuid        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    credential_id bytea       NOT NULL UNIQUE,
    credential    jsonb       NOT NULL,
    -- passkey: discoverable, user-verified, logs in without a password.
    -- security_key: a second factor after the password.
    kind          text        NOT NULL CHECK (kind IN ('passkey', 'security_key')),
    name          text        NOT NULL CHECK (char_length(name) BETWEEN 1 AND 64),
    created_at    timestamptz NOT NULL,
    last_used_at  timestamptz
);

CREATE INDEX webauthn_credentials_user_id_idx ON webauthn_credentials (user_id);

-- A WebAuthn ceremony in progress: the challenge the server issued, kept
-- until the browser answers. Each is used once and expires after minutes.
CREATE TABLE webauthn_ceremonies (
    id         uuid        PRIMARY KEY,
    user_id    uuid        REFERENCES users (id) ON DELETE CASCADE,
    purpose    text        NOT NULL CHECK (purpose IN ('register_passkey', 'register_security_key', 'second_factor', 'passkey_login')),
    session    jsonb       NOT NULL,
    name       text        NOT NULL DEFAULT '',
    expires_at timestamptz NOT NULL
);

-- +goose Down
DROP TABLE webauthn_ceremonies;
DROP TABLE webauthn_credentials;
