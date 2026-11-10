-- +goose Up
-- Friends, blocks and privacy settings (spec §4.2).
CREATE TABLE friend_requests (
    sender_id    uuid        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    recipient_id uuid        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    created_at   timestamptz NOT NULL,
    PRIMARY KEY (sender_id, recipient_id),
    CHECK (sender_id <> recipient_id)
);
CREATE INDEX friend_requests_recipient_idx ON friend_requests (recipient_id);

-- A friendship is stored once in each direction, so "my friends" is one
-- index lookup.
CREATE TABLE friendships (
    user_id    uuid        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    friend_id  uuid        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    created_at timestamptz NOT NULL,
    PRIMARY KEY (user_id, friend_id),
    CHECK (user_id <> friend_id)
);

CREATE TABLE blocks (
    blocker_id uuid        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    blocked_id uuid        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    created_at timestamptz NOT NULL,
    PRIMARY KEY (blocker_id, blocked_id),
    CHECK (blocker_id <> blocked_id)
);
CREATE INDEX blocks_blocked_idx ON blocks (blocked_id);

-- Missing rows mean the defaults below.
CREATE TABLE privacy_settings (
    user_id                uuid PRIMARY KEY REFERENCES users (id) ON DELETE CASCADE,
    who_can_dm             text NOT NULL CHECK (who_can_dm IN ('everyone', 'friends', 'friends_and_server_members')),
    who_can_friend_request text NOT NULL CHECK (who_can_friend_request IN ('everyone', 'server_members', 'nobody'))
);

-- +goose Down
DROP TABLE privacy_settings;
DROP TABLE blocks;
DROP TABLE friendships;
DROP TABLE friend_requests;
