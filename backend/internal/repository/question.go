package repository

import (
	"context"
	"fmt"

	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/halva2251/stackunderflow/internal/model"
)

type QuestionRepository struct {
	pool *pgxpool.Pool
}

func NewQuestionRepository(pool *pgxpool.Pool) *QuestionRepository {
	return &QuestionRepository{pool: pool}
}

func (r *QuestionRepository) Create(ctx context.Context, userID, title, body string) (*model.Question, error) {
	var q model.Question
	err := r.pool.QueryRow(ctx,
		`INSERT INTO questions (user_id, title, body)
		 VALUES ($1, $2, $3)
		 RETURNING id, user_id, title, body, created_at`,
		userID, title, body,
	).Scan(&q.ID, &q.UserID, &q.Title, &q.Body, &q.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("create question: %w", err)
	}
	return &q, nil
}

func (r *QuestionRepository) GetByID(ctx context.Context, id string) (*model.Question, error) {
	var q model.Question
	err := r.pool.QueryRow(ctx,
		`SELECT id, user_id, title, body, created_at
		 FROM questions WHERE id = $1`,
		id,
	).Scan(&q.ID, &q.UserID, &q.Title, &q.Body, &q.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("get question: %w", err)
	}
	return &q, nil
}
