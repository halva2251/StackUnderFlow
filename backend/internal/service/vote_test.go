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

// --- Mock VoteRepo ---

type mockVoteRepo struct {
	vote      *model.Vote
	upsertErr error
	deleteErr error
	getErr    error
	// Track calls for counter verification
	execCalls []execCall
}

type execCall struct {
	sql  string
	args []any
}

func (m *mockVoteRepo) Upsert(_ context.Context, _ repository.Querier, userID, targetType, targetID string, value int) (*model.Vote, error) {
	if m.upsertErr != nil {
		return nil, m.upsertErr
	}
	return &model.Vote{
		ID:         "v-123",
		UserID:     userID,
		TargetType: targetType,
		TargetID:   targetID,
		Value:      value,
	}, nil
}

func (m *mockVoteRepo) Delete(_ context.Context, _ repository.Querier, _, _, _ string) error {
	return m.deleteErr
}

func (m *mockVoteRepo) GetByUserAndTarget(_ context.Context, _ repository.Querier, _, _, _ string) (*model.Vote, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	if m.vote != nil {
		return m.vote, nil
	}
	return nil, repository.ErrNotFound
}

// errRow implements pgx.Row and returns an error on Scan.
type errRow struct{ err error }

func (r errRow) Scan(_ ...any) error { return r.err }

// mockTxWithExec tracks Exec calls for counter update verification.
type mockTxWithExec struct {
	commitErr error
	execCalls []execCall
}

func (m *mockTxWithExec) QueryRow(_ context.Context, _ string, _ ...any) pgx.Row {
	return errRow{err: pgx.ErrNoRows}
}
func (m *mockTxWithExec) Query(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
	return nil, nil
}
func (m *mockTxWithExec) Exec(_ context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	m.execCalls = append(m.execCalls, execCall{sql: sql, args: args})
	return pgconn.NewCommandTag("UPDATE 1"), nil
}
func (m *mockTxWithExec) Commit(_ context.Context) error   { return m.commitErr }
func (m *mockTxWithExec) Rollback(_ context.Context) error { return nil }

type mockTxBeginnerWithExec struct {
	tx *mockTxWithExec
}

func (m *mockTxBeginnerWithExec) Begin(_ context.Context) (repository.Tx, error) {
	return m.tx, nil
}

// --- Tests: Vote ---

func TestVote_NewUpvote(t *testing.T) {
	tx := &mockTxWithExec{}
	svc := NewVoteService(&mockTxBeginnerWithExec{tx: tx}, &mockVoteRepo{})

	vote, err := svc.Vote(context.Background(), "u-1", "question", "q-1", 1)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if vote.Value != 1 {
		t.Errorf("expected value 1, got %d", vote.Value)
	}
	if vote.TargetType != "question" {
		t.Errorf("expected target_type 'question', got %s", vote.TargetType)
	}
}

func TestVote_NewDownvote(t *testing.T) {
	tx := &mockTxWithExec{}
	svc := NewVoteService(&mockTxBeginnerWithExec{tx: tx}, &mockVoteRepo{})

	vote, err := svc.Vote(context.Background(), "u-1", "answer", "a-1", -1)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if vote.Value != -1 {
		t.Errorf("expected value -1, got %d", vote.Value)
	}
}

func TestVote_ChangeUpvoteToDownvote(t *testing.T) {
	tx := &mockTxWithExec{}
	repo := &mockVoteRepo{
		vote: &model.Vote{ID: "v-1", UserID: "u-1", TargetType: "question", TargetID: "q-1", Value: 1},
	}
	svc := NewVoteService(&mockTxBeginnerWithExec{tx: tx}, repo)

	vote, err := svc.Vote(context.Background(), "u-1", "question", "q-1", -1)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if vote.Value != -1 {
		t.Errorf("expected value -1, got %d", vote.Value)
	}
}

func TestVote_SameValueIdempotent(t *testing.T) {
	tx := &mockTxWithExec{}
	repo := &mockVoteRepo{
		vote: &model.Vote{ID: "v-1", UserID: "u-1", TargetType: "question", TargetID: "q-1", Value: 1},
	}
	svc := NewVoteService(&mockTxBeginnerWithExec{tx: tx}, repo)

	vote, err := svc.Vote(context.Background(), "u-1", "question", "q-1", 1)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if vote.ID != "v-1" {
		t.Errorf("expected existing vote ID v-1, got %s", vote.ID)
	}
}

func TestVote_InvalidValue(t *testing.T) {
	svc := NewVoteService(&mockTxBeginner{}, &mockVoteRepo{})

	for _, val := range []int{0, 2, -2, 100} {
		_, err := svc.Vote(context.Background(), "u-1", "question", "q-1", val)

		if !errors.Is(err, ErrValidation) {
			t.Errorf("value=%d: expected ErrValidation, got %v", val, err)
		}
	}
}

func TestVote_InvalidTargetType(t *testing.T) {
	svc := NewVoteService(&mockTxBeginner{}, &mockVoteRepo{})

	_, err := svc.Vote(context.Background(), "u-1", "comment", "c-1", 1)

	if !errors.Is(err, ErrValidation) {
		t.Errorf("expected ErrValidation for invalid target type, got %v", err)
	}
}

func TestVote_TargetNotFound(t *testing.T) {
	// mockTxWithExec that returns UPDATE 0 (target doesn't exist)
	tx := &mockTxNotFound{}
	svc := NewVoteService(&mockTxBeginnerNotFound{tx: tx}, &mockVoteRepo{})

	_, err := svc.Vote(context.Background(), "u-1", "question", "nonexistent", 1)

	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

// --- Tests: RemoveVote ---

func TestRemoveVote_Success(t *testing.T) {
	tx := &mockTxWithExec{}
	repo := &mockVoteRepo{
		vote: &model.Vote{ID: "v-1", UserID: "u-1", TargetType: "question", TargetID: "q-1", Value: 1},
	}
	svc := NewVoteService(&mockTxBeginnerWithExec{tx: tx}, repo)

	err := svc.RemoveVote(context.Background(), "u-1", "question", "q-1")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRemoveVote_NotFound(t *testing.T) {
	tx := &mockTxWithExec{}
	repo := &mockVoteRepo{} // GetByUserAndTarget returns ErrNotFound by default
	svc := NewVoteService(&mockTxBeginnerWithExec{tx: tx}, repo)

	err := svc.RemoveVote(context.Background(), "u-1", "question", "q-1")

	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestRemoveVote_InvalidTargetType(t *testing.T) {
	svc := NewVoteService(&mockTxBeginner{}, &mockVoteRepo{})

	err := svc.RemoveVote(context.Background(), "u-1", "invalid", "x-1")

	if !errors.Is(err, ErrValidation) {
		t.Errorf("expected ErrValidation, got %v", err)
	}
}

// --- Helper mocks for target-not-found scenario ---

type mockTxNotFound struct{}

func (m *mockTxNotFound) QueryRow(_ context.Context, _ string, _ ...any) pgx.Row {
	return errRow{err: pgx.ErrNoRows}
}
func (m *mockTxNotFound) Query(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
	return nil, nil
}
func (m *mockTxNotFound) Exec(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
	return pgconn.NewCommandTag("UPDATE 0"), nil
}
func (m *mockTxNotFound) Commit(_ context.Context) error   { return nil }
func (m *mockTxNotFound) Rollback(_ context.Context) error { return nil }

type mockTxBeginnerNotFound struct {
	tx *mockTxNotFound
}

func (m *mockTxBeginnerNotFound) Begin(_ context.Context) (repository.Tx, error) {
	return m.tx, nil
}
