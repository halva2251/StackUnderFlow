package handler

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/halva2251/stackunderflow/internal/middleware"
	"github.com/halva2251/stackunderflow/internal/model"
	"github.com/halva2251/stackunderflow/internal/service"
)

// questionService defines what the handler needs from the service layer.
type questionService interface {
	CreateQuestion(ctx context.Context, userID, title, body string) (*model.QuestionResponse, error)
	GetQuestion(ctx context.Context, id string) (*model.QuestionResponse, error)
	Argue(ctx context.Context, questionID, userID, argument string) (*model.Answer, error)
}

type QuestionHandler struct {
	service questionService
}

func NewQuestionHandler(svc questionService) *QuestionHandler {
	return &QuestionHandler{service: svc}
}

func (h *QuestionHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req model.CreateQuestionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "INVALID_JSON", "invalid request body")
		return
	}

	userID := middleware.GetUserID(r.Context())
	if userID == "" {
		WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
		return
	}

	result, err := h.service.CreateQuestion(r.Context(), userID, req.Title, req.Body)
	if err != nil {
		if errors.Is(err, service.ErrValidation) {
			WriteError(w, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
			return
		}
		slog.Error("failed to create question", "error", err)
		WriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to create question")
		return
	}

	WriteJSON(w, http.StatusCreated, result)
}

func (h *QuestionHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	result, err := h.service.GetQuestion(r.Context(), id)
	if err != nil {
		if errors.Is(err, service.ErrNotFound) {
			WriteError(w, http.StatusNotFound, "NOT_FOUND", "question not found")
			return
		}
		slog.Error("failed to get question", "error", err)
		WriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to get question")
		return
	}

	WriteJSON(w, http.StatusOK, result)
}

func (h *QuestionHandler) Argue(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	var req model.ArgueRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "INVALID_JSON", "invalid request body")
		return
	}

	userID := middleware.GetUserID(r.Context())
	if userID == "" {
		WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
		return
	}

	answer, err := h.service.Argue(r.Context(), id, userID, req.Argument)
	if err != nil {
		if errors.Is(err, service.ErrNotFound) {
			WriteError(w, http.StatusNotFound, "NOT_FOUND", "question not found")
			return
		}
		if errors.Is(err, service.ErrValidation) {
			WriteError(w, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
			return
		}
		slog.Error("failed to generate answer", "error", err)
		WriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to generate answer")
		return
	}

	WriteJSON(w, http.StatusCreated, answer)
}
