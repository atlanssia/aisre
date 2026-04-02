# AI RCA Workbench — 后端详细设计（HLD / LLD，研发实施版）

> 目标：为后端研发团队提供可直接拆分模块、定义 package、开始编码的详细设计。

---

## 1. 后端设计目标

基于工程约束，后端设计原则：

1. **以 `aisre` 作为 Go 工程根目录**，不再引入额外 `backend` 层级
2. **严格契约优先（Contract First）**：接口、DTO、错误码、测试用例先行
3. **Chi 作为唯一 HTTP Router**，避免 Fiber/Gin
4. **slog 统一日志标准**，兼容 OTel TraceID 注入
5. **SQLite 作为 MVP 唯一存储**，避免 Redis / PostgreSQL 复杂度
6. **严格 TDD 开发模式**：测试先于实现，覆盖正常流、异常流、边界场景
7. **前后端同仓**：`web/` 位于根目录，采用 React + Tailwind + shadcn/ui

---

## 2. 工程目录

```text
aisre/
├── cmd/
│   └── server/
│       └── main.go
├── internal/
│   ├── api/                # HTTP handlers + Chi router
│   │   ├── router.go
│   │   ├── middleware.go
│   │   ├── incident_handler.go
│   │   ├── report_handler.go
│   │   └── feedback_handler.go
│   ├── contract/           # DTO, request/response, error codes
│   │   ├── incident.go
│   │   ├── report.go
│   │   ├── feedback.go
│   │   ├── errors.go
│   │   └── tool.go
│   ├── incident/           # Incident business logic
│   ├── analysis/           # Core analysis engine
│   │   ├── service.go
│   │   ├── context_builder.go
│   │   ├── evidence_ranker.go
│   │   ├── recommendation.go
│   │   └── confidence.go
│   ├── report/             # Report management
│   ├── adapter/            # Tool Provider interface + implementations
│   │   ├── provider.go
│   │   └── openobserve/
│   ├── tool/               # Tool orchestration
│   │   ├── logs_tool.go
│   │   ├── trace_tool.go
│   │   ├── metrics_tool.go
│   │   └── topology_tool.go
│   ├── prompt/             # Prompt templates & builder
│   ├── feedback/           # Feedback logic
│   ├── store/              # SQLite repository layer
│   │   ├── sqlite.go
│   │   ├── incident_repo.go
│   │   ├── report_repo.go
│   │   └── feedback_repo.go
│   └── testkit/            # Shared test utilities
├── migrations/             # SQLite migrations
├── test/                   # Integration & E2E tests
│   ├── contract/
│   ├── api/
│   ├── analysis/
│   ├── adapter/
│   └── e2e/
├── web/                    # Frontend (React + Tailwind + shadcn/ui)
│   ├── src/
│   │   ├── pages/
│   │   ├── components/
│   │   ├── lib/
│   │   └── api/
│   └── tailwind.config.ts
├── configs/
├── go.mod
└── Makefile
```

关键决策：

- **移除 `pkg/`** — 所有公共类型放 `internal/contract/`
- **移除根级 `adapter/`** — adapter 移入 `internal/adapter/`
- **新增 `contract/`** — 契约先行，DTO 集中管理
- **新增 `testkit/`** — 共享测试工具
- **新增 `web/`** — 前端同仓
- **新增 `store/`** — 存储层抽象

---

## 3. Contract First 设计

契约必须先于模块实现。所有 DTO 定义在 `internal/contract/`。

### 3.1 Incident Contract

```go
package contract

type CreateIncidentRequest struct {
    Source    string `json:"source"`
    Service   string `json:"service"`
    Severity  string `json:"severity"`
    TimeRange string `json:"time_range"`
    TraceID   string `json:"trace_id,omitempty"`
}

type CreateIncidentResponse struct {
    IncidentID int64  `json:"incident_id"`
    ReportID   int64  `json:"report_id"`
    Status     string `json:"status"`
}
```

### 3.2 RCA Report Contract

```go
package contract

type RCAReport struct {
    Summary         string   `json:"summary"`
    RootCause       string   `json:"root_cause"`
    Confidence      float64  `json:"confidence"`
    EvidenceIDs     []string `json:"evidence_ids"`
    Recommendations []string `json:"recommendations"`
}
```

### 3.3 Feedback Contract

```go
package contract

type FeedbackRequest struct {
    Rating      int    `json:"rating"`       // 1-5
    Comment     string `json:"comment"`
    ActionTaken string `json:"action_taken"` // accepted, partial, rejected
}

type FeedbackResponse struct {
    ID        int64  `json:"id"`
    ReportID  int64  `json:"report_id"`
    Status    string `json:"status"`
}
```

### 3.4 Error Contract

```go
package contract

type ErrorResponse struct {
    Error string `json:"error"`
    Code  string `json:"code"`
}

const (
    ErrCodeInvalidRequest  = "INVALID_REQUEST"
    ErrCodeNotFound        = "NOT_FOUND"
    ErrCodeInternal        = "INTERNAL_ERROR"
    ErrCodeAdapterTimeout  = "ADAPTER_TIMEOUT"
    ErrCodeLLMFailed       = "LLM_FAILED"
)
```

### 3.5 Tool Contract

```go
package contract

type ToolResult struct {
    Name    string         `json:"name"`
    Summary string         `json:"summary"`
    Score   float64        `json:"score"`
    Payload map[string]any `json:"payload"`
}
```

---

## 4. HTTP 层（Chi）

### Router

```go
r := chi.NewRouter()
r.Use(middleware.RequestID)
r.Use(middleware.Recoverer)

r.Post("/api/v1/incidents/analyze", h.CreateIncident)
r.Get("/api/v1/reports/{id}", h.GetReport)
r.Get("/api/v1/reports", h.ListReports)
r.Get("/api/v1/reports/{id}/evidence", h.GetEvidence)
r.Post("/api/v1/reports/{id}/feedback", h.Feedback)
r.Post("/api/v1/alerts/webhook", h.Webhook)
```

### Middleware

- `RequestID` — 请求追踪
- `Recoverer` — panic 恢复
- `slog.RequestLogger` — 请求日志
- CORS（开发模式）

---

## 5. 日志标准（slog）

```go
logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
logger.Info("incident created",
    "incident_id", incidentID,
    "service", req.Service,
    "trace_id", req.TraceID,
)
```

日志规范字段：

| Field | Description |
|-------|-------------|
| `request_id` | HTTP request ID |
| `trace_id` | Distributed trace ID |
| `incident_id` | Incident identifier |
| `report_id` | Report identifier |
| `adapter` | Adapter name |
| `latency_ms` | Operation latency |

---

## 6. 存储层（SQLite）

### 6.1 incidents

```sql
CREATE TABLE incidents (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    source TEXT NOT NULL,
    service_name TEXT NOT NULL,
    severity TEXT NOT NULL,
    status TEXT NOT NULL,
    trace_id TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
```

### 6.2 reports

```sql
CREATE TABLE reports (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    incident_id INTEGER NOT NULL,
    summary TEXT NOT NULL,
    root_cause TEXT NOT NULL,
    confidence REAL,
    report_json TEXT NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
```

### 6.3 evidence_items

```sql
CREATE TABLE evidence_items (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    report_id INTEGER NOT NULL,
    evidence_type TEXT NOT NULL,
    score REAL,
    payload TEXT NOT NULL,
    source_url TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
```

### 6.4 feedback

```sql
CREATE TABLE feedback (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    report_id INTEGER NOT NULL,
    rating INTEGER NOT NULL,
    comment TEXT,
    action_taken TEXT NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
```

### 6.5 Store 接口

```go
type IncidentRepo interface {
    Create(ctx context.Context, inc *Incident) (int64, error)
    GetByID(ctx context.Context, id int64) (*Incident, error)
    List(ctx context.Context, filter IncidentFilter) ([]Incident, error)
}

type ReportRepo interface {
    Create(ctx context.Context, report *Report) (int64, error)
    GetByID(ctx context.Context, id int64) (*Report, error)
    List(ctx context.Context, filter ReportFilter) ([]Report, error)
}

type FeedbackRepo interface {
    Create(ctx context.Context, fb *Feedback) (int64, error)
    ListByReport(ctx context.Context, reportID int64) ([]Feedback, error)
}
```

---

## 7. Analysis 模块

```go
type Service interface {
    AnalyzeIncident(ctx context.Context, incidentID int64) (*contract.RCAReport, error)
}
```

Pipeline:

```text
Incident → ContextBuilder → ToolRetrieval → EvidenceRanker → LLM RCA → Recommendation → ConfidenceScore → Report
```

---

## 8. Tool & Adapter

### ToolProvider（在 internal/adapter/）

```go
type ToolProvider interface {
    Name() string
    SearchLogs(ctx context.Context, q LogQuery) ([]LogRecord, error)
    GetTrace(ctx context.Context, traceID string) (*TraceData, error)
    QueryMetric(ctx context.Context, q MetricQuery) (*MetricSeries, error)
}
```

### Tool 层（在 internal/tool/）

```go
type Tool interface {
    Name() string
    Execute(ctx context.Context, incident *Incident) (*contract.ToolResult, error)
}
```

Tool 层编排多个 Adapter 调用，输出统一的 `ToolResult`。

---

## 9. Frontend 目录

```text
web/
├── src/
│   ├── pages/
│   │   ├── AlertWorkbench.tsx
│   │   ├── RCAReport.tsx
│   │   └── History.tsx
│   ├── components/
│   │   ├── TldrCard.tsx
│   │   ├── Timeline.tsx
│   │   ├── EvidenceCard.tsx
│   │   └── FeedbackForm.tsx
│   ├── lib/
│   │   └── api.ts
│   └── api/
├── tailwind.config.ts
└── package.json
```

技术栈：React + TypeScript + TailwindCSS + shadcn/ui

---

## 10. 严格 TDD 规范

原则：

> **No implementation before failing tests**

### 10.1 Contract Test（`test/contract/`）

必须先写：

- request schema validation
- response schema validation
- error schema validation

### 10.2 API Handler Test（`test/api/`）

覆盖场景：

| Scenario | Description |
|----------|-------------|
| 正常创建 | 请求参数合法，返回 201 |
| 参数缺失 | 缺少必填字段，返回 400 |
| 无效 trace_id | trace_id 格式错误，返回 400 |
| Adapter timeout | 后端超时，返回 504 |
| SQLite write fail | 数据库写入失败，返回 500 |

### 10.3 Analysis Test（`test/analysis/`）

覆盖场景：

| Scenario | Description |
|----------|-------------|
| 单日志根因 | 仅日志证据，定位根因 |
| Trace 根因 | Trace 证据定位根因 |
| 多证据排序 | 多信号融合，正确排序 |
| 无证据 fallback | 无可用证据，返回低置信度 |
| LLM 返回空 | LLM 无输出，graceful 降级 |

### 10.4 Adapter Test（`test/adapter/`）

覆盖场景：

| Scenario | Description |
|----------|-------------|
| 查询成功 | 正常返回数据 |
| 401 未授权 | Token 过期/无效 |
| Timeout | 请求超时 |
| Malformed response | 响应格式异常 |
| Empty traces | 无 trace 数据 |

### 10.5 E2E Test（`test/e2e/`）

完整链路：

```text
Webhook → CreateIncident → Analyze → PersistReport → QueryReport → Feedback
```

---

## 11. 完备测试用例清单

### Incident

- severity 非法值
- 空 service
- 重复 webhook
- trace_id 缺失

### Report

- report 不存在
- confidence 越界
- recommendations 为空

### Feedback

- 重复提交
- 非法 rating（< 1 或 > 5）

### SQLite

- db lock
- migration fail
- disk full

### OO Adapter

- query SQL 错误
- stream 不存在
- trace drilldown URL 异常

---

## 12. Makefile / CI 规范

```makefile
.PHONY: test test-contract test-api test-e2e lint build

test:
	go test ./...

test-contract:
	go test ./test/contract/...

test-api:
	go test ./test/api/...

test-e2e:
	go test ./test/e2e/...

lint:
	golangci-lint run

build:
	go build -o bin/aisre ./cmd/server
```

CI Gate:

| Layer | Coverage Target |
|-------|----------------|
| Contract tests | 100% |
| API handlers | > 90% |
| Analysis engine | > 85% |
| Adapter | > 90% |

---

## 13. Sprint 拆分（TDD 驱动）

### Sprint A

- contract 定义
- sqlite store
- api tests

### Sprint B

- OO adapter tests
- tool tests

### Sprint C

- analysis tests
- prompt tests
- report tests

### Sprint D

- e2e tests
- perf tests
- regression suite
