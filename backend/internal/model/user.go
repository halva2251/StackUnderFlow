package model

import "time"

type User struct {
	ID           string    `json:"id"`
	Provider     string    `json:"provider"`
	ProviderID   string    `json:"provider_id"`
	Username     string    `json:"username"`
	Email        string    `json:"email,omitempty"`
	AvatarURL    string    `json:"avatar_url,omitempty"`
	PasswordHash string    `json:"-"`
	CreatedAt    time.Time `json:"created_at"`
}
