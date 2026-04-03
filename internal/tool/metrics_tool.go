package tool

import (
	"context"
	"fmt"
	"time"

	"github.com/atlanssia/aisre/internal/adapter/openobserve"
	"github.com/atlanssia/aisre/internal/contract"
)

// MetricsTool queries metric anomalies related to the incident from OpenObserve.
type MetricsTool struct {
	provider openobserve.Provider
	stream   string
}

// MetricsToolConfig holds configuration for the MetricsTool.
type MetricsToolConfig struct {
	Provider openobserve.Provider
	Stream   string // default metric stream name
}

// NewMetricsTool creates a new metric query tool.
func NewMetricsTool(cfg MetricsToolConfig) *MetricsTool {
	stream := cfg.Stream
	if stream == "" {
		stream = "default"
	}
	return &MetricsTool{
		provider: cfg.Provider,
		stream:   stream,
	}
}

// Name returns the tool identifier.
func (t *MetricsTool) Name() string { return "metrics" }

// Execute queries metrics for the incident's service.
func (t *MetricsTool) Execute(ctx context.Context, incident *contract.Incident) ([]contract.ToolResult, error) {
	now := time.Now()
	startTime := now.Add(-1 * time.Hour).UnixMicro()
	endTime := now.UnixMicro()

	q := openobserve.MetricQuery{
		Stream:    t.stream,
		Service:   incident.ServiceName,
		StartTime: startTime,
		EndTime:   endTime,
		Interval:  "1m",
	}

	results, err := t.provider.QueryMetric(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("metrics_tool: query failed: %w", err)
	}
	return results, nil
}
