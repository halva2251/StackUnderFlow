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
func (s *VoteService) Vote(ctx context.Context, userID, targetType, targetID string, value int) (*model.Vote, error) {
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
	// This prevents race conditions where two votes compute incorrect counter deltas.
	if err := lockTarget(ctx, tx, targetType, targetID); err != nil {
		return nil, err
	}

	// Check for existing vote to calculate counter deltas
	oldVote, err := s.votes.GetByUserAndTarget(ctx, tx, userID, targetType, targetID)
	if err != nil && !errors.Is(err, repository.ErrNotFound) {
		return nil, fmt.Errorf("get existing vote: %w", err)
	}

	// If voting the same way, return the existing vote (idempotent)
	if oldVote != nil && oldVote.Value == value {
		return oldVote, nil
	}

	// Upsert the vote
	vote, err := s.votes.Upsert(ctx, tx, userID, targetType, targetID, value)
	if err != nil {
		return nil, fmt.Errorf("upsert vote: %w", err)
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

	if err := updateCounters(ctx, tx, targetType, targetID, upDelta, downDelta); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit transaction: %w", err)
	}

	return vote, nil
}

// RemoveVote removes a user's vote on a question or answer.
func (s *VoteService) RemoveVote(ctx context.Context, userID, targetType, targetID string) error {
	if err := validateTargetType(targetType); err != nil {
		return err
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer repository.RollbackTx(ctx, tx)

	// Lock the target row to serialize concurrent vote removals.
	if err := lockTarget(ctx, tx, targetType, targetID); err != nil {
		return err
	}

	// Get existing vote to know which counter to decrement
	oldVote, err := s.votes.GetByUserAndTarget(ctx, tx, userID, targetType, targetID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return fmt.Errorf("vote %w", ErrNotFound)
		}
		return fmt.Errorf("get existing vote: %w", err)
	}

	if err := s.votes.Delete(ctx, tx, userID, targetType, targetID); err != nil {
		return fmt.Errorf("delete vote: %w", err)
	}

	// Decrement the appropriate counter
	var upDelta, downDelta int
	if oldVote.Value == 1 {
		upDelta = -1
	} else {
		downDelta = -1
	}

	if err := updateCounters(ctx, tx, targetType, targetID, upDelta, downDelta); err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}

	return nil
}

func validateTargetType(targetType string) error {
	if targetType != "question" && targetType != "answer" {
		return fmt.Errorf("invalid target type %q: %w", targetType, ErrValidation)
	}
	return nil
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
func updateCounters(ctx context.Context, q repository.Querier, targetType, targetID string, upDelta, downDelta int) error {
	var stmt string
	switch targetType {
	case "question":
		stmt = `UPDATE questions SET upvotes = upvotes + $1, downvotes = downvotes + $2 WHERE id = $3`
	case "answer":
		stmt = `UPDATE answers SET upvotes = upvotes + $1, downvotes = downvotes + $2 WHERE id = $3`
	default:
		return fmt.Errorf("update counters: invalid target type %q: %w", targetType, ErrValidation)
	}

	tag, err := q.Exec(ctx, stmt, upDelta, downDelta, targetID)
	if err != nil {
		return fmt.Errorf("update %s counters: %w", targetType, err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("%s %w", targetType, ErrNotFound)
	}
	return nil
}
