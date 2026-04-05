// Package testkit provides shared test utilities for the aisre project.
package testkit

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/atlanssia/aisre/internal/analysis"
	"github.com/atlanssia/aisre/internal/store"
	_ "modernc.org/sqlite"
)

// MockRCAOutput returns a valid RCA JSON output for mock LLM servers.
var MockRCAOutput = map[string]any{
	"summary":      "Test analysis summary",
	"root_cause":   "Test root cause",
	"confidence":   0.85,
	"hypotheses":   []any{},
	"evidence_ids": []any{},
	"blast_radius": []any{},
	"actions": map[string]any{
		"immediate":  []string{"Check service health"},
		"fix":        []string{"Review recent deployments"},
		"prevention": []string{"Add monitoring"},
	},
	"timeline": []any{
		map[string]any{
			"time":        "2025-01-15T10:00:00Z",
			"type":        "alert",
			"service":     "test-service",
			"description": "High error rate detected",
			"severity":    "critical",
		},
	},
	"uncertainties": []any{},
}

// NewTestDB creates an in-memory SQLite database with all migrations applied.
// It returns the *sql.DB and a cleanup function that should be deferred.
func NewTestDB(t *testing.T) (*sql.DB, func()) {
	t.Helper()

	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("testkit: open in-memory sqlite: %v", err)
	}

	migrationsPath := resolveMigrationsPath(t)
	if err := store.RunMigrations(db, migrationsPath); err != nil {
		_ = db.Close()
		t.Fatalf("testkit: run migrations: %v", err)
	}

	return db, func() { _ = db.Close() }
}

// resolveMigrationsPath locates the migrations directory relative to this file.
func resolveMigrationsPath(t *testing.T) string {
	t.Helper()

	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("testkit: cannot determine source file location")
	}
	return filepath.Join(filepath.Dir(thisFile), "..", "..", "migrations")
}

// NewTestIncident creates a sample store.Incident with reasonable defaults.
func NewTestIncident(t *testing.T) *store.Incident {
	t.Helper()
	return &store.Incident{
		Source:      "prometheus",
		ServiceName: "api-gateway",
		Severity:    "high",
		Status:      "open",
		TraceID:     "trace-test-001",
	}
}

// NewTestReport creates a sample store.Report for the given incidentID.
func NewTestReport(t *testing.T, incidentID int64) *store.Report {
	t.Helper()
	return &store.Report{
		IncidentID: incidentID,
		Summary:    "Test analysis summary",
		RootCause:  "Test root cause",
		Confidence: 0.85,
		ReportJSON: `{"summary":"Test analysis summary","root_cause":"Test root cause","confidence":0.85}`,
		Status:     "completed",
	}
}

// NewMockLLMServer creates an httptest.Server that simulates an OpenAI-compatible
// chat completions endpoint. The caller must close the server (typically via t.Cleanup).
func NewMockLLMServer() *httptest.Server {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		if r.URL.Path != "/v1/chat/completions" {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		rcaJSON, _ := json.Marshal(MockRCAOutput)

		resp := map[string]any{
			"id": "chatcmpl-testkit-mock",
			"choices": []map[string]any{
				{
					"message": map[string]any{
						"role":    "assistant",
						"content": string(rcaJSON),
					},
					"finish_reason": "stop",
				},
			},
			"usage": map[string]any{
				"prompt_tokens":     100,
				"completion_tokens": 50,
				"total_tokens":      150,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	})
	return httptest.NewServer(handler)
}

// NewMockLLMClient creates an analysis.LLMClient backed by a mock server.
// The server is automatically cleaned up via t.Cleanup.
func NewMockLLMClient(t *testing.T) *analysis.LLMClient {
	t.Helper()

	server := NewMockLLMServer()
	t.Cleanup(server.Close)

	return analysis.NewLLMClient(analysis.LLMConfig{
		BaseURL:   server.URL,
		APIKey:    "test-key",
		Model:     "gpt-4",
		MaxTokens: 4096,
	})
}

// WaitFor repeatedly polls condition until true or timeout elapses.
func WaitFor(t *testing.T, condition func() bool, timeout, interval time.Duration) {
	t.Helper()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(interval)
	}
	t.Fatalf("testkit: WaitFor condition not met within %s", timeout)
}

// SetupFullDB creates a test database with all repos. Cleanup via t.Cleanup.
func SetupFullDB(t *testing.T) (
	*sql.DB,
	store.IncidentRepo,
	store.ReportRepo,
	store.EvidenceRepo,
	store.FeedbackRepo,
) {
	t.Helper()

	db, cleanup := NewTestDB(t)
	t.Cleanup(cleanup)

	return db,
		store.NewIncidentRepo(db),
		store.NewReportRepo(db),
		store.NewEvidenceRepo(db),
		store.NewFeedbackRepo(db)
}

// SeedIncident creates a test incident in the database and returns the ID.
func SeedIncident(t *testing.T, ctx context.Context, repo store.IncidentRepo) int64 {
	t.Helper()

	inc := NewTestIncident(t)
	id, err := repo.Create(ctx, inc)
	if err != nil {
		t.Fatalf("testkit: seed incident: %v", err)
	}
	return id
}
