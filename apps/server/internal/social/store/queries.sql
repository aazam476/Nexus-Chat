-- name: FindUserByUsername :one
SELECT id FROM users WHERE username = $1;

-- name: UserExists :one
SELECT EXISTS (SELECT 1 FROM users WHERE id = $1);

-- name: Relationship :one
-- Everything that decides what one user may do towards another.
SELECT
    EXISTS (SELECT 1 FROM friendships f WHERE f.user_id = sqlc.arg(me) AND f.friend_id = sqlc.arg(other)) AS friends,
    EXISTS (SELECT 1 FROM friend_requests r WHERE r.sender_id = sqlc.arg(me) AND r.recipient_id = sqlc.arg(other)) AS outgoing,
    EXISTS (SELECT 1 FROM friend_requests r WHERE r.sender_id = sqlc.arg(other) AND r.recipient_id = sqlc.arg(me)) AS incoming,
    EXISTS (SELECT 1 FROM blocks b WHERE b.blocker_id = sqlc.arg(me) AND b.blocked_id = sqlc.arg(other)) AS i_blocked,
    EXISTS (SELECT 1 FROM blocks b WHERE b.blocker_id = sqlc.arg(other) AND b.blocked_id = sqlc.arg(me)) AS blocked_me;

-- name: InsertFriendRequest :exec
INSERT INTO friend_requests (sender_id, recipient_id, created_at) VALUES ($1, $2, $3);

-- name: AcceptFriendRequest :execrows
-- Turns the request from sender to recipient into a friendship, in one
-- statement: no request, no friendship.
WITH accepted AS (
    DELETE FROM friend_requests WHERE sender_id = sqlc.arg(sender) AND recipient_id = sqlc.arg(recipient)
    RETURNING sender_id, recipient_id
)
INSERT INTO friendships (user_id, friend_id, created_at)
SELECT pair.user_id, pair.friend_id, sqlc.arg(now)::timestamptz
FROM accepted
CROSS JOIN LATERAL (VALUES (accepted.sender_id, accepted.recipient_id), (accepted.recipient_id, accepted.sender_id)) AS pair (user_id, friend_id);

-- name: DeleteFriendRequest :execrows
DELETE FROM friend_requests WHERE sender_id = $1 AND recipient_id = $2;

-- name: DeleteFriendship :execrows
DELETE FROM friendships
WHERE (user_id = sqlc.arg(me) AND friend_id = sqlc.arg(other)) OR (user_id = sqlc.arg(other) AND friend_id = sqlc.arg(me));

-- name: Block :exec
-- Blocking also ends any friendship and pending request between the two.
WITH unfriended AS (
    DELETE FROM friendships
    WHERE (user_id = sqlc.arg(blocker) AND friend_id = sqlc.arg(blocked)) OR (user_id = sqlc.arg(blocked) AND friend_id = sqlc.arg(blocker))
), withdrawn AS (
    DELETE FROM friend_requests
    WHERE (sender_id = sqlc.arg(blocker) AND recipient_id = sqlc.arg(blocked)) OR (sender_id = sqlc.arg(blocked) AND recipient_id = sqlc.arg(blocker))
)
INSERT INTO blocks (blocker_id, blocked_id, created_at) VALUES (sqlc.arg(blocker), sqlc.arg(blocked), sqlc.arg(now))
ON CONFLICT DO NOTHING;

-- name: Unblock :execrows
DELETE FROM blocks WHERE blocker_id = $1 AND blocked_id = $2;

-- name: ListFriends :many
SELECT u.id, u.username, u.display_name, u.avatar_id, f.created_at AS since
FROM friendships f JOIN users u ON u.id = f.friend_id
WHERE f.user_id = $1 ORDER BY u.username;

-- name: ListIncomingRequests :many
SELECT u.id, u.username, u.display_name, u.avatar_id, r.created_at AS since
FROM friend_requests r JOIN users u ON u.id = r.sender_id
WHERE r.recipient_id = $1 ORDER BY r.created_at;

-- name: ListOutgoingRequests :many
SELECT u.id, u.username, u.display_name, u.avatar_id, r.created_at AS since
FROM friend_requests r JOIN users u ON u.id = r.recipient_id
WHERE r.sender_id = $1 ORDER BY r.created_at;

-- name: ListBlocked :many
SELECT u.id, u.username, u.display_name, u.avatar_id, b.created_at AS since
FROM blocks b JOIN users u ON u.id = b.blocked_id
WHERE b.blocker_id = $1 ORDER BY u.username;

-- name: GetPrivacy :one
-- The user's settings, or the defaults when they never changed them.
SELECT
    COALESCE((SELECT p.who_can_dm FROM privacy_settings p WHERE p.user_id = sqlc.arg(user_id)), 'friends_and_server_members')::text AS who_can_dm,
    COALESCE((SELECT p.who_can_friend_request FROM privacy_settings p WHERE p.user_id = sqlc.arg(user_id)), 'everyone')::text AS who_can_friend_request;

-- name: PutPrivacy :exec
INSERT INTO privacy_settings (user_id, who_can_dm, who_can_friend_request) VALUES ($1, $2, $3)
ON CONFLICT (user_id) DO UPDATE SET who_can_dm = EXCLUDED.who_can_dm, who_can_friend_request = EXCLUDED.who_can_friend_request;
