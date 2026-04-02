package tool

import (
	"context"
	"fmt"
	"testing"

	"github.com/atlanssia/aisre/internal/contract"
	"log/slog"
)

type mockTool struct {
	name    string
	results []contract.ToolResult
	err     error
}

func (m *mockTool) Name() string { return m.name }
func (m *mockTool) Execute(ctx context.Context, incident *contract.Incident) ([]contract.ToolResult, error) {
	return m.results, m.err
}

func TestOrchestrator_ExecuteAll(t *testing.T) {
	inc := &contract.Incident{ID: 1, ServiceName: "api-gateway", Severity: "high"}

	t.Run("single tool success", func(t *testing.T) {
		tools := []Tool{
			&mockTool{name: "logs", results: []contract.ToolResult{
				{Name: "log1", Summary: "error log", Score: 0.9},
			}},
		}
		orch := NewOrchestrator(tools, slog.Default())
		results, err := orch.ExecuteAll(context.Background(), inc)
		if err != nil {
			t.Fatal(err)
		}
		if len(results) != 1 {
			t.Fatalf("expected 1, got %d", len(results))
		}
	})

	t.Run("multiple tools sorted by score", func(t *testing.T) {
		tools := []Tool{
			&mockTool{name: "logs", results: []contract.ToolResult{
				{Name: "log1", Summary: "error", Score: 0.7},
			}},
			&mockTool{name: "traces", results: []contract.ToolResult{
				{Name: "trace1", Summary: "slow span", Score: 0.95},
			}},
			&mockTool{name: "metrics", results: []contract.ToolResult{
				{Name: "metric1", Summary: "spike", Score: 0.6},
			}},
		}
		orch := NewOrchestrator(tools, slog.Default())
		results, err := orch.ExecuteAll(context.Background(), inc)
		if err != nil {
			t.Fatal(err)
		}
		if len(results) != 3 {
			t.Fatalf("expected 3, got %d", len(results))
		}
		if results[0].Score != 0.95 {
			t.Errorf("expected highest score first, got %f", results[0].Score)
		}
	})

	t.Run("partial failure continues", func(t *testing.T) {
		tools := []Tool{
			&mockTool{name: "good", results: []contract.ToolResult{
				{Name: "ok", Summary: "works", Score: 0.8},
			}},
			&mockTool{name: "bad", err: fmt.Errorf("connection refused")},
		}
		orch := NewOrchestrator(tools, slog.Default())
		results, err := orch.ExecuteAll(context.Background(), inc)
		if err != nil {
			t.Fatal("should not error on partial failure")
		}
		if len(results) != 1 {
			t.Fatalf("expected 1 result from good tool, got %d", len(results))
		}
	})

	t.Run("all tools fail returns error", func(t *testing.T) {
		tools := []Tool{
			&mockTool{name: "a", err: fmt.Errorf("failed")},
			&mockTool{name: "b", err: fmt.Errorf("also failed")},
		}
		orch := NewOrchestrator(tools, slog.Default())
		_, err := orch.ExecuteAll(context.Background(), inc)
		if err == nil {
			t.Error("expected error when all tools fail")
		}
	})
}
