-- +goose Up
-- Group DM icons (spec §4.3): the current picture's random ID and format,
-- processed like profile pictures. A new upload gets a new ID, so the
-- picture can be cached forever.
ALTER TABLE conversations
    ADD COLUMN icon_id     uuid UNIQUE,
    ADD COLUMN icon_format text CHECK (icon_format IN ('png', 'jpeg', 'gif')),
    ADD CONSTRAINT conversations_icon_complete CHECK ((icon_id IS NULL) = (icon_format IS NULL)),
    ADD CONSTRAINT conversations_icon_groups_only CHECK (kind = 'group' OR icon_id IS NULL);

-- +goose Down
ALTER TABLE conversations
    DROP CONSTRAINT conversations_icon_groups_only,
    DROP CONSTRAINT conversations_icon_complete,
    DROP COLUMN icon_format,
    DROP COLUMN icon_id;
