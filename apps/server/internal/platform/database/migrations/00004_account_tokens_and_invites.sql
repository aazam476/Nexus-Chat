-- +goose Up
-- account_tokens are single-use links sent by email: verifying an address
-- and resetting a password. Only a SHA-256 hash of each token is stored.
CREATE TABLE account_tokens (
    id         uuid        PRIMARY KEY,
    user_id    uuid        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    purpose    text        NOT NULL CHECK (purpose IN ('verify_email', 'reset_password')),
    token_hash bytea       NOT NULL UNIQUE CHECK (octet_length(token_hash) = 32),
    created_at timestamptz NOT NULL,
    expires_at timestamptz NOT NULL
);

CREATE INDEX account_tokens_user_id_idx ON account_tokens (user_id, purpose);

-- instance_invites let people register when registration is invite-only.
CREATE TABLE instance_invites (
    id         uuid        PRIMARY KEY,
    code_hash  bytea       NOT NULL UNIQUE CHECK (octet_length(code_hash) = 32),
    created_by uuid        REFERENCES users (id) ON DELETE SET NULL,
    max_uses   integer     CHECK (max_uses IS NULL OR max_uses > 0),
    uses       integer     NOT NULL DEFAULT 0,
    expires_at timestamptz,
    created_at timestamptz NOT NULL
);

-- +goose Down
DROP TABLE instance_invites;
DROP TABLE account_tokens;
