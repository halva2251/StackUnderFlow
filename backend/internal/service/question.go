package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/halva2251/stackunderflow/internal/ai"
	"github.com/halva2251/stackunderflow/internal/model"
	"github.com/halva2251/stackunderflow/internal/repository"
)

var (
	ErrNotFound        = errors.New("not found")
	ErrValidation      = errors.New("validation error")
	ErrQuestionMissing = fmt.Errorf("question %w", ErrNotFound)
)

const (
	maxTitleLen   = 300
	maxBodyLen    = 10000
	maxArgLen     = 5000
	maxArgueDepth = 3
)

// TxBeginner can begin a database transaction. *pgxpool.Pool satisfies this interface.
type TxBeginner interface {
	Begin(ctx context.Context) (repository.Tx, error)
}

// pgxTxBeginner adapts *pgxpool.Pool to TxBeginner.
type pgxTxBeginner struct {
	pool *pgxpool.Pool
}

// NewTxBeginner wraps a *pgxpool.Pool so it satisfies TxBeginner.
func NewTxBeginner(pool *pgxpool.Pool) TxBeginner {
	return &pgxTxBeginner{pool: pool}
}

func (b *pgxTxBeginner) Begin(ctx context.Context) (repository.Tx, error) {
	tx, err := b.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	return tx, nil
}

// QuestionRepo defines the data access the service needs for questions.
type QuestionRepo interface {
	Create(ctx context.Context, q repository.Querier, userID, title, body string) (*model.Question, error)
	GetByID(ctx context.Context, q repository.Querier, id string) (*model.Question, error)
	GetByIDForUpdate(ctx context.Context, q repository.Querier, id string) (*model.Question, error)
}

// AnswerRepo defines the data access the service needs for answers.
type AnswerRepo interface {
	Create(ctx context.Context, q repository.Querier, questionID, userID string, depth int, userPrompt, aiResponse string) (*model.Answer, error)
	GetByQuestionID(ctx context.Context, q repository.Querier, questionID string) ([]model.Answer, error)
	GetMaxDepth(ctx context.Context, q repository.Querier, questionID string) (int, error)
	GetByID(ctx context.Context, q repository.Querier, id string) (*model.Answer, error)
}

type QuestionService struct {
	db        TxBeginner
	questions QuestionRepo
	answers   AnswerRepo
	aiClient  ai.Client
}

func NewQuestionService(db TxBeginner, questions QuestionRepo, answers AnswerRepo, aiClient ai.Client) *QuestionService {
	return &QuestionService{
		db:        db,
		questions: questions,
		answers:   answers,
		aiClient:  aiClient,
	}
}

func (s *QuestionService) CreateQuestion(ctx context.Context, userID, title, body string) (*model.QuestionResponse, error) {
	if title == "" {
		return nil, fmt.Errorf("title is required: %w", ErrValidation)
	}
	if body == "" {
		return nil, fmt.Errorf("body is required: %w", ErrValidation)
	}
	if len(title) > maxTitleLen {
		return nil, fmt.Errorf("title too long (max %d chars): %w", maxTitleLen, ErrValidation)
	}
	if len(body) > maxBodyLen {
		return nil, fmt.Errorf("body too long (max %d chars): %w", maxBodyLen, ErrValidation)
	}

	// Generate AI response OUTSIDE the transaction (external service call)
	systemPrompt := ai.GetSystemPrompt(0)
	userPrompt := fmt.Sprintf("Question: %s\n\n%s", title, body)

	aiResponse, err := s.aiClient.GenerateAnswer(ctx, systemPrompt, userPrompt)
	if err != nil {
		return nil, fmt.Errorf("generate answer: %w", err)
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin transaction: %w", err)
	}
	defer repository.RollbackTx(ctx, tx)

	question, err := s.questions.Create(ctx, tx, userID, title, body)
	if err != nil {
		return nil, fmt.Errorf("create question: %w", err)
	}

	answer, err := s.answers.Create(ctx, tx, question.ID, "", 0, "", aiResponse)
	if err != nil {
		return nil, fmt.Errorf("save answer: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit transaction: %w", err)
	}

	return &model.QuestionResponse{
		Question: *question,
		Answers:  []model.Answer{*answer},
	}, nil
}

func (s *QuestionService) GetQuestion(ctx context.Context, id string) (*model.QuestionResponse, error) {
	// Use a non-transactional pool read via a nil-tx compatible querier.
	// We begin a read transaction here to reuse the same TxBeginner interface.
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin transaction: %w", err)
	}
	defer repository.RollbackTx(ctx, tx)

	question, err := s.questions.GetByID(ctx, tx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrQuestionMissing
		}
		return nil, fmt.Errorf("get question: %w", err)
	}

	answers, err := s.answers.GetByQuestionID(ctx, tx, id)
	if err != nil {
		return nil, fmt.Errorf("get answers: %w", err)
	}

	return &model.QuestionResponse{
		Question: *question,
		Answers:  answers,
	}, nil
}

func (s *QuestionService) Argue(ctx context.Context, questionID, userID, argument string) (*model.Answer, error) {
	if argument == "" {
		return nil, fmt.Errorf("argument is required: %w", ErrValidation)
	}
	if len(argument) > maxArgLen {
		return nil, fmt.Errorf("argument too long (max %d chars): %w", maxArgLen, ErrValidation)
	}

	// Phase 1: Read transaction — lock the question row with SELECT FOR UPDATE to
	// prevent concurrent argue races writing at the same depth. Commit before
	// calling the external AI service to keep the lock duration short.
	readTx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin read transaction: %w", err)
	}
	defer repository.RollbackTx(ctx, readTx)

	// Lock the row so no concurrent request can read the same maxDepth
	// and then write a duplicate depth entry.
	question, err := s.questions.GetByIDForUpdate(ctx, readTx, questionID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrQuestionMissing
		}
		return nil, fmt.Errorf("get question for update: %w", err)
	}

	maxDepth, err := s.answers.GetMaxDepth(ctx, readTx, questionID)
	if err != nil {
		return nil, fmt.Errorf("get max depth: %w", err)
	}

	// Enforce the maximum argue depth before doing any more work
	if maxDepth >= maxArgueDepth {
		return nil, fmt.Errorf("maximum argue depth reached: %w", ErrValidation)
	}

	nextDepth := maxDepth + 1

	// Build context from existing answers; only the last AI response is needed
	answers, err := s.answers.GetByQuestionID(ctx, readTx, questionID)
	if err != nil {
		return nil, fmt.Errorf("get answers: %w", err)
	}

	var lastAIResponse string
	if len(answers) > 0 {
		lastAIResponse = answers[len(answers)-1].AIResponse
	}

	// Commit the read transaction before calling the external AI service.
	// Holding the transaction open across a network call would keep the row
	// lock for the full duration of the AI request, blocking other operations.
	if err := readTx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit read transaction: %w", err)
	}

	// Phase 2: Call AI outside any transaction
	systemPrompt := ai.GetSystemPrompt(nextDepth)
	userPrompt := fmt.Sprintf(
		"Original question: %s\n\n%s\n\nYour previous answer: %s\n\nUser's response: %s",
		question.Title, question.Body, lastAIResponse, argument,
	)

	aiResponse, err := s.aiClient.GenerateAnswer(ctx, systemPrompt, userPrompt)
	if err != nil {
		return nil, fmt.Errorf("generate answer: %w", err)
	}

	// Phase 3: Short write transaction — persist the new answer only
	writeTx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin write transaction: %w", err)
	}
	defer repository.RollbackTx(ctx, writeTx)

	answer, err := s.answers.Create(ctx, writeTx, questionID, userID, nextDepth, argument, aiResponse)
	if err != nil {
		return nil, fmt.Errorf("save answer: %w", err)
	}

	if err := writeTx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit write transaction: %w", err)
	}

	return answer, nil
}
