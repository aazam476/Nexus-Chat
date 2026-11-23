-- +goose Up
-- Read state in server channels (spec §4.5), like conversation_reads: how
-- far each member has read each text channel, by the delivery ID of the
-- last live copy they saw there.
CREATE TABLE channel_reads (
    channel_id   uuid        NOT NULL REFERENCES server_channels (id) ON DELETE CASCADE,
    user_id      uuid        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    last_read_id bigint      NOT NULL CHECK (last_read_id >= 0),
    updated_at   timestamptz NOT NULL,
    PRIMARY KEY (channel_id, user_id)
);
CREATE INDEX mls_messages_channel_idx ON mls_messages (channel_id, id) WHERE channel_id IS NOT NULL;

-- +goose Down
DROP INDEX mls_messages_channel_idx;
DROP TABLE channel_reads;
