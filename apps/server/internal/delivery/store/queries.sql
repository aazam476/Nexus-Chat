-- name: EnsureGroup :one
-- The conversation's group state, created at epoch 0 on first use.
INSERT INTO mls_groups (conversation_id, updated_at) VALUES ($1, $2)
ON CONFLICT (conversation_id) DO UPDATE SET conversation_id = EXCLUDED.conversation_id
RETURNING *;

-- name: AdvanceEpoch :one
-- Accepts a commit: moves the group from the epoch the commit was made in
-- to the next. Only one commit per epoch can match, so the first valid
-- one wins (docs/e2ee-design.md §3.2).
UPDATE mls_groups SET epoch = epoch + 1, group_info = sqlc.arg(group_info), updated_at = sqlc.arg(now)
WHERE conversation_id = sqlc.arg(conversation_id) AND epoch = sqlc.arg(expected_epoch)
RETURNING epoch;

-- name: InsertMessage :one
INSERT INTO mls_messages (conversation_id, kind, epoch, sender_id, data, created_at, channel_id, thread_root_id)
VALUES ($1, $2, $3, $4, $5, $6, sqlc.narg(channel_id), sqlc.narg(thread_root_id))
RETURNING id;

-- name: MessagesAfter :many
SELECT id, kind, epoch, sender_id, data, created_at, channel_id, thread_root_id FROM mls_messages
WHERE conversation_id = $1 AND id > sqlc.arg(after_id)
ORDER BY id LIMIT sqlc.arg(row_limit);

-- name: InsertArchive :exec
INSERT INTO archived_messages (conversation_id, message_id, epoch, sender_id, sealed, created_at, channel_id, thread_root_id)
VALUES ($1, $2, $3, $4, $5, $6, sqlc.narg(channel_id), sqlc.narg(thread_root_id));

-- name: ArchiveBefore :many
-- Archive copies older than `before`, newest first, for scrolling back.
SELECT message_id, epoch, sender_id, sealed, created_at FROM archived_messages
WHERE conversation_id = $1 AND created_at < sqlc.arg(before)
ORDER BY created_at DESC, message_id LIMIT sqlc.arg(row_limit);

-- name: ChannelArchiveBefore :many
-- A server channel's archive copies from `since` up to `before`, newest
-- first, with the MLS group each was sent in: a channel moves between
-- audience groups as its audience changes. Replies in threads are left
-- out; a thread's archive has them.
SELECT a.message_id, c.mls_group_id, a.epoch, a.sender_id, a.sealed, a.created_at
FROM archived_messages a JOIN conversations c ON c.id = a.conversation_id
WHERE a.channel_id = sqlc.arg(channel_id) AND a.thread_root_id IS NULL
  AND a.created_at < sqlc.arg(before) AND a.created_at >= sqlc.arg(since)
ORDER BY a.created_at DESC, a.message_id LIMIT sqlc.arg(row_limit);

-- name: ThreadArchiveBefore :many
-- Like ChannelArchiveBefore, for the replies in one thread.
SELECT a.message_id, c.mls_group_id, a.epoch, a.sender_id, a.sealed, a.created_at
FROM archived_messages a JOIN conversations c ON c.id = a.conversation_id
WHERE a.channel_id = sqlc.arg(channel_id) AND a.thread_root_id = sqlc.arg(root_message_id)
  AND a.created_at < sqlc.arg(before) AND a.created_at >= sqlc.arg(since)
ORDER BY a.created_at DESC, a.message_id LIMIT sqlc.arg(row_limit);

-- name: ThreadRoot :one
-- The sender of a channel message that can carry a thread: one in the
-- channel that is not itself a reply in a thread.
SELECT sender_id FROM archived_messages
WHERE channel_id = sqlc.arg(channel_id) AND message_id = sqlc.arg(message_id) AND thread_root_id IS NULL
LIMIT 1;

-- name: ThreadExists :one
SELECT EXISTS (
    SELECT 1 FROM channel_threads WHERE channel_id = sqlc.arg(channel_id) AND root_message_id = sqlc.arg(root_message_id)
);

-- name: RecordThreadReply :exec
-- Starts the thread on its first reply, or counts another one.
INSERT INTO channel_threads AS threads (channel_id, root_message_id, created_by, created_at, reply_count, last_reply_at)
VALUES (sqlc.arg(channel_id), sqlc.arg(root_message_id), sqlc.arg(sender_id), sqlc.arg(now), 1, sqlc.arg(now))
ON CONFLICT (channel_id, root_message_id) DO UPDATE
SET reply_count = threads.reply_count + 1, last_reply_at = EXCLUDED.last_reply_at;

-- name: FollowThread :execrows
INSERT INTO thread_follows (channel_id, root_message_id, user_id)
SELECT threads.channel_id, threads.root_message_id, sqlc.arg(user_id)::uuid FROM channel_threads AS threads
WHERE threads.channel_id = sqlc.arg(channel_id) AND threads.root_message_id = sqlc.arg(root_message_id)
ON CONFLICT DO NOTHING;

-- name: UnfollowThread :exec
DELETE FROM thread_follows
WHERE channel_id = sqlc.arg(channel_id) AND root_message_id = sqlc.arg(root_message_id) AND user_id = sqlc.arg(user_id);

-- name: ChannelThreads :many
-- A channel's threads, most recently active first, and whether the user follows each.
SELECT threads.root_message_id, threads.reply_count, threads.created_at, threads.last_reply_at,
    EXISTS (
        SELECT 1 FROM thread_follows AS follows
        WHERE follows.channel_id = threads.channel_id AND follows.root_message_id = threads.root_message_id
          AND follows.user_id = sqlc.arg(user_id)
    ) AS following
FROM channel_threads AS threads
WHERE threads.channel_id = sqlc.arg(channel_id)
ORDER BY threads.last_reply_at DESC, threads.root_message_id
LIMIT 100;

-- name: DeleteExpiredMessages :execrows
-- Live copies older than the cut-off; archive copies stay.
DELETE FROM mls_messages WHERE created_at < $1;

-- name: MarkRead :one
-- Moves the member's read position forward, never back and never past the
-- conversation's newest message, and returns where it now is.
INSERT INTO conversation_reads AS reads (conversation_id, user_id, last_read_id, updated_at)
VALUES (
    sqlc.arg(conversation_id), sqlc.arg(user_id),
    LEAST(sqlc.arg(last_read_id)::bigint, (SELECT COALESCE(MAX(id), 0) FROM mls_messages WHERE conversation_id = sqlc.arg(conversation_id))),
    sqlc.arg(now)
)
ON CONFLICT (conversation_id, user_id) DO UPDATE
SET last_read_id = GREATEST(reads.last_read_id, EXCLUDED.last_read_id), updated_at = EXCLUDED.updated_at
RETURNING last_read_id;

-- name: ReadStates :many
-- For each of the user's conversations: how far they read, and how many
-- application messages from others came after, counting up to 1000.
SELECT
    members.conversation_id,
    COALESCE(reads.last_read_id, 0)::bigint AS last_read_id,
    (
        SELECT count(*) FROM (
            SELECT 1 FROM mls_messages AS messages
            WHERE messages.conversation_id = members.conversation_id
              AND messages.kind = 'application'
              AND messages.id > COALESCE(reads.last_read_id, 0)
              AND messages.sender_id IS DISTINCT FROM members.user_id
            LIMIT 1000
        ) AS unread
    )::bigint AS unread_count
FROM conversation_members AS members
JOIN conversations AS conversation ON conversation.id = members.conversation_id AND conversation.kind <> 'audience'
LEFT JOIN conversation_reads AS reads
    ON reads.conversation_id = members.conversation_id AND reads.user_id = members.user_id
WHERE members.user_id = sqlc.arg(user_id)
ORDER BY members.conversation_id;

-- name: MarkChannelRead :one
-- Like MarkRead, for a server channel's messages in its audience's group.
INSERT INTO channel_reads AS reads (channel_id, user_id, last_read_id, updated_at)
VALUES (
    sqlc.arg(channel_id), sqlc.arg(user_id),
    LEAST(sqlc.arg(last_read_id)::bigint, (SELECT COALESCE(MAX(id), 0) FROM mls_messages WHERE channel_id = sqlc.arg(channel_id))),
    sqlc.arg(now)
)
ON CONFLICT (channel_id, user_id) DO UPDATE
SET last_read_id = GREATEST(reads.last_read_id, EXCLUDED.last_read_id), updated_at = EXCLUDED.updated_at
RETURNING last_read_id;

-- name: ChannelReadStates :many
-- For each of the given channels: how far the user read it, and how many
-- application messages from others came after, counting up to 1000.
SELECT
    channel.id::uuid AS channel_id,
    COALESCE(reads.last_read_id, 0)::bigint AS last_read_id,
    (
        SELECT count(*) FROM (
            SELECT 1 FROM mls_messages AS messages
            WHERE messages.channel_id = channel.id
              AND messages.kind = 'application'
              AND messages.thread_root_id IS NULL
              AND messages.id > COALESCE(reads.last_read_id, 0)
              AND messages.sender_id IS DISTINCT FROM sqlc.arg(user_id)::uuid
            LIMIT 1000
        ) AS unread
    )::bigint AS unread_count
FROM unnest(sqlc.arg(channel_ids)::uuid[]) AS channel(id)
LEFT JOIN channel_reads AS reads ON reads.channel_id = channel.id AND reads.user_id = sqlc.arg(user_id)::uuid
ORDER BY channel.id;
