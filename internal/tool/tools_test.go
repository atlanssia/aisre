package tool

import (
	"context"
	"fmt"
	"testing"

	"github.com/atlanssia/aisre/internal/adapter/openobserve"
	"github.com/atlanssia/aisre/internal/contract"
)

// mockProvider implements openobserve.Provider for testing.
type mockProvider struct {
	logsResult   []contract.ToolResult
	logsErr      error
	traceResult  []contract.ToolResult
	traceErr     error
	metricResult []contract.ToolResult
	metricErr    error
}

func (m *mockProvider) SearchLogs(_ context.Context, _ openobserve.LogQuery) ([]contract.ToolResult, error) {
	return m.logsResult, m.logsErr
}

func (m *mockProvider) SearchTrace(_ context.Context, _ openobserve.TraceQuery) ([]contract.ToolResult, error) {
	return m.traceResult, m.traceErr
}

func (m *mockProvider) QueryMetric(_ context.Context, _ openobserve.MetricQuery) ([]contract.ToolResult, error) {
	return m.metricResult, m.metricErr
}

func (m *mockProvider) BuildDrilldownURL(_ openobserve.DrilldownRef) (string, error) {
	return "http://localhost:5080/web/logs?stream=default", nil
}

// --- LogsTool Tests ---

func TestLogsTool_Name(t *testing.T) {
	tool := NewLogsTool(LogsToolConfig{Provider: &mockProvider{}})
	if tool.Name() != "logs" {
		t.Errorf("expected 'logs', got %q", tool.Name())
	}
}

func TestLogsTool_Execute_Success(t *testing.T) {
	mock := &mockProvider{
		logsResult: []contract.ToolResult{
			{Name: "critical_log_cluster", Summary: "connection refused to redis:6379", Score: 0.9, Payload: map[string]any{"service": "api-gateway"}},
			{Name: "critical_log_cluster", Summary: "timeout waiting for redis response", Score: 0.7, Payload: map[string]any{"service": "api-gateway"}},
		},
	}

	tool := NewLogsTool(LogsToolConfig{Provider: mock})
	incident := &contract.Incident{
		ID:          1,
		ServiceName: "api-gateway",
		Severity:    "critical",
	}

	results, err := tool.Execute(context.Background(), incident)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if results[0].Score != 0.9 {
		t.Errorf("expected score 0.9, got %f", results[0].Score)
	}
}

func TestLogsTool_Execute_ProviderError(t *testing.T) {
	mock := &mockProvider{
		logsErr: fmt.Errorf("connection refused"),
	}

	tool := NewLogsTool(LogsToolConfig{Provider: mock})
	incident := &contract.Incident{
		ID:          1,
		ServiceName: "api-gateway",
	}

	results, err := tool.Execute(context.Background(), incident)
	if err == nil {
		t.Error("expected error when provider fails")
	}
	if results != nil {
		t.Errorf("expected nil results on error, got %v", results)
	}
}

func TestLogsTool_Execute_EmptyServiceName(t *testing.T) {
	mock := &mockProvider{
		logsResult: []contract.ToolResult{},
	}

	tool := NewLogsTool(LogsToolConfig{Provider: mock})
	incident := &contract.Incident{
		ID:          1,
		ServiceName: "",
		Severity:    "high",
	}

	results, err := tool.Execute(context.Background(), incident)
	if err != nil {
		t.Fatal(err)
	}
	if results == nil {
		t.Error("expected non-nil results slice")
	}
}

// --- TraceTool Tests ---

func TestTraceTool_Name(t *testing.T) {
	tool := NewTraceTool(TraceToolConfig{Provider: &mockProvider{}})
	if tool.Name() != "traces" {
		t.Errorf("expected 'traces', got %q", tool.Name())
	}
}

func TestTraceTool_Execute_WithTraceID(t *testing.T) {
	mock := &mockProvider{
		traceResult: []contract.ToolResult{
			{Name: "slowest_span", Summary: "api-gateway GET /api/payments 2.5s", Score: 0.9, Payload: map[string]any{"trace_id": "abc-123"}},
		},
	}

	tool := NewTraceTool(TraceToolConfig{Provider: mock})
	incident := &contract.Incident{
		ID:          1,
		ServiceName: "api-gateway",
		TraceID:     "abc-123",
		Severity:    "critical",
	}

	results, err := tool.Execute(context.Background(), incident)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
}

func TestTraceTool_Execute_NoTraceID(t *testing.T) {
	mock := &mockProvider{
		traceResult: []contract.ToolResult{},
	}

	tool := NewTraceTool(TraceToolConfig{Provider: mock})
	incident := &contract.Incident{
		ID:          1,
		ServiceName: "api-gateway",
		Severity:    "high",
	}

	results, err := tool.Execute(context.Background(), incident)
	if err != nil {
		t.Fatal(err)
	}
	if results == nil {
		t.Error("expected non-nil results")
	}
}

func TestTraceTool_Execute_ProviderError(t *testing.T) {
	mock := &mockProvider{
		traceErr: fmt.Errorf("timeout"),
	}

	tool := NewTraceTool(TraceToolConfig{Provider: mock})
	incident := &contract.Incident{
		ID:          1,
		ServiceName: "svc",
		TraceID:     "trace-123",
	}

	_, err := tool.Execute(context.Background(), incident)
	if err == nil {
		t.Error("expected error when provider fails")
	}
}

// --- MetricsTool Tests ---

func TestMetricsTool_Name(t *testing.T) {
	tool := NewMetricsTool(MetricsToolConfig{Provider: &mockProvider{}})
	if tool.Name() != "metrics" {
		t.Errorf("expected 'metrics', got %q", tool.Name())
	}
}

func TestMetricsTool_Execute_Success(t *testing.T) {
	mock := &mockProvider{
		metricResult: []contract.ToolResult{
			{Name: "metric_anomaly", Summary: "error_rate = 0.15", Score: 0.6, Payload: map[string]any{"metric": "error_rate"}},
		},
	}

	tool := NewMetricsTool(MetricsToolConfig{Provider: mock})
	incident := &contract.Incident{
		ID:          1,
		ServiceName: "api-gateway",
		Severity:    "critical",
	}

	results, err := tool.Execute(context.Background(), incident)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
}

func TestMetricsTool_Execute_ProviderError(t *testing.T) {
	mock := &mockProvider{
		metricErr: fmt.Errorf("service unavailable"),
	}

	tool := NewMetricsTool(MetricsToolConfig{Provider: mock})
	incident := &contract.Incident{
		ID:          1,
		ServiceName: "svc",
	}

	_, err := tool.Execute(context.Background(), incident)
	if err == nil {
		t.Error("expected error when provider fails")
	}
}

func TestMetricsTool_Execute_EmptyServiceName(t *testing.T) {
	mock := &mockProvider{
		metricResult: []contract.ToolResult{},
	}

	tool := NewMetricsTool(MetricsToolConfig{Provider: mock})
	incident := &contract.Incident{
		ID:          1,
		ServiceName: "",
	}

	results, err := tool.Execute(context.Background(), incident)
	if err != nil {
		t.Fatal(err)
	}
	if results == nil {
		t.Error("expected non-nil results")
	}
}

// --- Integration: Orchestrator with real tool types ---

func TestOrchestrator_WithToolImplementations(t *testing.T) {
	logProvider := &mockProvider{
		logsResult: []contract.ToolResult{
			{Name: "critical_log_cluster", Summary: "error in svc", Score: 0.85},
		},
	}
	traceProvider := &mockProvider{
		traceResult: []contract.ToolResult{
			{Name: "slowest_span", Summary: "slow span", Score: 0.95},
		},
	}
	metricProvider := &mockProvider{
		metricResult: []contract.ToolResult{
			{Name: "metric_anomaly", Summary: "cpu spike", Score: 0.6},
		},
	}

	tools := []Tool{
		NewLogsTool(LogsToolConfig{Provider: logProvider}),
		NewTraceTool(TraceToolConfig{Provider: traceProvider}),
		NewMetricsTool(MetricsToolConfig{Provider: metricProvider}),
	}

	orch := NewOrchestrator(tools, nil)
	incident := &contract.Incident{
		ID:          1,
		ServiceName: "api-gateway",
		Severity:    "critical",
		TraceID:     "trace-123",
	}

	results, err := orch.ExecuteAll(context.Background(), incident)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}
	// Should be sorted by score descending
	if results[0].Score < results[1].Score {
		t.Errorf("results not sorted by score: %f should be >= %f", results[0].Score, results[1].Score)
	}
}

func TestOrchestrator_PartialToolFailure(t *testing.T) {
	goodProvider := &mockProvider{
		logsResult: []contract.ToolResult{
			{Name: "log", Summary: "ok", Score: 0.7},
		},
	}
	badProvider := &mockProvider{
		traceErr: fmt.Errorf("connection refused"),
	}
	metricProvider := &mockProvider{
		metricResult: []contract.ToolResult{
			{Name: "metric", Summary: "ok", Score: 0.5},
		},
	}

	tools := []Tool{
		NewLogsTool(LogsToolConfig{Provider: goodProvider}),
		NewTraceTool(TraceToolConfig{Provider: badProvider}),
		NewMetricsTool(MetricsToolConfig{Provider: metricProvider}),
	}

	orch := NewOrchestrator(tools, nil)
	incident := &contract.Incident{
		ID:          1,
		ServiceName: "svc",
	}

	results, err := orch.ExecuteAll(context.Background(), incident)
	if err != nil {
		t.Fatal("should not error on partial failure")
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results (one tool failed), got %d", len(results))
	}
}
