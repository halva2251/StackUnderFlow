package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/halva2251/stackunderflow/internal/model"
)

func WriteJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		slog.Error("failed to encode json response", "error", err)
	}
}

func WriteError(w http.ResponseWriter, status int, code string, message string) {
	WriteJSON(w, status, model.ErrorResponse{
		Error: model.ErrorDetail{
			Code:    code,
			Message: message,
		},
	})
}
