package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/halva2251/stackunderflow/internal/middleware"
	"github.com/halva2251/stackunderflow/internal/model"
	"github.com/halva2251/stackunderflow/internal/service"
)

// mockQuestionService implements the questionService interface for testing
type mockQuestionService struct {
	createResult   *model.QuestionResponse
	getResult      *model.QuestionResponse
	listResult     *model.QuestionListResponse
	argueResult    *model.Answer
	err            error
}

func (m *mockQuestionService) CreateQuestion(ctx context.Context, userID, title, body string) (*model.QuestionResponse, error) {
	if m.err != nil {
		return nil, m.err
	}
	if m.createResult != nil {
		return m.createResult, nil
	}
	return &model.QuestionResponse{
		Question: model.Question{ID: "q-123", UserID: userID, Title: title, Body: body},
		Answers:  []model.Answer{{ID: "a-123", Depth: 0, AIResponse: "Wrong answer!"}},
	}, nil
}

func (m *mockQuestionService) GetQuestion(ctx context.Context, id string) (*model.QuestionResponse, error) {
	if m.err != nil {
		return nil, m.err
	}
	if m.getResult != nil {
		return m.getResult, nil
	}
	return &model.QuestionResponse{
		Question: model.Question{ID: id, Title: "Test", Body: "Body"},
		Answers:  []model.Answer{{ID: "a-1", Depth: 0, AIResponse: "Wrong"}},
	}, nil
}

func (m *mockQuestionService) Argue(ctx context.Context, questionID, userID, argument string) (*model.Answer, error) {
	if m.err != nil {
		return nil, m.err
	}
	if m.argueResult != nil {
		return m.argueResult, nil
	}
	return &model.Answer{ID: "a-456", QuestionID: questionID, Depth: 1, UserPrompt: argument, AIResponse: "Doubling down!"}, nil
}

func (m *mockQuestionService) ListQuestions(ctx context.Context, sort string, limit, offset int) (*model.QuestionListResponse, error) {
	if m.err != nil {
		return nil, m.err
	}
	if m.listResult != nil {
		return m.listResult, nil
	}
	return &model.QuestionListResponse{
		Questions: []model.Question{
			{ID: "q-1", Title: "First"},
			{ID: "q-2", Title: "Second"},
		},
		Total:  10,
		Limit:  limit,
		Offset: offset,
		Sort:   sort,
	}, nil
}

func withChiParam(req *http.Request, key, value string) *http.Request {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add(key, value)
	return req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
}

func withUserID(req *http.Request, userID string) *http.Request {
	ctx := context.WithValue(req.Context(), middleware.UserIDKey, userID)
	return req.WithContext(ctx)
}

func TestQuestionHandler_Create_Success(t *testing.T) {
	// Arrange
	h := NewQuestionHandler(&mockQuestionService{})
	body, _ := json.Marshal(model.CreateQuestionRequest{Title: "Test", Body: "Test body"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/questions", bytes.NewReader(body))
	req = withUserID(req, "user-1")
	w := httptest.NewRecorder()

	// Act
	h.Create(w, req)

	// Assert
	if w.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d", w.Code)
	}

	var result model.QuestionResponse
	if err := json.NewDecoder(w.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}
	if result.Question.Title != "Test" {
		t.Errorf("expected title 'Test', got %s", result.Question.Title)
	}
}

func TestQuestionHandler_Create_InvalidJSON(t *testing.T) {
	// Arrange
	h := NewQuestionHandler(&mockQuestionService{})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/questions", bytes.NewReader([]byte("not json")))
	req = withUserID(req, "user-1")
	w := httptest.NewRecorder()

	// Act
	h.Create(w, req)

	// Assert
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestQuestionHandler_Create_ValidationError(t *testing.T) {
	// Arrange
	h := NewQuestionHandler(&mockQuestionService{
		err: fmt.Errorf("title is required: %w", service.ErrValidation),
	})
	body, _ := json.Marshal(model.CreateQuestionRequest{Title: "", Body: "body"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/questions", bytes.NewReader(body))
	req = withUserID(req, "user-1")
	w := httptest.NewRecorder()

	// Act
	h.Create(w, req)

	// Assert
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestQuestionHandler_Create_InternalError(t *testing.T) {
	// Arrange
	h := NewQuestionHandler(&mockQuestionService{err: fmt.Errorf("ai exploded")})
	body, _ := json.Marshal(model.CreateQuestionRequest{Title: "Test", Body: "Body"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/questions", bytes.NewReader(body))
	req = withUserID(req, "user-1")
	w := httptest.NewRecorder()

	// Act
	h.Create(w, req)

	// Assert
	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", w.Code)
	}
}

func TestQuestionHandler_Get_Success(t *testing.T) {
	// Arrange
	h := NewQuestionHandler(&mockQuestionService{})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/questions/q-1", nil)
	req = withChiParam(req, "id", "q-1")
	w := httptest.NewRecorder()

	// Act
	h.Get(w, req)

	// Assert
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestQuestionHandler_Get_NotFound(t *testing.T) {
	// Arrange
	h := NewQuestionHandler(&mockQuestionService{err: service.ErrQuestionMissing})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/questions/nonexistent", nil)
	req = withChiParam(req, "id", "nonexistent")
	w := httptest.NewRecorder()

	// Act
	h.Get(w, req)

	// Assert
	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestQuestionHandler_Argue_Success(t *testing.T) {
	// Arrange
	h := NewQuestionHandler(&mockQuestionService{})
	body, _ := json.Marshal(model.ArgueRequest{Argument: "You're wrong!"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/questions/q-1/argue", bytes.NewReader(body))
	req = withUserID(req, "user-1")
	req = withChiParam(req, "id", "q-1")
	w := httptest.NewRecorder()

	// Act
	h.Argue(w, req)

	// Assert
	if w.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d", w.Code)
	}
}

func TestQuestionHandler_Argue_InvalidJSON(t *testing.T) {
	// Arrange
	h := NewQuestionHandler(&mockQuestionService{})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/questions/q-1/argue", bytes.NewReader([]byte("bad")))
	req = withUserID(req, "user-1")
	req = withChiParam(req, "id", "q-1")
	w := httptest.NewRecorder()

	// Act
	h.Argue(w, req)

	// Assert
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestQuestionHandler_Create_MissingAuth(t *testing.T) {
	h := NewQuestionHandler(&mockQuestionService{})
	body, _ := json.Marshal(model.CreateQuestionRequest{Title: "Test", Body: "Test body"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/questions", bytes.NewReader(body))
	w := httptest.NewRecorder()

	h.Create(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestQuestionHandler_Argue_MissingAuth(t *testing.T) {
	h := NewQuestionHandler(&mockQuestionService{})
	body, _ := json.Marshal(model.ArgueRequest{Argument: "wrong!"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/questions/q-1/argue", bytes.NewReader(body))
	req = withChiParam(req, "id", "q-1")
	w := httptest.NewRecorder()

	h.Argue(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestQuestionHandler_Argue_QuestionNotFound(t *testing.T) {
	// Arrange
	h := NewQuestionHandler(&mockQuestionService{err: service.ErrQuestionMissing})
	body, _ := json.Marshal(model.ArgueRequest{Argument: "wrong!"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/questions/nonexistent/argue", bytes.NewReader(body))
	req = withUserID(req, "user-1")
	req = withChiParam(req, "id", "nonexistent")
	w := httptest.NewRecorder()

	// Act
	h.Argue(w, req)

	// Assert
	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestQuestionHandler_List_Success(t *testing.T) {
	h := NewQuestionHandler(&mockQuestionService{})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/questions?sort=newest&limit=10&offset=0", nil)
	w := httptest.NewRecorder()

	h.List(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var result model.QuestionListResponse
	if err := json.NewDecoder(w.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}
	if len(result.Questions) != 2 {
		t.Errorf("expected 2 questions, got %d", len(result.Questions))
	}
	if result.Total != 10 {
		t.Errorf("expected total 10, got %d", result.Total)
	}
	if result.Sort != "newest" {
		t.Errorf("expected sort 'newest', got %s", result.Sort)
	}
}

func TestQuestionHandler_List_DefaultSort(t *testing.T) {
	h := NewQuestionHandler(&mockQuestionService{})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/questions", nil)
	w := httptest.NewRecorder()

	h.List(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestQuestionHandler_List_InternalError(t *testing.T) {
	h := NewQuestionHandler(&mockQuestionService{err: fmt.Errorf("db error")})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/questions?sort=newest", nil)
	w := httptest.NewRecorder()

	h.List(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", w.Code)
	}
}
