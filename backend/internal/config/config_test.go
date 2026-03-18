package config

import (
	"testing"
)

func TestLoad_Defaults(t *testing.T) {
	// Arrange
	t.Setenv("GROQ_API_KEY", "test-key")
	t.Setenv("JWT_SECRET", "test-secret")

	// Act
	cfg, err := Load()

	// Assert
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Port != 8080 {
		t.Errorf("expected port 8080, got %d", cfg.Port)
	}
	if cfg.GroqBaseURL != "https://api.groq.com/openai/v1" {
		t.Errorf("expected default groq base url, got %s", cfg.GroqBaseURL)
	}
	if cfg.GroqModel != "llama-3.3-70b-versatile" {
		t.Errorf("expected default groq model, got %s", cfg.GroqModel)
	}
}

func TestLoad_OverridesFromEnv(t *testing.T) {
	// Arrange
	t.Setenv("GROQ_API_KEY", "test-key")
	t.Setenv("JWT_SECRET", "test-secret")
	t.Setenv("PORT", "9090")
	t.Setenv("DATABASE_URL", "postgres://custom:custom@localhost/testdb")
	t.Setenv("GROQ_MODEL", "custom-model")

	// Act
	cfg, err := Load()

	// Assert
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Port != 9090 {
		t.Errorf("expected port 9090, got %d", cfg.Port)
	}
	if cfg.DatabaseURL != "postgres://custom:custom@localhost/testdb" {
		t.Errorf("expected custom database url, got %s", cfg.DatabaseURL)
	}
	if cfg.GroqModel != "custom-model" {
		t.Errorf("expected custom-model, got %s", cfg.GroqModel)
	}
}

func TestLoad_MissingRequiredFields(t *testing.T) {
	tests := []struct {
		name    string
		setEnv  func(t *testing.T)
		wantErr bool
	}{
		{
			name: "missing GROQ_API_KEY",
			setEnv: func(t *testing.T) {
				t.Setenv("JWT_SECRET", "test-secret")
			},
			wantErr: true,
		},
		{
			name: "missing JWT_SECRET",
			setEnv: func(t *testing.T) {
				t.Setenv("GROQ_API_KEY", "test-key")
			},
			wantErr: true,
		},
		{
			name: "all required fields set",
			setEnv: func(t *testing.T) {
				t.Setenv("GROQ_API_KEY", "test-key")
				t.Setenv("JWT_SECRET", "test-secret")
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			tt.setEnv(t)

			// Act
			_, err := Load()

			// Assert
			if (err != nil) != tt.wantErr {
				t.Errorf("Load() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestLoad_OptionalOAuthFields(t *testing.T) {
	// Arrange
	t.Setenv("GROQ_API_KEY", "test-key")
	t.Setenv("JWT_SECRET", "test-secret")
	t.Setenv("GITHUB_CLIENT_ID", "gh-id")
	t.Setenv("GITHUB_CLIENT_SECRET", "gh-secret")
	t.Setenv("DISCORD_CLIENT_ID", "dc-id")
	t.Setenv("DISCORD_CLIENT_SECRET", "dc-secret")

	// Act
	cfg, err := Load()

	// Assert
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.GitHubClientID != "gh-id" {
		t.Errorf("expected gh-id, got %s", cfg.GitHubClientID)
	}
	if cfg.GitHubClientSecret != "gh-secret" {
		t.Errorf("expected gh-secret, got %s", cfg.GitHubClientSecret)
	}
	if cfg.DiscordClientID != "dc-id" {
		t.Errorf("expected dc-id, got %s", cfg.DiscordClientID)
	}
	if cfg.DiscordClientSecret != "dc-secret" {
		t.Errorf("expected dc-secret, got %s", cfg.DiscordClientSecret)
	}
}
