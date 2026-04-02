package openobserve

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"log/slog"
)

func TestClient_SearchLogs(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer testtoken" {
			t.Errorf("expected Bearer testtoken, got %s", r.Header.Get("Authorization"))
		}
		if r.URL.Path != "/api/default/_search" {
			t.Errorf("expected /api/default/_search, got %s", r.URL.Path)
		}

		resp := map[string]any{
			"hits": []map[string]any{
				{
					"_timestamp":  float64(time.Now().UnixMicro()),
					"log":         "connection refused to redis:6379",
					"service":     "api-gateway",
					"level":       "error",
					"trace_id":    "abc-123",
				},
				{
					"_timestamp":  float64(time.Now().UnixMicro()),
					"log":         "timeout waiting for redis response",
					"service":     "api-gateway",
					"level":       "error",
					"trace_id":    "abc-123",
				},
			},
			"total":     2,
			"took":      15,
			"scan_size": 1024,
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	cfg := Config{
		BaseURL: server.URL,
		OrgID:   "default",
		Token:   "testtoken",
		Timeout: 5 * time.Second,
	}
	client, err := NewClient(cfg, slog.Default())
	if err != nil {
		t.Fatal(err)
	}

	results, err := client.SearchLogs(context.Background(), LogQuery{
		Stream:   "default",
		Service:  "api-gateway",
		Keywords: []string{"error"},
		StartTime: time.Now().Add(-1 * time.Hour).UnixMicro(),
		EndTime:   time.Now().UnixMicro(),
		Limit:     10,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if results[0].Name != "critical_log_cluster" {
		t.Errorf("expected critical_log_cluster, got %s", results[0].Name)
	}
}

func TestClient_SearchTrace(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{
			"hits": []map[string]any{
				{
					"trace_id":      "abc-123",
					"service_name":  "api-gateway",
					"operation_name": "GET /api/payments",
					"duration":      "2.5s",
					"status":        "error",
				},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	cfg := Config{BaseURL: server.URL, OrgID: "default", Token: "testtoken", Timeout: 5 * time.Second}
	client, _ := NewClient(cfg, slog.Default())

	results, err := client.SearchTrace(context.Background(), TraceQuery{
		Stream:   "default",
		TraceID:  "abc-123",
		StartTime: time.Now().Add(-1 * time.Hour).UnixMicro(),
		EndTime:   time.Now().UnixMicro(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Name != "slowest_span" {
		t.Errorf("expected slowest_span, got %s", results[0].Name)
	}
}

func TestClient_QueryMetric(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{
			"hits": []map[string]any{
				{
					"metric":    "error_rate",
					"service":   "api-gateway",
					"value":     0.15,
					"timestamp": float64(time.Now().Unix()),
				},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	cfg := Config{BaseURL: server.URL, OrgID: "default", Token: "testtoken", Timeout: 5 * time.Second}
	client, _ := NewClient(cfg, slog.Default())

	results, err := client.QueryMetric(context.Background(), MetricQuery{
		Stream:   "default",
		Service:  "api-gateway",
		Metric:   "error_rate",
		StartTime: time.Now().Add(-1 * time.Hour).UnixMicro(),
		EndTime:   time.Now().UnixMicro(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
}

func TestClient_SearchLogs_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "internal error"})
	}))
	defer server.Close()

	cfg := Config{BaseURL: server.URL, OrgID: "default", Token: "testtoken", Timeout: 5 * time.Second}
	client, _ := NewClient(cfg, slog.Default())

	_, err := client.SearchLogs(context.Background(), LogQuery{
		Stream:    "default",
		StartTime: 1,
		EndTime:   2,
	})
	if err == nil {
		t.Error("expected error for 500 response")
	}
}

func TestClient_BuildDrilldownURL(t *testing.T) {
	cfg := Config{BaseURL: "http://oo.example.com", OrgID: "default", Token: "test", Timeout: 5 * time.Second}
	client, _ := NewClient(cfg, slog.Default())

	url, err := client.BuildDrilldownURL(DrilldownRef{
		Type:      "logs",
		Stream:    "default",
		StartTime: 1000000,
		EndTime:   2000000,
	})
	if err != nil {
		t.Fatal(err)
	}
	if url == "" {
		t.Error("expected non-empty URL")
	}
}

func TestNewClient_Validation(t *testing.T) {
	_, err := NewClient(Config{}, nil)
	if err == nil {
		t.Error("expected error for empty config")
	}

	_, err = NewClient(Config{BaseURL: "http://localhost"}, nil)
	if err == nil {
		t.Error("expected error for missing orgID")
	}
}

func TestMapLogHit_Truncation(t *testing.T) {
	longLog := make([]byte, 300)
	for i := range longLog {
		longLog[i] = 'a'
	}
	hit := map[string]any{"log": string(longLog)}
	result := mapLogHit(hit, 0.5)
	if len(result.Summary) > 203 { // 200 + "..."
		t.Errorf("summary should be truncated, got len %d: %s", len(result.Summary), result.Summary)
	}
}

func TestMapLogHit_Short(t *testing.T) {
	hit := map[string]any{"log": "short message"}
	result := mapLogHit(hit, 0.9)
	if result.Summary != "short message" {
		t.Errorf("expected 'short message', got %s", result.Summary)
	}
}

func TestMapSpan(t *testing.T) {
	span := map[string]any{
		"service_name":   "api-gateway",
		"operation_name": "GET /api/payments",
		"duration":       "2.5s",
	}
	result := mapSpan(span, 0.8)
	if result.Score != 0.8 {
		t.Errorf("expected 0.8, got %f", result.Score)
	}
	expected := "api-gateway GET /api/payments 2.5s"
	if result.Summary != expected {
		t.Errorf("expected %q, got %q", expected, result.Summary)
	}
}
