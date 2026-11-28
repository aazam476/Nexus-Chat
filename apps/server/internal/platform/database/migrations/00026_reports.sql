-- +goose Up
-- User reports (spec §4.7, ADR 0010). A report holds the reported message
-- as its reporter revealed it, with the sender's signature (the evidence),
-- and the instance's signed receipt for the stored copy. Moderators'
-- devices check both. Reports about server channels go to the server's
-- moderators, who can pass them on to the instance admins; reports about
-- direct and group messages go to the instance admins.
CREATE TABLE reports (
    id              uuid        PRIMARY KEY,
    server_id       uuid        REFERENCES servers (id) ON DELETE CASCADE,
    channel_id      uuid        REFERENCES server_channels (id) ON DELETE SET NULL,
    conversation_id uuid        REFERENCES conversations (id) ON DELETE SET NULL,
    message_id      uuid        NOT NULL,
    reporter_id     uuid        REFERENCES users (id) ON DELETE SET NULL,
    sender_id       uuid        REFERENCES users (id) ON DELETE SET NULL,
    evidence        bytea       NOT NULL CHECK (octet_length(evidence) BETWEEN 1 AND 4194304),
    receipt         bytea       NOT NULL,
    receipt_sig     bytea       NOT NULL CHECK (octet_length(receipt_sig) = 64),
    comment         text        NOT NULL DEFAULT '' CHECK (char_length(comment) <= 1000),
    status          text        NOT NULL DEFAULT 'open' CHECK (status IN ('open', 'resolved')),
    escalated       boolean     NOT NULL DEFAULT false,
    resolution      text        NOT NULL DEFAULT '' CHECK (char_length(resolution) <= 1000),
    resolved_by     uuid        REFERENCES users (id) ON DELETE SET NULL,
    resolved_at     timestamptz,
    created_at      timestamptz NOT NULL,
    CHECK ((status = 'resolved') = (resolved_at IS NOT NULL))
);
CREATE UNIQUE INDEX reports_once_idx ON reports (reporter_id, message_id) WHERE reporter_id IS NOT NULL;
CREATE INDEX reports_server_idx ON reports (server_id, status, created_at DESC) WHERE server_id IS NOT NULL;
CREATE INDEX reports_admin_idx ON reports (status, created_at DESC) WHERE server_id IS NULL OR escalated;

-- +goose Down
DROP TABLE reports;
