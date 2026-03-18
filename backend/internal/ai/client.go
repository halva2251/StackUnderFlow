package ai

import "context"

// Client generates AI responses. Implementations can target different
// providers (Groq, self-hosted model, etc.) without changing callers.
type Client interface {
	GenerateAnswer(ctx context.Context, systemPrompt, userPrompt string) (string, error)
}
