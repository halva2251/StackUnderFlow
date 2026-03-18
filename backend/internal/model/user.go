package model

import "time"

type User struct {
	ID         string    `json:"id"`
	Provider   string    `json:"provider"`
	ProviderID string    `json:"provider_id"`
	Username   string    `json:"username"`
	AvatarURL  string    `json:"avatar_url,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
}
