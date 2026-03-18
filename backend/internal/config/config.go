package config

import (
	"fmt"

	"github.com/caarlos0/env/v11"
)

type Config struct {
	Port        int    `env:"PORT"         envDefault:"8080"`
	DatabaseURL string `env:"DATABASE_URL" envDefault:"postgres://postgres:postgres@localhost:5432/stackunderflow?sslmode=disable"`

	GroqAPIKey  string `env:"GROQ_API_KEY,required"`
	GroqBaseURL string `env:"GROQ_BASE_URL" envDefault:"https://api.groq.com/openai/v1"`
	GroqModel   string `env:"GROQ_MODEL"    envDefault:"llama-3.3-70b-versatile"`

	JWTSecret string `env:"JWT_SECRET,required"`

	GitHubClientID     string `env:"GITHUB_CLIENT_ID"`
	GitHubClientSecret string `env:"GITHUB_CLIENT_SECRET"`

	DiscordClientID     string `env:"DISCORD_CLIENT_ID"`
	DiscordClientSecret string `env:"DISCORD_CLIENT_SECRET"`
}

func Load() (*Config, error) {
	cfg, err := env.ParseAs[Config]()
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}
	return &cfg, nil
}
