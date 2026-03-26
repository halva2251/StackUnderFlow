//go:build integration

package repository

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/halva2251/stackunderflow/internal/database"
	"github.com/jackc/pgx/v5/pgxpool"
)

func setupTest(t *testing.T) (*UserRepository, *pgxpool.Pool) {
	t.Helper()
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL not set, skipping integration tests")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	pool, err := database.Connect(ctx, databaseURL)
	if err != nil {
		t.Fatalf("failed to connect to test database: %v", err)
	}
	t.Cleanup(func() { pool.Close() })
	return NewUserRepository(pool), pool
}

func TestUserRepository_Create_Success(t *testing.T) {
	repo, pool := setupTest(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	username := "testuser_create_" + time.Now().Format("20060102150405")

	user, err := repo.Create(ctx, "local", "", username, "test@example.com", "hashedpassword")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if user == nil {
		t.Fatal("expected user, got nil")
	}
	t.Cleanup(func() {
		ctx2, cancel2 := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel2()
		_, _ = pool.Exec(ctx2, "DELETE FROM users WHERE id = $1", user.ID)
	})
	if user.Username != username {
		t.Errorf("expected username %s, got %s", username, user.Username)
	}
	if user.Provider != "local" {
		t.Errorf("expected provider 'local', got %s", user.Provider)
	}
	if user.Email != "test@example.com" {
		t.Errorf("expected email 'test@example.com', got %s", user.Email)
	}
}

func TestUserRepository_GetByUsername_Success(t *testing.T) {
	repo, pool := setupTest(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	username := "testuser_get_" + time.Now().Format("20060102150405")

	created, err := repo.Create(ctx, "local", "", username, "test@example.com", "hashedpassword")
	if err != nil {
		t.Fatalf("failed to create user: %v", err)
	}
	t.Cleanup(func() {
		ctx2, cancel2 := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel2()
		_, _ = pool.Exec(ctx2, "DELETE FROM users WHERE id = $1", created.ID)
	})

	user, err := repo.GetByUsername(ctx, username)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if user == nil {
		t.Fatal("expected user, got nil")
	}
	if user.ID != created.ID {
		t.Errorf("expected id %s, got %s", created.ID, user.ID)
	}
	if user.Username != username {
		t.Errorf("expected username %s, got %s", username, user.Username)
	}
}

func TestUserRepository_GetByUsername_NotFound(t *testing.T) {
	repo, _ := setupTest(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	user, err := repo.GetByUsername(ctx, "nonexistentuser12345")

	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got: %v", err)
	}
	if user != nil {
		t.Errorf("expected nil user, got: %+v", user)
	}
}

func TestUserRepository_GetByID_Success(t *testing.T) {
	repo, pool := setupTest(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	username := "testuser_getid_" + time.Now().Format("20060102150405")

	created, err := repo.Create(ctx, "local", "", username, "test@example.com", "hashedpassword")
	if err != nil {
		t.Fatalf("failed to create user: %v", err)
	}
	t.Cleanup(func() {
		ctx2, cancel2 := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel2()
		_, _ = pool.Exec(ctx2, "DELETE FROM users WHERE id = $1", created.ID)
	})

	user, err := repo.GetByID(ctx, created.ID)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if user == nil {
		t.Fatal("expected user, got nil")
	}
	if user.ID != created.ID {
		t.Errorf("expected id %s, got %s", created.ID, user.ID)
	}
	if user.Username != username {
		t.Errorf("expected username %s, got %s", username, user.Username)
	}
}

func TestUserRepository_GetByID_NotFound(t *testing.T) {
	repo, _ := setupTest(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	user, err := repo.GetByID(ctx, "00000000-0000-0000-0000-000000000000")

	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got: %v", err)
	}
	if user != nil {
		t.Errorf("expected nil user, got: %+v", user)
	}
}

func TestUserRepository_UpsertByProvider_Create(t *testing.T) {
	repo, pool := setupTest(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	providerID := "oauth_12345_" + time.Now().Format("20060102150405")
	username := "oauthuser_" + time.Now().Format("20060102150405")

	user, err := repo.UpsertByProvider(ctx, "github", providerID, username, "https://example.com/avatar.png")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if user == nil {
		t.Fatal("expected user, got nil")
	}
	t.Cleanup(func() {
		ctx2, cancel2 := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel2()
		_, _ = pool.Exec(ctx2, "DELETE FROM users WHERE id = $1", user.ID)
	})
	if user.Provider != "github" {
		t.Errorf("expected provider 'github', got %s", user.Provider)
	}
	if user.ProviderID != providerID {
		t.Errorf("expected provider_id %s, got %s", providerID, user.ProviderID)
	}
	if user.Username != username {
		t.Errorf("expected username %s, got %s", username, user.Username)
	}
}

func TestUserRepository_UpsertByProvider_Update(t *testing.T) {
	repo, pool := setupTest(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	providerID := "oauth_update_" + time.Now().Format("20060102150405")
	username := "oauthuser_original_" + time.Now().Format("20060102150405")

	created, err := repo.UpsertByProvider(ctx, "google", providerID, username, "https://example.com/avatar.png")
	if err != nil {
		t.Fatalf("failed to create user: %v", err)
	}
	t.Cleanup(func() {
		ctx2, cancel2 := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel2()
		_, _ = pool.Exec(ctx2, "DELETE FROM users WHERE id = $1", created.ID)
	})

	newUsername := "oauthuser_updated_" + time.Now().Format("20060102150405")
	user, err := repo.UpsertByProvider(ctx, "google", providerID, newUsername, "https://example.com/newavatar.png")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if user == nil {
		t.Fatal("expected user, got nil")
	}
	if user.ID != created.ID {
		t.Errorf("expected same id %s, got %s", created.ID, user.ID)
	}
	if user.Username != newUsername {
		t.Errorf("expected updated username %s, got %s", newUsername, user.Username)
	}
}
