-- Records which encryption key version sealed account_number_encrypted so keys
-- can be rotated without rewriting historical rows. Rows created before key
-- versioning were sealed with the original (v1) key, so the column defaults to
-- 'v1' and existing data keeps decrypting after this migration is applied.
ALTER TABLE bank_accounts
    ADD COLUMN key_version VARCHAR(32) NOT NULL DEFAULT 'v1';

-- Lets the rotation tool cheaply find rows that are not yet on the active key
-- version (WHERE key_version <> $active).
CREATE INDEX idx_bank_accounts_key_version ON bank_accounts (key_version);
