package service

import (
	"context"
	"errors"
	"testing"

	"github.com/halva2251/stackunderflow/internal/model"
)

// mockAIClient implements ai.Client for testing
type mockAIClient struct {
	response string
	err      error
}

func (m *mockAIClient) GenerateAnswer(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	return m.response, m.err
}

// mockQuestionRepo implements QuestionRepo for testing
type mockQuestionRepo struct {
	question *model.Question
	err      error
}

func (m *mockQuestionRepo) Create(ctx context.Context, userID, title, body string) (*model.Question, error) {
	if m.err != nil {
		return nil, m.err
	}
	return &model.Question{
		ID:     "q-123",
		UserID: userID,
		Title:  title,
		Body:   body,
	}, nil
}

func (m *mockQuestionRepo) GetByID(ctx context.Context, id string) (*model.Question, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.question, nil
}

// mockAnswerRepo implements AnswerRepo for testing
type mockAnswerRepo struct {
	answers  []model.Answer
	maxDepth int
	err      error
}

func (m *mockAnswerRepo) Create(ctx context.Context, questionID string, depth int, userPrompt, aiResponse string) (*model.Answer, error) {
	if m.err != nil {
		return nil, m.err
	}
	return &model.Answer{
		ID:         "a-123",
		QuestionID: questionID,
		Depth:      depth,
		UserPrompt: userPrompt,
		AIResponse: aiResponse,
	}, nil
}

func (m *mockAnswerRepo) GetByQuestionID(ctx context.Context, questionID string) ([]model.Answer, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.answers, nil
}

func (m *mockAnswerRepo) GetMaxDepth(ctx context.Context, questionID string) (int, error) {
	if m.err != nil {
		return 0, m.err
	}
	return m.maxDepth, nil
}

func (m *mockAnswerRepo) GetByID(ctx context.Context, id string) (*model.Answer, error) {
	if m.err != nil {
		return nil, m.err
	}
	if len(m.answers) > 0 {
		return &m.answers[0], nil
	}
	return nil, nil
}

func TestCreateQuestion_Success(t *testing.T) {
	// Arrange
	svc := NewQuestionService(
		&mockQuestionRepo{},
		&mockAnswerRepo{},
		&mockAIClient{response: "Just use recursion!"},
	)

	// Act
	result, err := svc.CreateQuestion(context.Background(), "user-1", "How to sort?", "How do I sort a list?")

	// Assert
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Question.Title != "How to sort?" {
		t.Errorf("expected title 'How to sort?', got %s", result.Question.Title)
	}
	if len(result.Answers) != 1 {
		t.Fatalf("expected 1 answer, got %d", len(result.Answers))
	}
	if result.Answers[0].AIResponse != "Just use recursion!" {
		t.Errorf("expected AI response 'Just use recursion!', got %s", result.Answers[0].AIResponse)
	}
	if result.Answers[0].Depth != 0 {
		t.Errorf("expected depth 0, got %d", result.Answers[0].Depth)
	}
}

func TestCreateQuestion_EmptyTitle(t *testing.T) {
	// Arrange
	svc := NewQuestionService(
		&mockQuestionRepo{},
		&mockAnswerRepo{},
		&mockAIClient{response: "whatever"},
	)

	// Act
	_, err := svc.CreateQuestion(context.Background(), "user-1", "", "some body")

	// Assert
	if err == nil {
		t.Fatal("expected validation error")
	}
	if !errors.Is(err, ErrValidation) {
		t.Errorf("expected ErrValidation, got %v", err)
	}
}

func TestCreateQuestion_EmptyBody(t *testing.T) {
	// Arrange
	svc := NewQuestionService(
		&mockQuestionRepo{},
		&mockAnswerRepo{},
		&mockAIClient{response: "whatever"},
	)

	// Act
	_, err := svc.CreateQuestion(context.Background(), "user-1", "title", "")

	// Assert
	if err == nil {
		t.Fatal("expected validation error")
	}
	if !errors.Is(err, ErrValidation) {
		t.Errorf("expected ErrValidation, got %v", err)
	}
}

func TestCreateQuestion_AIFailure(t *testing.T) {
	// Arrange
	svc := NewQuestionService(
		&mockQuestionRepo{},
		&mockAnswerRepo{},
		&mockAIClient{err: errors.New("groq api down")},
	)

	// Act
	_, err := svc.CreateQuestion(context.Background(), "user-1", "title", "body")

	// Assert
	if err == nil {
		t.Fatal("expected error when AI fails")
	}
}

func TestGetQuestion_Success(t *testing.T) {
	// Arrange
	question := &model.Question{ID: "q-1", UserID: "u-1", Title: "Test", Body: "Test body"}
	answers := []model.Answer{
		{ID: "a-1", QuestionID: "q-1", Depth: 0, AIResponse: "Wrong answer"},
	}

	svc := NewQuestionService(
		&mockQuestionRepo{question: question},
		&mockAnswerRepo{answers: answers},
		&mockAIClient{},
	)

	// Act
	result, err := svc.GetQuestion(context.Background(), "q-1")

	// Assert
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Question.ID != "q-1" {
		t.Errorf("expected question id q-1, got %s", result.Question.ID)
	}
	if len(result.Answers) != 1 {
		t.Errorf("expected 1 answer, got %d", len(result.Answers))
	}
}

func TestGetQuestion_NotFound(t *testing.T) {
	// Arrange
	svc := NewQuestionService(
		&mockQuestionRepo{question: nil},
		&mockAnswerRepo{},
		&mockAIClient{},
	)

	// Act
	_, err := svc.GetQuestion(context.Background(), "nonexistent")

	// Assert
	if err == nil {
		t.Fatal("expected error for nonexistent question")
	}
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestArgue_Success(t *testing.T) {
	// Arrange
	question := &model.Question{ID: "q-1", UserID: "u-1", Title: "Test", Body: "Test body"}
	answers := []model.Answer{
		{ID: "a-1", QuestionID: "q-1", Depth: 0, AIResponse: "Wrong answer"},
	}

	svc := NewQuestionService(
		&mockQuestionRepo{question: question},
		&mockAnswerRepo{answers: answers, maxDepth: 0},
		&mockAIClient{response: "I'm doubling down!"},
	)

	// Act
	result, err := svc.Argue(context.Background(), "q-1", "u-1", "That's wrong!")

	// Assert
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Depth != 1 {
		t.Errorf("expected depth 1, got %d", result.Depth)
	}
	if result.AIResponse != "I'm doubling down!" {
		t.Errorf("expected 'I'm doubling down!', got %s", result.AIResponse)
	}
	if result.UserPrompt != "That's wrong!" {
		t.Errorf("expected user prompt 'That's wrong!', got %s", result.UserPrompt)
	}
}

func TestArgue_EmptyArgument(t *testing.T) {
	// Arrange
	svc := NewQuestionService(
		&mockQuestionRepo{},
		&mockAnswerRepo{},
		&mockAIClient{},
	)

	// Act
	_, err := svc.Argue(context.Background(), "q-1", "u-1", "")

	// Assert
	if err == nil {
		t.Fatal("expected validation error")
	}
	if !errors.Is(err, ErrValidation) {
		t.Errorf("expected ErrValidation, got %v", err)
	}
}

func TestArgue_QuestionNotFound(t *testing.T) {
	// Arrange
	svc := NewQuestionService(
		&mockQuestionRepo{question: nil},
		&mockAnswerRepo{},
		&mockAIClient{},
	)

	// Act
	_, err := svc.Argue(context.Background(), "nonexistent", "u-1", "argument")

	// Assert
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestArgue_AIFailure(t *testing.T) {
	// Arrange
	question := &model.Question{ID: "q-1", UserID: "u-1", Title: "Test", Body: "Test body"}

	svc := NewQuestionService(
		&mockQuestionRepo{question: question},
		&mockAnswerRepo{maxDepth: 0},
		&mockAIClient{err: errors.New("api down")},
	)

	// Act
	_, err := svc.Argue(context.Background(), "q-1", "u-1", "argument")

	// Assert
	if err == nil {
		t.Fatal("expected error when AI fails")
	}
}

func TestArgue_DepthEscalation(t *testing.T) {
	tests := []struct {
		name          string
		currentMax    int
		expectedDepth int
	}{
		{name: "depth 0 to 1", currentMax: 0, expectedDepth: 1},
		{name: "depth 1 to 2", currentMax: 1, expectedDepth: 2},
		{name: "depth 2 to 3", currentMax: 2, expectedDepth: 3},
		{name: "depth 3 to 4", currentMax: 3, expectedDepth: 4},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			question := &model.Question{ID: "q-1", Title: "Test", Body: "Body"}
			answers := []model.Answer{
				{ID: "a-1", Depth: tt.currentMax, AIResponse: "previous answer"},
			}

			svc := NewQuestionService(
				&mockQuestionRepo{question: question},
				&mockAnswerRepo{answers: answers, maxDepth: tt.currentMax},
				&mockAIClient{response: "escalated response"},
			)

			// Act
			result, err := svc.Argue(context.Background(), "q-1", "u-1", "I disagree")

			// Assert
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result.Depth != tt.expectedDepth {
				t.Errorf("expected depth %d, got %d", tt.expectedDepth, result.Depth)
			}
		})
	}
}
