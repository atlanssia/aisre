package adapter

import "context"

// ToolProvider defines the interface for observability backend adapters.
// All backends (OpenObserve, SigNoz, etc.) implement this interface.
type ToolProvider interface {
	// Name returns the adapter identifier (e.g. "openobserve").
	Name() string

	// SearchLogs searches for log records matching the query.
	SearchLogs(ctx context.Context, q LogQuery) ([]LogRecord, error)

	// GetTrace retrieves a trace by its ID.
	GetTrace(ctx context.Context, traceID string) (*TraceData, error)

	// QueryMetric queries metric time series data.
	QueryMetric(ctx context.Context, q MetricQuery) (*MetricSeries, error)
}

// LogQuery represents parameters for searching logs.
type LogQuery struct {
	Service   string
	Level     string
	Query     string
	StartTime string
	EndTime   string
	Limit     int
}

// LogRecord represents a single log record.
type LogRecord struct {
	Timestamp string
	Level     string
	Service   string
	Message   string
	Fields    map[string]interface{}
}

// TraceData represents a distributed trace.
type TraceData struct {
	TraceID  string
	Spans    []Span
	Duration string
	Services []string
}

// Span represents a single span in a trace.
type Span struct {
	SpanID     string
	ParentID   string
	Service    string
	Operation  string
	Duration   string
	Status     string
	Attributes map[string]interface{}
}

// MetricQuery represents parameters for querying metrics.
type MetricQuery struct {
	Service   string
	Metric    string
	StartTime string
	EndTime   string
	Step      string
}

// MetricSeries represents a time series of metric data.
type MetricSeries struct {
	Metric string
	Labels map[string]string
	Points []MetricPoint
}

// MetricPoint represents a single data point.
type MetricPoint struct {
	Timestamp string
	Value     float64
}
