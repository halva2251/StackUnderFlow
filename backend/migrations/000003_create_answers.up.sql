CREATE TABLE IF NOT EXISTS answers (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    question_id UUID NOT NULL REFERENCES questions(id) ON DELETE CASCADE,
    depth       INT NOT NULL DEFAULT 0,
    user_prompt TEXT,
    ai_response TEXT NOT NULL,
    upvotes     INT NOT NULL DEFAULT 0,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_answers_question_id ON answers(question_id);
CREATE INDEX idx_answers_upvotes ON answers(upvotes DESC);
