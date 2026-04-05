package api

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/atlanssia/aisre/internal/contract"
	"github.com/atlanssia/aisre/internal/incident"
	"github.com/atlanssia/aisre/internal/store"
	_ "modernc.org/sqlite"
)

// mockAnalysisService implements AnalysisService for API-level testing.
type mockAnalysisService struct {
	incRepo      store.IncidentRepo
	reportRepo   store.ReportRepo
	evidenceRepo store.EvidenceRepo
}

func (m *mockAnalysisService) AnalyzeIncident(ctx context.Context, incidentID int64) (*contract.ReportResponse, error) {
	inc, err := m.incRepo.GetByID(ctx, incidentID)
	if err != nil {
		return nil, fmt.Errorf("incident not found: %w", err)
	}

	summary := "Database connection pool exhaustion in " + inc.ServiceName
	rootCause := "Connection leak in authentication module"
	confidence := 0.85

	report := &store.Report{
		IncidentID: incidentID,
		Summary:    summary,
		RootCause:  rootCause,
		Confidence: confidence,
		ReportJSON: `{"summary":"mock analysis"}`,
	}
	id, err := m.reportRepo.Create(ctx, report)
	if err != nil {
		return nil, err
	}

	// Save mock evidence
	_, _ = m.evidenceRepo.Create(ctx, &store.Evidence{
		ReportID:     id,
		EvidenceType: "log",
		Score:        0.9,
		Payload:      `{"message":"connection refused"}`,
	})
	_, _ = m.evidenceRepo.Create(ctx, &store.Evidence{
		ReportID:     id,
		EvidenceType: "trace",
		Score:        0.75,
		Payload:      `{"duration_ms":5000}`,
	})

	return &contract.ReportResponse{
		ID:         id,
		IncidentID: incidentID,
		Summary:    summary,
		RootCause:  rootCause,
		Confidence: confidence,
		Evidence: []contract.EvidenceItem{
			{ID: "ev_001", Type: "log", Score: 0.9, Summary: "connection refused"},
			{ID: "ev_002", Type: "trace", Score: 0.75, Summary: "slow span"},
		},
		Recommendations: []string{"Restart service pods", "Fix connection leak"},
		CreatedAt:       "2025-01-15T10:30:00Z",
	}, nil
}

func (m *mockAnalysisService) GetReport(ctx context.Context, reportID int64) (*contract.ReportResponse, error) {
	report, err := m.reportRepo.GetByID(ctx, reportID)
	if err != nil {
		return nil, err
	}
	evidenceItems, _ := m.evidenceRepo.ListByReport(ctx, reportID)

	evidence := make([]contract.EvidenceItem, len(evidenceItems))
	for i, ev := range evidenceItems {
		var payload map[string]any
		if ev.Payload != "" {
			json.Unmarshal([]byte(ev.Payload), &payload)
		}
		evidence[i] = contract.EvidenceItem{
			Type: ev.EvidenceType, Score: ev.Score, Payload: payload,
		}
	}

	return &contract.ReportResponse{
		ID:         report.ID,
		IncidentID: report.IncidentID,
		Summary:    report.Summary,
		RootCause:  report.RootCause,
		Confidence: report.Confidence,
		Evidence:   evidence,
		CreatedAt:  report.CreatedAt,
	}, nil
}

func (m *mockAnalysisService) GetEvidence(ctx context.Context, reportID int64) ([]contract.EvidenceItem, error) {
	items, err := m.evidenceRepo.ListByReport(ctx, reportID)
	if err != nil {
		return nil, err
	}
	result := make([]contract.EvidenceItem, len(items))
	for i, ev := range items {
		var payload map[string]any
		if ev.Payload != "" {
			json.Unmarshal([]byte(ev.Payload), &payload)
		}
		result[i] = contract.EvidenceItem{
			Type: ev.EvidenceType, Score: ev.Score, Payload: payload,
		}
	}
	return result, nil
}

func setupAPIWithAnalysis(t *testing.T) http.Handler {
	t.Helper()
	dbPath := t.Name() + ".db"
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		db.Close()
		_ = os.Remove(dbPath)
	})
	if err := store.RunMigrations(db, "../../migrations"); err != nil {
		t.Fatal(err)
	}
	incRepo := store.NewIncidentRepo(db)
	incSvc := incident.NewService(incRepo)
	analysisSvc := &mockAnalysisService{
		incRepo:      incRepo,
		reportRepo:   store.NewReportRepo(db),
		evidenceRepo: store.NewEvidenceRepo(db),
	}
	return NewRouterWithAnalysis(incSvc, analysisSvc)
}

func TestAnalyzeIncident(t *testing.T) {
	router := setupAPIWithAnalysis(t)

	// Create an incident first
	body, _ := json.Marshal(map[string]string{
		"source": "test", "service": "api-gateway", "severity": "high",
	})
	req := httptest.NewRequest("POST", "/api/v1/incidents", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	var incResp map[string]any
	json.Unmarshal(w.Body.Bytes(), &incResp)
	incidentID := jsonFloatToString(incResp["incident_id"])

	// Analyze the incident
	req = httptest.NewRequest("POST", "/api/v1/incidents/"+incidentID+"/analyze", nil)
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["summary"] == nil {
		t.Error("expected summary in response")
	}
	if resp["root_cause"] == nil {
		t.Error("expected root_cause in response")
	}
	if resp["confidence"] == nil {
		t.Error("expected confidence in response")
	}
	if resp["id"] == nil {
		t.Error("expected id in response")
	}
}

func TestAnalyzeIncident_NotFound(t *testing.T) {
	router := setupAPIWithAnalysis(t)

	req := httptest.NewRequest("POST", "/api/v1/incidents/9999/analyze", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestAnalyzeIncident_InvalidID(t *testing.T) {
	router := setupAPIWithAnalysis(t)

	req := httptest.NewRequest("POST", "/api/v1/incidents/abc/analyze", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestGetReport(t *testing.T) {
	router := setupAPIWithAnalysis(t)

	// Create incident + analyze
	body, _ := json.Marshal(map[string]string{
		"source": "test", "service": "svc", "severity": "low",
	})
	req := httptest.NewRequest("POST", "/api/v1/incidents", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	var incResp map[string]any
	json.Unmarshal(w.Body.Bytes(), &incResp)
	incidentID := jsonFloatToString(incResp["incident_id"])

	// Analyze to create a report
	req = httptest.NewRequest("POST", "/api/v1/incidents/"+incidentID+"/analyze", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	var analysisResp map[string]any
	json.Unmarshal(w.Body.Bytes(), &analysisResp)
	reportID := jsonFloatToString(analysisResp["id"])

	// Get the report
	req = httptest.NewRequest("GET", "/api/v1/reports/"+reportID, nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var report map[string]any
	json.Unmarshal(w.Body.Bytes(), &report)
	if report["summary"] == nil {
		t.Error("expected summary in report")
	}
	if report["evidence"] == nil {
		t.Error("expected evidence in report")
	}
}

func TestGetReport_NotFound(t *testing.T) {
	router := setupAPIWithAnalysis(t)

	req := httptest.NewRequest("GET", "/api/v1/reports/9999", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestGetReport_InvalidID(t *testing.T) {
	router := setupAPIWithAnalysis(t)

	req := httptest.NewRequest("GET", "/api/v1/reports/abc", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestGetEvidence(t *testing.T) {
	router := setupAPIWithAnalysis(t)

	// Create incident + analyze
	body, _ := json.Marshal(map[string]string{
		"source": "test", "service": "svc", "severity": "low",
	})
	req := httptest.NewRequest("POST", "/api/v1/incidents", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	var incResp map[string]any
	json.Unmarshal(w.Body.Bytes(), &incResp)
	incidentID := jsonFloatToString(incResp["incident_id"])

	// Analyze to create a report with evidence
	req = httptest.NewRequest("POST", "/api/v1/incidents/"+incidentID+"/analyze", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	var analysisResp map[string]any
	json.Unmarshal(w.Body.Bytes(), &analysisResp)
	reportID := jsonFloatToString(analysisResp["id"])

	// Get evidence
	req = httptest.NewRequest("GET", "/api/v1/reports/"+reportID+"/evidence", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var evidence []map[string]any
	json.Unmarshal(w.Body.Bytes(), &evidence)
	if len(evidence) == 0 {
		t.Error("expected at least one evidence item")
	}
}

func TestGetEvidence_ReportNotFound(t *testing.T) {
	router := setupAPIWithAnalysis(t)

	req := httptest.NewRequest("GET", "/api/v1/reports/9999/evidence", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Should return empty array for non-existent report (no error)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestGetEvidence_InvalidReportID(t *testing.T) {
	router := setupAPIWithAnalysis(t)

	req := httptest.NewRequest("GET", "/api/v1/reports/abc/evidence", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

// jsonFloatToString converts a JSON number (float64) to its string representation.
func jsonFloatToString(v any) string {
	switch val := v.(type) {
	case float64:
		return fmt.Sprintf("%.0f", val)
	case string:
		return val
	default:
		return fmt.Sprintf("%v", val)
	}
}
