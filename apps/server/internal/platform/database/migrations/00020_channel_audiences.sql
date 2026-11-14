-- +goose Up
-- Server channels are encrypted in one MLS group per audience: the set of
-- members who hold VIEW_CHANNEL (docs/e2ee-design.md §3.1). An audience is
-- stored as a conversation of kind 'audience' that belongs to its server, so
-- the delivery service, external joins and membership directives work for
-- it exactly as for group DMs. Its members are the users entitled to be in
-- the group; the servers feature keeps them in step with permissions.
ALTER TABLE conversations DROP CONSTRAINT conversations_kind_check;
ALTER TABLE conversations ADD CONSTRAINT conversations_kind_check CHECK (kind IN ('dm', 'group', 'audience'));
ALTER TABLE conversations ADD COLUMN server_id uuid REFERENCES servers (id) ON DELETE CASCADE;
ALTER TABLE conversations ADD CONSTRAINT conversations_audience_server_check CHECK ((kind = 'audience') = (server_id IS NOT NULL));
CREATE INDEX conversations_server_idx ON conversations (server_id) WHERE server_id IS NOT NULL;

-- A text channel's group is its audience's; it moves to another audience
-- when its audience changes. Categories and voice channels have none.
ALTER TABLE server_channels DROP COLUMN mls_group_id;
ALTER TABLE server_channels ADD COLUMN audience_id uuid REFERENCES conversations (id) ON DELETE SET NULL;
ALTER TABLE server_channels ADD CONSTRAINT server_channels_audience_check CHECK (kind = 'text' OR audience_id IS NULL);

-- Application messages in an audience's group name their channel, in the
-- clear, so the server can check SEND_MESSAGES and members can tell the
-- channels apart before decrypting. Members check it against the channel
-- inside the encrypted message.
ALTER TABLE mls_messages ADD COLUMN channel_id uuid REFERENCES server_channels (id) ON DELETE CASCADE;
ALTER TABLE archived_messages ADD COLUMN channel_id uuid REFERENCES server_channels (id) ON DELETE CASCADE;
CREATE INDEX archived_messages_channel_idx ON archived_messages (channel_id, created_at DESC) WHERE channel_id IS NOT NULL;

-- +goose Down
DROP INDEX archived_messages_channel_idx;
ALTER TABLE archived_messages DROP COLUMN channel_id;
ALTER TABLE mls_messages DROP COLUMN channel_id;
ALTER TABLE server_channels DROP COLUMN audience_id;
ALTER TABLE server_channels ADD COLUMN mls_group_id bytea UNIQUE CHECK (mls_group_id IS NULL OR octet_length(mls_group_id) = 16);
DELETE FROM conversations WHERE kind = 'audience';
DROP INDEX conversations_server_idx;
ALTER TABLE conversations DROP CONSTRAINT conversations_audience_server_check;
ALTER TABLE conversations DROP COLUMN server_id;
ALTER TABLE conversations DROP CONSTRAINT conversations_kind_check;
ALTER TABLE conversations ADD CONSTRAINT conversations_kind_check CHECK (kind IN ('dm', 'group'));
