package middleware

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

type mockValidator struct {
	userID string
	err    error
}

func (m *mockValidator) ValidateToken(token string) (string, error) {
	return m.userID, m.err
}

func TestAuth_ValidToken(t *testing.T) {
	// Arrange
	validator := &mockValidator{userID: "u-123"}
	handler := Auth(validator)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID := GetUserID(r.Context())
		w.Write([]byte(userID))
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer valid-token")
	w := httptest.NewRecorder()

	// Act
	handler.ServeHTTP(w, req)

	// Assert
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	if w.Body.String() != "u-123" {
		t.Errorf("expected user id u-123, got %s", w.Body.String())
	}
}

func TestAuth_MissingHeader(t *testing.T) {
	// Arrange
	validator := &mockValidator{userID: "u-123"}
	handler := Auth(validator)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called")
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()

	// Act
	handler.ServeHTTP(w, req)

	// Assert
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestAuth_InvalidToken(t *testing.T) {
	// Arrange
	validator := &mockValidator{err: fmt.Errorf("invalid")}
	handler := Auth(validator)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called")
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer bad-token")
	w := httptest.NewRecorder()

	// Act
	handler.ServeHTTP(w, req)

	// Assert
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestAuth_MalformedHeader(t *testing.T) {
	tests := []struct {
		name   string
		header string
	}{
		{name: "no bearer prefix", header: "just-a-token"},
		{name: "basic auth", header: "Basic dXNlcjpwYXNz"},
		{name: "empty bearer", header: "Bearer "},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			validator := &mockValidator{userID: "u-123"}
			handler := Auth(validator)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				t.Error("handler should not be called")
			}))

			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.Header.Set("Authorization", tt.header)
			w := httptest.NewRecorder()

			// Act
			handler.ServeHTTP(w, req)

			// Assert
			if w.Code != http.StatusUnauthorized {
				t.Errorf("expected 401, got %d", w.Code)
			}
		})
	}
}

func TestOptionalAuth_WithToken(t *testing.T) {
	// Arrange
	validator := &mockValidator{userID: "u-123"}
	handler := OptionalAuth(validator)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID := GetUserID(r.Context())
		w.Write([]byte(userID))
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer valid-token")
	w := httptest.NewRecorder()

	// Act
	handler.ServeHTTP(w, req)

	// Assert
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	if w.Body.String() != "u-123" {
		t.Errorf("expected user id u-123, got %s", w.Body.String())
	}
}

func TestOptionalAuth_WithoutToken(t *testing.T) {
	// Arrange
	validator := &mockValidator{userID: "u-123"}
	handler := OptionalAuth(validator)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID := GetUserID(r.Context())
		if userID != "" {
			t.Error("expected empty user id without token")
		}
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()

	// Act
	handler.ServeHTTP(w, req)

	// Assert
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}
