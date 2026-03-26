-- Drop single-column index made redundant by the composite index below
DROP INDEX IF EXISTS idx_answers_question_id;

-- Composite index on answers for GetMaxDepth query
CREATE INDEX idx_answers_question_depth ON answers(question_id, depth);

-- Index on users.username for login lookups
CREATE INDEX idx_users_username ON users(username);

-- Better composite index for answers with ordering
CREATE INDEX idx_answers_question_created ON answers(question_id, created_at DESC);
