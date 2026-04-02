package openobserve

import (
	"context"
	"time"

	"github.com/atlanssia/aisre/internal/contract"
)

// Config holds the OpenObserve connection configuration.
type Config struct {
	BaseURL string
	OrgID   string
	Token   string
	Timeout time.Duration
	Retries int
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() Config {
	return Config{
		Timeout: 10 * time.Second,
		Retries: 2,
	}
}

// Provider defines the OpenObserve adapter interface.
type Provider interface {
	// SearchLogs queries OO logs and returns normalized results.
	SearchLogs(ctx context.Context, q LogQuery) ([]contract.ToolResult, error)

	// SearchTrace queries OO traces and returns normalized results.
	SearchTrace(ctx context.Context, q TraceQuery) ([]contract.ToolResult, error)

	// QueryMetric queries OO metrics via SQL aggregation.
	QueryMetric(ctx context.Context, q MetricQuery) ([]contract.ToolResult, error)

	// BuildDrilldownURL generates a URL linking to the OO UI.
	BuildDrilldownURL(ref DrilldownRef) (string, error)
}

// TraceQuery represents a structured trace search query.
type TraceQuery struct {
	Stream    string
	TraceID   string
	Service   string
	StartTime int64
	EndTime   int64
	Limit     int
}

// MetricQuery represents a structured metric aggregation query.
type MetricQuery struct {
	Stream    string
	Service   string
	Metric    string
	StartTime int64
	EndTime   int64
	Interval  string
}

// DrilldownRef holds parameters for building an OO UI drill-down URL.
type DrilldownRef struct {
	Type      string // "logs", "traces", "metrics"
	Stream    string
	TraceID   string
	StartTime int64
	EndTime   int64
	SQL       string
}

// LogQuery represents a structured logs search query.
type LogQuery struct {
	Stream    string
	Service   string
	Keywords  []string
	StartTime int64
	EndTime   int64
	Limit     int
}
