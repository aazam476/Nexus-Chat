-- +goose Up
-- Encrypted attachments (spec §4.5, docs/e2ee-design.md §7). The server
-- keeps only ciphertext, in the file store, and knows each file's size and
-- conversation; the key travels inside the MLS-encrypted message.
CREATE TABLE attachments (
    id              uuid        PRIMARY KEY,
    conversation_id uuid        NOT NULL REFERENCES conversations (id) ON DELETE CASCADE,
    uploader_id     uuid        REFERENCES users (id) ON DELETE SET NULL,
    size            bigint      NOT NULL CHECK (size > 0),
    created_at      timestamptz NOT NULL
);
CREATE INDEX attachments_conversation_idx ON attachments (conversation_id);

-- +goose Down
DROP TABLE attachments;
