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

type mockVoteService struct {
	resp *model.VoteResponse
	err  error
}

func (m *mockVoteService) Vote(_ context.Context, _, _, _ string, _ int) (*model.VoteResponse, error) {
	if m.err != nil {
		return nil, m.err
	}
	if m.resp != nil {
		return m.resp, nil
	}
	return &model.VoteResponse{Upvotes: 10, Downvotes: 5}, nil
}

func (m *mockVoteService) RemoveVote(_ context.Context, _, _, _ string) (*model.VoteResponse, error) {
	if m.err != nil {
		return nil, m.err
	}
	if m.resp != nil {
		return m.resp, nil
	}
	return &model.VoteResponse{Upvotes: 9, Downvotes: 5}, nil
}

func TestVoteHandler_VoteOn_Success(t *testing.T) {
	h := NewVoteHandler(&mockVoteService{})
	body, _ := json.Marshal(model.VoteRequest{Value: 1})
	req := httptest.NewRequest(http.MethodPut, "/api/v1/questions/q-1/vote", bytes.NewReader(body))
	req = withUserID(req, "user-1")
	req = withChiParam(req, "id", "q-1")
	w := httptest.NewRecorder()

	h.VoteOn("question")(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var result model.VoteResponse
	if err := json.NewDecoder(w.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}
	if result.Upvotes != 10 {
		t.Errorf("expected upvotes 10, got %d", result.Upvotes)
	}
}

func TestVoteHandler_VoteOn_Downvote(t *testing.T) {
	h := NewVoteHandler(&mockVoteService{})
	body, _ := json.Marshal(model.VoteRequest{Value: -1})
	req := httptest.NewRequest(http.MethodPut, "/api/v1/answers/a-1/vote", bytes.NewReader(body))
	req = withUserID(req, "user-1")
	req = withChiParam(req, "id", "a-1")
	w := httptest.NewRecorder()

	h.VoteOn("answer")(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestVoteHandler_VoteOn_InvalidJSON(t *testing.T) {
	h := NewVoteHandler(&mockVoteService{})
	req := httptest.NewRequest(http.MethodPut, "/api/v1/questions/q-1/vote", bytes.NewReader([]byte("bad")))
	req = withUserID(req, "user-1")
	req = withChiParam(req, "id", "q-1")
	w := httptest.NewRecorder()

	h.VoteOn("question")(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestVoteHandler_VoteOn_MissingAuth(t *testing.T) {
	h := NewVoteHandler(&mockVoteService{})
	body, _ := json.Marshal(model.VoteRequest{Value: 1})
	req := httptest.NewRequest(http.MethodPut, "/api/v1/questions/q-1/vote", bytes.NewReader(body))
	req = withChiParam(req, "id", "q-1")
	w := httptest.NewRecorder()

	h.VoteOn("question")(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestVoteHandler_VoteOn_ValidationError(t *testing.T) {
	h := NewVoteHandler(&mockVoteService{
		err: fmt.Errorf("value must be 1 or -1: %w", service.ErrValidation),
	})
	body, _ := json.Marshal(model.VoteRequest{Value: 5})
	req := httptest.NewRequest(http.MethodPut, "/api/v1/questions/q-1/vote", bytes.NewReader(body))
	req = withUserID(req, "user-1")
	req = withChiParam(req, "id", "q-1")
	w := httptest.NewRecorder()

	h.VoteOn("question")(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestVoteHandler_VoteOn_NotFound(t *testing.T) {
	h := NewVoteHandler(&mockVoteService{
		err: fmt.Errorf("question %w", service.ErrNotFound),
	})
	body, _ := json.Marshal(model.VoteRequest{Value: 1})
	req := httptest.NewRequest(http.MethodPut, "/api/v1/questions/nonexistent/vote", bytes.NewReader(body))
	req = withUserID(req, "user-1")
	req = withChiParam(req, "id", "nonexistent")
	w := httptest.NewRecorder()

	h.VoteOn("question")(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestVoteHandler_VoteOn_InternalError(t *testing.T) {
	h := NewVoteHandler(&mockVoteService{err: fmt.Errorf("db exploded")})
	body, _ := json.Marshal(model.VoteRequest{Value: 1})
	req := httptest.NewRequest(http.MethodPut, "/api/v1/questions/q-1/vote", bytes.NewReader(body))
	req = withUserID(req, "user-1")
	req = withChiParam(req, "id", "q-1")
	w := httptest.NewRecorder()

	h.VoteOn("question")(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", w.Code)
	}
}

func TestVoteHandler_RemoveVoteOn_Success(t *testing.T) {
	h := NewVoteHandler(&mockVoteService{})
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/questions/q-1/vote", nil)
	req = withUserID(req, "user-1")
	req = withChiParam(req, "id", "q-1")
	w := httptest.NewRecorder()

	h.RemoveVoteOn("question")(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var result model.VoteResponse
	if err := json.NewDecoder(w.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}
	if result.Upvotes != 9 {
		t.Errorf("expected upvotes 9, got %d", result.Upvotes)
	}
}

func TestVoteHandler_RemoveVoteOn_MissingAuth(t *testing.T) {
	h := NewVoteHandler(&mockVoteService{})
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/questions/q-1/vote", nil)
	req = withChiParam(req, "id", "q-1")
	w := httptest.NewRecorder()

	h.RemoveVoteOn("question")(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestVoteHandler_RemoveVoteOn_NotFound(t *testing.T) {
	h := NewVoteHandler(&mockVoteService{
		err: fmt.Errorf("vote %w", service.ErrNotFound),
	})
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/questions/q-1/vote", nil)
	req = withUserID(req, "user-1")
	req = withChiParam(req, "id", "q-1")
	w := httptest.NewRecorder()

	h.RemoveVoteOn("question")(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}
