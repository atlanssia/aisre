# M2: OpenObserve Adapter + Tool Layer - Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Connect to OpenObserve for logs/traces/metrics search, build tool orchestrator for multi-signal evidence retrieval.

**Architecture:** OO HTTP Client → Provider interface → Tool Orchestrator → parallel evidence collection

**Tech Stack:** Go stdlib net/http, encoding/json, context, sync

---

## Overview and Scope

M2 builds on the completed M1 (Incident CRUD + Webhook + SQLite). The goal is to connect the aisre platform to a real OpenObserve instance so the system can search logs, traces, and metrics, and normalize all results into `contract.ToolResult` for downstream RCA consumption.

The work is organized into 8 tasks, strictly following TDD (red-green-refactor). Each task specifies test files first, then implementation files.

---

## Key Architectural Observations

**Two separate interfaces exist.** The OO adapter package defines its own `Provider` interface in `internal/adapter/openobserve/contract.go`, which differs from the general `ToolProvider` interface in `internal/adapter/provider.go`. The OO-specific `Provider` returns `[]contract.ToolResult` directly (higher level), while `ToolProvider` returns raw domain types (`[]LogRecord`, `*TraceData`, `*MetricSeries`). The M2 implementation should satisfy the OO-specific `Provider` interface since it is the one that aligns with the OO adapter spec document and returns already-normalized results. The general `ToolProvider` can be satisfied later via an adapter wrapper if needed.

**OO API shape (confirmed from documentation).**
- Logs/Metrics/Traces SQL queries all go through `POST /api/{org}/_search` with body `{ "query": { "sql": "...", "start_time": ..., "end_time": ..., "from": 0, "size": N } }`.
- Response: `{ "took": int, "hits": [...], "total": int, "from": int, "size": int, "scan_size": int }`.
- Trace metadata uses `GET /api/{org}/{stream}/traces/latest?start_time=...&end_time=...&from=0&size=N`.
- Authentication is via Basic auth header (email:password base64 encoded).

**Existing code.**
- `contract.go` already defines `Config`, `Client` struct, `NewClient`, and the `Provider` interface.
- `mapper.go` already defines `mapLogHit` and `mapSpan` helper functions.
- `errors.go` already defines `AdapterError` type with sentinel values.

**No new dependencies needed.** The project uses only stdlib `net/http` and `encoding/json` for HTTP client work.

---

## Task 1: OO HTTP Client Core (`client.go`)

**Objective:** Implement the HTTP request layer that all search methods will share. This includes authentication, request building, response parsing, and error mapping.

### Files

- Create: `internal/adapter/openobserve/client.go`
- Create: `internal/adapter/openobserve/client_test.go`

### Test-First (client_test.go)

Write tests using `net/http/httptest` for the following scenarios:

1. **TestNewClient_MissingBaseURL** -- verifies `NewClient` returns error when `Config.BaseURL` is empty.
2. **TestNewClient_MissingOrgID** -- verifies error when `Config.OrgID` is empty.
3. **TestNewClient_MissingToken** -- verifies error when `Config.Token` is empty.
4. **TestNewClient_Success** -- verifies client created with correct fields.
5. **TestClient_doRequest_Success** -- httptest server returns 200 with JSON, verifies body is decoded.
6. **TestClient_doRequest_AuthHeader** -- verifies Basic auth header is set correctly (token as base64).
7. **TestClient_doRequest_401Response** -- server returns 401, verifies `ErrAuthFailed` is returned.
8. **TestClient_doRequest_400Response** -- server returns 400, verifies `ErrInvalidQuery` is returned.
9. **TestClient_doRequest_404Response** -- server returns 404, verifies `ErrStreamNotFound`.
10. **TestClient_doRequest_500Response** -- server returns 500, verifies `ErrProviderInternal`.
11. **TestClient_doRequest_Timeout** -- slow server exceeds context deadline, verifies `ErrProviderTimeout`.

### Implementation (client.go)

The key methods to implement on the existing `Client` struct:

```go
// doRequest sends an HTTP request to the OO API and returns the parsed response.
// It handles auth, error mapping, and response decoding.
func (c *Client) doRequest(ctx context.Context, method, path string, body any, result any) error
```

This method:
1. Serializes body to JSON (for POST requests).
2. Sets `Authorization: Basic {token}` header.
3. Sets `Content-Type: application/json`.
4. Sends the request with the client's http.Client (which already has timeout from Config).
5. Reads the response body.
6. If status is not 200, maps HTTP status to `AdapterError` using the error matrix from errors.go.
7. If status is 200, JSON-decodes into `result`.

The `Client` already has a private `http` field of type `*http.Client`. We also need a getter so tests can inject a custom transport. The simplest approach: add an `HTTPClient` field that is exported, or provide a `NewClientWithHTTP` constructor. Given the existing `NewClient` already creates an `http.Client` internally, the cleaner approach is to make the internal `http` field settable via an option or to add a test-only constructor.

**Recommended pattern:** Add an unexported method and use the existing `http` field. In tests, construct a `Client` directly and set the `http` field. Since tests are in the same package (`package openobserve`), they can access unexported fields.

### OO Search Request/Response Types

Define in `client.go` (or a separate `types.go` file if you prefer):

```go
// searchRequest is the body sent to POST /api/{org}/_search.
type searchRequest struct {
    Query searchQuery `json:"query"`
}

type searchQuery struct {
    SQL       string `json:"sql"`
    StartTime int64  `json:"start_time"`
    EndTime   int64  `json:"end_time"`
    From      int    `json:"from"`
    Size      int    `json:"size"`
}

// searchResponse is the response from OO _search endpoint.
type searchResponse struct {
    Took     int64            `json:"took"`
    Hits     []map[string]any `json:"hits"`
    Total    int64            `json:"total"`
    From     int              `json:"from"`
    Size     int              `json:"size"`
    ScanSize int64            `json:"scan_size"`
}
```

---

## Task 2: SearchLogs Implementation (`logs.go`)

**Objective:** Implement the `SearchLogs` method on `Client` to satisfy the `Provider` interface.

### Files

- Create: `internal/adapter/openobserve/logs.go`
- Create: `internal/adapter/openobserve/logs_test.go`

### Test-First (logs_test.go)

1. **TestClient_SearchLogs_Success** -- mock server returns hits, verify `[]contract.ToolResult` with correct Name="critical_log_cluster", Summary truncated at 200 chars, Score based on hit relevance.
2. **TestClient_SearchLogs_EmptyHits** -- server returns `{ "hits": [] }`, verify empty slice, no error.
3. **TestClient_SearchLogs_WithServiceFilter** -- verify the generated SQL contains `service_name = '{service}'`.
4. **TestClient_SearchLogs_WithKeywords** -- verify SQL contains `str_match(log, '{keyword}')` for each keyword.
5. **TestClient_SearchLogs_DefaultLimit** -- when `LogQuery.Limit` is 0, verify default of 100 is used.
6. **TestClient_SearchLogs_TimeRangeMandatory** -- verify the SQL always includes time range in the request body.
7. **TestClient_SearchLogs_AuthError** -- mock 401 response.
8. **TestClient_SearchLogs_StreamNotFound** -- mock 404 response.

### Implementation (logs.go)

```go
// SearchLogs implements Provider.SearchLogs.
// It builds a SQL query from LogQuery, sends it to OO _search,
// and maps each hit to a contract.ToolResult.
func (c *Client) SearchLogs(ctx context.Context, q LogQuery) ([]contract.ToolResult, error)
```

SQL building logic:

```go
func buildLogsSQL(q LogQuery) string {
    // Base: SELECT * FROM "{stream}"
    // WHERE conditions:
    //   - service_name = '{service}' (if Service non-empty)
    //   - str_match(log, '{keyword}') for each keyword
    // ORDER BY _timestamp DESC
    // LIMIT {limit}
}
```

The existing `mapLogHit` in `mapper.go` handles individual hit mapping. The `SearchLogs` method iterates over `searchResponse.Hits`, calls `mapLogHit` for each, and collects results. Scoring logic: use a simple heuristic -- hits with "error"/"fatal" in level get higher score (0.9), others get a base score (0.5). Keep this simple for MVP.

---

## Task 3: SearchTrace Implementation (`traces.go`)

**Objective:** Implement `SearchTrace` with two-stage query: first get trace metadata, then optionally get span details.

### Files

- Create: `internal/adapter/openobserve/traces.go`
- Create: `internal/adapter/openobserve/traces_test.go`

### Test-First (traces_test.go)

1. **TestClient_SearchTrace_ByTraceID** -- mock server for `_search` with trace_id filter, verify spans are mapped to ToolResult with Name="slowest_span".
2. **TestClient_SearchTrace_ByService** -- verify SQL includes `service_name` filter.
3. **TestClient_SearchTrace_EmptyResult** -- empty hits, returns empty slice.
4. **TestClient_SearchTrace_MalformedDuration** -- duration field missing or non-string, verify graceful degradation (summary still produced).
5. **TestClient_SearchTrace_DefaultLimit** -- when Limit is 0, verify default 50.

### Implementation (traces.go)

For M2, keep it simple: use the `_search` SQL approach for both metadata and span queries. The two-stage optimization (GET traces/latest then selective spans) can be added later.

```go
// SearchTrace implements Provider.SearchTrace.
func (c *Client) SearchTrace(ctx context.Context, q TraceQuery) ([]contract.ToolResult, error)
```

SQL building:

```go
func buildTraceSQL(q TraceQuery) string {
    // If TraceID is set: SELECT * FROM "{stream}" WHERE trace_id = '{traceID}'
    // If Service is set: add service_name = '{service}'
    // ORDER BY start_time
    // LIMIT {limit}
}
```

Use the existing `mapSpan` from `mapper.go` for individual span mapping.

---

## Task 4: QueryMetric Implementation (`metrics.go`)

**Objective:** Implement `QueryMetric` using SQL aggregation over log data (not PromQL), as specified in the OO adapter spec.

### Files

- Create: `internal/adapter/openobserve/metrics.go`
- Create: `internal/adapter/openobserve/metrics_test.go`

### Test-First (metrics_test.go)

1. **TestClient_QueryMetric_ErrorRate** -- MetricQuery with Metric="error_rate", verify SQL uses `COUNT(*) WHERE level='error'` with histogram.
2. **TestClient_QueryMetric_P95** -- Metric="p95", verify appropriate SQL aggregation.
3. **TestClient_QueryMetric_IntervalDefault** -- verify default interval "1 minute" used when Interval is empty.
4. **TestClient_QueryMetric_EmptyResult** -- no hits, returns empty slice.
5. **TestClient_QueryMetric_InvalidMetric** -- unknown metric type returns error.

### Implementation (metrics.go)

```go
// QueryMetric implements Provider.QueryMetric.
func (c *Client) QueryMetric(ctx context.Context, q MetricQuery) ([]contract.ToolResult, error)
```

SQL templates for supported metrics:

- **error_rate**: `SELECT histogram(_timestamp, '{interval}') AS ts, COUNT(*) AS errors FROM "{stream}" WHERE level='error' [AND service_name='{service}'] GROUP BY ts ORDER BY ts`
- **p95**: `SELECT histogram(_timestamp, '{interval}') AS ts, approx_percentile_cont(0.95, duration) AS p95 FROM "{stream}" [WHERE service_name='{service}'] GROUP BY ts ORDER BY ts`
- For generic metrics: `SELECT histogram(_timestamp, '{interval}') AS ts, COUNT(*) AS value FROM "{stream}" [WHERE service_name='{service}'] GROUP BY ts ORDER BY ts`

Each result row maps to a `contract.ToolResult` with:
- Name: "metric_data_point"
- Summary: formatted string like "error_rate at {ts}: {value}"
- Score: 0.7 (base metric score, can be refined in M3)
- Payload: the raw row data

---

## Task 5: BuildDrilldownURL Implementation (`drilldown.go`)

**Objective:** Generate OO UI URLs that link back to the source platform for drill-down.

### Files

- Create: `internal/adapter/openobserve/drilldown.go`
- Create: `internal/adapter/openobserve/drilldown_test.go`

### Test-First (drilldown_test.go)

1. **TestBuildDrilldownURL_Logs** -- Type="logs", verify URL is `{base}/web/logs?stream={stream}&sql={encoded_sql}&from={start}&to={end}`.
2. **TestBuildDrilldownURL_Traces** -- Type="traces", TraceID set, verify URL includes `trace_id` parameter.
3. **TestBuildDrilldownURL_Metrics** -- Type="metrics", verify URL includes stream and time range.
4. **TestBuildDrilldownURL_EmptyBaseURL** -- returns error.
5. **TestBuildDrilldownURL_InvalidType** -- unknown type returns error.
6. **TestBuildDrilldownURL_SQLEscaping** -- SQL with special characters is properly URL-encoded.

### Implementation (drilldown.go)

```go
// BuildDrilldownURL implements Provider.BuildDrilldownURL.
func (c *Client) BuildDrilldownURL(ref DrilldownRef) (string, error)
```

URL patterns from the spec:
- Logs: `{baseURL}/web/logs?stream={stream}&sql={url_encode(sql)}&from={start_us}&to={end_us}`
- Traces: `{baseURL}/web/traces?stream={stream}&trace_id={traceID}`
- Metrics: `{baseURL}/web/logs?stream={stream}&sql={url_encode(sql)}&from={start_us}&to={end_us}` (metrics via logs SQL for now)

This method does not need HTTP -- it is pure URL construction.

---

## Task 6: Tool Orchestrator (`internal/tool/orchestrator.go`)

**Objective:** Create the orchestration layer that coordinates multi-signal retrieval (logs -> traces -> metrics) for a given incident context.

### Files

- Create: `internal/tool/orchestrator.go`
- Create: `internal/tool/orchestrator_test.go`

### Design

The Orchestrator depends on the OO Provider interface (from `internal/adapter/openobserve`), not the raw `ToolProvider` from `internal/adapter/provider.go`. It takes an incident context and orchestrates calls in sequence:

1. Search logs first (primary signal)
2. If incident has a TraceID, search traces
3. Query metrics for the time range
4. Collect all ToolResults, deduplicate, sort by score descending
5. Return top N results

### Test-First (orchestrator_test.go)

Use a mock Provider:

```go
type mockProvider struct {
    logs    []contract.ToolResult
    traces  []contract.ToolResult
    metrics []contract.ToolResult
    err     error
}
```

Tests:
1. **TestOrchestrator_Execute_AllSignals** -- mock returns logs, traces, metrics. Verify all are collected and sorted by score.
2. **TestOrchestrator_Execute_LogsOnly** -- no TraceID, traces return empty. Verify logs + metrics only.
3. **TestOrchestrator_Execute_LogsError** -- logs search fails. Verify error propagated.
4. **TestOrchestrator_Execute_TraceError** -- trace search fails but does not block other results (graceful degradation).
5. **TestOrchestrator_Execute_Deduplication** -- duplicate results are removed.
6. **TestOrchestrator_Execute_TopN** -- only top N results returned (configurable limit).

### Implementation (orchestrator.go)

```go
package tool

// Orchestrator coordinates multi-signal tool execution for an incident.
type Orchestrator struct {
    provider ooProvider  // interface from openobserve package
    logger   *slog.Logger
}

// ooProvider is a local interface matching openobserve.Provider.
// Defined here to avoid direct package dependency in tests.
type ooProvider interface {
    SearchLogs(ctx context.Context, q openobserve.LogQuery) ([]contract.ToolResult, error)
    SearchTrace(ctx context.Context, q openobserve.TraceQuery) ([]contract.ToolResult, error)
    QueryMetric(ctx context.Context, q openobserve.MetricQuery) ([]contract.ToolResult, error)
}

// Execute runs all tool queries for the given incident context.
func (o *Orchestrator) Execute(ctx context.Context, incident *contract.Incident, timeRange TimeRange) ([]contract.ToolResult, error)
```

The `TimeRange` struct:

```go
type TimeRange struct {
    StartTime int64 // microseconds
    EndTime   int64 // microseconds
}
```

**Important dependency decision:** The `internal/tool` package needs to depend on `internal/adapter/openobserve` for the query types (`LogQuery`, `TraceQuery`, `MetricQuery`). This is acceptable for M2 since OO is the only backend. For M3+ when multiple backends exist, this should be refactored to use the generic `ToolProvider` interface from `internal/adapter/provider.go`. For now, YAGNI.

---

## Task 7: Tool Implementations (`internal/tool/`)

**Objective:** Implement concrete `Tool` interface implementations for logs, traces, and metrics.

### Files

- Create: `internal/tool/logs_tool.go`
- Create: `internal/tool/logs_tool_test.go`
- Create: `internal/tool/trace_tool.go`
- Create: `internal/tool/trace_tool_test.go`
- Create: `internal/tool/metrics_tool.go`
- Create: `internal/tool/metrics_tool_test.go`

### Test-First Pattern (same for all three tools)

Each tool test mocks the provider and verifies:

1. **Execute_Success** -- provider returns results, tool returns aggregated ToolResult.
2. **Execute_ProviderError** -- provider returns error, tool returns error.
3. **Execute_EmptyResult** -- provider returns empty, tool returns nil result (not error).
4. **Execute_Name** -- returns correct tool name.

### Implementation Pattern

Each tool struct embeds a reference to the OO provider and implements the `Tool` interface from `internal/tool/tool.go`:

```go
type LogsTool struct {
    provider ooProvider
}

func (t *LogsTool) Name() string { return "search_logs" }

func (t *LogsTool) Execute(ctx context.Context, incident *contract.Incident) (*contract.ToolResult, error) {
    // Build LogQuery from incident fields
    // Call provider.SearchLogs
    // Aggregate results into a single summary ToolResult
}
```

---

## Task 8: Integration Tests with httptest Mock OO Server

**Objective:** End-to-end tests that verify the full adapter stack against a mock OO server, plus optional real-OO integration tests.

### Files

- Create: `test/adapter/openobserve/mock_server.go`
- Create: `test/adapter/openobserve/logs_integration_test.go`
- Create: `test/adapter/openobserve/traces_integration_test.go`
- Create: `test/adapter/openobserve/metrics_integration_test.go`
- Create: `test/adapter/openobserve/drilldown_integration_test.go`

### mock_server.go

A shared httptest server that simulates OO API responses:

```go
// NewMockServer creates an httptest.Server that simulates the OO API.
// Handlers can be customized per test using the handlerFunc field.
type MockOO struct {
    Server        *httptest.Server
    SearchHandler http.HandlerFunc
}
```

The mock server provides:
- `POST /api/{org}/_search` -- returns configurable search results
- `GET /api/{org}/{stream}/traces/latest` -- returns configurable trace metadata
- Default handlers for auth failures, stream not found, etc.

### Integration Test Patterns

Each integration test:
1. Creates a `MockOO` with specific response data
2. Creates an OO `Client` pointing at the mock server URL
3. Calls the method under test
4. Verifies the output matches expected `contract.ToolResult` shape

### Real OO Integration Tests (optional, tagged with build tag)

Create a separate file with `//go:build integration` tag that runs against a real OO instance at `http://localhost:5080`. These tests validate that the adapter works with actual OO responses:

- File: `test/adapter/openobserve/real_oo_test.go`
- Run with: `go test -tags=integration ./test/adapter/openobserve/...`
- Uses config from `configs/local.yaml` for connection details

---

## Task Sequencing and Dependencies

```
Task 1 (client.go)          -- no dependencies
Task 2 (logs.go)            -- depends on Task 1
Task 3 (traces.go)          -- depends on Task 1
Task 4 (metrics.go)         -- depends on Task 1
Task 5 (drilldown.go)       -- depends on Task 1 (for baseURL)
Task 6 (orchestrator.go)    -- depends on Tasks 2, 3, 4
Task 7 (tool impls)         -- depends on Task 6
Task 8 (integration tests)  -- depends on Tasks 2, 3, 4, 5
```

Tasks 2, 3, 4, and 5 can be implemented in parallel after Task 1 is complete. Task 6 requires all search methods. Task 7 and Task 8 can proceed in parallel after Task 6.

---

## What NOT to Do (YAGNI)

- Do not build a generic multi-backend abstraction layer yet. OO is the only backend for M2.
- Do not implement retry logic in the HTTP client. The existing Config has a Retries field but for M2, a single request with error mapping is sufficient.
- Do not implement streaming search. Use standard request/response.
- Do not implement caching. That is a Phase 2 concern.
- Do not implement the generic `ToolProvider` interface adapter yet. The OO-specific `Provider` interface is sufficient.
- Do not implement LLM integration. That is M3.
- Do not create new migration files. No database changes are needed for M2.

---

## File Summary

All files to create (23 new files, 0 modifications to existing files):

| File | Type | Task |
|------|------|------|
| `internal/adapter/openobserve/client.go` | impl | 1 |
| `internal/adapter/openobserve/client_test.go` | test | 1 |
| `internal/adapter/openobserve/logs.go` | impl | 2 |
| `internal/adapter/openobserve/logs_test.go` | test | 2 |
| `internal/adapter/openobserve/traces.go` | impl | 3 |
| `internal/adapter/openobserve/traces_test.go` | test | 3 |
| `internal/adapter/openobserve/metrics.go` | impl | 4 |
| `internal/adapter/openobserve/metrics_test.go` | test | 4 |
| `internal/adapter/openobserve/drilldown.go` | impl | 5 |
| `internal/adapter/openobserve/drilldown_test.go` | test | 5 |
| `internal/tool/orchestrator.go` | impl | 6 |
| `internal/tool/orchestrator_test.go` | test | 6 |
| `internal/tool/logs_tool.go` | impl | 7 |
| `internal/tool/logs_tool_test.go` | test | 7 |
| `internal/tool/trace_tool.go` | impl | 7 |
| `internal/tool/trace_tool_test.go` | test | 7 |
| `internal/tool/metrics_tool.go` | impl | 7 |
| `internal/tool/metrics_tool_test.go` | test | 7 |
| `test/adapter/openobserve/mock_server.go` | test util | 8 |
| `test/adapter/openobserve/logs_integration_test.go` | test | 8 |
| `test/adapter/openobserve/traces_integration_test.go` | test | 8 |
| `test/adapter/openobserve/metrics_integration_test.go` | test | 8 |
| `test/adapter/openobserve/drilldown_integration_test.go` | test | 8 |

No existing files need modification for M2 scope. The `contract.go`, `mapper.go`, and `errors.go` skeletons are already correct and will be consumed as-is.

---

### Critical Files for Implementation

- `internal/adapter/openobserve/contract.go` -- defines Provider interface, query types, Client struct, and Config that all M2 code implements against
- `internal/adapter/openobserve/mapper.go` -- contains mapLogHit and mapSpan helpers used by SearchLogs and SearchTrace
- `internal/adapter/openobserve/errors.go` -- defines AdapterError and sentinel values that client.go must return
- `internal/tool/tool.go` -- defines the Tool interface that logs_tool, trace_tool, metrics_tool must satisfy
- `internal/contract/tool.go` -- defines ToolResult, the unified output type for all adapter methods
