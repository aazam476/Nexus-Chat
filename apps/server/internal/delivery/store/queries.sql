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
INSERT INTO mls_messages (conversation_id, kind, epoch, sender_id, data, created_at)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING id;

-- name: MessagesAfter :many
SELECT id, kind, epoch, sender_id, data, created_at FROM mls_messages
WHERE conversation_id = $1 AND id > sqlc.arg(after_id)
ORDER BY id LIMIT sqlc.arg(row_limit);

-- name: InsertArchive :exec
INSERT INTO archived_messages (conversation_id, message_id, epoch, sender_id, sealed, created_at)
VALUES ($1, $2, $3, $4, $5, $6);

-- name: ArchiveBefore :many
-- Archive copies older than `before`, newest first, for scrolling back.
SELECT message_id, epoch, sender_id, sealed, created_at FROM archived_messages
WHERE conversation_id = $1 AND created_at < sqlc.arg(before)
ORDER BY created_at DESC, message_id LIMIT sqlc.arg(row_limit);

-- name: DeleteExpiredMessages :execrows
-- Live copies older than the cut-off; archive copies stay.
DELETE FROM mls_messages WHERE created_at < $1;
