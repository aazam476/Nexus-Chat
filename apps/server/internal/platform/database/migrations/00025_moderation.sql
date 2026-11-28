-- +goose Up
-- Moderation (spec §4.7).

-- The audit log: every moderation and configuration action in a server,
-- kept for 90 days. `details` holds what the action changed, never message
-- content.
CREATE TABLE server_audit_log (
    id          bigint      GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    server_id   uuid        NOT NULL REFERENCES servers (id) ON DELETE CASCADE,
    actor_id    uuid        REFERENCES users (id) ON DELETE SET NULL,
    action      text        NOT NULL CHECK (action ~ '^[a-z]+(\.[a-z_]+)+$'),
    target_kind text        CHECK (target_kind IN ('server', 'channel', 'role', 'member', 'invite', 'emoji', 'message', 'report')),
    target_id   text        CHECK (octet_length(target_id) <= 64),
    details     jsonb       NOT NULL DEFAULT '{}'::jsonb,
    created_at  timestamptz NOT NULL,
    CHECK ((target_kind IS NULL) = (target_id IS NULL))
);
CREATE INDEX server_audit_log_server_idx ON server_audit_log (server_id, id DESC);
CREATE INDEX server_audit_log_time_idx ON server_audit_log (created_at);

-- Who is banned from a server: they cannot join again until unbanned.
CREATE TABLE server_bans (
    server_id  uuid        NOT NULL REFERENCES servers (id) ON DELETE CASCADE,
    user_id    uuid        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    banned_by  uuid        REFERENCES users (id) ON DELETE SET NULL,
    reason     text        NOT NULL DEFAULT '' CHECK (char_length(reason) <= 512),
    created_at timestamptz NOT NULL,
    PRIMARY KEY (server_id, user_id)
);

-- A live copy names the message its archive copy carries, so a moderator's
-- deletion can remove both.
ALTER TABLE mls_messages ADD COLUMN message_id uuid;
CREATE INDEX mls_messages_message_idx ON mls_messages (conversation_id, message_id) WHERE message_id IS NOT NULL;
CREATE INDEX archived_messages_sender_idx ON archived_messages (conversation_id, sender_id, created_at DESC);

-- +goose Down
DROP INDEX archived_messages_sender_idx;
DROP INDEX mls_messages_message_idx;
ALTER TABLE mls_messages DROP COLUMN message_id;
DROP TABLE server_bans;
DROP TABLE server_audit_log;
