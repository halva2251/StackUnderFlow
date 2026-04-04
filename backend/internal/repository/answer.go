package repository

import (
	"context"
	"fmt"

	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/halva2251/stackunderflow/internal/model"
)

type AnswerRepository struct {
	*Repository
}

func NewAnswerRepository(pool *pgxpool.Pool) *AnswerRepository {
	return &AnswerRepository{Repository: NewRepository(pool)}
}

// Create creates a new answer using the provided Querier (transaction or pool).
// userID is the ID of the user who argued; empty string for AI-generated initial answers (depth 0).
func (r *AnswerRepository) Create(ctx context.Context, q Querier, questionID, userID string, depth int, userPrompt, aiResponse string) (*model.Answer, error) {
	var a model.Answer
	err := q.QueryRow(ctx,
		`INSERT INTO answers (question_id, user_id, depth, user_prompt, ai_response)
		 VALUES ($1, $2, $3, $4, $5)
		 RETURNING id, question_id, COALESCE(user_id::text, ''), depth, COALESCE(user_prompt, ''), ai_response, upvotes, downvotes, created_at`,
		questionID, nilIfEmpty(userID), depth, nilIfEmpty(userPrompt), aiResponse,
	).Scan(&a.ID, &a.QuestionID, &a.UserID, &a.Depth, &a.UserPrompt, &a.AIResponse, &a.Upvotes, &a.Downvotes, &a.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("create answer: %w", err)
	}
	return &a, nil
}

// GetByQuestionID retrieves answers by question ID using the provided Querier (transaction or pool).
func (r *AnswerRepository) GetByQuestionID(ctx context.Context, q Querier, questionID string) ([]model.Answer, error) {
	rows, err := q.Query(ctx,
		`SELECT id, question_id, COALESCE(user_id::text, ''), depth, COALESCE(user_prompt, ''), ai_response, upvotes, downvotes, created_at
		 FROM answers WHERE question_id = $1
		 ORDER BY depth ASC`,
		questionID,
	)
	if err != nil {
		return nil, fmt.Errorf("get answers: %w", err)
	}
	defer rows.Close()

	answers := make([]model.Answer, 0)
	for rows.Next() {
		var a model.Answer
		if err := rows.Scan(&a.ID, &a.QuestionID, &a.UserID, &a.Depth, &a.UserPrompt, &a.AIResponse, &a.Upvotes, &a.Downvotes, &a.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan answer: %w", err)
		}
		answers = append(answers, a)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate answers: %w", err)
	}

	return answers, nil
}

// GetMaxDepth retrieves the maximum depth of answers for a question using the provided Querier (transaction or pool).
func (r *AnswerRepository) GetMaxDepth(ctx context.Context, q Querier, questionID string) (int, error) {
	var depth int
	err := q.QueryRow(ctx,
		`SELECT COALESCE(MAX(depth), -1) FROM answers WHERE question_id = $1`,
		questionID,
	).Scan(&depth)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return -1, nil
		}
		return 0, fmt.Errorf("get max depth: %w", err)
	}
	return depth, nil
}

// GetByID retrieves an answer by ID using the provided Querier (transaction or pool).
func (r *AnswerRepository) GetByID(ctx context.Context, q Querier, id string) (*model.Answer, error) {
	var a model.Answer
	err := q.QueryRow(ctx,
		`SELECT id, question_id, COALESCE(user_id::text, ''), depth, COALESCE(user_prompt, ''), ai_response, upvotes, downvotes, created_at
		 FROM answers WHERE id = $1`,
		id,
	).Scan(&a.ID, &a.QuestionID, &a.UserID, &a.Depth, &a.UserPrompt, &a.AIResponse, &a.Upvotes, &a.Downvotes, &a.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get answer: %w", err)
	}
	return &a, nil
}
