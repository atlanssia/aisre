package api

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/atlanssia/aisre/internal/contract"
	"github.com/atlanssia/aisre/internal/incident"
	"github.com/atlanssia/aisre/internal/store"
	_ "modernc.org/sqlite"
)

// setupAPIWithFeedback creates a router with incident, analysis, and feedback endpoints.
func setupAPIWithFeedback(t *testing.T) http.Handler {
	t.Helper()
	dbPath := t.Name() + ".db"
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = db.Close()
		_ = os.Remove(dbPath)
	})
	if err := store.RunMigrations(db, "../../migrations"); err != nil {
		t.Fatal(err)
	}
	incRepo := store.NewIncidentRepo(db)
	incSvc := incident.NewService(incRepo)
	reportRepo := store.NewReportRepo(db)
	analysisSvc := &mockAnalysisService{
		incRepo:      incRepo,
		reportRepo:   reportRepo,
		evidenceRepo: store.NewEvidenceRepo(db),
	}
	feedbackRepo := store.NewFeedbackRepo(db)
	return NewRouterFull(incSvc, analysisSvc, feedbackRepo, reportRepo, nil)
}

// createIncidentAndReport is a test helper that creates an incident, analyzes it,
// and returns the router, report ID as a string.
func createIncidentAndReport(t *testing.T, router http.Handler) string {
	t.Helper()
	body, _ := json.Marshal(map[string]string{
		"source": "test", "service": "api-gateway", "severity": "high",
	})
	req := httptest.NewRequest("POST", "/api/v1/incidents", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("create incident: expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var incResp map[string]any
	json.Unmarshal(w.Body.Bytes(), &incResp)
	incidentID := jsonFloatToString(incResp["incident_id"])

	req = httptest.NewRequest("POST", "/api/v1/incidents/"+incidentID+"/analyze", nil)
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("analyze incident: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var analysisResp map[string]any
	json.Unmarshal(w.Body.Bytes(), &analysisResp)
	return jsonFloatToString(analysisResp["id"])
}

// ==================== Feedback Tests ====================

func TestSubmitFeedback(t *testing.T) {
	router := setupAPIWithFeedback(t)
	reportID := createIncidentAndReport(t, router)

	body, _ := json.Marshal(map[string]any{
		"rating":       5,
		"comment":      "Very helpful analysis",
		"user_id":      "user-123",
		"action_taken": "accepted",
	})
	req := httptest.NewRequest("POST", "/api/v1/reports/"+reportID+"/feedback", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var resp contract.FeedbackResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.ID == 0 {
		t.Error("expected non-zero id")
	}
	if resp.ReportID == 0 {
		t.Error("expected non-zero report_id")
	}
	if resp.Rating != 5 {
		t.Errorf("expected rating 5, got %d", resp.Rating)
	}
	if resp.Comment != "Very helpful analysis" {
		t.Errorf("expected comment, got %s", resp.Comment)
	}
	if resp.UserID != "user-123" {
		t.Errorf("expected user_id user-123, got %s", resp.UserID)
	}
}

func TestSubmitFeedback_InvalidRating_TooLow(t *testing.T) {
	router := setupAPIWithFeedback(t)
	reportID := createIncidentAndReport(t, router)

	body, _ := json.Marshal(map[string]any{
		"rating":  0,
		"comment": "bad",
		"user_id": "user-1",
	})
	req := httptest.NewRequest("POST", "/api/v1/reports/"+reportID+"/feedback", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSubmitFeedback_InvalidRating_TooHigh(t *testing.T) {
	router := setupAPIWithFeedback(t)
	reportID := createIncidentAndReport(t, router)

	body, _ := json.Marshal(map[string]any{
		"rating":  6,
		"comment": "great",
		"user_id": "user-1",
	})
	req := httptest.NewRequest("POST", "/api/v1/reports/"+reportID+"/feedback", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSubmitFeedback_InvalidReportID(t *testing.T) {
	router := setupAPIWithFeedback(t)

	body, _ := json.Marshal(map[string]any{
		"rating":  3,
		"comment": "ok",
		"user_id": "user-1",
	})
	req := httptest.NewRequest("POST", "/api/v1/reports/abc/feedback", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestSubmitFeedback_InvalidBody(t *testing.T) {
	router := setupAPIWithFeedback(t)
	reportID := createIncidentAndReport(t, router)

	req := httptest.NewRequest("POST", "/api/v1/reports/"+reportID+"/feedback", bytes.NewReader([]byte("not json")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestSubmitFeedback_ReportNotFound(t *testing.T) {
	router := setupAPIWithFeedback(t)

	body, _ := json.Marshal(map[string]any{
		"rating":  3,
		"comment": "ok",
		"user_id": "user-1",
	})
	req := httptest.NewRequest("POST", "/api/v1/reports/9999/feedback", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

// ==================== Search Reports Tests ====================

func TestSearchReports(t *testing.T) {
	router := setupAPIWithFeedback(t)
	_ = createIncidentAndReport(t, router)

	req := httptest.NewRequest("GET", "/api/v1/reports/search?q=Database", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var results []map[string]any
	json.Unmarshal(w.Body.Bytes(), &results)
	if len(results) == 0 {
		t.Error("expected at least one search result")
	}
}

func TestSearchReports_NoResults(t *testing.T) {
	router := setupAPIWithFeedback(t)

	req := httptest.NewRequest("GET", "/api/v1/reports/search?q=nonexistent_keyword_xyz", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	if w.Body.String() == "null" {
		t.Error("expected empty array, got null")
	}
}

func TestSearchReports_WithFilters(t *testing.T) {
	router := setupAPIWithFeedback(t)
	_ = createIncidentAndReport(t, router)

	req := httptest.NewRequest("GET", "/api/v1/reports/search?q=Database&service=api-gateway&severity=high&limit=10&offset=0", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var results []map[string]any
	json.Unmarshal(w.Body.Bytes(), &results)
	if len(results) == 0 {
		t.Error("expected at least one search result with filters")
	}
}

func TestSearchReports_ServiceFilter_NoMatch(t *testing.T) {
	router := setupAPIWithFeedback(t)
	_ = createIncidentAndReport(t, router)

	req := httptest.NewRequest("GET", "/api/v1/reports/search?q=Database&service=nonexistent-service", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var results []map[string]any
	json.Unmarshal(w.Body.Bytes(), &results)
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

func TestSearchReports_MissingQuery(t *testing.T) {
	router := setupAPIWithFeedback(t)

	req := httptest.NewRequest("GET", "/api/v1/reports/search", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSearchReports_InvalidLimit(t *testing.T) {
	router := setupAPIWithFeedback(t)

	req := httptest.NewRequest("GET", "/api/v1/reports/search?q=test&limit=abc", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestSearchReports_InvalidOffset(t *testing.T) {
	router := setupAPIWithFeedback(t)

	req := httptest.NewRequest("GET", "/api/v1/reports/search?q=test&offset=abc", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

// Ensure mockAnalysisService still compiles with the interface.
var _ AnalysisService = (*mockAnalysisService)(nil)

// Ensure the full-featured router is usable.
func TestFullRouter_HealthCheck(t *testing.T) {
	router := setupAPIWithFeedback(t)

	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}
