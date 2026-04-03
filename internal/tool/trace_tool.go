package tool

import (
	"context"
	"fmt"
	"time"

	"github.com/atlanssia/aisre/internal/adapter/openobserve"
	"github.com/atlanssia/aisre/internal/contract"
)

// TraceTool searches for trace data related to the incident in OpenObserve.
type TraceTool struct {
	provider openobserve.Provider
	stream   string
}

// TraceToolConfig holds configuration for the TraceTool.
type TraceToolConfig struct {
	Provider openobserve.Provider
	Stream   string // default trace stream name
}

// NewTraceTool creates a new trace search tool.
func NewTraceTool(cfg TraceToolConfig) *TraceTool {
	stream := cfg.Stream
	if stream == "" {
		stream = "default"
	}
	return &TraceTool{
		provider: cfg.Provider,
		stream:   stream,
	}
}

// Name returns the tool identifier.
func (t *TraceTool) Name() string { return "traces" }

// Execute searches for traces related to the incident.
func (t *TraceTool) Execute(ctx context.Context, incident *contract.Incident) ([]contract.ToolResult, error) {
	now := time.Now()
	startTime := now.Add(-1 * time.Hour).UnixMicro()
	endTime := now.UnixMicro()

	q := openobserve.TraceQuery{
		Stream:    t.stream,
		TraceID:   incident.TraceID,
		Service:   incident.ServiceName,
		StartTime: startTime,
		EndTime:   endTime,
		Limit:     20,
	}

	results, err := t.provider.SearchTrace(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("trace_tool: search failed: %w", err)
	}
	return results, nil
}
