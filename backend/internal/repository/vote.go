package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/halva2251/stackunderflow/internal/model"
)

type VoteRepository struct {
	*Repository
}

func NewVoteRepository(pool *pgxpool.Pool) *VoteRepository {
	return &VoteRepository{Repository: NewRepository(pool)}
}

// Upsert creates a new vote or updates the value of an existing one.
func (r *VoteRepository) Upsert(ctx context.Context, q Querier, userID, targetType, targetID string, value int) (*model.Vote, error) {
	var v model.Vote
	err := q.QueryRow(ctx,
		`INSERT INTO votes (user_id, target_type, target_id, value)
		 VALUES ($1, $2, $3, $4)
		 ON CONFLICT (user_id, target_type, target_id)
		 DO UPDATE SET value = EXCLUDED.value
		 RETURNING id, user_id, target_type, target_id, value, created_at`,
		userID, targetType, targetID, value,
	).Scan(&v.ID, &v.UserID, &v.TargetType, &v.TargetID, &v.Value, &v.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("upsert vote: %w", err)
	}
	return &v, nil
}

// Delete removes a vote. Returns ErrNotFound if no vote exists.
func (r *VoteRepository) Delete(ctx context.Context, q Querier, userID, targetType, targetID string) error {
	tag, err := q.Exec(ctx,
		`DELETE FROM votes
		 WHERE user_id = $1 AND target_type = $2 AND target_id = $3`,
		userID, targetType, targetID,
	)
	if err != nil {
		return fmt.Errorf("delete vote: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// GetByUserAndTarget retrieves a user's vote on a specific target.
func (r *VoteRepository) GetByUserAndTarget(ctx context.Context, q Querier, userID, targetType, targetID string) (*model.Vote, error) {
	var v model.Vote
	err := q.QueryRow(ctx,
		`SELECT id, user_id, target_type, target_id, value, created_at
		 FROM votes
		 WHERE user_id = $1 AND target_type = $2 AND target_id = $3`,
		userID, targetType, targetID,
	).Scan(&v.ID, &v.UserID, &v.TargetType, &v.TargetID, &v.Value, &v.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get vote: %w", err)
	}
	return &v, nil
}
