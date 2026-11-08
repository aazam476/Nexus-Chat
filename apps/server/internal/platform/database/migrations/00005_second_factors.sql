-- +goose Up
-- Second factors (spec §4.1): TOTP apps, FIDO2 security keys and passkeys,
-- and single-use backup codes. A user may have several of each.

-- TOTP secrets are encrypted with the server's TOTP key (AES-256-GCM)
-- before they are stored, so a database leak alone does not reveal them.
CREATE TABLE totp_credentials (
    id               uuid        PRIMARY KEY,
    user_id          uuid        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    name             text        NOT NULL CHECK (char_length(name) BETWEEN 1 AND 64),
    encrypted_secret bytea       NOT NULL,
    -- NULL until the user proves the app works by entering a code.
    confirmed_at     timestamptz,
    -- The last 30-second step accepted, so one code cannot be used twice.
    last_used_step   bigint      NOT NULL DEFAULT 0,
    created_at       timestamptz NOT NULL
);

CREATE INDEX totp_credentials_user_id_idx ON totp_credentials (user_id);

-- Only SHA-256 hashes of backup codes are stored. A used code is deleted.
CREATE TABLE backup_codes (
    user_id    uuid  NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    code_hash  bytea NOT NULL CHECK (octet_length(code_hash) = 32),
    PRIMARY KEY (user_id, code_hash)
);

-- A login that passed the password check but still needs a second factor.
ALTER TABLE account_tokens DROP CONSTRAINT account_tokens_purpose_check;
ALTER TABLE account_tokens ADD CONSTRAINT account_tokens_purpose_check
    CHECK (purpose IN ('verify_email', 'reset_password', 'mfa_login'));

-- +goose Down
ALTER TABLE account_tokens DROP CONSTRAINT account_tokens_purpose_check;
DELETE FROM account_tokens WHERE purpose = 'mfa_login';
ALTER TABLE account_tokens ADD CONSTRAINT account_tokens_purpose_check
    CHECK (purpose IN ('verify_email', 'reset_password'));
DROP TABLE backup_codes;
DROP TABLE totp_credentials;
