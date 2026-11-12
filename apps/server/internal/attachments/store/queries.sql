-- name: InsertAttachment :exec
INSERT INTO attachments (id, conversation_id, uploader_id, size, created_at)
VALUES ($1, $2, $3, $4, $5);

-- name: AttachmentSize :one
-- The size of an attachment in a conversation; no row when it is not there.
SELECT size FROM attachments WHERE id = $1 AND conversation_id = $2;
