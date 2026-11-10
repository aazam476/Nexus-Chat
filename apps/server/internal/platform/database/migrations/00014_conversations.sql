-- +goose Up
-- DMs and group DMs (spec §4.3). Each conversation is one MLS group
-- (docs/e2ee-design.md §3.1), whose ID the server picks at creation.
CREATE TABLE conversations (
    id           uuid        PRIMARY KEY,
    kind         text        NOT NULL CHECK (kind IN ('dm', 'group')),
    -- Group DMs only; a DM shows the other person's name.
    name         text        CHECK (name IS NULL OR char_length(name) BETWEEN 1 AND 100),
    -- For a DM, the two user IDs in order, so a pair has one DM.
    dm_pair      text        UNIQUE,
    mls_group_id bytea       NOT NULL UNIQUE CHECK (octet_length(mls_group_id) = 16),
    created_by   uuid        REFERENCES users (id) ON DELETE SET NULL,
    created_at   timestamptz NOT NULL,
    CHECK ((kind = 'dm') = (dm_pair IS NOT NULL)),
    CHECK (kind = 'group' OR name IS NULL)
);

CREATE TABLE conversation_members (
    conversation_id uuid        NOT NULL REFERENCES conversations (id) ON DELETE CASCADE,
    user_id         uuid        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    joined_at       timestamptz NOT NULL,
    PRIMARY KEY (conversation_id, user_id)
);
CREATE INDEX conversation_members_user_idx ON conversation_members (user_id);

-- +goose Down
DROP TABLE conversation_members;
DROP TABLE conversations;
