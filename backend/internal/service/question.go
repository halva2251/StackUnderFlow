package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/halva2251/stackunderflow/internal/ai"
	"github.com/halva2251/stackunderflow/internal/model"
)

var (
	ErrNotFound        = errors.New("not found")
	ErrValidation      = errors.New("validation error")
	ErrQuestionMissing = fmt.Errorf("question %w", ErrNotFound)
)

// QuestionRepo defines the data access the service needs for questions.
type QuestionRepo interface {
	Create(ctx context.Context, userID, title, body string) (*model.Question, error)
	GetByID(ctx context.Context, id string) (*model.Question, error)
}

// AnswerRepo defines the data access the service needs for answers.
type AnswerRepo interface {
	Create(ctx context.Context, questionID string, depth int, userPrompt, aiResponse string) (*model.Answer, error)
	GetByQuestionID(ctx context.Context, questionID string) ([]model.Answer, error)
	GetMaxDepth(ctx context.Context, questionID string) (int, error)
	GetByID(ctx context.Context, id string) (*model.Answer, error)
}

type QuestionService struct {
	questions QuestionRepo
	answers   AnswerRepo
	aiClient  ai.Client
}

func NewQuestionService(questions QuestionRepo, answers AnswerRepo, aiClient ai.Client) *QuestionService {
	return &QuestionService{
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

	question, err := s.questions.Create(ctx, userID, title, body)
	if err != nil {
		return nil, fmt.Errorf("create question: %w", err)
	}

	// Generate the initial confidently wrong answer (depth 0)
	systemPrompt := ai.GetSystemPrompt(0)
	userPrompt := fmt.Sprintf("Question: %s\n\n%s", title, body)

	aiResponse, err := s.aiClient.GenerateAnswer(ctx, systemPrompt, userPrompt)
	if err != nil {
		return nil, fmt.Errorf("generate answer: %w", err)
	}

	answer, err := s.answers.Create(ctx, question.ID, 0, "", aiResponse)
	if err != nil {
		return nil, fmt.Errorf("save answer: %w", err)
	}

	return &model.QuestionResponse{
		Question: *question,
		Answers:  []model.Answer{*answer},
	}, nil
}

func (s *QuestionService) GetQuestion(ctx context.Context, id string) (*model.QuestionResponse, error) {
	question, err := s.questions.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get question: %w", err)
	}
	if question == nil {
		return nil, ErrQuestionMissing
	}

	answers, err := s.answers.GetByQuestionID(ctx, id)
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

	question, err := s.questions.GetByID(ctx, questionID)
	if err != nil {
		return nil, fmt.Errorf("get question: %w", err)
	}
	if question == nil {
		return nil, ErrQuestionMissing
	}

	maxDepth, err := s.answers.GetMaxDepth(ctx, questionID)
	if err != nil {
		return nil, fmt.Errorf("get max depth: %w", err)
	}

	nextDepth := maxDepth + 1
	systemPrompt := ai.GetSystemPrompt(nextDepth)

	// Build context: original question + last AI answer + user's argument
	answers, err := s.answers.GetByQuestionID(ctx, questionID)
	if err != nil {
		return nil, fmt.Errorf("get answers: %w", err)
	}

	var lastAIResponse string
	if len(answers) > 0 {
		lastAIResponse = answers[len(answers)-1].AIResponse
	}

	userPrompt := fmt.Sprintf(
		"Original question: %s\n\n%s\n\nYour previous answer: %s\n\nUser's response: %s",
		question.Title, question.Body, lastAIResponse, argument,
	)

	aiResponse, err := s.aiClient.GenerateAnswer(ctx, systemPrompt, userPrompt)
	if err != nil {
		return nil, fmt.Errorf("generate answer: %w", err)
	}

	answer, err := s.answers.Create(ctx, questionID, nextDepth, argument, aiResponse)
	if err != nil {
		return nil, fmt.Errorf("save answer: %w", err)
	}

	return answer, nil
}
