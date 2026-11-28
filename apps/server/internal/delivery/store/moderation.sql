-- Moderators delete messages by deleting their ciphertext (spec §4.7).

-- name: DeleteChannelArchive :one
-- A channel message's archive copy; returns who sent it and its thread.
DELETE FROM archived_messages
WHERE conversation_id = @conversation_id AND message_id = @message_id AND channel_id = @channel_id
RETURNING sender_id, thread_root_id;

-- name: DeleteLiveCopies :exec
DELETE FROM mls_messages WHERE conversation_id = @conversation_id AND message_id = @message_id AND kind = 'application';

-- name: UncountThreadReply :exec
UPDATE channel_threads SET reply_count = greatest(reply_count - 1, 0)
WHERE channel_id = @channel_id AND root_message_id = @root_message_id;

-- name: PurgeArchive :execrows
-- Everything userID sent to these groups since a time.
DELETE FROM archived_messages
WHERE conversation_id = ANY(@conversation_ids::uuid[]) AND sender_id = @user_id AND created_at >= @since;

-- name: PurgeLiveCopies :execrows
DELETE FROM mls_messages
WHERE conversation_id = ANY(@conversation_ids::uuid[]) AND sender_id = @user_id AND kind = 'application' AND created_at >= @since;

-- name: RecountThreads :exec
-- Thread reply counts in the channels of these groups, after a purge.
UPDATE channel_threads t SET reply_count = (
    SELECT count(*) FROM archived_messages a WHERE a.channel_id = t.channel_id AND a.thread_root_id = t.root_message_id
)
WHERE t.channel_id IN (SELECT c.id FROM server_channels c WHERE c.audience_id = ANY(@conversation_ids::uuid[]));

-- name: GetArchivedCopy :one
-- One archive copy, for a report about it.
SELECT conversation_id, message_id, epoch, sender_id, sealed, created_at, channel_id, thread_root_id FROM archived_messages
WHERE conversation_id = @conversation_id AND message_id = @message_id;
