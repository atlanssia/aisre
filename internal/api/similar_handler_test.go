package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/atlanssia/aisre/internal/contract"
)

type mockSimilarService struct {
	results       []contract.SimilarResult
	computeErr    error
	findErr       error
	computeCalled bool
	findCalled    bool
}

func (m *mockSimilarService) ComputeEmbedding(ctx context.Context, incidentID int64) error {
	m.computeCalled = true
	return m.computeErr
}

func (m *mockSimilarService) FindSimilar(ctx context.Context, incidentID int64, topK int, threshold float64) ([]contract.SimilarResult, error) {
	m.findCalled = true
	return m.results, m.findErr
}

func setupSimilarRouter(mock *mockSimilarService) http.Handler {
	r := chi.NewRouter()
	h := &handler{similarSvc: mock}

	r.Route("/api/v1", func(r chi.Router) {
		r.Use(contentTypeJSON)
		r.Get("/incidents/{id}/similar", h.getSimilar)
		r.Post("/incidents/{id}/embed", h.computeEmbedding)
	})

	return r
}

// ==================== GetSimilar Tests ====================

func TestSimilarHandler_GetSimilar_Success(t *testing.T) {
	mock := &mockSimilarService{
		results: []contract.SimilarResult{
			{IncidentID: 2, Similarity: 0.95, Service: "api-gateway", Severity: "high", Summary: "db timeout", RootCause: "conn pool"},
			{IncidentID: 3, Similarity: 0.82, Service: "api-gateway", Severity: "medium", Summary: "slow query", RootCause: "missing index"},
		},
	}
	router := setupSimilarRouter(mock)

	req := httptest.NewRequest("GET", "/api/v1/incidents/1/similar", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	if !mock.findCalled {
		t.Error("expected FindSimilar to be called")
	}

	var results []contract.SimilarResult
	if err := json.Unmarshal(w.Body.Bytes(), &results); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("expected 2 results, got %d", len(results))
	}
	if results[0].IncidentID != 2 {
		t.Errorf("expected incident_id 2, got %d", results[0].IncidentID)
	}
}

func TestSimilarHandler_GetSimilar_EmptyResults(t *testing.T) {
	mock := &mockSimilarService{
		results: []contract.SimilarResult{},
	}
	router := setupSimilarRouter(mock)

	req := httptest.NewRequest("GET", "/api/v1/incidents/1/similar", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// Should return empty array, not null
	if w.Body.String() == "null" {
		t.Error("expected empty array, got null")
	}

	var results []contract.SimilarResult
	if err := json.Unmarshal(w.Body.Bytes(), &results); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

func TestSimilarHandler_GetSimilar_NilResults(t *testing.T) {
	mock := &mockSimilarService{
		results: nil,
	}
	router := setupSimilarRouter(mock)

	req := httptest.NewRequest("GET", "/api/v1/incidents/1/similar", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// Should return empty array, not null
	if w.Body.String() == "null" {
		t.Error("expected empty array, got null")
	}
}

func TestSimilarHandler_GetSimilar_NilService(t *testing.T) {
	r := chi.NewRouter()
	h := &handler{similarSvc: nil}
	r.Get("/api/v1/incidents/{id}/similar", h.getSimilar)

	req := httptest.NewRequest("GET", "/api/v1/incidents/1/similar", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestSimilarHandler_GetSimilar_InvalidID(t *testing.T) {
	mock := &mockSimilarService{}
	router := setupSimilarRouter(mock)

	req := httptest.NewRequest("GET", "/api/v1/incidents/abc/similar", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}

	if mock.findCalled {
		t.Error("expected FindSimilar NOT to be called")
	}
}

func TestSimilarHandler_GetSimilar_DefaultQueryParams(t *testing.T) {
	mock := &mockSimilarService{
		results: []contract.SimilarResult{},
	}
	router := setupSimilarRouter(mock)

	req := httptest.NewRequest("GET", "/api/v1/incidents/1/similar", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSimilarHandler_GetSimilar_CustomQueryParams(t *testing.T) {
	mock := &mockSimilarService{
		results: []contract.SimilarResult{},
	}
	router := setupSimilarRouter(mock)

	req := httptest.NewRequest("GET", "/api/v1/incidents/1/similar?top_k=10&threshold=0.8", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSimilarHandler_GetSimilar_InvalidTopK(t *testing.T) {
	mock := &mockSimilarService{}
	router := setupSimilarRouter(mock)

	req := httptest.NewRequest("GET", "/api/v1/incidents/1/similar?top_k=abc", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestSimilarHandler_GetSimilar_InvalidThreshold(t *testing.T) {
	mock := &mockSimilarService{}
	router := setupSimilarRouter(mock)

	req := httptest.NewRequest("GET", "/api/v1/incidents/1/similar?threshold=abc", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestSimilarHandler_GetSimilar_ServiceError(t *testing.T) {
	mock := &mockSimilarService{
		findErr: errors.New("embedding not found"),
	}
	router := setupSimilarRouter(mock)

	req := httptest.NewRequest("GET", "/api/v1/incidents/1/similar", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d: %s", w.Code, w.Body.String())
	}
}

// ==================== ComputeEmbedding Tests ====================

func TestSimilarHandler_ComputeEmbedding_Success(t *testing.T) {
	mock := &mockSimilarService{}
	router := setupSimilarRouter(mock)

	req := httptest.NewRequest("POST", "/api/v1/incidents/1/embed", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("expected 204, got %d", w.Code)
	}

	if !mock.computeCalled {
		t.Error("expected ComputeEmbedding to be called")
	}
}

func TestSimilarHandler_ComputeEmbedding_NilService(t *testing.T) {
	r := chi.NewRouter()
	h := &handler{similarSvc: nil}
	r.Post("/api/v1/incidents/{id}/embed", h.computeEmbedding)

	req := httptest.NewRequest("POST", "/api/v1/incidents/1/embed", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestSimilarHandler_ComputeEmbedding_InvalidID(t *testing.T) {
	mock := &mockSimilarService{}
	router := setupSimilarRouter(mock)

	req := httptest.NewRequest("POST", "/api/v1/incidents/abc/embed", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}

	if mock.computeCalled {
		t.Error("expected ComputeEmbedding NOT to be called")
	}
}

func TestSimilarHandler_ComputeEmbedding_ServiceError(t *testing.T) {
	mock := &mockSimilarService{
		computeErr: errors.New("incident not found"),
	}
	router := setupSimilarRouter(mock)

	req := httptest.NewRequest("POST", "/api/v1/incidents/999/embed", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d: %s", w.Code, w.Body.String())
	}
}

// Ensure mockSimilarService satisfies the SimilarService interface.
var _ SimilarService = (*mockSimilarService)(nil)
