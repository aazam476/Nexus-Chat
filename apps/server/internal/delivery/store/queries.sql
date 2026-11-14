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
INSERT INTO mls_messages (conversation_id, kind, epoch, sender_id, data, created_at, channel_id)
VALUES ($1, $2, $3, $4, $5, $6, sqlc.narg(channel_id))
RETURNING id;

-- name: MessagesAfter :many
SELECT id, kind, epoch, sender_id, data, created_at, channel_id FROM mls_messages
WHERE conversation_id = $1 AND id > sqlc.arg(after_id)
ORDER BY id LIMIT sqlc.arg(row_limit);

-- name: InsertArchive :exec
INSERT INTO archived_messages (conversation_id, message_id, epoch, sender_id, sealed, created_at, channel_id)
VALUES ($1, $2, $3, $4, $5, $6, sqlc.narg(channel_id));

-- name: ArchiveBefore :many
-- Archive copies older than `before`, newest first, for scrolling back.
SELECT message_id, epoch, sender_id, sealed, created_at FROM archived_messages
WHERE conversation_id = $1 AND created_at < sqlc.arg(before)
ORDER BY created_at DESC, message_id LIMIT sqlc.arg(row_limit);

-- name: ChannelArchiveBefore :many
-- A server channel's archive copies from `since` up to `before`, newest
-- first, with the MLS group each was sent in: a channel moves between
-- audience groups as its audience changes.
SELECT a.message_id, c.mls_group_id, a.epoch, a.sender_id, a.sealed, a.created_at
FROM archived_messages a JOIN conversations c ON c.id = a.conversation_id
WHERE a.channel_id = sqlc.arg(channel_id) AND a.created_at < sqlc.arg(before) AND a.created_at >= sqlc.arg(since)
ORDER BY a.created_at DESC, a.message_id LIMIT sqlc.arg(row_limit);

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
