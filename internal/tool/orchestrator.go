package tool

import (
	"context"
	"fmt"
	"log/slog"
	"sort"

	"github.com/atlanssia/aisre/internal/contract"
)

// Orchestrator coordinates multi-signal evidence retrieval from ToolProviders.
type Orchestrator struct {
	tools   []Tool
	logger  *slog.Logger
}

// NewOrchestrator creates a new tool orchestrator.
func NewOrchestrator(tools []Tool, logger *slog.Logger) *Orchestrator {
	if logger == nil {
		logger = slog.Default()
	}
	return &Orchestrator{tools: tools, logger: logger}
}

// ExecuteAll runs all tools in parallel and collects results.
func (o *Orchestrator) ExecuteAll(ctx context.Context, incident *contract.Incident) ([]contract.ToolResult, error) {
	type toolOutput struct {
		name    string
		results []contract.ToolResult
		err     error
	}

	ch := make(chan toolOutput, len(o.tools))

	for _, t := range o.tools {
		go func(tool Tool) {
			results, err := t.Execute(ctx, incident)
			ch <- toolOutput{name: tool.Name(), results: results, err: err}
		}(t)
	}

	var allResults []contract.ToolResult
	var errors []string

	for i := 0; i < len(o.tools); i++ {
		out := <-ch
		if out.err != nil {
			o.logger.Warn("tool execution failed", "tool", out.name, "error", out.err)
			errors = append(errors, fmt.Sprintf("%s: %s", out.name, out.err))
			continue
		}
		if out.results != nil {
			allResults = append(allResults, out.results...)
		}
		o.logger.Debug("tool completed", "tool", out.name, "results", len(out.results))
	}

	if len(errors) == len(o.tools) {
		return nil, fmt.Errorf("all tools failed: %v", errors)
	}

	// Sort by score descending
	sort.Slice(allResults, func(i, j int) bool {
		return allResults[i].Score > allResults[j].Score
	})

	return allResults, nil
}
