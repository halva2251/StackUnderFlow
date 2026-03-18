package model

import "time"

type Upvote struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	AnswerID  string    `json:"answer_id"`
	CreatedAt time.Time `json:"created_at"`
}
