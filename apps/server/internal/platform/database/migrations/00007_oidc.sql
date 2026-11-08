-- +goose Up
-- Sign-in providers configured by the instance admin (ADR 0016). The client
-- secret is AES-256-GCM encrypted with the settings key; NULL for public
-- clients that use PKCE alone.
CREATE TABLE oidc_providers (
    id                   text        PRIMARY KEY CHECK (id ~ '^[a-z0-9-]{2,32}$'),
    display_name         text        NOT NULL CHECK (char_length(display_name) BETWEEN 1 AND 64),
    issuer               text        NOT NULL,
    client_id            text        NOT NULL CHECK (char_length(client_id) BETWEEN 1 AND 256),
    client_secret_sealed bytea,
    scopes               text[]      NOT NULL,
    allow_signup         boolean     NOT NULL,
    created_at           timestamptz NOT NULL,
    updated_at           timestamptz NOT NULL
);

-- A provider identity (issuer + subject) belongs to at most one account,
-- and an account has at most one identity per provider.
CREATE TABLE oidc_identities (
    provider_id text        NOT NULL REFERENCES oidc_providers (id) ON DELETE CASCADE,
    subject     text        NOT NULL,
    user_id     uuid        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    email       text        NOT NULL,
    created_at  timestamptz NOT NULL,
    PRIMARY KEY (provider_id, subject),
    UNIQUE (provider_id, user_id)
);

CREATE INDEX oidc_identities_user_id_idx ON oidc_identities (user_id);

-- A sign-in or link in progress. The state and completion code are stored
-- only as SHA-256 hashes. Claims are filled in by the callback once the ID
-- token is verified; the flow is deleted when the client completes it.
CREATE TABLE oidc_flows (
    id              uuid        PRIMARY KEY,
    provider_id     text        NOT NULL REFERENCES oidc_providers (id) ON DELETE CASCADE,
    -- NULL for a sign-in; the account being linked otherwise.
    user_id         uuid        REFERENCES users (id) ON DELETE CASCADE,
    state_hash      bytea       NOT NULL UNIQUE CHECK (octet_length(state_hash) = 32),
    nonce           text        NOT NULL,
    code_verifier   text        NOT NULL,
    return_to       text        NOT NULL,
    -- Hash of the invite code given at the start, like instance_invites.
    invite_hash     bytea,
    completion_hash bytea       CHECK (completion_hash IS NULL OR octet_length(completion_hash) = 32),
    claims          jsonb,
    expires_at      timestamptz NOT NULL
);

CREATE INDEX oidc_flows_expires_at_idx ON oidc_flows (expires_at);

-- +goose Down
DROP TABLE oidc_flows;
DROP TABLE oidc_identities;
DROP TABLE oidc_providers;
