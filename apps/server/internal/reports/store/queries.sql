-- name: InsertReport :exec
INSERT INTO reports (id, server_id, channel_id, conversation_id, message_id, reporter_id, sender_id, evidence, receipt, receipt_sig, comment, escalated, created_at)
VALUES (@id, sqlc.narg(server_id), sqlc.narg(channel_id), @conversation_id, @message_id, @reporter_id, sqlc.narg(sender_id), @evidence, @receipt, @receipt_sig, @comment, @escalated, @created_at);

-- name: ServerReports :many
-- A server's reports with the given status, newest first.
SELECT * FROM reports WHERE server_id = @server_id AND status = @status ORDER BY created_at DESC, id LIMIT 100;

-- name: AdminReports :many
-- Reports for the instance admins: about direct and group messages, or
-- passed on by a server's moderators (all of them when `everything`).
SELECT * FROM reports
WHERE status = @status AND (server_id IS NULL OR escalated OR @everything::boolean)
ORDER BY created_at DESC, id LIMIT 100;

-- name: GetReport :one
SELECT * FROM reports WHERE id = @id;

-- name: ResolveReport :execrows
UPDATE reports SET status = 'resolved', resolution = @resolution, resolved_by = @resolved_by, resolved_at = @resolved_at
WHERE id = @id AND status = 'open';

-- name: EscalateReport :execrows
UPDATE reports SET escalated = true WHERE id = @id AND status = 'open' AND NOT escalated;
