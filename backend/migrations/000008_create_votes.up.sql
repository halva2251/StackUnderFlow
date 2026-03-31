-- Drop old answer-only upvotes table
DROP TABLE IF EXISTS upvotes;

-- Add vote counter columns to questions
ALTER TABLE questions ADD COLUMN upvotes INT NOT NULL DEFAULT 0;
ALTER TABLE questions ADD COLUMN downvotes INT NOT NULL DEFAULT 0;

-- Add downvotes column to answers (upvotes already exists from migration 000003)
ALTER TABLE answers ADD COLUMN downvotes INT NOT NULL DEFAULT 0;

-- Create unified votes table supporting both questions and answers
CREATE TABLE IF NOT EXISTS votes (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    target_type TEXT NOT NULL CHECK (target_type IN ('question', 'answer')),
    target_id   UUID NOT NULL,
    value       INT NOT NULL CHECK (value IN (1, -1)),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(user_id, target_type, target_id)
);

CREATE INDEX idx_votes_target ON votes(target_type, target_id);
