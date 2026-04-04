package service

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/halva2251/stackunderflow/internal/model"
	"github.com/halva2251/stackunderflow/internal/repository"
)

// --- Mock transaction ---

type mockTx struct {
	commitErr error
}

func (m *mockTx) QueryRow(_ context.Context, _ string, _ ...any) pgx.Row {
	return errRow{err: pgx.ErrNoRows}
}
func (m *mockTx) Query(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
	return nil, nil
}
func (m *mockTx) Exec(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, nil
}
func (m *mockTx) Commit(_ context.Context) error   { return m.commitErr }
func (m *mockTx) Rollback(_ context.Context) error { return nil }

// --- Mock TxBeginner ---

type mockTxBeginner struct {
	tx  repository.Tx
	err error
}

func (m *mockTxBeginner) Begin(_ context.Context) (repository.Tx, error) {
	if m.err != nil {
		return nil, m.err
	}
	if m.tx != nil {
		return m.tx, nil
	}
	return &mockTx{}, nil
}

func (m *mockTxBeginner) Pool() repository.Querier {
	return &mockTx{}
}

// --- Mock AI client ---

type mockAIClient struct {
	response string
	err      error
}

func (m *mockAIClient) GenerateAnswer(_ context.Context, _, _ string) (string, error) {
	return m.response, m.err
}

// --- Mock QuestionRepo ---

type mockQuestionRepo struct {
	question      *model.Question
	questions     []model.Question
	listTotal     int
	err           error
}

func (m *mockQuestionRepo) Create(_ context.Context, _ repository.Querier, userID, title, body string) (*model.Question, error) {
	if m.err != nil {
		return nil, m.err
	}
	return &model.Question{ID: "q-123", UserID: userID, Title: title, Body: body}, nil
}

func (m *mockQuestionRepo) GetByID(_ context.Context, _ repository.Querier, _ string) (*model.Question, error) {
	if m.err != nil {
		return nil, m.err
	}
	if m.question == nil {
		return nil, repository.ErrNotFound
	}
	return m.question, nil
}

func (m *mockQuestionRepo) GetByIDForUpdate(_ context.Context, _ repository.Querier, _ string) (*model.Question, error) {
	if m.err != nil {
		return nil, m.err
	}
	if m.question == nil {
		return nil, repository.ErrNotFound
	}
	return m.question, nil
}

func (m *mockQuestionRepo) List(_ context.Context, _ repository.Querier, _ string, _, _ int) ([]model.Question, int, error) {
	if m.err != nil {
		return nil, 0, m.err
	}
	if m.questions == nil {
		return []model.Question{}, m.listTotal, nil
	}
	return m.questions, m.listTotal, nil
}

// --- Mock AnswerRepo ---

type mockAnswerRepo struct {
	answers  []model.Answer
	maxDepth int
	err      error
}

func (m *mockAnswerRepo) Create(_ context.Context, _ repository.Querier, questionID, userID string, depth int, userPrompt, aiResponse string) (*model.Answer, error) {
	if m.err != nil {
		return nil, m.err
	}
	return &model.Answer{
		ID:         "a-123",
		QuestionID: questionID,
		UserID:     userID,
		Depth:      depth,
		UserPrompt: userPrompt,
		AIResponse: aiResponse,
	}, nil
}

func (m *mockAnswerRepo) GetByQuestionID(_ context.Context, _ repository.Querier, _ string) ([]model.Answer, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.answers, nil
}

func (m *mockAnswerRepo) GetMaxDepth(_ context.Context, _ repository.Querier, _ string) (int, error) {
	if m.err != nil {
		return 0, m.err
	}
	return m.maxDepth, nil
}

func (m *mockAnswerRepo) GetByID(_ context.Context, _ repository.Querier, _ string) (*model.Answer, error) {
	if m.err != nil {
		return nil, m.err
	}
	if len(m.answers) > 0 {
		return &m.answers[0], nil
	}
	return nil, repository.ErrNotFound
}

// --- Helpers ---

func newSvc(db TxBeginner, q QuestionRepo, a AnswerRepo, ai *mockAIClient) *QuestionService {
	return NewQuestionService(db, q, a, ai)
}

// --- Tests: CreateQuestion ---

func TestCreateQuestion_Success(t *testing.T) {
	svc := newSvc(&mockTxBeginner{}, &mockQuestionRepo{}, &mockAnswerRepo{}, &mockAIClient{response: "Just use recursion!"})

	result, err := svc.CreateQuestion(context.Background(), "user-1", "How to sort?", "How do I sort a list?")

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
		t.Errorf("unexpected AI response: %s", result.Answers[0].AIResponse)
	}
	if result.Answers[0].Depth != 0 {
		t.Errorf("expected depth 0, got %d", result.Answers[0].Depth)
	}
}

func TestCreateQuestion_EmptyTitle(t *testing.T) {
	svc := newSvc(&mockTxBeginner{}, &mockQuestionRepo{}, &mockAnswerRepo{}, &mockAIClient{})

	_, err := svc.CreateQuestion(context.Background(), "user-1", "", "some body")

	if err == nil {
		t.Fatal("expected validation error")
	}
	if !errors.Is(err, ErrValidation) {
		t.Errorf("expected ErrValidation, got %v", err)
	}
}

func TestCreateQuestion_EmptyBody(t *testing.T) {
	svc := newSvc(&mockTxBeginner{}, &mockQuestionRepo{}, &mockAnswerRepo{}, &mockAIClient{})

	_, err := svc.CreateQuestion(context.Background(), "user-1", "title", "")

	if err == nil {
		t.Fatal("expected validation error")
	}
	if !errors.Is(err, ErrValidation) {
		t.Errorf("expected ErrValidation, got %v", err)
	}
}

func TestCreateQuestion_TitleTooLong(t *testing.T) {
	title := string(make([]byte, maxTitleLen+1))
	svc := newSvc(&mockTxBeginner{}, &mockQuestionRepo{}, &mockAnswerRepo{}, &mockAIClient{})

	_, err := svc.CreateQuestion(context.Background(), "user-1", title, "body")

	if !errors.Is(err, ErrValidation) {
		t.Errorf("expected ErrValidation for long title, got %v", err)
	}
}

func TestCreateQuestion_BodyTooLong(t *testing.T) {
	body := string(make([]byte, maxBodyLen+1))
	svc := newSvc(&mockTxBeginner{}, &mockQuestionRepo{}, &mockAnswerRepo{}, &mockAIClient{})

	_, err := svc.CreateQuestion(context.Background(), "user-1", "title", body)

	if !errors.Is(err, ErrValidation) {
		t.Errorf("expected ErrValidation for long body, got %v", err)
	}
}

func TestCreateQuestion_AIFailure(t *testing.T) {
	svc := newSvc(&mockTxBeginner{}, &mockQuestionRepo{}, &mockAnswerRepo{}, &mockAIClient{err: errors.New("groq api down")})

	_, err := svc.CreateQuestion(context.Background(), "user-1", "title", "body")

	if err == nil {
		t.Fatal("expected error when AI fails")
	}
}

// --- Tests: GetQuestion ---

func TestGetQuestion_Success(t *testing.T) {
	question := &model.Question{ID: "q-1", UserID: "u-1", Title: "Test", Body: "Test body"}
	answers := []model.Answer{{ID: "a-1", QuestionID: "q-1", Depth: 0, AIResponse: "Wrong answer"}}

	svc := newSvc(&mockTxBeginner{}, &mockQuestionRepo{question: question}, &mockAnswerRepo{answers: answers}, &mockAIClient{})

	result, err := svc.GetQuestion(context.Background(), "q-1")

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
	svc := newSvc(&mockTxBeginner{}, &mockQuestionRepo{question: nil}, &mockAnswerRepo{}, &mockAIClient{})

	_, err := svc.GetQuestion(context.Background(), "nonexistent")

	if err == nil {
		t.Fatal("expected error for nonexistent question")
	}
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

// --- Tests: Argue ---

func TestArgue_Success(t *testing.T) {
	question := &model.Question{ID: "q-1", UserID: "u-1", Title: "Test", Body: "Test body"}
	answers := []model.Answer{{ID: "a-1", QuestionID: "q-1", Depth: 0, AIResponse: "Wrong answer"}}

	svc := newSvc(&mockTxBeginner{}, &mockQuestionRepo{question: question}, &mockAnswerRepo{answers: answers, maxDepth: 0}, &mockAIClient{response: "I'm doubling down!"})

	result, err := svc.Argue(context.Background(), "q-1", "u-1", "That's wrong!")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Depth != 1 {
		t.Errorf("expected depth 1, got %d", result.Depth)
	}
	if result.AIResponse != "I'm doubling down!" {
		t.Errorf("unexpected AI response: %s", result.AIResponse)
	}
}

func TestArgue_EmptyArgument(t *testing.T) {
	svc := newSvc(&mockTxBeginner{}, &mockQuestionRepo{}, &mockAnswerRepo{}, &mockAIClient{})

	_, err := svc.Argue(context.Background(), "q-1", "u-1", "")

	if !errors.Is(err, ErrValidation) {
		t.Errorf("expected ErrValidation, got %v", err)
	}
}

func TestArgue_ArgumentTooLong(t *testing.T) {
	arg := string(make([]byte, maxArgLen+1))
	svc := newSvc(&mockTxBeginner{}, &mockQuestionRepo{}, &mockAnswerRepo{}, &mockAIClient{})

	_, err := svc.Argue(context.Background(), "q-1", "u-1", arg)

	if !errors.Is(err, ErrValidation) {
		t.Errorf("expected ErrValidation for long argument, got %v", err)
	}
}

func TestArgue_QuestionNotFound(t *testing.T) {
	svc := newSvc(&mockTxBeginner{}, &mockQuestionRepo{question: nil}, &mockAnswerRepo{}, &mockAIClient{})

	_, err := svc.Argue(context.Background(), "nonexistent", "u-1", "argument")

	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestArgue_MaxDepthReached(t *testing.T) {
	question := &model.Question{ID: "q-1", Title: "Test", Body: "Body"}
	svc := newSvc(&mockTxBeginner{}, &mockQuestionRepo{question: question}, &mockAnswerRepo{maxDepth: maxArgueDepth}, &mockAIClient{})

	_, err := svc.Argue(context.Background(), "q-1", "u-1", "push further")

	if !errors.Is(err, ErrValidation) {
		t.Errorf("expected ErrValidation at max depth, got %v", err)
	}
}

func TestArgue_AIFailure(t *testing.T) {
	question := &model.Question{ID: "q-1", UserID: "u-1", Title: "Test", Body: "Test body"}

	svc := newSvc(&mockTxBeginner{}, &mockQuestionRepo{question: question}, &mockAnswerRepo{maxDepth: 0}, &mockAIClient{err: errors.New("api down")})

	_, err := svc.Argue(context.Background(), "q-1", "u-1", "argument")

	if err == nil {
		t.Fatal("expected error when AI fails")
	}
}

func TestArgue_DepthEscalation(t *testing.T) {
	tests := []struct {
		name       string
		currentMax int
		wantDepth  int
	}{
		{"depth 0 to 1", 0, 1},
		{"depth 1 to 2", 1, 2},
		{"depth 2 to 3", 2, 3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			question := &model.Question{ID: "q-1", Title: "Test", Body: "Body"}
			answers := []model.Answer{{ID: "a-1", Depth: tt.currentMax, AIResponse: "previous"}}

			svc := newSvc(
				&mockTxBeginner{},
				&mockQuestionRepo{question: question},
				&mockAnswerRepo{answers: answers, maxDepth: tt.currentMax},
				&mockAIClient{response: "escalated response"},
			)

			result, err := svc.Argue(context.Background(), "q-1", "u-1", "I disagree")

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result.Depth != tt.wantDepth {
				t.Errorf("expected depth %d, got %d", tt.wantDepth, result.Depth)
			}
		})
	}
}

// --- Tests: ListQuestions ---

func TestListQuestions_Success(t *testing.T) {
	questions := []model.Question{
		{ID: "q-1", Title: "First", Body: "Body 1"},
		{ID: "q-2", Title: "Second", Body: "Body 2"},
	}
	svc := newSvc(&mockTxBeginner{}, &mockQuestionRepo{questions: questions, listTotal: 10}, &mockAnswerRepo{}, &mockAIClient{})

	result, err := svc.ListQuestions(context.Background(), "newest", 2, 0)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Questions) != 2 {
		t.Errorf("expected 2 questions, got %d", len(result.Questions))
	}
	if result.Total != 10 {
		t.Errorf("expected total 10, got %d", result.Total)
	}
	if result.Limit != 2 {
		t.Errorf("expected limit 2, got %d", result.Limit)
	}
	if result.Offset != 0 {
		t.Errorf("expected offset 0, got %d", result.Offset)
	}
	if result.Sort != "newest" {
		t.Errorf("expected sort 'newest', got %s", result.Sort)
	}
}

func TestListQuestions_MostUpvoted(t *testing.T) {
	questions := []model.Question{
		{ID: "q-1", Title: "Top", Body: "Body", Upvotes: 5},
	}
	svc := newSvc(&mockTxBeginner{}, &mockQuestionRepo{questions: questions, listTotal: 1}, &mockAnswerRepo{}, &mockAIClient{})

	result, err := svc.ListQuestions(context.Background(), "most_upvoted", 10, 5)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Questions) != 1 {
		t.Errorf("expected 1 question, got %d", len(result.Questions))
	}
	if result.Sort != "most_upvoted" {
		t.Errorf("expected sort 'most_upvoted', got %s", result.Sort)
	}
}

func TestListQuestions_Empty(t *testing.T) {
	svc := newSvc(&mockTxBeginner{}, &mockQuestionRepo{questions: []model.Question{}}, &mockAnswerRepo{}, &mockAIClient{})

	result, err := svc.ListQuestions(context.Background(), "newest", 10, 0)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Questions) != 0 {
		t.Errorf("expected 0 questions, got %d", len(result.Questions))
	}
}

func TestListQuestions_InvalidSort(t *testing.T) {
	svc := newSvc(&mockTxBeginner{}, &mockQuestionRepo{}, &mockAnswerRepo{}, &mockAIClient{})

	_, err := svc.ListQuestions(context.Background(), "invalid", 10, 0)

	if !errors.Is(err, ErrValidation) {
		t.Errorf("expected ErrValidation, got %v", err)
	}
}

func TestListQuestions_LimitTooLarge(t *testing.T) {
	svc := newSvc(&mockTxBeginner{}, &mockQuestionRepo{}, &mockAnswerRepo{}, &mockAIClient{})

	_, err := svc.ListQuestions(context.Background(), "newest", 101, 0)

	if !errors.Is(err, ErrValidation) {
		t.Errorf("expected ErrValidation for large limit, got %v", err)
	}
}

func TestListQuestions_NegativeOffset(t *testing.T) {
	svc := newSvc(&mockTxBeginner{}, &mockQuestionRepo{}, &mockAnswerRepo{}, &mockAIClient{})

	_, err := svc.ListQuestions(context.Background(), "newest", 10, -1)

	if !errors.Is(err, ErrValidation) {
		t.Errorf("expected ErrValidation for negative offset, got %v", err)
	}
}
