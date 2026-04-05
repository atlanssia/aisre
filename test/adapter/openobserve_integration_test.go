//go:build integration

package adapter_test

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/atlanssia/aisre/internal/adapter/openobserve"
)

const (
	defaultOOBaseURL = "http://localhost:5080"
	defaultOOOrgID   = "default"
	defaultOOUser    = "root@example.com"
	defaultOOPass    = "Complexpass#123"
)

// ooTestEnv holds shared test state for OO integration tests.
type ooTestEnv struct {
	baseURL string
	client  *openobserve.Client
}

// TestOOIntegration runs all OpenObserve integration tests.
// These tests require a running OO instance and are gated behind
// the "integration" build tag.
//
// Run with:
//
//	go test ./test/adapter/... -tags=integration -v -timeout=60s
func TestOOIntegration(t *testing.T) {
	baseURL := os.Getenv("OO_BASE_URL")
	if baseURL == "" {
		baseURL = defaultOOBaseURL
	}

	// Ping OO to check availability.
	if !ooIsReachable(t, baseURL) {
		t.Skipf("OpenObserve not reachable at %s - skipping integration tests", baseURL)
	}

	// Authenticate and create client.
	client := newOOClient(t, baseURL)

	env := &ooTestEnv{
		baseURL: baseURL,
		client:  client,
	}

	t.Run("Auth", env.testAuth)
	t.Run("SearchLogs", env.testSearchLogs)
	t.Run("SearchTraces", env.testSearchTraces)
	t.Run("QueryMetrics", env.testQueryMetrics)
	t.Run("BuildDrilldownURL", env.testBuildDrilldownURL)
}

// ooIsReachable checks if the OO instance is running by hitting
// the login endpoint.
func ooIsReachable(t *testing.T, baseURL string) bool {
	t.Helper()
	client := &http.Client{Timeout: 3 * time.Second}
	loginURL := fmt.Sprintf("%s/auth/login", baseURL)
	body := fmt.Sprintf(`{"name":"%s","password":"%s"}`, defaultOOUser, defaultOOPass)
	resp, err := client.Post(loginURL, "application/json", strings.NewReader(body))
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

// newOOClient creates an authenticated OO client using Basic auth.
func newOOClient(t *testing.T, baseURL string) *openobserve.Client {
	t.Helper()

	// Build Basic auth token: "Basic <base64(user:pass)>"
	creds := fmt.Sprintf("%s:%s", defaultOOUser, defaultOOPass)
	token := "Basic " + base64.StdEncoding.EncodeToString([]byte(creds))

	cfg := openobserve.Config{
		BaseURL: baseURL,
		OrgID:   defaultOOOrgID,
		Token:   token,
		Timeout: 10 * time.Second,
		Retries: 1,
	}

	client, err := openobserve.NewClient(cfg, nil)
	if err != nil {
		t.Fatalf("failed to create OO client: %v", err)
	}
	return client
}

// wideTimeRange returns a start/end time in microseconds that covers
// all ingested test data (up to 30 days back).
func wideTimeRange() (start, end int64) {
	now := time.Now()
	return now.Add(-720 * time.Hour).UnixMicro(), now.UnixMicro()
}

// testAuth verifies that the client can authenticate against OO.
func (env *ooTestEnv) testAuth(t *testing.T) {
	loginURL := fmt.Sprintf("%s/auth/login", env.baseURL)
	body := fmt.Sprintf(`{"name":"%s","password":"%s"}`, defaultOOUser, defaultOOPass)

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Post(loginURL, "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("login request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200 from login, got %d", resp.StatusCode)
	}

	// Verify the auth cookie is set.
	cookies := resp.Cookies()
	found := false
	for _, c := range cookies {
		if c.Name == "auth_tokens" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected auth_tokens cookie in login response")
	}
}

// testSearchLogs verifies SearchLogs can query the real OO instance.
// Uses a wide time range and keyword "timeout" which exists in the test data.
func (env *ooTestEnv) testSearchLogs(t *testing.T) {
	start, end := wideTimeRange()
	q := openobserve.LogQuery{
		Stream:    "default",
		Keywords:  []string{"timeout"},
		StartTime: start,
		EndTime:   end,
		Limit:     5,
	}

	results, err := env.client.SearchLogs(context.Background(), q)
	if err != nil {
		t.Fatalf("SearchLogs failed: %v", err)
	}

	if len(results) == 0 {
		t.Fatal("SearchLogs returned 0 results, expected at least 1 hit for 'timeout'")
	}

	for i, r := range results {
		t.Logf("  [%d] name=%s score=%.2f summary=%.80s", i, r.Name, r.Score, r.Summary)
	}

	// Verify the result contains "timeout" in the summary.
	found := false
	for _, r := range results {
		if strings.Contains(strings.ToLower(r.Summary), "timeout") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected at least one result containing 'timeout'")
	}
}

// testSearchTraces verifies SearchTrace can query the real OO instance.
func (env *ooTestEnv) testSearchTraces(t *testing.T) {
	start, end := wideTimeRange()
	q := openobserve.TraceQuery{
		Stream:    "default",
		StartTime: start,
		EndTime:   end,
		Limit:     5,
	}

	results, err := env.client.SearchTrace(context.Background(), q)
	if err != nil {
		t.Fatalf("SearchTrace failed: %v", err)
	}

	if len(results) == 0 {
		t.Fatal("SearchTrace returned 0 results, expected at least 1 span")
	}

	for i, r := range results {
		t.Logf("  [%d] name=%s score=%.2f summary=%.80s", i, r.Name, r.Score, r.Summary)
	}
}

// testQueryMetrics verifies QueryMetric can aggregate data from the real OO instance.
func (env *ooTestEnv) testQueryMetrics(t *testing.T) {
	start, end := wideTimeRange()
	q := openobserve.MetricQuery{
		Stream:    "default",
		Service:   "payment-service",
		StartTime: start,
		EndTime:   end,
	}

	results, err := env.client.QueryMetric(context.Background(), q)
	if err != nil {
		t.Fatalf("QueryMetric failed: %v", err)
	}

	if len(results) == 0 {
		t.Fatal("QueryMetric returned 0 results, expected at least 1 aggregation row")
	}

	for i, r := range results {
		t.Logf("  [%d] name=%s score=%.2f summary=%.80s", i, r.Name, r.Score, r.Summary)
	}
}

// testBuildDrilldownURL tests URL generation for all drill-down types.
func (env *ooTestEnv) testBuildDrilldownURL(t *testing.T) {
	now := time.Now().Unix()
	tests := []struct {
		name    string
		ref     openobserve.DrilldownRef
		wantPfx string
	}{
		{
			name: "logs drilldown",
			ref: openobserve.DrilldownRef{
				Type:      "logs",
				Stream:    "default",
				StartTime: now - 3600,
				EndTime:   now,
			},
			wantPfx: env.baseURL + "/web/logs?stream=default",
		},
		{
			name: "traces drilldown with trace_id",
			ref: openobserve.DrilldownRef{
				Type:      "traces",
				Stream:    "default",
				TraceID:   "abc123",
				StartTime: now - 3600,
				EndTime:   now,
			},
			wantPfx: env.baseURL + "/web/traces?stream=default",
		},
		{
			name: "traces drilldown without trace_id",
			ref: openobserve.DrilldownRef{
				Type:      "traces",
				Stream:    "default",
				StartTime: now - 3600,
				EndTime:   now,
			},
			wantPfx: env.baseURL + "/web/traces?stream=default",
		},
		{
			name: "metrics drilldown",
			ref: openobserve.DrilldownRef{
				Type:      "metrics",
				Stream:    "default",
				StartTime: now - 3600,
				EndTime:   now,
			},
			wantPfx: env.baseURL + "/web/metrics?stream=default",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			url, err := env.client.BuildDrilldownURL(tc.ref)
			if err != nil {
				t.Fatalf("BuildDrilldownURL returned error: %v", err)
			}
			if url == "" {
				t.Fatal("BuildDrilldownURL returned empty URL")
			}
			if len(url) < len(tc.wantPfx) || url[:len(tc.wantPfx)] != tc.wantPfx {
				t.Errorf("URL = %q, want prefix %q", url, tc.wantPfx)
			}
			t.Logf("drilldown URL: %s", url)
		})
	}

	// Test unknown type returns error.
	t.Run("unknown type", func(t *testing.T) {
		_, err := env.client.BuildDrilldownURL(openobserve.DrilldownRef{
			Type: "unknown",
		})
		if err == nil {
			t.Error("expected error for unknown drilldown type, got nil")
		}
	})
}
