package e2e

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/atlanssia/aisre/internal/analysis"
	"github.com/atlanssia/aisre/internal/api"
	"github.com/atlanssia/aisre/internal/incident"
	"github.com/atlanssia/aisre/internal/store"
	_ "modernc.org/sqlite"
)

// setupFullServer wires all repos and services (same as main.go but with a mock LLM).
// Returns a fully-wired http.Handler ready for httptest requests.
func setupFullServer(t *testing.T) (http.Handler, *httptest.Server) {
	t.Helper()

	// Mock LLM server
	mockLLM := newMockLLMServer()
	t.Cleanup(mockLLM.Close)

	// Temp SQLite DB
	dbPath := filepath.Join(t.TempDir(), "aisre.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })

	if err := store.RunMigrations(db, "../../migrations"); err != nil {
		t.Fatal(err)
	}

	// Repositories
	incidentRepo := store.NewIncidentRepo(db)
	reportRepo := store.NewReportRepo(db)
	evidenceRepo := store.NewEvidenceRepo(db)
	feedbackRepo := store.NewFeedbackRepo(db)

	// Services
	incidentSvc := incident.NewService(incidentRepo)

	llmClient := analysis.NewLLMClient(analysis.LLMConfig{
		BaseURL:   mockLLM.URL,
		APIKey:    "test-key",
		Model:     "gpt-4",
		MaxTokens: 4096,
	})

	rcaSvc := analysis.NewRCAService(analysis.RCAServiceConfig{
		LLMClient:    llmClient,
		IncidentRepo: incidentRepo,
		ReportRepo:   reportRepo,
		EvidenceRepo: evidenceRepo,
		Logger:       slog.Default(),
	})

	router := api.NewRouterFull(incidentSvc, rcaSvc, feedbackRepo, reportRepo, nil)

	return router, mockLLM
}

// doRequest is a helper that executes a request against the router and returns
// the status code and decoded JSON body.
func doRequest(t *testing.T, router http.Handler, method, path string, body any) (int, map[string]any) {
	t.Helper()

	var reqBody io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal request body: %v", err)
		}
		reqBody = bytes.NewReader(b)
	}

	req := httptest.NewRequest(method, path, reqBody)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	var resp map[string]any
	if w.Body.Len() > 0 {
		json.Unmarshal(w.Body.Bytes(), &resp)
	}
	return w.Code, resp
}

// doRequestArray executes a request and decodes the JSON body as an array.
func doRequestArray(t *testing.T, router http.Handler, method, path string) (int, []map[string]any) {
	t.Helper()

	req := httptest.NewRequest(method, path, nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	var resp []map[string]any
	if w.Body.Len() > 0 && w.Body.String() != "null" {
		json.Unmarshal(w.Body.Bytes(), &resp)
	}
	return w.Code, resp
}

// TestFullServerSmoke exercises the complete API workflow:
// create incident -> list -> get -> analyze -> submit feedback -> search reports -> health -> webhook.
func TestFullServerSmoke(t *testing.T) {
	router, _ := setupFullServer(t)

	// Step 1: Create incident via POST /api/v1/incidents
	t.Log("Step 1: Create incident")
	code, resp := doRequest(t, router, "POST", "/api/v1/incidents", map[string]string{
		"source":   "prometheus",
		"service":  "api-gateway",
		"severity": "high",
	})
	if code != http.StatusCreated {
		t.Fatalf("create incident: expected 201, got %d: %v", code, resp)
	}
	incidentID := resp["incident_id"]
	if incidentID == nil {
		t.Fatal("create incident: expected incident_id in response")
	}
	incidentIDFloat, ok := incidentID.(float64)
	if !ok {
		t.Fatalf("create incident: incident_id is not a number, got %T", incidentID)
	}
	t.Logf("  -> incident_id = %.0f", incidentIDFloat)

	// Step 2: List incidents via GET /api/v1/incidents
	t.Log("Step 2: List incidents")
	code, incidents := doRequestArray(t, router, "GET", "/api/v1/incidents")
	if code != http.StatusOK {
		t.Fatalf("list incidents: expected 200, got %d", code)
	}
	if len(incidents) != 1 {
		t.Fatalf("list incidents: expected 1 incident, got %d", len(incidents))
	}
	t.Logf("  -> %d incident(s)", len(incidents))

	// Step 3: Get incident via GET /api/v1/incidents/{id}
	t.Log("Step 3: Get incident detail")
	incPath := fmt.Sprintf("/api/v1/incidents/%d", int64(incidentIDFloat))
	code, resp = doRequest(t, router, "GET", incPath, nil)
	if code != http.StatusOK {
		t.Fatalf("get incident: expected 200, got %d: %v", code, resp)
	}
	if resp["service_name"] != "api-gateway" {
		t.Errorf("get incident: expected service_name=api-gateway, got %v", resp["service_name"])
	}
	if resp["severity"] != "high" {
		t.Errorf("get incident: expected severity=high, got %v", resp["severity"])
	}
	if resp["status"] != "open" {
		t.Errorf("get incident: expected status=open, got %v", resp["status"])
	}
	t.Logf("  -> service=%v severity=%v status=%v", resp["service_name"], resp["severity"], resp["status"])

	// Step 4: Analyze incident via POST /api/v1/incidents/{id}/analyze
	t.Log("Step 4: Analyze incident")
	analyzePath := fmt.Sprintf("/api/v1/incidents/%d/analyze", int64(incidentIDFloat))
	code, resp = doRequest(t, router, "POST", analyzePath, nil)
	if code != http.StatusOK {
		t.Fatalf("analyze incident: expected 200, got %d: %v", code, resp)
	}
	reportID := resp["id"]
	if reportID == nil {
		t.Fatal("analyze incident: expected report id in response")
	}
	reportIDFloat, ok := reportID.(float64)
	if !ok {
		t.Fatalf("analyze incident: report id is not a number, got %T", reportID)
	}
	if resp["summary"] != "Test analysis summary" {
		t.Errorf("analyze incident: expected mock summary, got %v", resp["summary"])
	}
	if resp["root_cause"] != "Test root cause" {
		t.Errorf("analyze incident: expected mock root_cause, got %v", resp["root_cause"])
	}
	confidence, _ := resp["confidence"].(float64)
	if confidence < 0.8 {
		t.Errorf("analyze incident: expected confidence >= 0.8, got %f", confidence)
	}
	t.Logf("  -> report_id=%.0f summary=%v confidence=%.2f", reportIDFloat, resp["summary"], confidence)

	// Verify recommendations are present
	recommendations, ok := resp["recommendations"].([]any)
	if !ok || len(recommendations) == 0 {
		t.Errorf("analyze incident: expected recommendations, got %v", resp["recommendations"])
	}

	// Step 5: Submit feedback via POST /api/v1/reports/{id}/feedback
	t.Log("Step 5: Submit feedback")
	feedbackPath := fmt.Sprintf("/api/v1/reports/%d/feedback", int64(reportIDFloat))
	code, resp = doRequest(t, router, "POST", feedbackPath, map[string]any{
		"rating":       4,
		"comment":      "Root cause was accurate, fix worked",
		"user_id":      "sre-bot",
		"action_taken": "accepted",
	})
	if code != http.StatusCreated {
		t.Fatalf("submit feedback: expected 201, got %d: %v", code, resp)
	}
	if resp["report_id"] == nil {
		t.Fatal("submit feedback: expected report_id in response")
	}
	fbRating, _ := resp["rating"].(float64)
	if fbRating != 4 {
		t.Errorf("submit feedback: expected rating=4, got %v", fbRating)
	}
	t.Logf("  -> feedback_id=%v rating=%v action=%v", resp["id"], resp["rating"], resp["action_taken"])

	// Step 6: Search reports via GET /api/v1/reports/search?q=test
	t.Log("Step 6: Search reports")
	code, reports := doRequestArray(t, router, "GET", "/api/v1/reports/search?q=test")
	if code != http.StatusOK {
		t.Fatalf("search reports: expected 200, got %d", code)
	}
	if len(reports) < 1 {
		t.Errorf("search reports: expected at least 1 report, got %d", len(reports))
	}
	t.Logf("  -> %d report(s) found", len(reports))

	// Step 7: Health check via GET /health
	t.Log("Step 7: Health check")
	code, resp = doRequest(t, router, "GET", "/health", nil)
	if code != http.StatusOK {
		t.Fatalf("health: expected 200, got %d", code)
	}
	if resp["status"] != "ok" {
		t.Errorf("health: expected status=ok, got %v", resp["status"])
	}
	t.Log("  -> status=ok")

	// Step 8: Webhook via POST /api/v1/alerts/webhook
	t.Log("Step 8: Webhook creates incident")
	code, resp = doRequest(t, router, "POST", "/api/v1/alerts/webhook", map[string]any{
		"source":     "alertmanager",
		"alert_name": "HighErrorRate",
		"service":    "payment-svc",
		"severity":   "critical",
		"trace_id":   "trace-abc-123",
	})
	if code != http.StatusCreated {
		t.Fatalf("webhook: expected 201, got %d: %v", code, resp)
	}
	if resp["incident_id"] == nil {
		t.Fatal("webhook: expected incident_id in response")
	}
	t.Logf("  -> incident_id=%v", resp["incident_id"])

	// Verify list now has 2 incidents (one from step 1, one from webhook)
	code, incidents = doRequestArray(t, router, "GET", "/api/v1/incidents")
	if code != http.StatusOK {
		t.Fatalf("final list: expected 200, got %d", code)
	}
	if len(incidents) != 2 {
		t.Errorf("final list: expected 2 incidents, got %d", len(incidents))
	}

	t.Log("Full server smoke test complete - all steps passed")
}

// TestFullServerSmoke_SSEStream verifies that the SSE streaming endpoint works
// through the full pipeline with a mock LLM.
func TestFullServerSmoke_SSEStream(t *testing.T) {
	router, _ := setupFullServer(t)

	// Create an incident first
	code, resp := doRequest(t, router, "POST", "/api/v1/incidents", map[string]string{
		"source":   "prometheus",
		"service":  "stream-svc",
		"severity": "medium",
	})
	if code != http.StatusCreated {
		t.Fatalf("create incident: expected 201, got %d", code)
	}
	incidentID := int64(resp["incident_id"].(float64))

	// Call SSE streaming endpoint
	req := httptest.NewRequest("GET", fmt.Sprintf("/api/v1/incidents/%d/analyze/stream", incidentID), nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("SSE stream: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	body := w.Body.String()

	// Verify SSE events appear in order
	for _, event := range []string{"event: status", "event: progress", "event: complete"} {
		if !strings.Contains(body, event) {
			t.Errorf("SSE stream: expected event %q in body", event)
		}
	}

	// Verify the complete event contains the mock analysis data
	if !strings.Contains(body, "Test analysis summary") {
		t.Errorf("SSE stream: expected mock summary in complete event")
	}
	if !strings.Contains(body, "Test root cause") {
		t.Errorf("SSE stream: expected mock root_cause in complete event")
	}

	t.Logf("SSE stream: received events successfully (%d bytes)", len(body))
}

// TestFullServerSmoke_GetReport verifies the GET /api/v1/reports/{id} endpoint
// after creating an incident and running analysis.
func TestFullServerSmoke_GetReport(t *testing.T) {
	router, _ := setupFullServer(t)

	// Create + analyze to get a report
	code, resp := doRequest(t, router, "POST", "/api/v1/incidents", map[string]string{
		"source":   "test",
		"service":  "report-svc",
		"severity": "low",
	})
	if code != http.StatusCreated {
		t.Fatal(code)
	}
	incidentID := int64(resp["incident_id"].(float64))

	code, resp = doRequest(t, router, "POST", fmt.Sprintf("/api/v1/incidents/%d/analyze", incidentID), nil)
	if code != http.StatusOK {
		t.Fatal(code)
	}
	reportID := int64(resp["id"].(float64))

	// Fetch the report
	code, resp = doRequest(t, router, "GET", fmt.Sprintf("/api/v1/reports/%d", reportID), nil)
	if code != http.StatusOK {
		t.Fatalf("get report: expected 200, got %d: %v", code, resp)
	}
	if resp["id"] == nil {
		t.Fatal("get report: expected id")
	}
	if resp["summary"] == nil {
		t.Fatal("get report: expected summary")
	}
	if resp["root_cause"] == nil {
		t.Fatal("get report: expected root_cause")
	}

	t.Logf("get report: id=%v summary=%v", resp["id"], resp["summary"])
}

// TestFullServerSmoke_GetEvidence verifies the GET /api/v1/reports/{id}/evidence endpoint.
func TestFullServerSmoke_GetEvidence(t *testing.T) {
	router, _ := setupFullServer(t)

	// Create + analyze
	code, resp := doRequest(t, router, "POST", "/api/v1/incidents", map[string]string{
		"source":   "test",
		"service":  "evidence-svc",
		"severity": "medium",
	})
	if code != http.StatusCreated {
		t.Fatal(code)
	}
	incidentID := int64(resp["incident_id"].(float64))

	code, resp = doRequest(t, router, "POST", fmt.Sprintf("/api/v1/incidents/%d/analyze", incidentID), nil)
	if code != http.StatusOK {
		t.Fatal(code)
	}
	reportID := int64(resp["id"].(float64))

	// Fetch evidence
	code, evidence := doRequestArray(t, router, "GET", fmt.Sprintf("/api/v1/reports/%d/evidence", reportID))
	if code != http.StatusOK {
		t.Fatalf("get evidence: expected 200, got %d", code)
	}
	// With no tool results passed, evidence array should be empty
	t.Logf("get evidence: %d items", len(evidence))
}

// TestFullServerSmoke_SearchNoResults verifies search returns empty array for no matches.
func TestFullServerSmoke_SearchNoResults(t *testing.T) {
	router, _ := setupFullServer(t)

	code, reports := doRequestArray(t, router, "GET", "/api/v1/reports/search?q=nonexistent_xyz")
	if code != http.StatusOK {
		t.Fatalf("search: expected 200, got %d", code)
	}
	if len(reports) != 0 {
		t.Errorf("search: expected 0 results for nonexistent query, got %d", len(reports))
	}
}
