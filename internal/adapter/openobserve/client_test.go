package openobserve

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
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
					"message":     "connection refused to redis:6379",
					"service":     "api-gateway",
					"level":       "error",
					"trace_id":    "abc-123",
				},
				{
					"_timestamp":  float64(time.Now().UnixMicro()),
					"message":     "timeout waiting for redis response",
					"service":     "api-gateway",
					"level":       "error",
					"trace_id":    "abc-123",
				},
			},
			"total":     2,
			"took":      15,
			"scan_size": 1024,
		}
		_ = json.NewEncoder(w).Encode(resp)
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
					"trace_id":     "abc-123",
					"service":      "api-gateway",
					"span_id":      "GET /api/payments",
					"duration_ms":  float64(2500),
					"status_code":  float64(500),
				},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
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
		_ = json.NewEncoder(w).Encode(resp)
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
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "internal error"})
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
	hit := map[string]any{"message": string(longLog)}
	result := mapLogHit(hit, 0.5)
	if len(result.Summary) > 203 { // 200 + "..."
		t.Errorf("summary should be truncated, got len %d: %s", len(result.Summary), result.Summary)
	}
}

func TestMapLogHit_Short(t *testing.T) {
	hit := map[string]any{"message": "short message"}
	result := mapLogHit(hit, 0.9)
	if result.Summary != "short message" {
		t.Errorf("expected 'short message', got %s", result.Summary)
	}
}

func TestSanitize(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"clean string", "api-gateway", "api-gateway"},
		{"single quotes", "'; DROP TABLE users;--", " DROP TABLE users--"},
		{"double quotes", `"value"`, "value"},
		{"backslash", `test\'; DROP`, "test DROP"},
		{"semicolon", "service; SELECT *", "service SELECT *"},
		{"combined injection", `' OR '1'='1`, " OR 1=1"},
		{"unicode safe", "日本語サービス", "日本語サービス"},
		{"empty", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sanitize(tt.input)
			if got != tt.want {
				t.Errorf("sanitize(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestBuildLogSQL_InjectionPrevention(t *testing.T) {
	cfg := Config{BaseURL: "http://localhost", OrgID: "default", Token: "test", Timeout: 5 * time.Second}
	client, _ := NewClient(cfg, slog.Default())

	tests := []struct {
		name             string
		query            LogQuery
		mustNotContain   string
		mustContainValue string // the sanitized value that must appear in SQL
	}{
		{
			name: "service quote injection neutralized",
			query: LogQuery{
				Stream:  "default",
				Service: "' OR '1'='1",
			},
			mustNotContain:   "' OR '",
			mustContainValue: " OR 1=1",
		},
		{
			name: "keyword injection neutralized",
			query: LogQuery{
				Stream:   "default",
				Keywords: []string{"'; DROP TABLE logs;--"},
			},
			mustNotContain:   "';",
			mustContainValue: " DROP TABLE logs--",
		},
		{
			name: "backslash injection neutralized",
			query: LogQuery{
				Stream:   "default",
				Keywords: []string{`\'; OR 1=1--`},
			},
			mustNotContain:   `\'`,
			mustContainValue: " OR 1=1--",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sql := client.buildLogSQL(tt.query)
			if strings.Contains(sql, tt.mustNotContain) {
				t.Errorf("SQL contains injection pattern %q: %s", tt.mustNotContain, sql)
			}
			if !strings.Contains(sql, tt.mustContainValue) {
				t.Errorf("SQL does not contain sanitized value %q: %s", tt.mustContainValue, sql)
			}
		})
	}
}

func TestBuildTraceSQL_InjectionPrevention(t *testing.T) {
	cfg := Config{BaseURL: "http://localhost", OrgID: "default", Token: "test", Timeout: 5 * time.Second}
	client, _ := NewClient(cfg, slog.Default())

	tests := []struct {
		name             string
		query            TraceQuery
		mustNotContain   string
		mustContainValue string
	}{
		{
			name: "trace_id injection neutralized",
			query: TraceQuery{
				Stream:  "default",
				TraceID: "' OR '1'='1",
			},
			mustNotContain:   "' OR '",
			mustContainValue: " OR 1=1",
		},
		{
			name: "service injection neutralized",
			query: TraceQuery{
				Stream:  "default",
				Service: "'; DROP TABLE traces;--",
			},
			mustNotContain:   "';",
			mustContainValue: " DROP TABLE traces--",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sql := client.buildTraceSQL(tt.query)
			if strings.Contains(sql, tt.mustNotContain) {
				t.Errorf("SQL contains injection pattern %q: %s", tt.mustNotContain, sql)
			}
			if !strings.Contains(sql, tt.mustContainValue) {
				t.Errorf("SQL does not contain sanitized value %q: %s", tt.mustContainValue, sql)
			}
		})
	}
}

func TestBuildMetricSQL_InjectionPrevention(t *testing.T) {
	cfg := Config{BaseURL: "http://localhost", OrgID: "default", Token: "test", Timeout: 5 * time.Second}
	client, _ := NewClient(cfg, slog.Default())

	tests := []struct {
		name             string
		query            MetricQuery
		mustNotContain   string
		mustContainValue string
	}{
		{
			name: "service injection neutralized",
			query: MetricQuery{
				Stream:  "default",
				Service: "' OR '1'='1",
			},
			mustNotContain:   "' OR '",
			mustContainValue: " OR 1=1",
		},
		{
			name: "metric field no longer used in SQL (GROUP BY aggregation)",
			query: MetricQuery{
				Stream:  "default",
				Service: "api-gateway",
				Metric:  "'; DROP TABLE metrics;--",
			},
			// buildMetricSQL no longer embeds Metric in SQL (uses GROUP BY aggregation)
			mustNotContain:   "DROP TABLE",
			mustContainValue: "GROUP BY",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sql := client.buildMetricSQL(tt.query)
			if strings.Contains(sql, tt.mustNotContain) {
				t.Errorf("SQL contains injection pattern %q: %s", tt.mustNotContain, sql)
			}
			if !strings.Contains(sql, tt.mustContainValue) {
				t.Errorf("SQL does not contain sanitized value %q: %s", tt.mustContainValue, sql)
			}
		})
	}
}

func TestMapSpan(t *testing.T) {
	span := map[string]any{
		"service":     "api-gateway",
		"span_id":     "GET /api/payments",
		"duration_ms": float64(2500),
	}
	result := mapSpan(span, 0.8)
	if result.Score != 0.8 {
		t.Errorf("expected 0.8, got %f", result.Score)
	}
	expected := "api-gateway GET /api/payments 2500ms"
	if result.Summary != expected {
		t.Errorf("expected %q, got %q", expected, result.Summary)
	}
}

func TestMapSpan_NilDuration(t *testing.T) {
	span := map[string]any{
		"service": "api-gateway",
		"span_id": "GET /api/payments",
	}
	result := mapSpan(span, 0.5)
	if !strings.Contains(result.Summary, "?ms") {
		t.Errorf("expected ?ms for nil duration, got %q", result.Summary)
	}
}
