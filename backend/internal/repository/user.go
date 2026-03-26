package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/halva2251/stackunderflow/internal/model"
)

type UserRepository struct {
	*Repository
}

func NewUserRepository(pool *pgxpool.Pool) *UserRepository {
	return &UserRepository{Repository: NewRepository(pool)}
}

func (r *UserRepository) Create(ctx context.Context, provider, providerID, username, email, passwordHash string) (*model.User, error) {
	return r.CreateTx(ctx, r.pool, provider, providerID, username, email, passwordHash)
}

func (r *UserRepository) CreateTx(ctx context.Context, q Querier, provider, providerID, username, email, passwordHash string) (*model.User, error) {
	var u model.User
	err := q.QueryRow(ctx,
		`INSERT INTO users (provider, provider_id, username, email, password_hash)
		 VALUES ($1, $2, $3, $4, $5)
		 RETURNING id, provider, provider_id, username, COALESCE(email, ''), COALESCE(avatar_url, ''), created_at`,
		provider, nilIfEmpty(providerID), username, nilIfEmpty(email), nilIfEmpty(passwordHash),
	).Scan(&u.ID, &u.Provider, &u.ProviderID, &u.Username, &u.Email, &u.AvatarURL, &u.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("create user: %w", err)
	}
	return &u, nil
}

func (r *UserRepository) GetByUsername(ctx context.Context, username string) (*model.User, error) {
	return r.GetByUsernameTx(ctx, r.pool, username)
}

func (r *UserRepository) GetByUsernameTx(ctx context.Context, q Querier, username string) (*model.User, error) {
	var u model.User
	err := q.QueryRow(ctx,
		`SELECT id, provider, provider_id, username, COALESCE(email, ''), COALESCE(avatar_url, ''), COALESCE(password_hash, ''), created_at
		 FROM users WHERE username = $1 AND provider = 'local'`,
		username,
	).Scan(&u.ID, &u.Provider, &u.ProviderID, &u.Username, &u.Email, &u.AvatarURL, &u.PasswordHash, &u.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get user by username: %w", err)
	}
	return &u, nil
}

func (r *UserRepository) GetByID(ctx context.Context, id string) (*model.User, error) {
	return r.GetByIDTx(ctx, r.pool, id)
}

func (r *UserRepository) GetByIDTx(ctx context.Context, q Querier, id string) (*model.User, error) {
	var u model.User
	err := q.QueryRow(ctx,
		`SELECT id, provider, provider_id, username, COALESCE(email, ''), COALESCE(avatar_url, ''), created_at
		 FROM users WHERE id = $1`,
		id,
	).Scan(&u.ID, &u.Provider, &u.ProviderID, &u.Username, &u.Email, &u.AvatarURL, &u.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get user by id: %w", err)
	}
	return &u, nil
}

// UpsertByProvider creates or updates a user from an OAuth provider.
func (r *UserRepository) UpsertByProvider(ctx context.Context, provider, providerID, username, avatarURL string) (*model.User, error) {
	return r.UpsertByProviderTx(ctx, r.pool, provider, providerID, username, avatarURL)
}

// UpsertByProviderTx creates or updates a user from an OAuth provider within a transaction.
func (r *UserRepository) UpsertByProviderTx(ctx context.Context, q Querier, provider, providerID, username, avatarURL string) (*model.User, error) {
	var u model.User
	err := q.QueryRow(ctx,
		`INSERT INTO users (provider, provider_id, username, avatar_url)
		 VALUES ($1, $2, $3, $4)
		 ON CONFLICT (provider, provider_id) DO UPDATE
		 SET username = EXCLUDED.username, avatar_url = EXCLUDED.avatar_url
		 RETURNING id, provider, provider_id, username, COALESCE(email, ''), COALESCE(avatar_url, ''), created_at`,
		provider, providerID, username, nilIfEmpty(avatarURL),
	).Scan(&u.ID, &u.Provider, &u.ProviderID, &u.Username, &u.Email, &u.AvatarURL, &u.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("upsert user: %w", err)
	}
	return &u, nil
}
