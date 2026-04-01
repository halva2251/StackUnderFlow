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
	*Repository
}

func NewQuestionRepository(pool *pgxpool.Pool) *QuestionRepository {
	return &QuestionRepository{Repository: NewRepository(pool)}
}

// Create creates a new question using the provided Querier (transaction or pool).
func (r *QuestionRepository) Create(ctx context.Context, q Querier, userID, title, body string) (*model.Question, error) {
	return r.CreateTx(ctx, q, userID, title, body)
}

func (r *QuestionRepository) CreateTx(ctx context.Context, q Querier, userID, title, body string) (*model.Question, error) {
	var question model.Question
	err := q.QueryRow(ctx,
		`INSERT INTO questions (user_id, title, body)
		 VALUES ($1, $2, $3)
		 RETURNING id, user_id, title, body, upvotes, downvotes, created_at`,
		userID, title, body,
	).Scan(&question.ID, &question.UserID, &question.Title, &question.Body, &question.Upvotes, &question.Downvotes, &question.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("create question: %w", err)
	}
	return &question, nil
}

// GetByID retrieves a question by ID using the provided Querier (transaction or pool).
func (r *QuestionRepository) GetByID(ctx context.Context, q Querier, id string) (*model.Question, error) {
	return r.GetByIDTx(ctx, q, id)
}

// GetByIDForUpdate retrieves a question by ID with a row-level lock (SELECT ... FOR UPDATE).
// Must be called within an active transaction.
func (r *QuestionRepository) GetByIDForUpdate(ctx context.Context, q Querier, id string) (*model.Question, error) {
	var question model.Question
	err := q.QueryRow(ctx,
		`SELECT id, user_id, title, body, upvotes, downvotes, created_at
		 FROM questions WHERE id = $1 FOR UPDATE`,
		id,
	).Scan(&question.ID, &question.UserID, &question.Title, &question.Body, &question.Upvotes, &question.Downvotes, &question.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get question for update: %w", err)
	}
	return &question, nil
}

func (r *QuestionRepository) GetByIDTx(ctx context.Context, q Querier, id string) (*model.Question, error) {
	var question model.Question
	err := q.QueryRow(ctx,
		`SELECT id, user_id, title, body, upvotes, downvotes, created_at
		 FROM questions WHERE id = $1`,
		id,
	).Scan(&question.ID, &question.UserID, &question.Title, &question.Body, &question.Upvotes, &question.Downvotes, &question.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get question: %w", err)
	}
	return &question, nil
}

// List retrieves questions sorted by the given order, with pagination.
func (r *QuestionRepository) List(ctx context.Context, q Querier, sort string, limit, offset int) ([]model.Question, int, error) {
	// Get total count
	var total int
	err := q.QueryRow(ctx, "SELECT COUNT(*) FROM questions").Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count questions: %w", err)
	}

	// Get paginated questions — sort is already validated at service layer
	var rows pgx.Rows
	if sort == "most_upvoted" {
		rows, err = q.Query(ctx,
			`SELECT id, user_id, title, body, upvotes, downvotes, created_at
			 FROM questions
			 ORDER BY upvotes DESC, created_at DESC
			 LIMIT $1 OFFSET $2`,
			limit, offset,
		)
	} else {
		rows, err = q.Query(ctx,
			`SELECT id, user_id, title, body, upvotes, downvotes, created_at
			 FROM questions
			 ORDER BY created_at DESC
			 LIMIT $1 OFFSET $2`,
			limit, offset,
		)
	}
	if err != nil {
		return nil, 0, fmt.Errorf("list questions: %w", err)
	}
	defer rows.Close()

	questions := make([]model.Question, 0)
	for rows.Next() {
		var question model.Question
		if err := rows.Scan(&question.ID, &question.UserID, &question.Title, &question.Body, &question.Upvotes, &question.Downvotes, &question.CreatedAt); err != nil {
			return nil, 0, fmt.Errorf("scan question: %w", err)
		}
		questions = append(questions, question)
	}

	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate questions: %w", err)
	}

	return questions, total, nil
}
