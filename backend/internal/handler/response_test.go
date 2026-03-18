package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/halva2251/stackunderflow/internal/model"
)

func TestWriteJSON(t *testing.T) {
	// Arrange
	w := httptest.NewRecorder()
	data := map[string]string{"status": "ok"}

	// Act
	WriteJSON(w, http.StatusOK, data)

	// Assert
	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("expected content-type application/json, got %s", ct)
	}

	var result map[string]string
	if err := json.NewDecoder(w.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if result["status"] != "ok" {
		t.Errorf("expected status ok, got %s", result["status"])
	}
}

func TestWriteError(t *testing.T) {
	// Arrange
	w := httptest.NewRecorder()

	// Act
	WriteError(w, http.StatusNotFound, "NOT_FOUND", "resource not found")

	// Assert
	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}

	var result model.ErrorResponse
	if err := json.NewDecoder(w.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if result.Error.Code != "NOT_FOUND" {
		t.Errorf("expected code NOT_FOUND, got %s", result.Error.Code)
	}
	if result.Error.Message != "resource not found" {
		t.Errorf("expected message 'resource not found', got %s", result.Error.Message)
	}
}

func TestWriteError_StatusCodes(t *testing.T) {
	tests := []struct {
		name   string
		status int
		code   string
	}{
		{name: "bad request", status: http.StatusBadRequest, code: "BAD_REQUEST"},
		{name: "unauthorized", status: http.StatusUnauthorized, code: "UNAUTHORIZED"},
		{name: "internal error", status: http.StatusInternalServerError, code: "INTERNAL_ERROR"},
		{name: "service unavailable", status: http.StatusServiceUnavailable, code: "UNAVAILABLE"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			w := httptest.NewRecorder()

			// Act
			WriteError(w, tt.status, tt.code, "test message")

			// Assert
			if w.Code != tt.status {
				t.Errorf("expected status %d, got %d", tt.status, w.Code)
			}

			var result model.ErrorResponse
			if err := json.NewDecoder(w.Body).Decode(&result); err != nil {
				t.Fatalf("failed to decode: %v", err)
			}
			if result.Error.Code != tt.code {
				t.Errorf("expected code %s, got %s", tt.code, result.Error.Code)
			}
		})
	}
}
