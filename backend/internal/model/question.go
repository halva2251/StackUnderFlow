package model

import "time"

type Question struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	Title     string    `json:"title"`
	Body      string    `json:"body"`
	Upvotes   int       `json:"upvotes"`
	Downvotes int       `json:"downvotes"`
	CreatedAt time.Time `json:"created_at"`
}
