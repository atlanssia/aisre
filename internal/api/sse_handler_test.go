package api

import (
	"bufio"
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// createIncidentForSSE is a test helper that creates an incident via the API.
func createIncidentForSSE(t *testing.T, router http.Handler, service, severity string) {
	t.Helper()
	body, _ := json.Marshal(map[string]string{
		"source": "test", "service": service, "severity": severity,
	})
	req := httptest.NewRequest("POST", "/api/v1/incidents", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("create incident: expected 201, got %d: %s", w.Code, w.Body.String())
	}
}

func TestStreamAnalysis_SSEHeaders(t *testing.T) {
	router := setupAPIWithAnalysis(t)
	createIncidentForSSE(t, router, "api-gateway", "high")

	req := httptest.NewRequest("GET", "/api/v1/incidents/1/analyze/stream", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// SSE endpoint should return 200 with correct headers
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	if ct := w.Header().Get("Content-Type"); ct != "text/event-stream" {
		t.Errorf("expected Content-Type 'text/event-stream', got %q", ct)
	}
	if cc := w.Header().Get("Cache-Control"); cc != "no-cache" {
		t.Errorf("expected Cache-Control 'no-cache', got %q", cc)
	}
	if conn := w.Header().Get("Connection"); conn != "keep-alive" {
		t.Errorf("expected Connection 'keep-alive', got %q", conn)
	}
}

func TestStreamAnalysis_InvalidIncidentID(t *testing.T) {
	router := setupAPIWithAnalysis(t)

	req := httptest.NewRequest("GET", "/api/v1/incidents/abc/analyze/stream", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Should return JSON error (not SSE) for bad ID
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestStreamAnalysis_IncidentNotFound(t *testing.T) {
	router := setupAPIWithAnalysis(t)

	req := httptest.NewRequest("GET", "/api/v1/incidents/9999/analyze/stream", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// SSE stream starts with 200, then emits an error event for not-found incidents
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for SSE stream, got %d: %s", w.Code, w.Body.String())
	}

	events := parseSSEEvents(w.Body.String())
	hasError := false
	for _, ev := range events {
		if ev.eventType == "error" {
			hasError = true
			if !strings.Contains(ev.data, "analysis failed") {
				t.Errorf("error event should mention 'analysis failed', got: %s", ev.data)
			}
		}
	}
	if !hasError {
		t.Error("expected an error SSE event for non-existent incident")
	}
}

func TestStreamAnalysis_AnalysisServiceNotConfigured(t *testing.T) {
	router := setupAPI(t) // no analysis service

	req := httptest.NewRequest("GET", "/api/v1/incidents/1/analyze/stream", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503, got %d", w.Code)
	}
}

func TestStreamAnalysis_EventSequence(t *testing.T) {
	router := setupAPIWithAnalysis(t)
	createIncidentForSSE(t, router, "api-gateway", "high")

	req := httptest.NewRequest("GET", "/api/v1/incidents/1/analyze/stream", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	events := parseSSEEvents(w.Body.String())
	if len(events) < 3 {
		t.Fatalf("expected at least 3 events, got %d:\n%v", len(events), events)
	}

	// First event should be a status event
	if events[0].eventType != "status" {
		t.Errorf("first event type: expected 'status', got %q", events[0].eventType)
	}

	// Last event should be "complete"
	last := events[len(events)-1]
	if last.eventType != "complete" {
		t.Errorf("last event type: expected 'complete', got %q", last.eventType)
	}

	// The complete event should contain report fields (summary, root_cause, etc.)
	var completeData map[string]any
	if err := json.Unmarshal([]byte(last.data), &completeData); err != nil {
		t.Fatalf("failed to parse complete event data: %v", err)
	}
	if completeData["summary"] == nil {
		t.Error("expected 'summary' in complete event data")
	}
	if completeData["root_cause"] == nil {
		t.Error("expected 'root_cause' in complete event data")
	}
	if completeData["confidence"] == nil {
		t.Error("expected 'confidence' in complete event data")
	}
}

func TestStreamAnalysis_ProgressEvents(t *testing.T) {
	router := setupAPIWithAnalysis(t)
	createIncidentForSSE(t, router, "api-gateway", "high")

	req := httptest.NewRequest("GET", "/api/v1/incidents/1/analyze/stream", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	events := parseSSEEvents(w.Body.String())

	// Should contain at least one progress event
	hasProgress := false
	for _, ev := range events {
		if ev.eventType == "progress" {
			hasProgress = true
			var data map[string]any
			if err := json.Unmarshal([]byte(ev.data), &data); err != nil {
				t.Fatalf("failed to parse progress data: %v", err)
			}
			if data["percent"] == nil {
				t.Error("expected 'percent' in progress event data")
			}
		}
	}
	if !hasProgress {
		t.Error("expected at least one progress event")
	}
}

func TestStreamAnalysis_SSEFormat(t *testing.T) {
	router := setupAPIWithAnalysis(t)
	createIncidentForSSE(t, router, "api-gateway", "high")

	req := httptest.NewRequest("GET", "/api/v1/incidents/1/analyze/stream", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	body := w.Body.String()
	scanner := bufio.NewScanner(strings.NewReader(body))
	lineCount := 0
	for scanner.Scan() {
		line := scanner.Text()
		lineCount++
		// Each non-empty line must start with "event: " or "data: "
		if line != "" && !strings.HasPrefix(line, "event: ") && !strings.HasPrefix(line, "data: ") {
			t.Errorf("line %d: unexpected format: %q", lineCount, line)
		}
	}
}

func TestStreamAnalysis_ContextCancellation(t *testing.T) {
	// Verify the handler respects context cancellation
	router := setupAPIWithAnalysis(t)
	createIncidentForSSE(t, router, "api-gateway", "high")

	req := httptest.NewRequest("GET", "/api/v1/incidents/1/analyze/stream", nil)
	w := httptest.NewRecorder()

	// The httptest.ResponseRecorder does not support real context cancellation,
	// but we verify the handler completes successfully with a normal context.
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}
