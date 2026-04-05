package openobserve

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/atlanssia/aisre/internal/adapter"
)

// TestProviderAdapter_ImplementsInterface verifies that ProviderAdapter
// satisfies the generic adapter.ToolProvider interface at compile time.
func TestProviderAdapter_ImplementsInterface(t *testing.T) {
	var _ adapter.ToolProvider = (*ProviderAdapter)(nil)
}

func TestProviderAdapter_Name(t *testing.T) {
	client, _ := NewClient(Config{
		BaseURL: "http://localhost:5080",
		OrgID:   "default",
		Token:   "test",
		Timeout: 5 * time.Second,
	}, slog.Default())
	pa := NewProviderAdapter(client)

	if got := pa.Name(); got != "openobserve" {
		t.Errorf("Name() = %q, want %q", got, "openobserve")
	}
}

func TestProviderAdapter_SearchLogs(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{
			"hits": []map[string]any{
				{
					"_timestamp": float64(time.Now().UnixMicro()),
					"message":    "connection refused to redis:6379",
					"service":    "api-gateway",
					"level":      "error",
					"trace_id":   "abc-123",
				},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client, _ := NewClient(Config{
		BaseURL: server.URL,
		OrgID:   "default",
		Token:   "testtoken",
		Timeout: 5 * time.Second,
	}, slog.Default())
	pa := NewProviderAdapter(client)

	records, err := pa.SearchLogs(context.Background(), adapter.LogQuery{
		Service:   "api-gateway",
		Level:     "error",
		Query:     "redis",
		StartTime: time.Now().Add(-1 * time.Hour).Format(time.RFC3339),
		EndTime:   time.Now().Format(time.RFC3339),
		Limit:     10,
	})
	if err != nil {
		t.Fatalf("SearchLogs() error: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("SearchLogs() returned %d records, want 1", len(records))
	}

	rec := records[0]
	if rec.Service != "api-gateway" {
		t.Errorf("Service = %q, want %q", rec.Service, "api-gateway")
	}
	if rec.Level != "error" {
		t.Errorf("Level = %q, want %q", rec.Level, "error")
	}
	if rec.Message == "" {
		t.Error("Message should not be empty")
	}
	if rec.Timestamp == "" {
		t.Error("Timestamp should not be empty")
	}
}

func TestProviderAdapter_SearchLogs_EmptyHits(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{
			"hits": []map[string]any{},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client, _ := NewClient(Config{
		BaseURL: server.URL,
		OrgID:   "default",
		Token:   "testtoken",
		Timeout: 5 * time.Second,
	}, slog.Default())
	pa := NewProviderAdapter(client)

	records, err := pa.SearchLogs(context.Background(), adapter.LogQuery{
		StartTime: time.Now().Add(-1 * time.Hour).Format(time.RFC3339),
		EndTime:   time.Now().Format(time.RFC3339),
	})
	if err != nil {
		t.Fatalf("SearchLogs() error: %v", err)
	}
	if len(records) != 0 {
		t.Errorf("SearchLogs() returned %d records, want 0", len(records))
	}
}

func TestProviderAdapter_GetTrace(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{
			"hits": []map[string]any{
				{
					"trace_id":       "trace-001",
					"span_id":        "span-001",
					"parent_span_id": "",
					"service_name":   "api-gateway",
					"operation_name": "GET /api/payments",
					"duration":       2500000, // 2.5s in microseconds
					"status":         "error",
				},
				{
					"trace_id":       "trace-001",
					"span_id":        "span-002",
					"parent_span_id": "span-001",
					"service_name":   "payment-service",
					"operation_name": "ProcessPayment",
					"duration":       2000000,
					"status":         "ok",
				},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client, _ := NewClient(Config{
		BaseURL: server.URL,
		OrgID:   "default",
		Token:   "testtoken",
		Timeout: 5 * time.Second,
	}, slog.Default())
	pa := NewProviderAdapter(client)

	trace, err := pa.GetTrace(context.Background(), "trace-001")
	if err != nil {
		t.Fatalf("GetTrace() error: %v", err)
	}
	if trace.TraceID != "trace-001" {
		t.Errorf("TraceID = %q, want %q", trace.TraceID, "trace-001")
	}
	if len(trace.Spans) != 2 {
		t.Fatalf("Spans count = %d, want 2", len(trace.Spans))
	}
	if trace.Spans[0].Service != "api-gateway" {
		t.Errorf("Span[0].Service = %q, want %q", trace.Spans[0].Service, "api-gateway")
	}
	if trace.Spans[0].Operation != "GET /api/payments" {
		t.Errorf("Span[0].Operation = %q, want %q", trace.Spans[0].Operation, "GET /api/payments")
	}
	if trace.Spans[0].Status != "error" {
		t.Errorf("Span[0].Status = %q, want %q", trace.Spans[0].Status, "error")
	}
	if trace.Spans[1].ParentID != "span-001" {
		t.Errorf("Span[1].ParentID = %q, want %q", trace.Spans[1].ParentID, "span-001")
	}

	// Check services list deduplication
	if len(trace.Services) != 2 {
		t.Errorf("Services count = %d, want 2", len(trace.Services))
	}
}

func TestProviderAdapter_GetTrace_NotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{
			"hits": []map[string]any{},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client, _ := NewClient(Config{
		BaseURL: server.URL,
		OrgID:   "default",
		Token:   "testtoken",
		Timeout: 5 * time.Second,
	}, slog.Default())
	pa := NewProviderAdapter(client)

	trace, err := pa.GetTrace(context.Background(), "nonexistent")
	if err != nil {
		t.Fatalf("GetTrace() error: %v", err)
	}
	if trace != nil {
		t.Errorf("GetTrace() expected nil for empty result, got %+v", trace)
	}
}

func TestProviderAdapter_QueryMetric(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{
			"hits": []map[string]any{
				{
					"metric":    "error_rate",
					"service":   "api-gateway",
					"value":     0.15,
					"timestamp": float64(time.Now().Unix()),
				},
				{
					"metric":    "error_rate",
					"service":   "api-gateway",
					"value":     0.20,
					"timestamp": float64(time.Now().Add(-1 * time.Minute).Unix()),
				},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client, _ := NewClient(Config{
		BaseURL: server.URL,
		OrgID:   "default",
		Token:   "testtoken",
		Timeout: 5 * time.Second,
	}, slog.Default())
	pa := NewProviderAdapter(client)

	series, err := pa.QueryMetric(context.Background(), adapter.MetricQuery{
		Service:   "api-gateway",
		Metric:    "error_rate",
		StartTime: time.Now().Add(-1 * time.Hour).Format(time.RFC3339),
		EndTime:   time.Now().Format(time.RFC3339),
	})
	if err != nil {
		t.Fatalf("QueryMetric() error: %v", err)
	}
	if series == nil {
		t.Fatal("QueryMetric() returned nil series")
	}
	if series.Metric != "error_rate" {
		t.Errorf("Metric = %q, want %q", series.Metric, "error_rate")
	}
	if len(series.Points) != 2 {
		t.Fatalf("Points count = %d, want 2", len(series.Points))
	}
	if series.Points[0].Value <= 0 {
		t.Errorf("Points[0].Value = %f, want > 0", series.Points[0].Value)
	}
}

func TestProviderAdapter_QueryMetric_EmptyHits(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{
			"hits": []map[string]any{},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client, _ := NewClient(Config{
		BaseURL: server.URL,
		OrgID:   "default",
		Token:   "testtoken",
		Timeout: 5 * time.Second,
	}, slog.Default())
	pa := NewProviderAdapter(client)

	series, err := pa.QueryMetric(context.Background(), adapter.MetricQuery{
		Service:   "api-gateway",
		Metric:    "error_rate",
		StartTime: time.Now().Add(-1 * time.Hour).Format(time.RFC3339),
		EndTime:   time.Now().Format(time.RFC3339),
	})
	if err != nil {
		t.Fatalf("QueryMetric() error: %v", err)
	}
	if series != nil {
		t.Errorf("QueryMetric() expected nil for empty result, got %+v", series)
	}
}

func TestProviderAdapter_NewProviderAdapter_NilClient(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("NewProviderAdapter(nil) should panic")
		}
	}()
	NewProviderAdapter(nil)
}
