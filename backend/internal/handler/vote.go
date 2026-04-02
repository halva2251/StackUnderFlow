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

type voteService interface {
	Vote(ctx context.Context, userID, targetType, targetID string, value int) (*model.VoteResponse, error)
	RemoveVote(ctx context.Context, userID, targetType, targetID string) (*model.VoteResponse, error)
}

type VoteHandler struct {
	service voteService
}

func NewVoteHandler(svc voteService) *VoteHandler {
	return &VoteHandler{service: svc}
}

// VoteOn returns a handler that casts a vote on targets of the given type.
func (h *VoteHandler) VoteOn(targetType string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		targetID := chi.URLParam(r, "id")

		var req model.VoteRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			WriteError(w, http.StatusBadRequest, "INVALID_JSON", "invalid request body")
			return
		}

		userID := middleware.GetUserID(r.Context())
		if userID == "" {
			WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
			return
		}

		resp, err := h.service.Vote(r.Context(), userID, targetType, targetID, req.Value)
		if err != nil {
			if errors.Is(err, service.ErrValidation) {
				WriteError(w, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
				return
			}
			if errors.Is(err, service.ErrNotFound) {
				WriteError(w, http.StatusNotFound, "NOT_FOUND", targetType+" not found")
				return
			}
			slog.Error("failed to cast vote", "error", err)
			WriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to cast vote")
			return
		}

		WriteJSON(w, http.StatusOK, resp)
	}
}

// RemoveVoteOn returns a handler that removes a vote on targets of the given type.
func (h *VoteHandler) RemoveVoteOn(targetType string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		targetID := chi.URLParam(r, "id")

		userID := middleware.GetUserID(r.Context())
		if userID == "" {
			WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
			return
		}

		resp, err := h.service.RemoveVote(r.Context(), userID, targetType, targetID)
		if err != nil {
			if errors.Is(err, service.ErrValidation) {
				WriteError(w, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
				return
			}
			if errors.Is(err, service.ErrNotFound) {
				WriteError(w, http.StatusNotFound, "NOT_FOUND", "vote not found")
				return
			}
			slog.Error("failed to remove vote", "error", err)
			WriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to remove vote")
			return
		}

		WriteJSON(w, http.StatusOK, resp)
	}
}
