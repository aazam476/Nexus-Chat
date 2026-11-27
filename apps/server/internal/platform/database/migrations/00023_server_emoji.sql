-- +goose Up
-- Custom emoji (spec §4.5): per server, named [a-z0-9_]{2,32}, stored as
-- PNG or GIF at up to 128×128 and served publicly under their random ID,
-- like server icons. Names are unique within a server.
CREATE TABLE server_emoji (
    id         uuid PRIMARY KEY,
    server_id  uuid        NOT NULL REFERENCES servers (id) ON DELETE CASCADE,
    name       text        NOT NULL CHECK (name ~ '^[a-z0-9_]{2,32}$'),
    format     text        NOT NULL CHECK (format IN ('png', 'gif')),
    created_by uuid        REFERENCES users (id) ON DELETE SET NULL,
    created_at timestamptz NOT NULL,
    UNIQUE (server_id, name)
);

-- +goose Down
DROP TABLE server_emoji;
