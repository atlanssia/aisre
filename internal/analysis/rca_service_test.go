package analysis

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/atlanssia/aisre/internal/contract"
	"github.com/atlanssia/aisre/internal/store"
	_ "modernc.org/sqlite"
)

// setupRCATest creates a test database with migrations and all repos.
func setupRCATest(t *testing.T) (*sql.DB, store.IncidentRepo, store.ReportRepo, store.EvidenceRepo) {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		db.Close()
	})
	if err := store.RunMigrations(db, "../../migrations"); err != nil {
		t.Fatal(err)
	}
	return db, store.NewIncidentRepo(db), store.NewReportRepo(db), store.NewEvidenceRepo(db)
}

func TestRCAService_AnalyzeIncident(t *testing.T) {
	_, incRepo, reportRepo, evidenceRepo := setupRCATest(t)
	ctx := context.Background()

	// Create an incident
	incID, err := incRepo.Create(ctx, &store.Incident{
		Source:      "prometheus",
		ServiceName: "api-gateway",
		Severity:    "critical",
		Status:      "open",
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Run("successful analysis with mock LLM", func(t *testing.T) {
		// Mock LLM server
		rcaOutput := RCAOutput{
			Summary:     "Database connection pool exhaustion causing service degradation",
			RootCause:   "Connection leak in user-service auth module",
			Confidence:  0.92,
			EvidenceIDs: []string{"ev_001", "ev_002"},
			Hypotheses: []Hypothesis{
				{ID: "h1", Description: "Connection leak", Likelihood: 0.9, EvidenceIDs: []string{"ev_001"}},
			},
			BlastRadius: []string{"user-service", "api-gateway"},
			Actions: Actions{
				Immediate: []string{"Restart user-service pods"},
				ShortTerm: []string{"Fix connection leak in auth module"},
				LongTerm:  []string{"Add connection pool monitoring"},
			},
			Uncertainties: []string{"Cannot confirm if traffic spike is correlated"},
		}
		rcaJSON, _ := json.Marshal(rcaOutput)

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			resp := map[string]any{
				"id": "chatcmpl-123",
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
			json.NewEncoder(w).Encode(resp)
		}))
		defer server.Close()

		llmClient := NewLLMClient(LLMConfig{
			BaseURL:   server.URL,
			APIKey:    "test-key",
			Model:     "gpt-4",
			MaxTokens: 4096,
		})

		svc := NewRCAService(RCAServiceConfig{
			LLMClient:    llmClient,
			IncidentRepo: incRepo,
			ReportRepo:   reportRepo,
			EvidenceRepo: evidenceRepo,
			Logger:       slog.Default(),
		})

		// Mock tool results that would come from the tool orchestrator
		toolResults := []contract.ToolResult{
			{Name: "log_search", Summary: "connection refused errors in user-service", Score: 0.95, Payload: map[string]any{"count": 150}},
			{Name: "trace_analysis", Summary: "slow span in /api/users endpoint", Score: 0.8, Payload: map[string]any{"duration_ms": 5000}},
		}

		report, err := svc.AnalyzeIncidentWithEvidence(ctx, incID, toolResults)
		if err != nil {
			t.Fatal(err)
		}

		if report.ID <= 0 {
			t.Error("expected positive report ID")
		}
		if report.Summary == "" {
			t.Error("expected non-empty summary")
		}
		if report.RootCause == "" {
			t.Error("expected non-empty root cause")
		}
		if report.Confidence < 0.9 {
			t.Errorf("expected confidence >= 0.9, got %f", report.Confidence)
		}
	})

	t.Run("incident not found", func(t *testing.T) {
		llmClient := NewLLMClient(LLMConfig{
			BaseURL: "http://localhost",
			APIKey:  "test",
			Model:   "gpt-4",
		})

		svc := NewRCAService(RCAServiceConfig{
			LLMClient:    llmClient,
			IncidentRepo: incRepo,
			ReportRepo:   reportRepo,
			EvidenceRepo: evidenceRepo,
			Logger:       slog.Default(),
		})

		_, err := svc.AnalyzeIncident(ctx, 9999)
		if err == nil {
			t.Error("expected error for non-existent incident")
		}
	})

	t.Run("LLM failure", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(`{"error":{"message":"internal server error"}}`))
		}))
		defer server.Close()

		llmClient := NewLLMClient(LLMConfig{
			BaseURL: server.URL,
			APIKey:  "test-key",
			Model:   "gpt-4",
		})

		svc := NewRCAService(RCAServiceConfig{
			LLMClient:    llmClient,
			IncidentRepo: incRepo,
			ReportRepo:   reportRepo,
			EvidenceRepo: evidenceRepo,
			Logger:       slog.Default(),
		})

		_, err := svc.AnalyzeIncidentWithEvidence(ctx, incID, []contract.ToolResult{
			{Name: "test", Summary: "test", Score: 0.5},
		})
		if err == nil {
			t.Error("expected error for LLM failure")
		}
	})
}

func TestRCAService_GetReport(t *testing.T) {
	t.Run("retrieves report with evidence", func(t *testing.T) {
		// Create via repos directly
		_, incRepo, reportRepo, evidenceRepo := setupRCATest(t)
		ctx := context.Background()

		incID, _ := incRepo.Create(ctx, &store.Incident{
			Source: "test", ServiceName: "svc", Severity: "high", Status: "open",
		})

		reportID, _ := reportRepo.Create(ctx, &store.Report{
			IncidentID: incID,
			Summary:    "test summary",
			RootCause:  "test cause",
			Confidence: 0.85,
			ReportJSON: `{"summary":"test"}`,
		})

		evidenceRepo.Create(ctx, &store.Evidence{
			ReportID:     reportID,
			EvidenceType: "log",
			Score:        0.9,
			Payload:      `{"message":"error"}`,
		})

		llmClient := NewLLMClient(LLMConfig{
			BaseURL: "http://localhost",
			APIKey:  "test",
			Model:   "gpt-4",
		})

		svc := NewRCAService(RCAServiceConfig{
			LLMClient:    llmClient,
			IncidentRepo: incRepo,
			ReportRepo:   reportRepo,
			EvidenceRepo: evidenceRepo,
			Logger:       slog.Default(),
		})

		resp, err := svc.GetReport(ctx, reportID)
		if err != nil {
			t.Fatal(err)
		}
		if resp.ID != reportID {
			t.Errorf("expected report ID %d, got %d", reportID, resp.ID)
		}
		if resp.Summary != "test summary" {
			t.Errorf("expected 'test summary', got %s", resp.Summary)
		}
		if len(resp.Evidence) != 1 {
			t.Errorf("expected 1 evidence item, got %d", len(resp.Evidence))
		}
	})

	t.Run("report not found", func(t *testing.T) {
		_, incRepo, reportRepo, evidenceRepo := setupRCATest(t)
		ctx := context.Background()

		llmClient := NewLLMClient(LLMConfig{
			BaseURL: "http://localhost",
			APIKey:  "test",
			Model:   "gpt-4",
		})

		svc := NewRCAService(RCAServiceConfig{
			LLMClient:    llmClient,
			IncidentRepo: incRepo,
			ReportRepo:   reportRepo,
			EvidenceRepo: evidenceRepo,
			Logger:       slog.Default(),
		})

		_, err := svc.GetReport(ctx, 9999)
		if err == nil {
			t.Error("expected error for non-existent report")
		}
	})
}

func TestRCAService_GetEvidence(t *testing.T) {
	_, incRepo, reportRepo, evidenceRepo := setupRCATest(t)
	ctx := context.Background()

	incID, _ := incRepo.Create(ctx, &store.Incident{
		Source: "test", ServiceName: "svc", Severity: "low", Status: "open",
	})

	reportID, _ := reportRepo.Create(ctx, &store.Report{
		IncidentID: incID,
		Summary:    "test",
		Confidence: 0.8,
	})

	evidenceRepo.Create(ctx, &store.Evidence{
		ReportID: reportID, EvidenceType: "log", Score: 0.9, Payload: `{"msg":"err"}`,
	})
	evidenceRepo.Create(ctx, &store.Evidence{
		ReportID: reportID, EvidenceType: "trace", Score: 0.7, Payload: `{"duration":5000}`,
	})

	llmClient := NewLLMClient(LLMConfig{
		BaseURL: "http://localhost", APIKey: "test", Model: "gpt-4",
	})

	svc := NewRCAService(RCAServiceConfig{
		LLMClient:    llmClient,
		IncidentRepo: incRepo,
		ReportRepo:   reportRepo,
		EvidenceRepo: evidenceRepo,
		Logger:       slog.Default(),
	})

	items, err := svc.GetEvidence(ctx, reportID)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 evidence items, got %d", len(items))
	}
	// Should be sorted by score descending
	if items[0].Score < items[1].Score {
		t.Errorf("expected descending order: %f should be >= %f", items[0].Score, items[1].Score)
	}
}

func TestRCAService_AnalyzeIncident_NoToolResults(t *testing.T) {
	_, incRepo, reportRepo, evidenceRepo := setupRCATest(t)
	ctx := context.Background()

	incID, _ := incRepo.Create(ctx, &store.Incident{
		Source: "test", ServiceName: "svc", Severity: "low", Status: "open",
	})

	rcaOutput := RCAOutput{
		Summary:     "Insufficient data for analysis",
		RootCause:   "No evidence collected",
		Confidence:  0.1,
		EvidenceIDs: []string{},
	}
	rcaJSON, _ := json.Marshal(rcaOutput)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{
			"choices": []map[string]any{
				{"message": map[string]any{"role": "assistant", "content": string(rcaJSON)}, "finish_reason": "stop"},
			},
			"usage": map[string]any{"prompt_tokens": 10, "completion_tokens": 5, "total_tokens": 15},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	llmClient := NewLLMClient(LLMConfig{
		BaseURL: server.URL, APIKey: "test", Model: "gpt-4",
	})

	svc := NewRCAService(RCAServiceConfig{
		LLMClient:    llmClient,
		IncidentRepo: incRepo,
		ReportRepo:   reportRepo,
		EvidenceRepo: evidenceRepo,
		Logger:       slog.Default(),
	})

	report, err := svc.AnalyzeIncidentWithEvidence(ctx, incID, nil)
	if err != nil {
		t.Fatal(err)
	}
	if report == nil {
		t.Error("expected report even with no evidence")
	}
	if report.Confidence > 0.5 {
		t.Errorf("expected low confidence with no evidence, got %f", report.Confidence)
	}
}

// Test that RCAService implements the Service interface
func TestRCAService_ImplementsInterface(t *testing.T) {
	var _ Service = (*RCAService)(nil)
}

// Test FormatToolResultsForStorage
func TestFormatToolResultsForStorage(t *testing.T) {
	results := []contract.ToolResult{
		{Name: "log", Summary: "error log", Score: 0.9, Payload: map[string]any{"msg": "test"}},
	}
	data, err := json.Marshal(results)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) == 0 {
		t.Error("expected non-empty JSON")
	}
}

// mockOrchestrator implements the ToolOrchestrator interface for testing.
type mockOrchestrator struct {
	results []contract.ToolResult
	err     error
	called  bool
	inc     *contract.Incident
}

func (m *mockOrchestrator) ExecuteAll(ctx context.Context, incident *contract.Incident) ([]contract.ToolResult, error) {
	m.called = true
	m.inc = incident
	return m.results, m.err
}

func TestRCAService_AnalyzeIncident_UsesOrchestrator(t *testing.T) {
	_, incRepo, reportRepo, evidenceRepo := setupRCATest(t)
	ctx := context.Background()

	incID, err := incRepo.Create(ctx, &store.Incident{
		Source:      "prometheus",
		ServiceName: "api-gateway",
		Severity:    "critical",
		Status:      "open",
		TraceID:     "trace-abc-123",
	})
	if err != nil {
		t.Fatal(err)
	}

	rcaOutput := RCAOutput{
		Summary:     "Connection pool exhaustion",
		RootCause:   "Leak in auth module",
		Confidence:  0.92,
		EvidenceIDs: []string{"ev_001"},
		Actions: Actions{
			Immediate: []string{"Restart pods"},
		},
	}
	rcaJSON, _ := json.Marshal(rcaOutput)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{
			"choices": []map[string]any{
				{"message": map[string]any{"role": "assistant", "content": string(rcaJSON)}, "finish_reason": "stop"},
			},
			"usage": map[string]any{"prompt_tokens": 100, "completion_tokens": 50, "total_tokens": 150},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	llmClient := NewLLMClient(LLMConfig{
		BaseURL: server.URL, APIKey: "test", Model: "gpt-4",
	})

	orch := &mockOrchestrator{
		results: []contract.ToolResult{
			{Name: "critical_log_cluster", Summary: "connection refused", Score: 0.95},
			{Name: "slowest_span", Summary: "slow DB query", Score: 0.8},
		},
	}

	svc := NewRCAService(RCAServiceConfig{
		LLMClient:    llmClient,
		IncidentRepo: incRepo,
		ReportRepo:   reportRepo,
		EvidenceRepo: evidenceRepo,
		Orchestrator: orch,
		Logger:       slog.Default(),
	})

	report, err := svc.AnalyzeIncident(ctx, incID)
	if err != nil {
		t.Fatal(err)
	}

	// Verify orchestrator was called
	if !orch.called {
		t.Error("expected orchestrator to be called")
	}
	if orch.inc.ServiceName != "api-gateway" {
		t.Errorf("expected service 'api-gateway', got %q", orch.inc.ServiceName)
	}
	if report.Summary != "Connection pool exhaustion" {
		t.Errorf("unexpected summary: %s", report.Summary)
	}
	if report.Confidence != 0.92 {
		t.Errorf("expected confidence 0.92, got %f", report.Confidence)
	}
	// Evidence should have been saved from orchestrator results
	if len(report.Evidence) != 2 {
		t.Errorf("expected 2 evidence items, got %d", len(report.Evidence))
	}
}

func TestRCAService_AnalyzeIncident_OrchestratorFailure_GracefulDegradation(t *testing.T) {
	_, incRepo, reportRepo, evidenceRepo := setupRCATest(t)
	ctx := context.Background()

	incID, err := incRepo.Create(ctx, &store.Incident{
		Source: "test", ServiceName: "svc", Severity: "low", Status: "open",
	})
	if err != nil {
		t.Fatal(err)
	}

	rcaOutput := RCAOutput{
		Summary:     "Insufficient data",
		RootCause:   "No evidence collected",
		Confidence:  0.1,
		EvidenceIDs: []string{},
	}
	rcaJSON, _ := json.Marshal(rcaOutput)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{
			"choices": []map[string]any{
				{"message": map[string]any{"role": "assistant", "content": string(rcaJSON)}, "finish_reason": "stop"},
			},
			"usage": map[string]any{"prompt_tokens": 10, "completion_tokens": 5, "total_tokens": 15},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	llmClient := NewLLMClient(LLMConfig{
		BaseURL: server.URL, APIKey: "test", Model: "gpt-4",
	})

	orch := &mockOrchestrator{
		err: fmt.Errorf("all tools failed: [logs: connection refused, traces: timeout, metrics: unavailable]"),
	}

	svc := NewRCAService(RCAServiceConfig{
		LLMClient:    llmClient,
		IncidentRepo: incRepo,
		ReportRepo:   reportRepo,
		EvidenceRepo: evidenceRepo,
		Orchestrator: orch,
		Logger:       slog.Default(),
	})

	// Should NOT fail even when orchestrator fails — graceful degradation
	report, err := svc.AnalyzeIncident(ctx, incID)
	if err != nil {
		t.Fatalf("expected graceful degradation, got error: %v", err)
	}
	if report == nil {
		t.Error("expected report even with orchestrator failure")
	}
}

func TestRCAService_AnalyzeIncident_NoOrchestrator(t *testing.T) {
	_, incRepo, reportRepo, evidenceRepo := setupRCATest(t)
	ctx := context.Background()

	incID, err := incRepo.Create(ctx, &store.Incident{
		Source: "test", ServiceName: "svc", Severity: "low", Status: "open",
	})
	if err != nil {
		t.Fatal(err)
	}

	rcaOutput := RCAOutput{
		Summary:     "No evidence available",
		RootCause:   "Unknown",
		Confidence:  0.1,
		EvidenceIDs: []string{},
	}
	rcaJSON, _ := json.Marshal(rcaOutput)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{
			"choices": []map[string]any{
				{"message": map[string]any{"role": "assistant", "content": string(rcaJSON)}, "finish_reason": "stop"},
			},
			"usage": map[string]any{"prompt_tokens": 10, "completion_tokens": 5, "total_tokens": 15},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	llmClient := NewLLMClient(LLMConfig{
		BaseURL: server.URL, APIKey: "test", Model: "gpt-4",
	})

	// No orchestrator set — should still work with nil evidence
	svc := NewRCAService(RCAServiceConfig{
		LLMClient:    llmClient,
		IncidentRepo: incRepo,
		ReportRepo:   reportRepo,
		EvidenceRepo: evidenceRepo,
		// Orchestrator is nil
		Logger: slog.Default(),
	})

	report, err := svc.AnalyzeIncident(ctx, incID)
	if err != nil {
		t.Fatal(err)
	}
	if report == nil {
		t.Error("expected report even without orchestrator")
	}
}

// Test the complete flow with realistic data
func TestRCAService_FullPipeline(t *testing.T) {
	_, incRepo, reportRepo, evidenceRepo := setupRCATest(t)
	ctx := context.Background()

	incID, _ := incRepo.Create(ctx, &store.Incident{
		Source:      "alertmanager",
		ServiceName: "payment-service",
		Severity:    "critical",
		Status:      "open",
		TraceID:     "trace-abc-def",
	})

	// Build a detailed LLM response
	rcaOutput := RCAOutput{
		Summary:     "Payment processing failure due to database connection timeout",
		RootCause:   "Database connection pool exhausted after deployment v2.3.1",
		Confidence:  0.88,
		EvidenceIDs: []string{"ev_001", "ev_002", "ev_003"},
		Hypotheses: []Hypothesis{
			{ID: "h1", Description: "Pool exhaustion from deployment", Likelihood: 0.88, EvidenceIDs: []string{"ev_001", "ev_002"}},
			{ID: "h2", Description: "Network partition to DB", Likelihood: 0.2, EvidenceIDs: []string{"ev_003"}},
		},
		BlastRadius: []string{"payment-service", "order-service", "notification-service"},
		Actions: Actions{
			Immediate: []string{"Rollback deployment v2.3.1", "Increase connection pool to 100"},
			ShortTerm: []string{"Add connection pool metrics", "Review connection lifecycle"},
			LongTerm:  []string{"Implement circuit breaker pattern"},
		},
		Uncertainties: []string{"Network latency measurements unavailable"},
	}
	rcaJSON, _ := json.Marshal(rcaOutput)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{
			"id": fmt.Sprintf("chatcmpl-%d", incID),
			"choices": []map[string]any{
				{"message": map[string]any{"role": "assistant", "content": string(rcaJSON)}, "finish_reason": "stop"},
			},
			"usage": map[string]any{"prompt_tokens": 200, "completion_tokens": 100, "total_tokens": 300},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	llmClient := NewLLMClient(LLMConfig{
		BaseURL: server.URL, APIKey: "test", Model: "gpt-4", MaxTokens: 4096,
	})

	svc := NewRCAService(RCAServiceConfig{
		LLMClient:    llmClient,
		IncidentRepo: incRepo,
		ReportRepo:   reportRepo,
		EvidenceRepo: evidenceRepo,
		Logger:       slog.Default(),
	})

	toolResults := []contract.ToolResult{
		{Name: "log_search", Summary: "Connection timeout to database", Score: 0.95, Payload: map[string]any{"count": 500, "service": "payment-service"}},
		{Name: "trace_analysis", Summary: "Slow DB query in payment processing", Score: 0.85, Payload: map[string]any{"duration_ms": 10000}},
		{Name: "metric_query", Summary: "Connection pool utilization at 100%", Score: 0.78, Payload: map[string]any{"utilization": 100}},
	}

	report, err := svc.AnalyzeIncidentWithEvidence(ctx, incID, toolResults)
	if err != nil {
		t.Fatal(err)
	}

	// Verify report fields
	if report.Summary != "Payment processing failure due to database connection timeout" {
		t.Errorf("unexpected summary: %s", report.Summary)
	}
	if report.RootCause != "Database connection pool exhausted after deployment v2.3.1" {
		t.Errorf("unexpected root cause: %s", report.RootCause)
	}
	if report.Confidence != 0.88 {
		t.Errorf("expected confidence 0.88, got %f", report.Confidence)
	}
	if report.ID <= 0 {
		t.Error("expected positive report ID")
	}

	// Verify evidence was saved
	evidence, err := svc.GetEvidence(ctx, report.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(evidence) != 3 {
		t.Errorf("expected 3 evidence items, got %d", len(evidence))
	}

	// Verify evidence is sorted by score descending
	for i := 1; i < len(evidence); i++ {
		if evidence[i].Score > evidence[i-1].Score {
			t.Errorf("evidence not sorted: [%d]=%.2f > [%d]=%.2f", i, evidence[i].Score, i-1, evidence[i-1].Score)
		}
	}
}
