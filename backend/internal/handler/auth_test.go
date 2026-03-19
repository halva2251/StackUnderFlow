package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/halva2251/stackunderflow/internal/model"
	"github.com/halva2251/stackunderflow/internal/service"
)

type mockAuthService struct {
	result *model.AuthResponse
	err    error
}

func (m *mockAuthService) Register(ctx context.Context, username, password string) (*model.AuthResponse, error) {
	if m.err != nil {
		return nil, m.err
	}
	if m.result != nil {
		return m.result, nil
	}
	return &model.AuthResponse{
		Token: "test-token",
		User:  model.User{ID: "u-123", Username: username},
	}, nil
}

func (m *mockAuthService) Login(ctx context.Context, username, password string) (*model.AuthResponse, error) {
	if m.err != nil {
		return nil, m.err
	}
	if m.result != nil {
		return m.result, nil
	}
	return &model.AuthResponse{
		Token: "test-token",
		User:  model.User{ID: "u-123", Username: username},
	}, nil
}

func TestAuthHandler_Register_Success(t *testing.T) {
	h := NewAuthHandler(&mockAuthService{})
	body, _ := json.Marshal(model.RegisterRequest{Username: "user", Password: "password123"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", bytes.NewReader(body))
	w := httptest.NewRecorder()

	h.Register(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d", w.Code)
	}

	var result model.AuthResponse
	if err := json.NewDecoder(w.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}
	if result.Token == "" {
		t.Error("expected non-empty token")
	}
}

func TestAuthHandler_Register_InvalidJSON(t *testing.T) {
	h := NewAuthHandler(&mockAuthService{})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", bytes.NewReader([]byte("bad")))
	w := httptest.NewRecorder()

	h.Register(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestAuthHandler_Register_ValidationError(t *testing.T) {
	h := NewAuthHandler(&mockAuthService{err: service.ErrValidation})
	body, _ := json.Marshal(model.RegisterRequest{Username: "user", Password: "short"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", bytes.NewReader(body))
	w := httptest.NewRecorder()

	h.Register(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestAuthHandler_Register_UsernameTaken(t *testing.T) {
	h := NewAuthHandler(&mockAuthService{err: fmt.Errorf("%w: %w", service.ErrUsernameTaken, service.ErrValidation)})
	body, _ := json.Marshal(model.RegisterRequest{Username: "taken", Password: "password123"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", bytes.NewReader(body))
	w := httptest.NewRecorder()

	h.Register(w, req)

	if w.Code != http.StatusConflict {
		t.Errorf("expected 409, got %d", w.Code)
	}
}

func TestAuthHandler_Register_InternalError(t *testing.T) {
	h := NewAuthHandler(&mockAuthService{err: fmt.Errorf("db exploded")})
	body, _ := json.Marshal(model.RegisterRequest{Username: "user", Password: "password123"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", bytes.NewReader(body))
	w := httptest.NewRecorder()

	h.Register(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", w.Code)
	}
}

func TestAuthHandler_Login_InternalError(t *testing.T) {
	h := NewAuthHandler(&mockAuthService{err: fmt.Errorf("db exploded")})
	body, _ := json.Marshal(model.LoginRequest{Username: "user", Password: "password123"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(body))
	w := httptest.NewRecorder()

	h.Login(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", w.Code)
	}
}

func TestAuthHandler_Login_Success(t *testing.T) {
	h := NewAuthHandler(&mockAuthService{})
	body, _ := json.Marshal(model.LoginRequest{Username: "user", Password: "password123"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(body))
	w := httptest.NewRecorder()

	h.Login(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestAuthHandler_Login_BadCredentials(t *testing.T) {
	h := NewAuthHandler(&mockAuthService{err: service.ErrBadCredentials})
	body, _ := json.Marshal(model.LoginRequest{Username: "user", Password: "wrong"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(body))
	w := httptest.NewRecorder()

	h.Login(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestAuthHandler_Login_InvalidJSON(t *testing.T) {
	h := NewAuthHandler(&mockAuthService{})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader([]byte("bad")))
	w := httptest.NewRecorder()

	h.Login(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}
