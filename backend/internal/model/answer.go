package model

import "time"

type Answer struct {
	ID         string    `json:"id"`
	QuestionID string    `json:"question_id"`
	Depth      int       `json:"depth"`
	UserPrompt string    `json:"user_prompt,omitempty"`
	AIResponse string    `json:"ai_response"`
	Upvotes    int       `json:"upvotes"`
	CreatedAt  time.Time `json:"created_at"`
}
