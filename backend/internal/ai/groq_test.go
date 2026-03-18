package ai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestGroqClient_GenerateAnswer_Success(t *testing.T) {
	// Arrange
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/chat/completions" {
			t.Errorf("expected /chat/completions, got %s", r.URL.Path)
		}
		if auth := r.Header.Get("Authorization"); auth != "Bearer test-key" {
			t.Errorf("expected Bearer test-key, got %s", auth)
		}

		var req chatRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		if req.Model != "test-model" {
			t.Errorf("expected test-model, got %s", req.Model)
		}
		if len(req.Messages) != 2 {
			t.Fatalf("expected 2 messages, got %d", len(req.Messages))
		}

		resp := chatResponse{
			Choices: []chatChoice{
				{Message: chatMessage{Role: "assistant", Content: "Just use recursion!"}},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewGroqClient("test-key", server.URL, "test-model")

	// Act
	result, err := client.GenerateAnswer(context.Background(), "system prompt", "user question")

	// Assert
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "Just use recursion!" {
		t.Errorf("expected 'Just use recursion!', got %s", result)
	}
}

func TestGroqClient_GenerateAnswer_APIError(t *testing.T) {
	// Arrange
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		w.Write([]byte(`{"error": {"message": "rate limit exceeded", "type": "rate_limit_error"}}`))
	}))
	defer server.Close()

	client := NewGroqClient("test-key", server.URL, "test-model")

	// Act
	_, err := client.GenerateAnswer(context.Background(), "system", "user")

	// Assert
	if err == nil {
		t.Fatal("expected error for 429 response")
	}
}

func TestGroqClient_GenerateAnswer_EmptyChoices(t *testing.T) {
	// Arrange
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := chatResponse{Choices: []chatChoice{}}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewGroqClient("test-key", server.URL, "test-model")

	// Act
	_, err := client.GenerateAnswer(context.Background(), "system", "user")

	// Assert
	if err == nil {
		t.Fatal("expected error for empty choices")
	}
}

func TestGroqClient_GenerateAnswer_Timeout(t *testing.T) {
	// Arrange
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
	}))
	defer server.Close()

	client := NewGroqClient("test-key", server.URL, "test-model")

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// Act
	_, err := client.GenerateAnswer(ctx, "system", "user")

	// Assert
	if err == nil {
		t.Fatal("expected timeout error")
	}
}

func TestGroqClient_GenerateAnswer_InvalidJSON(t *testing.T) {
	// Arrange
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`not json at all`))
	}))
	defer server.Close()

	client := NewGroqClient("test-key", server.URL, "test-model")

	// Act
	_, err := client.GenerateAnswer(context.Background(), "system", "user")

	// Assert
	if err == nil {
		t.Fatal("expected error for invalid JSON response")
	}
}
