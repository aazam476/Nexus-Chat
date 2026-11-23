-- +goose Up
-- Server icons (spec §4.4), processed and served like group DM icons: the
-- current picture's random ID and format. A new upload gets a new ID, so
-- the picture can be cached forever.
ALTER TABLE servers
    ADD COLUMN icon_id     uuid UNIQUE,
    ADD COLUMN icon_format text CHECK (icon_format IN ('png', 'jpeg', 'gif')),
    ADD CONSTRAINT servers_icon_complete CHECK ((icon_id IS NULL) = (icon_format IS NULL));

-- +goose Down
ALTER TABLE servers
    DROP CONSTRAINT servers_icon_complete,
    DROP COLUMN icon_format,
    DROP COLUMN icon_id;
