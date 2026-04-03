package openobserve

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/atlanssia/aisre/internal/adapter"
)

// ProviderAdapter wraps an OO Client to implement the generic adapter.ToolProvider interface.
type ProviderAdapter struct {
	client *Client
}

// NewProviderAdapter creates a ProviderAdapter from an OO Client.
// Panics if client is nil.
func NewProviderAdapter(client *Client) *ProviderAdapter {
	if client == nil {
		panic("openobserve: NewProviderAdapter called with nil client")
	}
	return &ProviderAdapter{client: client}
}

// Name returns the adapter identifier.
func (pa *ProviderAdapter) Name() string {
	return "openobserve"
}

// parseTime parses a time string (RFC3339 or unix timestamp) to microseconds.
func parseTimeMicro(s string) int64 {
	if s == "" {
		return time.Now().UnixMicro()
	}
	// Try unix timestamp first
	if v, err := strconv.ParseInt(s, 10, 64); err == nil {
		return v
	}
	// Try RFC3339
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t.UnixMicro()
	}
	return time.Now().UnixMicro()
}

// SearchLogs searches for log records matching the query via the OO adapter.
func (pa *ProviderAdapter) SearchLogs(ctx context.Context, q adapter.LogQuery) ([]adapter.LogRecord, error) {
	ooq := LogQuery{
		Service:   q.Service,
		StartTime: parseTimeMicro(q.StartTime),
		EndTime:   parseTimeMicro(q.EndTime),
		Limit:     q.Limit,
	}

	results, err := pa.client.SearchLogs(ctx, ooq)
	if err != nil {
		return nil, fmt.Errorf("provider_adapter: search logs: %w", err)
	}

	records := make([]adapter.LogRecord, len(results))
	for i, tr := range results {
		level, _ := tr.Payload["level"].(string)
		service, _ := tr.Payload["service"].(string)
		msg, _ := tr.Payload["message"].(string)
		if msg == "" {
			msg = tr.Summary
		}
		ts, _ := tr.Payload["timestamp"].(string)
		if ts == "" {
			ts = time.Now().Format(time.RFC3339)
		}

		records[i] = adapter.LogRecord{
			Timestamp: ts,
			Level:     level,
			Service:   service,
			Message:   msg,
			Fields:    tr.Payload,
		}
	}
	return records, nil
}

// GetTrace retrieves a trace by its ID via the OO adapter.
func (pa *ProviderAdapter) GetTrace(ctx context.Context, traceID string) (*adapter.TraceData, error) {
	now := time.Now()
	results, err := pa.client.SearchTrace(ctx, TraceQuery{
		TraceID:   traceID,
		StartTime: now.Add(-1 * time.Hour).UnixMicro(),
		EndTime:   now.UnixMicro(),
	})
	if err != nil {
		return nil, fmt.Errorf("provider_adapter: get trace: %w", err)
	}

	if len(results) == 0 {
		return nil, nil
	}

	spans := make([]adapter.Span, 0, len(results))
	serviceSet := make(map[string]bool)
	for _, tr := range results {
		spanID, _ := tr.Payload["span_id"].(string)
		parentID, _ := tr.Payload["parent_span_id"].(string)
		service, _ := tr.Payload["service_name"].(string)
		operation, _ := tr.Payload["operation_name"].(string)
		duration, _ := tr.Payload["duration"].(string)
		status, _ := tr.Payload["status"].(string)

		if service != "" {
			serviceSet[service] = true
		}

		spans = append(spans, adapter.Span{
			SpanID:     spanID,
			ParentID:   parentID,
			Service:    service,
			Operation:  operation,
			Duration:   duration,
			Status:     status,
			Attributes: tr.Payload,
		})
	}

	services := make([]string, 0, len(serviceSet))
	for s := range serviceSet {
		services = append(services, s)
	}

	return &adapter.TraceData{
		TraceID:  traceID,
		Spans:    spans,
		Services: services,
	}, nil
}

// QueryMetric queries metric time series data via the OO adapter.
func (pa *ProviderAdapter) QueryMetric(ctx context.Context, q adapter.MetricQuery) (*adapter.MetricSeries, error) {
	ooq := MetricQuery{
		Service:   q.Service,
		Metric:    q.Metric,
		StartTime: parseTimeMicro(q.StartTime),
		EndTime:   parseTimeMicro(q.EndTime),
		Interval:  q.Step,
	}

	results, err := pa.client.QueryMetric(ctx, ooq)
	if err != nil {
		return nil, fmt.Errorf("provider_adapter: query metric: %w", err)
	}

	if len(results) == 0 {
		return nil, nil
	}

	points := make([]adapter.MetricPoint, len(results))
	for i, tr := range results {
		value, _ := tr.Payload["value"].(float64)
		ts, _ := tr.Payload["timestamp"].(string)
		if ts == "" {
			tsFloat, ok := tr.Payload["timestamp"].(float64)
			if ok {
				ts = time.Unix(int64(tsFloat), 0).Format(time.RFC3339)
			}
		}
		points[i] = adapter.MetricPoint{
			Timestamp: ts,
			Value:     value,
		}
	}

	return &adapter.MetricSeries{
		Metric: q.Metric,
		Points: points,
	}, nil
}
