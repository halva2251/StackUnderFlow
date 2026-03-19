package service

import (
	"context"
	"errors"
	"testing"

	"golang.org/x/crypto/bcrypt"

	"github.com/halva2251/stackunderflow/internal/model"
)

// mockUserRepo implements UserRepo for testing
type mockUserRepo struct {
	user      *model.User
	createErr error
}

func (m *mockUserRepo) Create(ctx context.Context, provider, providerID, username, email, passwordHash string) (*model.User, error) {
	if m.createErr != nil {
		return nil, m.createErr
	}
	return &model.User{
		ID:       "u-123",
		Provider: provider,
		Username: username,
	}, nil
}

func (m *mockUserRepo) GetByUsername(ctx context.Context, username string) (*model.User, error) {
	if m.user != nil && m.user.Username == username {
		return m.user, nil
	}
	return nil, nil
}

func (m *mockUserRepo) GetByID(ctx context.Context, id string) (*model.User, error) {
	return m.user, nil
}

func (m *mockUserRepo) UpsertByProvider(ctx context.Context, provider, providerID, username, avatarURL string) (*model.User, error) {
	return m.user, nil
}

const testSecret = "this-is-a-test-secret-that-is-at-least-32-bytes-long"

func newTestAuthService(t *testing.T, repo *mockUserRepo) *AuthService {
	t.Helper()
	svc, err := NewAuthService(repo, testSecret)
	if err != nil {
		t.Fatalf("failed to create auth service: %v", err)
	}
	return svc
}

func setMockHash(t *testing.T, repo *mockUserRepo, password string) {
	t.Helper()
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("failed to hash: %v", err)
	}
	if repo.user == nil {
		repo.user = &model.User{
			ID:       "u-123",
			Provider: "local",
			Username: "testuser",
		}
	}
	repo.user.PasswordHash = string(hash)
}

func TestRegister_Success(t *testing.T) {
	svc := newTestAuthService(t, &mockUserRepo{})

	result, err := svc.Register(context.Background(), "testuser", "password123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Token == "" {
		t.Error("expected non-empty token")
	}
	if result.User.Username != "testuser" {
		t.Errorf("expected username testuser, got %s", result.User.Username)
	}
}

func TestRegister_Validation(t *testing.T) {
	tests := []struct {
		name     string
		username string
		password string
		wantErr  error
	}{
		{name: "empty username", username: "", password: "12345678", wantErr: ErrInvalidUsername},
		{name: "short password", username: "user", password: "1234567", wantErr: ErrWeakPassword},
		{name: "long password", username: "user", password: string(make([]byte, 73)), wantErr: ErrPasswordTooLong},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := newTestAuthService(t, &mockUserRepo{})

			_, err := svc.Register(context.Background(), tt.username, tt.password)
			if err == nil {
				t.Fatal("expected error")
			}
			if !errors.Is(err, ErrValidation) {
				t.Errorf("expected ErrValidation, got %v", err)
			}
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("expected %v, got %v", tt.wantErr, err)
			}
		})
	}
}

func TestRegister_UsernameTaken(t *testing.T) {
	existing := &model.User{ID: "u-existing", Username: "taken"}
	svc := newTestAuthService(t, &mockUserRepo{user: existing})

	_, err := svc.Register(context.Background(), "taken", "password123")
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, ErrValidation) {
		t.Errorf("expected ErrValidation, got %v", err)
	}
	if !errors.Is(err, ErrUsernameTaken) {
		t.Errorf("expected ErrUsernameTaken, got %v", err)
	}
}

func TestLogin_Success(t *testing.T) {
	repo := &mockUserRepo{}
	setMockHash(t, repo, "password123")
	svc := newTestAuthService(t, repo)

	result, err := svc.Login(context.Background(), "testuser", "password123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Token == "" {
		t.Error("expected non-empty token")
	}
}

func TestLogin_WrongPassword(t *testing.T) {
	repo := &mockUserRepo{}
	setMockHash(t, repo, "correctpassword")
	svc := newTestAuthService(t, repo)

	_, err := svc.Login(context.Background(), "testuser", "wrongpassword")
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, ErrBadCredentials) {
		t.Errorf("expected ErrBadCredentials, got %v", err)
	}
}

func TestLogin_UserNotFound(t *testing.T) {
	svc := newTestAuthService(t, &mockUserRepo{user: nil})

	_, err := svc.Login(context.Background(), "noone", "password123")
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, ErrBadCredentials) {
		t.Errorf("expected ErrBadCredentials, got %v", err)
	}
}

func TestLogin_OAuthUserCannotLogin(t *testing.T) {
	oauthUser := &model.User{ID: "u-1", Provider: "github", Username: "ghuser"}
	svc := newTestAuthService(t, &mockUserRepo{user: oauthUser})

	_, err := svc.Login(context.Background(), "ghuser", "password123")
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, ErrBadCredentials) {
		t.Errorf("expected ErrBadCredentials, got %v", err)
	}
}

func TestValidateToken_Roundtrip(t *testing.T) {
	svc := newTestAuthService(t, &mockUserRepo{})
	regResult, err := svc.Register(context.Background(), "user", "password123")
	if err != nil {
		t.Fatalf("register failed: %v", err)
	}

	userID, err := svc.ValidateToken(regResult.Token)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if userID != regResult.User.ID {
		t.Errorf("expected user id %s, got %s", regResult.User.ID, userID)
	}
}

func TestValidateToken_Invalid(t *testing.T) {
	svc := newTestAuthService(t, &mockUserRepo{})

	_, err := svc.ValidateToken("garbage.token.here")
	if err == nil {
		t.Fatal("expected error for invalid token")
	}
}

func TestValidateToken_WrongSecret(t *testing.T) {
	svc1, err := NewAuthService(&mockUserRepo{}, "aaaaaaaaaabbbbbbbbbbccccccccccdd")
	if err != nil {
		t.Fatalf("create svc1: %v", err)
	}
	svc2, err := NewAuthService(&mockUserRepo{}, "ddccccccccccbbbbbbbbbbaaaaaaaaaa32")
	if err != nil {
		t.Fatalf("create svc2: %v", err)
	}

	regResult, err := svc1.Register(context.Background(), "user", "password123")
	if err != nil {
		t.Fatalf("register failed: %v", err)
	}

	_, err = svc2.ValidateToken(regResult.Token)
	if err == nil {
		t.Fatal("expected error for wrong secret")
	}
}

func TestNewAuthService_ShortSecret(t *testing.T) {
	_, err := NewAuthService(&mockUserRepo{}, "tooshort")
	if err == nil {
		t.Fatal("expected error for short JWT secret")
	}
}

func TestRegister_InvalidUsernameChars(t *testing.T) {
	tests := []struct {
		name     string
		username string
	}{
		{name: "spaces", username: "user name"},
		{name: "special chars", username: "user@name!"},
		{name: "too long", username: "aaaaaaaabbbbbbbbccccccccddddddddx"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := newTestAuthService(t, &mockUserRepo{})
			_, err := svc.Register(context.Background(), tt.username, "password123")
			if err == nil {
				t.Fatal("expected error")
			}
			if !errors.Is(err, ErrValidation) {
				t.Errorf("expected ErrValidation, got %v", err)
			}
		})
	}
}
