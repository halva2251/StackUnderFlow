package model

import "time"

type Answer struct {
	ID         string    `json:"id"`
	QuestionID string    `json:"question_id"`
	UserID     string    `json:"user_id,omitempty"`
	Depth      int       `json:"depth"`
	UserPrompt string    `json:"user_prompt,omitempty"`
	AIResponse string    `json:"ai_response"`
	Upvotes    int       `json:"upvotes"`
	Downvotes  int       `json:"downvotes"`
	CreatedAt  time.Time `json:"created_at"`
}
