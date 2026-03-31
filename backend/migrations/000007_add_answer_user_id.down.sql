DROP INDEX IF EXISTS idx_answers_user_id;

ALTER TABLE answers DROP COLUMN IF EXISTS user_id;
