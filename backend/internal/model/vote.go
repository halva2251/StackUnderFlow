package model

import "time"

type Vote struct {
	ID         string    `json:"id"`
	UserID     string    `json:"user_id"`
	TargetType string    `json:"target_type"`
	TargetID   string    `json:"target_id"`
	Value      int       `json:"value"`
	CreatedAt  time.Time `json:"created_at"`
}
