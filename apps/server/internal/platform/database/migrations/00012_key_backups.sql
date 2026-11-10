-- +goose Up
-- The encrypted key backup (docs/e2ee-design.md §5). The server keeps only
-- what the client encrypted: a per-user salt for deriving wrapping keys,
-- the backup key wrapped under each unlock method the user turned on, and
-- named blobs sealed with the backup key (identity keys, history keys,
-- search-index chunks). It cannot open any of them.
CREATE TABLE key_backups (
    user_id    uuid        PRIMARY KEY REFERENCES users (id) ON DELETE CASCADE,
    salt       bytea       NOT NULL CHECK (octet_length(salt) = 16),
    created_at timestamptz NOT NULL
);

CREATE TABLE key_backup_wrapped_keys (
    user_id     uuid        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    method      text        NOT NULL CHECK (method IN ('recovery-phrase', 'password', 'passkey')),
    wrapped_key bytea       NOT NULL CHECK (octet_length(wrapped_key) BETWEEN 1 AND 256),
    -- Method settings the client needs again, e.g. the Argon2id salt and
    -- costs, or the passkey's credential ID. Opaque to the server.
    parameters  bytea       CHECK (octet_length(parameters) <= 1024),
    updated_at  timestamptz NOT NULL,
    PRIMARY KEY (user_id, method)
);

CREATE TABLE key_backup_blobs (
    user_id    uuid        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    name       text        NOT NULL CHECK (name ~ '^[a-z0-9][a-z0-9_.-]{0,127}$'),
    data       bytea       NOT NULL CHECK (octet_length(data) BETWEEN 1 AND 1048576),
    updated_at timestamptz NOT NULL,
    PRIMARY KEY (user_id, name)
);

-- +goose Down
DROP TABLE key_backup_blobs;
DROP TABLE key_backup_wrapped_keys;
DROP TABLE key_backups;
