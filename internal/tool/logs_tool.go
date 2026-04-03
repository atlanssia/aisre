package tool

import (
	"context"
	"fmt"
	"time"

	"github.com/atlanssia/aisre/internal/adapter/openobserve"
	"github.com/atlanssia/aisre/internal/contract"
)

// LogsTool searches for relevant log entries in OpenObserve.
type LogsTool struct {
	provider openobserve.Provider
	stream   string
}

// LogsToolConfig holds configuration for the LogsTool.
type LogsToolConfig struct {
	Provider openobserve.Provider
	Stream   string // default log stream name
}

// NewLogsTool creates a new logs search tool.
func NewLogsTool(cfg LogsToolConfig) *LogsTool {
	stream := cfg.Stream
	if stream == "" {
		stream = "default"
	}
	return &LogsTool{
		provider: cfg.Provider,
		stream:   stream,
	}
}

// Name returns the tool identifier.
func (t *LogsTool) Name() string { return "logs" }

// Execute searches for log entries related to the incident.
func (t *LogsTool) Execute(ctx context.Context, incident *contract.Incident) ([]contract.ToolResult, error) {
	now := time.Now()
	startTime := now.Add(-1 * time.Hour).UnixMicro()
	endTime := now.UnixMicro()

	q := openobserve.LogQuery{
		Stream:    t.stream,
		Service:   incident.ServiceName,
		Keywords:  keywordsFromSeverity(incident.Severity),
		StartTime: startTime,
		EndTime:   endTime,
		Limit:     50,
	}

	results, err := t.provider.SearchLogs(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("logs_tool: search failed: %w", err)
	}
	return results, nil
}

// keywordsFromSeverity returns log search keywords based on incident severity.
func keywordsFromSeverity(severity string) []string {
	switch severity {
	case "critical":
		return []string{"error", "fatal", "panic"}
	case "high":
		return []string{"error", "warn"}
	case "warning", "medium":
		return []string{"warn", "error"}
	default:
		return []string{"error"}
	}
}
