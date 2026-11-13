-- +goose Up
-- Servers, their channels and roles (spec §4.4). Permissions are bit sets
-- (internal/permissions); the bits never change meaning.
CREATE TABLE servers (
    id                   uuid        PRIMARY KEY,
    name                 text        NOT NULL CHECK (char_length(name) BETWEEN 1 AND 100),
    description          text        NOT NULL DEFAULT '' CHECK (char_length(description) <= 1000),
    owner_id             uuid        NOT NULL REFERENCES users (id),
    -- The server's own attachment limit, below the instance's (ADR 0014).
    attachment_max_bytes bigint      CHECK (attachment_max_bytes IS NULL OR attachment_max_bytes > 0),
    -- Whether members' apps compress photos by default here (ADR 0014).
    compress_media       boolean     NOT NULL DEFAULT false,
    created_at           timestamptz NOT NULL
);

-- Roles in order: the higher the position, the more senior. @everyone is
-- at position 0 and every member has it.
CREATE TABLE server_roles (
    id          uuid    PRIMARY KEY,
    server_id   uuid    NOT NULL REFERENCES servers (id) ON DELETE CASCADE,
    name        text    NOT NULL CHECK (char_length(name) BETWEEN 1 AND 100),
    -- 0xRRGGBB, or no color.
    color       integer CHECK (color IS NULL OR color BETWEEN 0 AND 16777215),
    permissions bigint  NOT NULL CHECK (permissions >= 0),
    position    integer NOT NULL CHECK (position >= 0),
    everyone    boolean NOT NULL DEFAULT false,
    CHECK (NOT everyone OR position = 0)
);
CREATE UNIQUE INDEX server_roles_everyone_idx ON server_roles (server_id) WHERE everyone;
CREATE INDEX server_roles_server_idx ON server_roles (server_id, position);

CREATE TABLE server_members (
    server_id       uuid        NOT NULL REFERENCES servers (id) ON DELETE CASCADE,
    user_id         uuid        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    joined_at       timestamptz NOT NULL,
    timed_out_until timestamptz,
    PRIMARY KEY (server_id, user_id)
);
CREATE INDEX server_members_user_idx ON server_members (user_id);

CREATE TABLE server_member_roles (
    server_id uuid NOT NULL,
    user_id   uuid NOT NULL,
    role_id   uuid NOT NULL REFERENCES server_roles (id) ON DELETE CASCADE,
    PRIMARY KEY (server_id, user_id, role_id),
    FOREIGN KEY (server_id, user_id) REFERENCES server_members (server_id, user_id) ON DELETE CASCADE
);

-- Categories, text and voice channels. A channel sits in at most one
-- category; positions order categories, and channels within one.
CREATE TABLE server_channels (
    id           uuid        PRIMARY KEY,
    server_id    uuid        NOT NULL REFERENCES servers (id) ON DELETE CASCADE,
    kind         text        NOT NULL CHECK (kind IN ('category', 'text', 'voice')),
    name         text        NOT NULL CHECK (char_length(name) BETWEEN 1 AND 100),
    topic        text        NOT NULL DEFAULT '' CHECK (char_length(topic) <= 1024),
    parent_id    uuid        REFERENCES server_channels (id) ON DELETE SET NULL,
    position     integer     NOT NULL CHECK (position >= 0),
    -- Text channels are MLS groups, like conversations (docs/e2ee-design.md §3.1).
    mls_group_id bytea       UNIQUE CHECK (mls_group_id IS NULL OR octet_length(mls_group_id) = 16),
    created_at   timestamptz NOT NULL,
    CHECK ((kind = 'text') = (mls_group_id IS NOT NULL)),
    CHECK (kind <> 'category' OR parent_id IS NULL)
);
CREATE INDEX server_channels_server_idx ON server_channels (server_id, position);

-- A channel's allow and deny for one role or one member.
CREATE TABLE server_channel_overrides (
    channel_id  uuid   NOT NULL REFERENCES server_channels (id) ON DELETE CASCADE,
    target_kind text   NOT NULL CHECK (target_kind IN ('role', 'member')),
    target_id   uuid   NOT NULL,
    allow_bits  bigint NOT NULL DEFAULT 0 CHECK (allow_bits >= 0),
    deny_bits   bigint NOT NULL DEFAULT 0 CHECK (deny_bits >= 0),
    PRIMARY KEY (channel_id, target_kind, target_id),
    CHECK (allow_bits & deny_bits = 0)
);

-- Invite links: a random code, with an optional expiry and use limit.
CREATE TABLE server_invites (
    code       text        PRIMARY KEY CHECK (char_length(code) BETWEEN 8 AND 32),
    server_id  uuid        NOT NULL REFERENCES servers (id) ON DELETE CASCADE,
    created_by uuid        REFERENCES users (id) ON DELETE SET NULL,
    max_uses   integer     CHECK (max_uses IS NULL OR max_uses > 0),
    uses       integer     NOT NULL DEFAULT 0 CHECK (uses >= 0),
    expires_at timestamptz,
    created_at timestamptz NOT NULL
);
CREATE INDEX server_invites_server_idx ON server_invites (server_id);

-- +goose Down
DROP TABLE server_invites;
DROP TABLE server_channel_overrides;
DROP TABLE server_channels;
DROP TABLE server_member_roles;
DROP TABLE server_members;
DROP TABLE server_roles;
DROP TABLE servers;
