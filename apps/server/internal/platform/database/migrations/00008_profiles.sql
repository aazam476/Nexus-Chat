-- +goose Up
-- Profile fields and status (spec §4.1). "status" is what the user chose;
-- idle is also set automatically by the gateway, and offline is never
-- stored: it means no device is connected.
ALTER TABLE users
    ADD COLUMN bio                      text        NOT NULL DEFAULT '' CHECK (char_length(bio) <= 500),
    ADD COLUMN avatar_id                uuid,
    ADD COLUMN avatar_format            text        CHECK (avatar_format IN ('png', 'jpeg', 'gif')),
    ADD COLUMN status                   text        NOT NULL DEFAULT 'online' CHECK (status IN ('online', 'idle', 'dnd', 'invisible')),
    ADD COLUMN custom_status_text       text        CHECK (char_length(custom_status_text) BETWEEN 1 AND 128),
    ADD COLUMN custom_status_expires_at timestamptz,
    ADD CONSTRAINT users_avatar_complete CHECK ((avatar_id IS NULL) = (avatar_format IS NULL));

-- Avatars are served by ID; each avatar belongs to one account.
CREATE UNIQUE INDEX users_avatar_id_key ON users (avatar_id);
-- The sweeper looks for custom statuses that have expired.
CREATE INDEX users_custom_status_expires_at_idx ON users (custom_status_expires_at) WHERE custom_status_expires_at IS NOT NULL;

-- +goose Down
DROP INDEX users_custom_status_expires_at_idx;
DROP INDEX users_avatar_id_key;
ALTER TABLE users
    DROP CONSTRAINT users_avatar_complete,
    DROP COLUMN custom_status_expires_at,
    DROP COLUMN custom_status_text,
    DROP COLUMN status,
    DROP COLUMN avatar_format,
    DROP COLUMN avatar_id,
    DROP COLUMN bio;
