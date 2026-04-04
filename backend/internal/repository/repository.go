package repository

import (
	"context"
	"errors"
	"log/slog"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrNotFound is returned when a requested resource does not exist.
var ErrNotFound = errors.New("not found")

// Repository provides a generic base for all repositories with transaction support.
type Repository struct {
	pool *pgxpool.Pool
}

// NewRepository creates a new Repository with the given connection pool.
func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

// BeginTx starts a new transaction with the default isolation level.
func (r *Repository) BeginTx(ctx context.Context) (pgx.Tx, error) {
	return r.pool.Begin(ctx)
}

// BeginTxWithOptions starts a new transaction with the given options.
func (r *Repository) BeginTxWithOptions(ctx context.Context, opts pgx.TxOptions) (pgx.Tx, error) {
	return r.pool.BeginTx(ctx, opts)
}

// nilIfEmpty returns nil if the string is empty, otherwise returns a pointer to the string.
func nilIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// Tx combines Querier with transaction control. pgx.Tx satisfies this interface.
type Tx interface {
	Querier
	Commit(ctx context.Context) error
	Rollback(ctx context.Context) error
}

// RollbackTx safely rolls back a transaction, ignoring "transaction already closed" errors.
// Designed to be used with defer; logs unexpected rollback failures instead of returning
// them, since deferred calls cannot propagate return values.
func RollbackTx(ctx context.Context, tx Tx) {
	err := tx.Rollback(ctx)
	if err != nil && !errors.Is(err, pgx.ErrTxClosed) {
		slog.Error("rollback transaction failed", "error", err)
	}
}
