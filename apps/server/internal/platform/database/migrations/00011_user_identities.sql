-- +goose Up
-- Each user's published identity (docs/e2ee-design.md §2): the public
-- halves of their user master key and self-signing key, and the master
-- key's signature over the self-signing key. Clients pin the master key on
-- first contact and check every device certificate against the
-- self-signing key. The private halves never reach the server unencrypted.
CREATE TABLE user_identities (
    user_id                 uuid        PRIMARY KEY REFERENCES users (id) ON DELETE CASCADE,
    master_public_key       bytea       NOT NULL CHECK (octet_length(master_public_key) = 32),
    self_signing_public_key bytea       NOT NULL CHECK (octet_length(self_signing_public_key) = 32),
    self_signing_signature  bytea       NOT NULL CHECK (octet_length(self_signing_signature) = 64),
    updated_at              timestamptz NOT NULL
);

-- +goose Down
DROP TABLE user_identities;
