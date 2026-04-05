package openobserve

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"github.com/atlanssia/aisre/internal/contract"
)

// Client is the HTTP client for OpenObserve API.
type Client struct {
	baseURL  string
	orgID   string
	token   string
	username string
	password string
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
	if logger == nil {
		logger = slog.Default()
	}
	return &Client{
		baseURL:  cfg.BaseURL,
		orgID:   cfg.OrgID,
		token:   cfg.Token,
		username: cfg.Username,
		password: cfg.Password,
		http:    &http.Client{Timeout: cfg.Timeout},
		logger:  logger,
	}, nil
}

// Login authenticates against the OO API and stores the Bearer token.
func (c *Client) Login(ctx context.Context) error {
	if c.username == "" || c.password == "" {
		return fmt.Errorf("openobserve: username and password required for login")
	}

	payload, _ := json.Marshal(map[string]string{
		"name":     c.username,
		"password": c.password,
	})

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/auth/login", bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("openobserve: create login request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("openobserve: login request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("openobserve: login failed (status %d): %s", resp.StatusCode, string(respBody))
	}

	var loginResp struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal(respBody, &loginResp); err != nil {
		return fmt.Errorf("openobserve: parse login response: %w", err)
	}

	if loginResp.AccessToken == "" {
		return fmt.Errorf("openobserve: login succeeded but no access_token returned")
	}

	c.token = loginResp.AccessToken
	c.logger.Info("openobserve: login successful")
	return nil
}

// SearchLogs queries OO logs via Search API and returns normalized results.
func (c *Client) SearchLogs(ctx context.Context, q LogQuery) ([]contract.ToolResult, error) {
	payload := map[string]any{
		"query": map[string]any{
			"sql":       c.buildLogSQL(q),
			"start_time": q.StartTime,
			"end_time":   q.EndTime,
			"sql_mode":   "full",
		},
	}
	if q.Limit > 0 {
		payload["size"] = q.Limit
	}

	body, _ := json.Marshal(payload)
	respBody, err := c.doRequest(ctx, "POST", fmt.Sprintf("/api/%s/_search", c.orgID), body)
	if err != nil {
		return nil, fmt.Errorf("search logs: %w", err)
	}

	var resp struct {
		Hits []map[string]any `json:"hits"`
	}
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("parse log response: %w", err)
	}

	var results []contract.ToolResult
	for _, hit := range resp.Hits {
		score := scoreFromLevel(extractString(hit, "level"))
		results = append(results, mapLogHit(hit, score))
	}
	return results, nil
}

// SearchTrace queries OO traces and returns normalized results.
func (c *Client) SearchTrace(ctx context.Context, q TraceQuery) ([]contract.ToolResult, error) {
	sql := c.buildTraceSQL(q)

	payload := map[string]any{
		"query": map[string]any{
			"sql":        sql,
			"start_time": q.StartTime,
			"end_time":   q.EndTime,
		},
	}
	body, _ := json.Marshal(payload)
	respBody, err := c.doRequest(ctx, "POST", fmt.Sprintf("/api/%s/_search", c.orgID), body)
	if err != nil {
		return nil, fmt.Errorf("search trace: %w", err)
	}

	var resp struct {
		Hits []map[string]any `json:"hits"`
	}
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("parse trace response: %w", err)
	}

	var results []contract.ToolResult
	for i, hit := range resp.Hits {
		score := 0.9 - float64(i)*0.1
		results = append(results, mapSpan(hit, score))
	}
	return results, nil
}

// QueryMetric queries OO metrics via SQL aggregation.
func (c *Client) QueryMetric(ctx context.Context, q MetricQuery) ([]contract.ToolResult, error) {
	sql := c.buildMetricSQL(q)

	payload := map[string]any{
		"query": map[string]any{
			"sql":        sql,
			"start_time": q.StartTime,
			"end_time":   q.EndTime,
		},
	}
	body, _ := json.Marshal(payload)
	respBody, err := c.doRequest(ctx, "POST", fmt.Sprintf("/api/%s/_search", c.orgID), body)
	if err != nil {
		return nil, fmt.Errorf("query metric: %w", err)
	}

	var resp struct {
		Hits []map[string]any `json:"hits"`
	}
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("parse metric response: %w", err)
	}

	var results []contract.ToolResult
	for _, hit := range resp.Hits {
		level := extractString(hit, "level")
		cnt := fmt.Sprintf("%v", hit["cnt"])
		service := extractString(hit, "service")
		summary := fmt.Sprintf("%s: %s events (%s)", service, cnt, level)
		score := 0.5
		switch level {
		case "error", "fatal":
			score = 0.8
		case "warn", "warning":
			score = 0.6
		}
		results = append(results, contract.ToolResult{
			Name:    "metric_anomaly",
			Summary: summary,
			Score:   score,
			Payload: hit,
		})
	}
	return results, nil
}

// BuildDrilldownURL generates a URL linking to the OO UI for drill-down.
func (c *Client) BuildDrilldownURL(ref DrilldownRef) (string, error) {
	var path string
	switch ref.Type {
	case "logs":
		path = fmt.Sprintf("/web/logs?stream=%s&from=%d&to=%d", ref.Stream, ref.StartTime, ref.EndTime)
	case "traces":
		path = fmt.Sprintf("/web/traces?stream=%s&from=%d&to=%d", ref.Stream, ref.StartTime, ref.EndTime)
		if ref.TraceID != "" {
			path += "&trace_id=" + ref.TraceID
		}
	case "metrics":
		path = fmt.Sprintf("/web/metrics?stream=%s&from=%d&to=%d", ref.Stream, ref.StartTime, ref.EndTime)
	default:
		return "", fmt.Errorf("unknown drilldown type: %s", ref.Type)
	}
	return c.baseURL + path, nil
}

func (c *Client) doRequest(ctx context.Context, method, path string, body []byte) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.token != "" {
		auth := c.token
		if !strings.HasPrefix(auth, "Basic ") && !strings.HasPrefix(auth, "Bearer ") {
			auth = "Bearer " + auth
		}
		req.Header.Set("Authorization", auth)
	}

	c.logger.Debug("oo request", "method", method, "path", path)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("oo returned %d: %s", resp.StatusCode, string(respBody))
	}

	return respBody, nil
}

// sanitize removes characters that could enable SQL injection when user input
// is embedded in SQL strings sent to the OpenObserve _search API.
// OpenObserve accepts SQL as a plain string over HTTP with no parameterized
// query support, so we strip dangerous characters instead.
func sanitize(s string) string {
	s = strings.ReplaceAll(s, "'", "")
	s = strings.ReplaceAll(s, `"`, "")
	s = strings.ReplaceAll(s, "\\", "")
	s = strings.ReplaceAll(s, ";", "")
	return s
}

func (c *Client) buildLogSQL(q LogQuery) string {
	sql := fmt.Sprintf("SELECT * FROM \"%s\"", sanitize(q.Stream))
	var conditions []string
	if q.Service != "" {
		conditions = append(conditions, fmt.Sprintf("service = '%s'", sanitize(q.Service)))
	}
	for _, kw := range q.Keywords {
		conditions = append(conditions, fmt.Sprintf("message LIKE '%%%s%%'", sanitize(kw)))
	}
	if len(conditions) > 0 {
		sql += " WHERE " + conditions[0]
		for _, cond := range conditions[1:] {
			sql += " AND " + cond
		}
	}
	sql += " ORDER BY _timestamp DESC"
	if q.Limit > 0 {
		sql += fmt.Sprintf(" LIMIT %d", q.Limit)
	}
	return sql
}

// buildTraceSQL constructs the SQL query for trace searches with sanitized inputs.
func (c *Client) buildTraceSQL(q TraceQuery) string {
	sql := fmt.Sprintf("SELECT * FROM \"%s\"", sanitize(q.Stream))
	var conditions []string
	if q.TraceID != "" {
		conditions = append(conditions, fmt.Sprintf("trace_id = '%s'", sanitize(q.TraceID)))
	}
	if q.Service != "" {
		conditions = append(conditions, fmt.Sprintf("service_name = '%s'", sanitize(q.Service)))
	}
	for i, cond := range conditions {
		if i == 0 {
			sql += " WHERE "
		} else {
			sql += " AND "
		}
		sql += cond
	}
	sql += " ORDER BY duration_ms DESC"
	if q.Limit > 0 {
		sql += fmt.Sprintf(" LIMIT %d", q.Limit)
	}
	return sql
}

// buildMetricSQL constructs the SQL query for metric searches with sanitized inputs.
// OO streams are schema-less; we aggregate by level to detect error rate anomalies
// rather than assuming a specific metric column exists.
func (c *Client) buildMetricSQL(q MetricQuery) string {
	sql := fmt.Sprintf("SELECT level, count(*) as cnt, service FROM \"%s\"", sanitize(q.Stream))
	var conditions []string
	if q.Service != "" {
		conditions = append(conditions, fmt.Sprintf("service = '%s'", sanitize(q.Service)))
	}
	if len(conditions) > 0 {
		sql += " WHERE " + conditions[0]
		for _, cond := range conditions[1:] {
			sql += " AND " + cond
		}
	}
	sql += " GROUP BY level, service ORDER BY cnt DESC LIMIT 20"
	return sql
}

func scoreFromLevel(level string) float64 {
	switch level {
	case "error", "fatal":
		return 0.9
	case "warn", "warning":
		return 0.7
	default:
		return 0.5
	}
}
