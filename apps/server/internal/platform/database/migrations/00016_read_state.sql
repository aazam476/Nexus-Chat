-- +goose Up
-- Read state (spec §4.5): how far each member has read each conversation,
-- by the delivery ID of the last live copy they saw, so every device of
-- theirs shows the same unread markers. The server only counts messages;
-- it never sees their content.
CREATE TABLE conversation_reads (
    conversation_id uuid        NOT NULL REFERENCES conversations (id) ON DELETE CASCADE,
    user_id         uuid        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    last_read_id    bigint      NOT NULL CHECK (last_read_id >= 0),
    updated_at      timestamptz NOT NULL,
    PRIMARY KEY (conversation_id, user_id)
);

-- +goose Down
DROP TABLE conversation_reads;
