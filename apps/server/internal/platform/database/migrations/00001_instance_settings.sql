-- +goose Up
-- instance_settings holds every setting an admin can change at runtime
-- (spec §3.5: everything the setup wizard configures stays editable). Values
-- are JSON so new settings never need a schema change.
CREATE TABLE instance_settings (
    key        text        PRIMARY KEY CHECK (key ~ '^[a-z][a-z0-9_.]{0,63}$'),
    value      jsonb       NOT NULL,
    updated_at timestamptz NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE instance_settings;
