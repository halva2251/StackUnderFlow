DROP TABLE IF EXISTS votes;

ALTER TABLE answers DROP COLUMN IF EXISTS downvotes;

ALTER TABLE questions DROP COLUMN IF EXISTS downvotes;
ALTER TABLE questions DROP COLUMN IF EXISTS upvotes;

-- Recreate original upvotes table
CREATE TABLE IF NOT EXISTS upvotes (
    id        UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id   UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    answer_id UUID NOT NULL REFERENCES answers(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(user_id, answer_id)
);

CREATE INDEX idx_upvotes_answer_id ON upvotes(answer_id);
