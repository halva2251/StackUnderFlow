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

func TestAnswerRepository_Create_Success(t *testing.T) {
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
	answerRepo := NewAnswerRepository(pool)

	// Create user and question
	username := "testuser_answer_" + time.Now().Format("20060102150405")
	user, err := userRepo.Create(ctx, "local", "", username, "test@example.com", "hashedpassword")
	if err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	question, err := questionRepo.Create(ctx, pool, user.ID, "Test Question", "Question body")
	if err != nil {
		t.Fatalf("failed to create question: %v", err)
	}

	// Begin transaction for answer creation
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("failed to begin transaction: %v", err)
	}
	defer tx.Rollback(ctx)

	userPrompt := "Can you help me with this?"
	aiResponse := "Here is the solution..."

	// Act
	answer, err := answerRepo.Create(ctx, tx, question.ID, 0, userPrompt, aiResponse)

	// Assert
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if answer == nil {
		t.Fatal("expected answer, got nil")
	}
	if answer.QuestionID != question.ID {
		t.Errorf("expected question_id %s, got %s", question.ID, answer.QuestionID)
	}
	if answer.Depth != 0 {
		t.Errorf("expected depth 0, got %d", answer.Depth)
	}
	if answer.UserPrompt != userPrompt {
		t.Errorf("expected user_prompt %s, got %s", userPrompt, answer.UserPrompt)
	}
	if answer.AIResponse != aiResponse {
		t.Errorf("expected ai_response %s, got %s", aiResponse, answer.AIResponse)
	}

	// Cleanup
	tx.Rollback(ctx)
	_, _ = pool.Exec(ctx, "DELETE FROM answers WHERE question_id = $1", question.ID)
	_, _ = pool.Exec(ctx, "DELETE FROM questions WHERE id = $1", question.ID)
	_, _ = pool.Exec(ctx, "DELETE FROM users WHERE id = $1", user.ID)
}

func TestAnswerRepository_Create_ForeignKeyViolation(t *testing.T) {
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

	answerRepo := NewAnswerRepository(pool)

	// Begin transaction
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("failed to begin transaction: %v", err)
	}
	defer tx.Rollback(ctx)

	// Act - Try to create answer with non-existent question
	_, err = answerRepo.Create(ctx, tx, "00000000-0000-0000-0000-000000000000", 0, "Prompt", "Response")

	// Assert
	if err == nil {
		t.Fatal("expected error for foreign key violation, got nil")
	}
}

func TestAnswerRepository_GetByQuestionID_Success(t *testing.T) {
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
	answerRepo := NewAnswerRepository(pool)

	// Create user and question
	username := "testuser_geta_" + time.Now().Format("20060102150405")
	user, err := userRepo.Create(ctx, "local", "", username, "test@example.com", "hashedpassword")
	if err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	question, err := questionRepo.Create(ctx, pool, user.ID, "Test Question", "Question body")
	if err != nil {
		t.Fatalf("failed to create question: %v", err)
	}

	// Create multiple answers within a transaction
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("failed to begin transaction: %v", err)
	}

	answer1, err := answerRepo.Create(ctx, tx, question.ID, 0, "First prompt", "First response")
	if err != nil {
		tx.Rollback(ctx)
		t.Fatalf("failed to create first answer: %v", err)
	}
	answer2, err := answerRepo.Create(ctx, tx, question.ID, 1, "Second prompt", "Second response")
	if err != nil {
		tx.Rollback(ctx)
		t.Fatalf("failed to create second answer: %v", err)
	}

	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("failed to commit transaction: %v", err)
	}

	// Act
	tx2, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("failed to begin transaction: %v", err)
	}
	defer tx2.Rollback(ctx)

	answers, err := answerRepo.GetByQuestionID(ctx, tx2, question.ID)

	// Assert
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(answers) != 2 {
		t.Fatalf("expected 2 answers, got %d", len(answers))
	}
	// Verify ordering by depth
	if answers[0].ID != answer1.ID {
		t.Errorf("expected first answer id %s, got %s", answer1.ID, answers[0].ID)
	}
	if answers[1].ID != answer2.ID {
		t.Errorf("expected second answer id %s, got %s", answer2.ID, answers[1].ID)
	}
	if answers[0].Depth != 0 {
		t.Errorf("expected first answer depth 0, got %d", answers[0].Depth)
	}
	if answers[1].Depth != 1 {
		t.Errorf("expected second answer depth 1, got %d", answers[1].Depth)
	}

	// Cleanup
	_, _ = pool.Exec(ctx, "DELETE FROM answers WHERE question_id = $1", question.ID)
	_, _ = pool.Exec(ctx, "DELETE FROM questions WHERE id = $1", question.ID)
	_, _ = pool.Exec(ctx, "DELETE FROM users WHERE id = $1", user.ID)
}

func TestAnswerRepository_GetByQuestionID_Ordering(t *testing.T) {
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
	answerRepo := NewAnswerRepository(pool)

	// Create user and question
	username := "testuser_order_" + time.Now().Format("20060102150405")
	user, err := userRepo.Create(ctx, "local", "", username, "test@example.com", "hashedpassword")
	if err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	question, err := questionRepo.Create(ctx, pool, user.ID, "Test Question", "Question body")
	if err != nil {
		t.Fatalf("failed to create question: %v", err)
	}

	// Create answers out of order (depth 2, then 0, then 1)
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("failed to begin transaction: %v", err)
	}

	answerDepth2, err := answerRepo.Create(ctx, tx, question.ID, 2, "Depth 2 prompt", "Depth 2 response")
	if err != nil {
		tx.Rollback(ctx)
		t.Fatalf("failed to create depth 2 answer: %v", err)
	}
	answerDepth0, err := answerRepo.Create(ctx, tx, question.ID, 0, "Depth 0 prompt", "Depth 0 response")
	if err != nil {
		tx.Rollback(ctx)
		t.Fatalf("failed to create depth 0 answer: %v", err)
	}
	answerDepth1, err := answerRepo.Create(ctx, tx, question.ID, 1, "Depth 1 prompt", "Depth 1 response")
	if err != nil {
		tx.Rollback(ctx)
		t.Fatalf("failed to create depth 1 answer: %v", err)
	}

	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("failed to commit transaction: %v", err)
	}

	// Act
	tx2, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("failed to begin transaction: %v", err)
	}
	defer tx2.Rollback(ctx)

	answers, err := answerRepo.GetByQuestionID(ctx, tx2, question.ID)

	// Assert - Should be ordered by depth ascending
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(answers) != 3 {
		t.Fatalf("expected 3 answers, got %d", len(answers))
	}
	if answers[0].ID != answerDepth0.ID {
		t.Errorf("expected first answer (depth 0) id %s, got %s", answerDepth0.ID, answers[0].ID)
	}
	if answers[1].ID != answerDepth1.ID {
		t.Errorf("expected second answer (depth 1) id %s, got %s", answerDepth1.ID, answers[1].ID)
	}
	if answers[2].ID != answerDepth2.ID {
		t.Errorf("expected third answer (depth 2) id %s, got %s", answerDepth2.ID, answers[2].ID)
	}

	// Cleanup
	_, _ = pool.Exec(ctx, "DELETE FROM answers WHERE question_id = $1", question.ID)
	_, _ = pool.Exec(ctx, "DELETE FROM questions WHERE id = $1", question.ID)
	_, _ = pool.Exec(ctx, "DELETE FROM users WHERE id = $1", user.ID)
}

func TestAnswerRepository_GetByQuestionID_Empty(t *testing.T) {
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
	answerRepo := NewAnswerRepository(pool)

	// Create user and question (no answers)
	username := "testuser_empty_" + time.Now().Format("20060102150405")
	user, err := userRepo.Create(ctx, "local", "", username, "test@example.com", "hashedpassword")
	if err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	question, err := questionRepo.Create(ctx, pool, user.ID, "Test Question", "Question body")
	if err != nil {
		t.Fatalf("failed to create question: %v", err)
	}

	// Act
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("failed to begin transaction: %v", err)
	}
	defer tx.Rollback(ctx)

	answers, err := answerRepo.GetByQuestionID(ctx, tx, question.ID)

	// Assert
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if answers == nil {
		t.Fatal("expected empty slice, got nil")
	}
	if len(answers) != 0 {
		t.Errorf("expected 0 answers, got %d", len(answers))
	}

	// Cleanup
	_, _ = pool.Exec(ctx, "DELETE FROM questions WHERE id = $1", question.ID)
	_, _ = pool.Exec(ctx, "DELETE FROM users WHERE id = $1", user.ID)
}

func TestAnswerRepository_GetMaxDepth_WithAnswers(t *testing.T) {
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
	answerRepo := NewAnswerRepository(pool)

	// Create user and question
	username := "testuser_depth_" + time.Now().Format("20060102150405")
	user, err := userRepo.Create(ctx, "local", "", username, "test@example.com", "hashedpassword")
	if err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	question, err := questionRepo.Create(ctx, pool, user.ID, "Test Question", "Question body")
	if err != nil {
		t.Fatalf("failed to create question: %v", err)
	}

	// Create answers at different depths
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("failed to begin transaction: %v", err)
	}

	_, err = answerRepo.Create(ctx, tx, question.ID, 0, "Depth 0", "Response 0")
	if err != nil {
		tx.Rollback(ctx)
		t.Fatalf("failed to create depth 0 answer: %v", err)
	}
	_, err = answerRepo.Create(ctx, tx, question.ID, 2, "Depth 2", "Response 2")
	if err != nil {
		tx.Rollback(ctx)
		t.Fatalf("failed to create depth 2 answer: %v", err)
	}
	_, err = answerRepo.Create(ctx, tx, question.ID, 1, "Depth 1", "Response 1")
	if err != nil {
		tx.Rollback(ctx)
		t.Fatalf("failed to create depth 1 answer: %v", err)
	}

	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("failed to commit transaction: %v", err)
	}

	// Act
	tx2, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("failed to begin transaction: %v", err)
	}
	defer tx2.Rollback(ctx)

	maxDepth, err := answerRepo.GetMaxDepth(ctx, tx2, question.ID)

	// Assert
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if maxDepth != 2 {
		t.Errorf("expected max depth 2, got %d", maxDepth)
	}

	// Cleanup
	_, _ = pool.Exec(ctx, "DELETE FROM answers WHERE question_id = $1", question.ID)
	_, _ = pool.Exec(ctx, "DELETE FROM questions WHERE id = $1", question.ID)
	_, _ = pool.Exec(ctx, "DELETE FROM users WHERE id = $1", user.ID)
}

func TestAnswerRepository_GetMaxDepth_NoAnswers(t *testing.T) {
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
	answerRepo := NewAnswerRepository(pool)

	// Create user and question (no answers)
	username := "testuser_nodepth_" + time.Now().Format("20060102150405")
	user, err := userRepo.Create(ctx, "local", "", username, "test@example.com", "hashedpassword")
	if err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	question, err := questionRepo.Create(ctx, pool, user.ID, "Test Question", "Question body")
	if err != nil {
		t.Fatalf("failed to create question: %v", err)
	}

	// Act
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("failed to begin transaction: %v", err)
	}
	defer tx.Rollback(ctx)

	maxDepth, err := answerRepo.GetMaxDepth(ctx, tx, question.ID)

	// Assert
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if maxDepth != -1 {
		t.Errorf("expected max depth -1 (no answers), got %d", maxDepth)
	}

	// Cleanup
	_, _ = pool.Exec(ctx, "DELETE FROM questions WHERE id = $1", question.ID)
	_, _ = pool.Exec(ctx, "DELETE FROM users WHERE id = $1", user.ID)
}

func TestAnswerRepository_GetByID_Success(t *testing.T) {
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
	answerRepo := NewAnswerRepository(pool)

	// Create user, question, and answer
	username := "testuser_getbyid_" + time.Now().Format("20060102150405")
	user, err := userRepo.Create(ctx, "local", "", username, "test@example.com", "hashedpassword")
	if err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	question, err := questionRepo.Create(ctx, pool, user.ID, "Test Question", "Question body")
	if err != nil {
		t.Fatalf("failed to create question: %v", err)
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("failed to begin transaction: %v", err)
	}

	created, err := answerRepo.Create(ctx, tx, question.ID, 0, "Test prompt", "Test response")
	if err != nil {
		tx.Rollback(ctx)
		t.Fatalf("failed to create answer: %v", err)
	}

	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("failed to commit transaction: %v", err)
	}

	// Act
	tx2, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("failed to begin transaction: %v", err)
	}
	defer tx2.Rollback(ctx)

	answer, err := answerRepo.GetByID(ctx, tx2, created.ID)

	// Assert
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if answer == nil {
		t.Fatal("expected answer, got nil")
	}
	if answer.ID != created.ID {
		t.Errorf("expected id %s, got %s", created.ID, answer.ID)
	}
	if answer.QuestionID != question.ID {
		t.Errorf("expected question_id %s, got %s", question.ID, answer.QuestionID)
	}
	if answer.Depth != 0 {
		t.Errorf("expected depth 0, got %d", answer.Depth)
	}
	if answer.UserPrompt != "Test prompt" {
		t.Errorf("expected user_prompt 'Test prompt', got %s", answer.UserPrompt)
	}
	if answer.AIResponse != "Test response" {
		t.Errorf("expected ai_response 'Test response', got %s", answer.AIResponse)
	}

	// Cleanup
	_, _ = pool.Exec(ctx, "DELETE FROM answers WHERE id = $1", created.ID)
	_, _ = pool.Exec(ctx, "DELETE FROM questions WHERE id = $1", question.ID)
	_, _ = pool.Exec(ctx, "DELETE FROM users WHERE id = $1", user.ID)
}

func TestAnswerRepository_GetByID_NotFound(t *testing.T) {
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

	answerRepo := NewAnswerRepository(pool)

	// Act
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("failed to begin transaction: %v", err)
	}
	defer tx.Rollback(ctx)

	answer, err := answerRepo.GetByID(ctx, tx, "00000000-0000-0000-0000-000000000000")

	// Assert
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
	if answer != nil {
		t.Errorf("expected nil answer, got %+v", answer)
	}
}
