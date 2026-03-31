ALTER TABLE answers ADD COLUMN user_id UUID REFERENCES users(id);

CREATE INDEX idx_answers_user_id ON answers(user_id);
