package handler

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/halva2251/stackunderflow/internal/model"
	"github.com/halva2251/stackunderflow/internal/service"
)

type authService interface {
	Register(ctx context.Context, username, password string) (*model.AuthResponse, error)
	Login(ctx context.Context, username, password string) (*model.AuthResponse, error)
}

type AuthHandler struct {
	service authService
}

func NewAuthHandler(svc authService) *AuthHandler {
	return &AuthHandler{service: svc}
}

func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req model.RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "INVALID_JSON", "invalid request body")
		return
	}

	result, err := h.service.Register(r.Context(), req.Username, req.Password)
	if err != nil {
		if errors.Is(err, service.ErrUsernameTaken) {
			WriteError(w, http.StatusConflict, "USERNAME_TAKEN", "that username is already taken")
			return
		}
		if errors.Is(err, service.ErrValidation) {
			WriteError(w, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
			return
		}
		slog.Error("failed to register user", "error", err)
		WriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to register")
		return
	}

	WriteJSON(w, http.StatusCreated, result)
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req model.LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "INVALID_JSON", "invalid request body")
		return
	}

	result, err := h.service.Login(r.Context(), req.Username, req.Password)
	if err != nil {
		if errors.Is(err, service.ErrBadCredentials) {
			WriteError(w, http.StatusUnauthorized, "BAD_CREDENTIALS", "invalid username or password")
			return
		}
		slog.Error("failed to login", "error", err)
		WriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to login")
		return
	}

	WriteJSON(w, http.StatusOK, result)
}
