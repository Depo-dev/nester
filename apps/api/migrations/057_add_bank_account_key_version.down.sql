DROP INDEX IF EXISTS idx_bank_accounts_key_version;
ALTER TABLE bank_accounts DROP COLUMN IF EXISTS key_version;
