-- +goose Up
-- The MLS delivery service (RFC 9420 §14; docs/e2ee-design.md §3–4). The
-- server keeps each conversation's epoch and latest GroupInfo, the live
-- copies of messages (commits, application messages, welcomes and
-- membership directives) and the archive copies. It reads only cleartext
-- MLS header fields and cannot decrypt anything.
CREATE TABLE mls_groups (
    conversation_id uuid        PRIMARY KEY REFERENCES conversations (id) ON DELETE CASCADE,
    epoch           bigint      NOT NULL DEFAULT 0 CHECK (epoch >= 0),
    -- The signed GroupInfo the last committer uploaded, for external joins.
    group_info      bytea,
    updated_at      timestamptz NOT NULL
);

-- Live copies, in delivery order. Devices fetch what came after the last
-- one they saw. Kept until every device fetched them or 30 days passed.
CREATE TABLE mls_messages (
    id              bigint      GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    conversation_id uuid        NOT NULL REFERENCES conversations (id) ON DELETE CASCADE,
    kind            text        NOT NULL CHECK (kind IN ('commit', 'application', 'welcome', 'directive')),
    epoch           bigint      NOT NULL,
    sender_id       uuid        REFERENCES users (id) ON DELETE SET NULL,
    data            bytea       NOT NULL CHECK (octet_length(data) BETWEEN 1 AND 4194304),
    created_at      timestamptz NOT NULL
);
CREATE INDEX mls_messages_conversation_idx ON mls_messages (conversation_id, id);

-- Archive copies (docs/e2ee-design.md §4), by the client's message ID.
CREATE TABLE archived_messages (
    conversation_id uuid        NOT NULL REFERENCES conversations (id) ON DELETE CASCADE,
    message_id      uuid        NOT NULL,
    epoch           bigint      NOT NULL,
    sender_id       uuid        REFERENCES users (id) ON DELETE SET NULL,
    sealed          bytea       NOT NULL CHECK (octet_length(sealed) BETWEEN 1 AND 4194304),
    created_at      timestamptz NOT NULL,
    PRIMARY KEY (conversation_id, message_id)
);
CREATE INDEX archived_messages_time_idx ON archived_messages (conversation_id, created_at DESC);

-- +goose Down
DROP TABLE archived_messages;
DROP TABLE mls_messages;
DROP TABLE mls_groups;
