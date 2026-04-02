# AI RCA Workbench — OpenObserve Adapter SDK 规范

> 目标：定义 `aisre/internal/adapter/openobserve` 的标准 SDK 契约、目录结构、查询协议、错误模型、Drill-down 协议、测试规范。
> 原则：OO First、Contract First、Strict TDD、No UI Rebuild。

---

## 1. Adapter 设计目标

OO Adapter 的职责不是"复刻 OpenObserve API"，而是：

> **把 OO 原始 Logs / Traces / Metrics 能力压缩为 AI RCA 可消费的稳定工具协议。**

核心目标：

1. **Contract First**：统一 Tool 输出结构
2. **OO API 隔离**：屏蔽 SQL / Trace API 差异
3. **Drill-down Ready**：可一键跳转 OO 原始页面
4. **Time-range Mandatory**：强制时间窗，避免全表扫描
5. **Strict TDD**：Mock + Golden Case + Error Matrix

---

## 2. 目录结构

```text
internal/adapter/openobserve/
├── client.go       # HTTP Client + auth + retry
├── contract.go     # OO raw → internal contract mapping
├── mapper.go       # hits/span → ToolResult
├── logs.go         # Logs search implementation
├── traces.go       # Trace query implementation
├── metrics.go      # Metrics query implementation
├── drilldown.go    # UI deep link builder
├── errors.go       # OO error → BizError mapping
├── auth.go         # Basic / Bearer auth
└── client_test.go  # Unit tests
```

测试目录：

```text
test/adapter/openobserve/
├── contract_test.go
├── logs_test.go
├── traces_test.go
├── metrics_test.go
├── drilldown_test.go
└── e2e_test.go
```

---

## 3. Adapter 顶层契约

与 `internal/contract/tool.go` 严格对齐。

```go
package openobserve

type Provider interface {
    SearchLogs(ctx context.Context, q LogQuery) ([]contract.ToolResult, error)
    SearchTrace(ctx context.Context, q TraceQuery) ([]contract.ToolResult, error)
    QueryMetric(ctx context.Context, q MetricQuery) ([]contract.ToolResult, error)
    BuildDrilldownURL(ctx context.Context, ref DrilldownRef) (string, error)
}
```

设计红线：

> **上层 analysis 永远不直接依赖 OO API JSON。**

---

## 4. HTTP Client 规范

基于 Go stdlib `http.Client` + slog + context timeout + Basic/Bearer Auth。

```go
type Client struct {
    baseURL string
    orgID   string
    token   string
    http    *http.Client
    logger  *slog.Logger
}
```

默认配置：

- timeout: 10s
- retries: 2
- user-agent: `aisre-oo-adapter/1.0`

---

## 5. Logs Query 协议（Search API）

OO Logs 统一使用 Search API：

```http
POST /api/{org}/_search
```

### Query Builder

```go
type LogQuery struct {
    Stream    string
    Service   string
    Keywords  []string
    StartTime int64  // microseconds, mandatory
    EndTime   int64  // microseconds, mandatory
    Limit     int    // default 100
}
```

### SQL 模板

```sql
SELECT * FROM {stream}
WHERE service_name = '{service}'
AND str_match(log, '{keyword}')
ORDER BY _timestamp DESC
LIMIT {limit}
```

最佳实践：

- 必须限制时间窗（微秒精度）
- 默认 `LIMIT 100`
- 优先错误关键字
- 避免 `SELECT *` 长时间范围

---

## 6. Trace Query 协议

两段式查询策略：

### Step 1: Trace Metadata（快速获取）

```http
GET /api/{org}/{stream}/traces/latest
```

用于快速获取：trace_id, duration, service_name, first_event

### Step 2: Span Detail（深度分析）

复杂根因场景走 Search SQL：

```sql
SELECT * FROM default
WHERE trace_id = '{trace_id}'
ORDER BY start_time
```

### Trace Query

```go
type TraceQuery struct {
    Stream    string
    TraceID   string
    Service   string
    StartTime int64
    EndTime   int64
    Limit     int // default 50
}
```

AI RCA 优化策略：

- 先 metadata，再 selective spans
- span 默认 Top 50
- 大 trace 分页

---

## 7. Metrics Query 协议

MVP 推荐：统一走 Search SQL 聚合，不额外引入 PromQL Adapter。

```sql
SELECT histogram(_timestamp, '1 minute') AS ts,
       COUNT(*) AS errors
FROM {stream}
WHERE level='error'
GROUP BY ts
ORDER BY ts
```

输出给 RCA：

- baseline drift
- error spike
- p95 trend（Phase 2）

---

## 8. 标准输出映射

所有 OO 结果必须映射为统一 `contract.ToolResult`：

```go
type ToolResult struct {
    Name      string
    Summary   string
    Score     float64
    Timestamp int64
    Payload   map[string]any
}
```

### 日志映射

```go
ToolResult{
    Name:    "critical_log_cluster",
    Summary: "Redis timeout repeated 238x",
    Score:   0.94,
}
```

### Trace 映射

```go
ToolResult{
    Name:    "slowest_span",
    Summary: "payment redis get 4.2s",
    Score:   0.97,
}
```

> 这是 RCA 的核心稳定协议。

---

## 9. Drill-down URL 协议

不能只返回数据，必须支持跳转 OpenObserve UI。

```go
type DrilldownRef struct {
    Type      string // "logs" | "traces" | "metrics"
    Stream    string
    TraceID   string
    StartTime int64
    EndTime   int64
    SQL       string
}
```

### URL 生成规则

**Logs：**

```text
{base}/web/logs?stream=app_logs&sql=...
```

**Traces：**

```text
{base}/web/traces?stream=default&trace_id=abc123
```

前端 RCA 页面直接使用。

---

## 10. 错误模型映射

OO API 错误必须转业务错误。

```go
type AdapterError struct {
    Code       string // business error code
    HTTPStatus int
    Message    string
    Retryable  bool
}
```

错误矩阵：

| HTTP Status | Error Code | Retryable |
|-------------|-----------|-----------|
| 400 | `INVALID_QUERY` | false |
| 401 | `AUTH_FAILED` | false |
| 404 | `STREAM_NOT_FOUND` | false |
| 500 | `PROVIDER_INTERNAL` | true |
| timeout | `PROVIDER_TIMEOUT` | true |

---

## 11. TDD 测试规范

### Contract Tests

必须先写：

- ToolResult schema validation
- AdapterError schema validation
- DrilldownRef schema validation

### Logs Tests

| Scenario | Description |
|----------|-------------|
| 正常查询 | 返回 hits，映射为 ToolResult |
| 空 hits | 返回空 slice，无 error |
| SQL 语法错误 | 400 → INVALID_QUERY |
| stream 不存在 | 404 → STREAM_NOT_FOUND |
| 401 | AUTH_FAILED |
| 超大时间范围 | 拒绝或截断 |

### Trace Tests

| Scenario | Description |
|----------|-------------|
| latest 成功 | 返回 trace metadata |
| trace_id span 查询 | 返回 span 列表 |
| empty spans | 返回空 slice |
| malformed duration | 格式化降级 |

### Drill-down Tests

| Scenario | Description |
|----------|-------------|
| logs URL encode | SQL 参数正确编码 |
| trace URL | trace_id 正确拼装 |
| SQL escape | 防注入 |
| invalid baseURL | 返回 error |

### E2E Golden Cases

固定样本：

- redis timeout
- db slow query
- dify workflow timeout
- .NET grpc downstream timeout

---

## 12. 扩展规范（为 SigNoz 铺路）

后续直接复制到 `internal/adapter/signoz/`，必须保持：

- 相同 `ToolResult` 输出
- 相同 `DrilldownRef` 协议
- 相同 Error Matrix
- 相同 TDD 用例模板

这样 analysis 层无需感知 provider。

---

## 13. 产品级红线

### Adapter 负责

- query
- normalize
- drilldown URL
- retry
- auth

### Adapter 不负责

- RCA reasoning
- evidence ranking
- recommendation
- UI formatting

> **Adapter = Data Normalization Layer, not Intelligence Layer**
