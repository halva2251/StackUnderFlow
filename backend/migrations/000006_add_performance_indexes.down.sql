DROP INDEX IF EXISTS idx_answers_question_created;
DROP INDEX IF EXISTS idx_users_username;
DROP INDEX IF EXISTS idx_answers_question_depth;
CREATE INDEX IF NOT EXISTS idx_answers_question_id ON answers(question_id);
