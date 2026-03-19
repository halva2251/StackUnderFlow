DROP INDEX IF EXISTS idx_users_local_username;
DROP INDEX IF EXISTS idx_users_email;

ALTER TABLE users
    DROP COLUMN IF EXISTS email,
    DROP COLUMN IF EXISTS password_hash;
