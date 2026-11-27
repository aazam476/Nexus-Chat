-- +goose Up
-- Threads (spec §4.5): replies on a channel message, encrypted in the
-- channel's audience group like the channel's other messages. Both copies
-- of a reply name the message the thread is on, so devices can file it
-- and the server can list a thread. Message IDs are chosen by senders, so
-- a thread is identified within its channel.
ALTER TABLE mls_messages ADD COLUMN thread_root_id uuid;
ALTER TABLE archived_messages ADD COLUMN thread_root_id uuid;
CREATE INDEX archived_messages_thread_idx ON archived_messages (channel_id, thread_root_id, created_at DESC)
    WHERE thread_root_id IS NOT NULL;

CREATE TABLE channel_threads (
    channel_id      uuid        NOT NULL REFERENCES server_channels (id) ON DELETE CASCADE,
    root_message_id uuid        NOT NULL,
    created_by      uuid        REFERENCES users (id) ON DELETE SET NULL,
    created_at      timestamptz NOT NULL,
    reply_count     integer     NOT NULL DEFAULT 0 CHECK (reply_count >= 0),
    last_reply_at   timestamptz NOT NULL,
    PRIMARY KEY (channel_id, root_message_id)
);
CREATE INDEX channel_threads_activity_idx ON channel_threads (channel_id, last_reply_at DESC);

-- Who follows a thread; followed threads notify (M8).
CREATE TABLE thread_follows (
    channel_id      uuid NOT NULL,
    root_message_id uuid NOT NULL,
    user_id         uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    PRIMARY KEY (channel_id, root_message_id, user_id),
    FOREIGN KEY (channel_id, root_message_id) REFERENCES channel_threads (channel_id, root_message_id) ON DELETE CASCADE
);

-- +goose Down
DROP TABLE thread_follows;
DROP TABLE channel_threads;
DROP INDEX archived_messages_thread_idx;
ALTER TABLE archived_messages DROP COLUMN thread_root_id;
ALTER TABLE mls_messages DROP COLUMN thread_root_id;
