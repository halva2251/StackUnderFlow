//go:build integration

package repository

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/halva2251/stackunderflow/internal/database"
)

func TestQuestionRepository_Create_Success(t *testing.T) {
	// Arrange
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL not set, skipping integration test")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := database.Connect(ctx, databaseURL)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer pool.Close()

	userRepo := NewUserRepository(pool)
	questionRepo := NewQuestionRepository(pool)

	// Create a user first
	username := "testuser_question_" + time.Now().Format("20060102150405")
	user, err := userRepo.Create(ctx, "local", "", username, "test@example.com", "hashedpassword")
	if err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	title := "Test Question Title " + time.Now().Format("20060102150405")
	body := "This is the body of the test question."

	// Act
	question, err := questionRepo.Create(ctx, pool, user.ID, title, body)

	// Assert
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if question == nil {
		t.Fatal("expected question, got nil")
	}
	if question.UserID != user.ID {
		t.Errorf("expected user_id %s, got %s", user.ID, question.UserID)
	}
	if question.Title != title {
		t.Errorf("expected title %s, got %s", title, question.Title)
	}
	if question.Body != body {
		t.Errorf("expected body %s, got %s", body, question.Body)
	}

	// Cleanup
	_, _ = pool.Exec(ctx, "DELETE FROM questions WHERE id = $1", question.ID)
	_, _ = pool.Exec(ctx, "DELETE FROM users WHERE id = $1", user.ID)
}

func TestQuestionRepository_Create_ForeignKeyViolation(t *testing.T) {
	// Arrange
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL not set, skipping integration test")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := database.Connect(ctx, databaseURL)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer pool.Close()

	questionRepo := NewQuestionRepository(pool)

	// Act - Try to create question with non-existent user
	_, err = questionRepo.Create(ctx, pool, "00000000-0000-0000-0000-000000000000", "Test Title", "Test Body")

	// Assert
	if err == nil {
		t.Fatal("expected error for foreign key violation, got nil")
	}
}

func TestQuestionRepository_GetByID_Success(t *testing.T) {
	// Arrange
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL not set, skipping integration test")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := database.Connect(ctx, databaseURL)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer pool.Close()

	userRepo := NewUserRepository(pool)
	questionRepo := NewQuestionRepository(pool)

	// Create a user and question
	username := "testuser_getq_" + time.Now().Format("20060102150405")
	user, err := userRepo.Create(ctx, "local", "", username, "test@example.com", "hashedpassword")
	if err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	title := "Get Question Test " + time.Now().Format("20060102150405")
	body := "This is the question body."
	created, err := questionRepo.Create(ctx, pool, user.ID, title, body)
	if err != nil {
		t.Fatalf("failed to create question: %v", err)
	}

	// Act
	question, err := questionRepo.GetByID(ctx, pool, created.ID)

	// Assert
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if question == nil {
		t.Fatal("expected question, got nil")
	}
	if question.ID != created.ID {
		t.Errorf("expected id %s, got %s", created.ID, question.ID)
	}
	if question.UserID != user.ID {
		t.Errorf("expected user_id %s, got %s", user.ID, question.UserID)
	}
	if question.Title != title {
		t.Errorf("expected title %s, got %s", title, question.Title)
	}
	if question.Body != body {
		t.Errorf("expected body %s, got %s", body, question.Body)
	}

	// Cleanup
	_, _ = pool.Exec(ctx, "DELETE FROM questions WHERE id = $1", created.ID)
	_, _ = pool.Exec(ctx, "DELETE FROM users WHERE id = $1", user.ID)
}

func TestQuestionRepository_GetByID_NotFound(t *testing.T) {
	// Arrange
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL not set, skipping integration test")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := database.Connect(ctx, databaseURL)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer pool.Close()

	questionRepo := NewQuestionRepository(pool)

	// Act
	question, err := questionRepo.GetByID(ctx, pool, "00000000-0000-0000-0000-000000000000")

	// Assert — GetByID returns ErrNotFound for non-existent IDs
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got: %v", err)
	}
	if question != nil {
		t.Errorf("expected nil, got question: %+v", question)
	}
}
