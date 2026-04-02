package openobserve

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/atlanssia/aisre/internal/contract"
)

// Provider defines the OpenObserve adapter interface.
// All methods return normalized contract.ToolResult, never raw OO JSON.
type Provider interface {
	// SearchLogs queries OO logs via Search API and returns normalized results.
	SearchLogs(ctx context.Context, q LogQuery) ([]contract.ToolResult, error)

	// SearchTrace queries OO traces and returns normalized results.
	SearchTrace(ctx context.Context, q TraceQuery) ([]contract.ToolResult, error)

	// QueryMetric queries OO metrics via SQL aggregation.
	QueryMetric(ctx context.Context, q MetricQuery) ([]contract.ToolResult, error)

	// BuildDrilldownURL generates a URL linking to the OO UI for drill-down.
	BuildDrilldownURL(ref DrilldownRef) (string, error)
}

// LogQuery represents a structured logs search query.
// StartTime and EndTime are mandatory (microseconds) to avoid full-table scans.
type LogQuery struct {
	Stream    string
	Service   string
	Keywords  []string
	StartTime int64 // microseconds, mandatory
	EndTime   int64 // microseconds, mandatory
	Limit     int
}

// TraceQuery represents a structured trace search query.
type TraceQuery struct {
	Stream    string
	TraceID   string
	Service   string
	StartTime int64 // microseconds
	EndTime   int64 // microseconds
	Limit     int
}

// MetricQuery represents a structured metric aggregation query.
type MetricQuery struct {
	Stream    string
	Service   string
	Metric    string // e.g. "error_rate", "p95"
	StartTime int64
	EndTime   int64
	Interval  string // e.g. "1 minute"
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

// Client is the HTTP client for OpenObserve API.
type Client struct {
	baseURL string
	orgID   string
	token   string
	http    *http.Client
	logger  *slog.Logger
}

// NewClient creates a new OO adapter client.
func NewClient(cfg Config, logger *slog.Logger) (*Client, error) {
	if cfg.BaseURL == "" {
		return nil, fmt.Errorf("openobserve: baseURL is required")
	}
	if cfg.OrgID == "" {
		return nil, fmt.Errorf("openobserve: orgID is required")
	}
	if cfg.Token == "" {
		return nil, fmt.Errorf("openobserve: token is required")
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &Client{
		baseURL: cfg.BaseURL,
		orgID:   cfg.OrgID,
		token:   cfg.Token,
		http: &http.Client{
			Timeout: cfg.Timeout,
		},
		logger: logger,
	}, nil
}
