package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/halva2251/stackunderflow/internal/model"
	"github.com/halva2251/stackunderflow/internal/repository"
)

// VoteRepo defines the data access the service needs for votes.
type VoteRepo interface {
	Upsert(ctx context.Context, q repository.Querier, userID, targetType, targetID string, value int) (*model.Vote, error)
	Delete(ctx context.Context, q repository.Querier, userID, targetType, targetID string) error
	GetByUserAndTarget(ctx context.Context, q repository.Querier, userID, targetType, targetID string) (*model.Vote, error)
}

type VoteService struct {
	db    TxBeginner
	votes VoteRepo
}

func NewVoteService(db TxBeginner, votes VoteRepo) *VoteService {
	return &VoteService{db: db, votes: votes}
}

// Vote casts or changes a vote on a question or answer.
func (s *VoteService) Vote(ctx context.Context, userID, targetType, targetID string, value int) (*model.VoteResponse, error) {
	if err := validateTargetType(targetType); err != nil {
		return nil, err
	}
	if value != 1 && value != -1 {
		return nil, fmt.Errorf("value must be 1 or -1: %w", ErrValidation)
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin transaction: %w", err)
	}
	defer repository.RollbackTx(ctx, tx)

	// Lock the target row to serialize concurrent votes on the same target.
	if err := lockTarget(ctx, tx, targetType, targetID); err != nil {
		return nil, err
	}

	// Check for existing vote to calculate counter deltas
	oldVote, err := s.votes.GetByUserAndTarget(ctx, tx, userID, targetType, targetID)
	if err != nil && !errors.Is(err, repository.ErrNotFound) {
		return nil, fmt.Errorf("get existing vote: %w", err)
	}

	// Calculate counter deltas
	var upDelta, downDelta int
	if oldVote == nil {
		// New vote
		if value == 1 {
			upDelta = 1
		} else {
			downDelta = 1
		}
	} else {
		// If voting the same way, fetch and return current counts (idempotent)
		if oldVote.Value == value {
			up, down, err := getTargetCounts(ctx, tx, targetType, targetID)
			if err != nil {
				return nil, err
			}
			return &model.VoteResponse{Upvotes: up, Downvotes: down}, nil
		}

		// Changed vote: undo old, apply new
		if oldVote.Value == 1 {
			upDelta = -1
		} else {
			downDelta = -1
		}
		if value == 1 {
			upDelta++
		} else {
			downDelta++
		}
	}

	// Upsert the vote
	_, err = s.votes.Upsert(ctx, tx, userID, targetType, targetID, value)
	if err != nil {
		return nil, fmt.Errorf("upsert vote: %w", err)
	}

	up, down, err := updateCounters(ctx, tx, targetType, targetID, upDelta, downDelta)
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit transaction: %w", err)
	}

	return &model.VoteResponse{Upvotes: up, Downvotes: down}, nil
}

// RemoveVote removes a user's vote on a question or answer.
func (s *VoteService) RemoveVote(ctx context.Context, userID, targetType, targetID string) (*model.VoteResponse, error) {
	if err := validateTargetType(targetType); err != nil {
		return nil, err
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin transaction: %w", err)
	}
	defer repository.RollbackTx(ctx, tx)

	// Lock the target row to serialize concurrent vote removals.
	if err := lockTarget(ctx, tx, targetType, targetID); err != nil {
		return nil, err
	}

	// Get existing vote to know which counter to decrement
	oldVote, err := s.votes.GetByUserAndTarget(ctx, tx, userID, targetType, targetID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, fmt.Errorf("vote %w", ErrNotFound)
		}
		return nil, fmt.Errorf("get existing vote: %w", err)
	}

	if err := s.votes.Delete(ctx, tx, userID, targetType, targetID); err != nil {
		return nil, fmt.Errorf("delete vote: %w", err)
	}

	// Decrement the appropriate counter
	var upDelta, downDelta int
	if oldVote.Value == 1 {
		upDelta = -1
	} else {
		downDelta = -1
	}

	up, down, err := updateCounters(ctx, tx, targetType, targetID, upDelta, downDelta)
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit transaction: %w", err)
	}

	return &model.VoteResponse{Upvotes: up, Downvotes: down}, nil
}

func validateTargetType(targetType string) error {
	if targetType != "question" && targetType != "answer" {
		return fmt.Errorf("invalid target type %q: %w", targetType, ErrValidation)
	}
	return nil
}

// getTargetCounts returns current vote counts for a target without modifying them.
func getTargetCounts(ctx context.Context, q repository.Querier, targetType, targetID string) (int, int, error) {
	var up, down int
	var stmt string
	switch targetType {
	case "question":
		stmt = `SELECT upvotes, downvotes FROM questions WHERE id = $1`
	case "answer":
		stmt = `SELECT upvotes, downvotes FROM answers WHERE id = $1`
	default:
		return 0, 0, fmt.Errorf("invalid target type %q", targetType)
	}

	err := q.QueryRow(ctx, stmt, targetID).Scan(&up, &down)
	if err != nil {
		return 0, 0, fmt.Errorf("get %s counts: %w", targetType, err)
	}
	return up, down, nil
}

// lockTarget acquires a row-level lock on the target (question or answer) to
// serialize concurrent vote operations. Returns ErrNotFound if the target does not exist.
func lockTarget(ctx context.Context, q repository.Querier, targetType, targetID string) error {
	var stmt string
	switch targetType {
	case "question":
		stmt = `UPDATE questions SET id = id WHERE id = $1`
	case "answer":
		stmt = `UPDATE answers SET id = id WHERE id = $1`
	default:
		return fmt.Errorf("invalid target type %q: %w", targetType, ErrValidation)
	}

	tag, err := q.Exec(ctx, stmt, targetID)
	if err != nil {
		return fmt.Errorf("lock %s: %w", targetType, err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("%s %w", targetType, ErrNotFound)
	}
	return nil
}

// updateCounters atomically adjusts the cached vote counters on the target table.
func updateCounters(ctx context.Context, q repository.Querier, targetType, targetID string, upDelta, downDelta int) (int, int, error) {
	var stmt string
	switch targetType {
	case "question":
		stmt = `UPDATE questions SET upvotes = upvotes + $1, downvotes = downvotes + $2 WHERE id = $3 RETURNING upvotes, downvotes`
	case "answer":
		stmt = `UPDATE answers SET upvotes = upvotes + $1, downvotes = downvotes + $2 WHERE id = $3 RETURNING upvotes, downvotes`
	default:
		return 0, 0, fmt.Errorf("update counters: invalid target type %q: %w", targetType, ErrValidation)
	}

	var up, down int
	err := q.QueryRow(ctx, stmt, upDelta, downDelta, targetID).Scan(&up, &down)
	if err != nil {
		return 0, 0, fmt.Errorf("update %s counters: %w", targetType, err)
	}
	return up, down, nil
}
