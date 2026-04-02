package analysis

import (
	"testing"

	"github.com/atlanssia/aisre/internal/contract"
)

func TestContextBuilder_Build(t *testing.T) {
	cb := NewContextBuilder()

	t.Run("builds context from incident and tool results", func(t *testing.T) {
		incident := &contract.Incident{
			ID:          1,
			Source:      "prometheus",
			ServiceName: "api-gateway",
			Severity:    "critical",
			TraceID:     "trace-abc-123",
			CreatedAt:   "2025-01-15T10:30:00Z",
		}

		toolResults := []contract.ToolResult{
			{Name: "log_search", Summary: "connection refused errors in user-service", Score: 0.95, Payload: map[string]any{
				"count":    150,
				"service":  "user-service",
				"level":    "error",
				"message":  "connection refused to db:5432",
			}},
			{Name: "trace_analysis", Summary: "slow span in /api/users endpoint", Score: 0.8, Payload: map[string]any{
				"duration_ms": 5000,
				"endpoint":   "/api/users",
				"service":    "api-gateway",
			}},
			{Name: "metric_query", Summary: "CPU spike on user-service", Score: 0.6, Payload: map[string]any{
				"metric":    "cpu_usage",
				"value":     95.3,
				"service":   "user-service",
			}},
		}

		context := cb.Build(incident, toolResults)

		if context.Incident == nil {
			t.Error("expected incident in context")
		}
		if context.Incident.ServiceName != "api-gateway" {
			t.Errorf("expected api-gateway, got %s", context.Incident.ServiceName)
		}
		if context.Incident.Severity != "critical" {
			t.Errorf("expected critical, got %s", context.Incident.Severity)
		}
		if len(context.ToolResults) != 3 {
			t.Errorf("expected 3 tool results, got %d", len(context.ToolResults))
		}
	})

	t.Run("handles empty tool results", func(t *testing.T) {
		incident := &contract.Incident{
			ID:          2,
			ServiceName: "payment-svc",
			Severity:    "high",
		}

		context := cb.Build(incident, nil)
		if context.Incident == nil {
			t.Error("expected incident in context")
		}
		if len(context.ToolResults) != 0 {
			t.Errorf("expected 0 tool results, got %d", len(context.ToolResults))
		}
	})

	t.Run("handles nil incident", func(t *testing.T) {
		context := cb.Build(nil, []contract.ToolResult{
			{Name: "test", Summary: "test", Score: 0.5},
		})
		if context.Incident != nil {
			t.Error("expected nil incident")
		}
	})
}

func TestContextBuilder_BuildPrompt(t *testing.T) {
	cb := NewContextBuilder()

	incident := &contract.Incident{
		ID:          1,
		Source:      "prometheus",
		ServiceName: "api-gateway",
		Severity:    "critical",
		TraceID:     "trace-abc-123",
	}

	toolResults := []contract.ToolResult{
		{Name: "log_search", Summary: "connection refused errors", Score: 0.95, Payload: map[string]any{"service": "user-service"}},
	}

	context := cb.Build(incident, toolResults)
	prompt := cb.BuildPrompt(context)

	if prompt == "" {
		t.Error("expected non-empty prompt")
	}
	// Verify the prompt contains key information
	if len(prompt) < 50 {
		t.Errorf("prompt seems too short: %d chars", len(prompt))
	}
}

func TestContextBuilder_FormatToolResults(t *testing.T) {
	cb := NewContextBuilder()

	results := []contract.ToolResult{
		{Name: "logs", Summary: "error logs found", Score: 0.9, Payload: map[string]any{"count": 50}},
		{Name: "traces", Summary: "slow trace detected", Score: 0.7, Payload: map[string]any{"duration_ms": 3000}},
	}

	formatted := cb.FormatToolResults(results)
	if formatted == "" {
		t.Error("expected non-empty formatted results")
	}
}
