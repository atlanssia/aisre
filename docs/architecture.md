# AI RCA Workbench — 架构设计

> 版本：v1.0

---

## 1. 架构原则

设计遵循五大原则：

1. **轻平台化，避免过度编排** — 不构建复杂工作流引擎，用简单的 pipeline 处理
2. **Tool First，后端插件化** — 所有观测后端通过统一接口接入
3. **AI 信息压缩优先** — 目标是压缩信息，不是展示原始数据
4. **与观测平台解耦** — 不依赖任何单一观测平台
5. **为自动修复预留接口** — Phase 4 实现闭环自动修复

---

## 2. 总体架构

```text
┌────────────────────────────────────────────┐
│                Web UI Portal               │
│ Alert Workbench / RCA Report / Timeline    │
└───────────────────┬────────────────────────┘
                    │
┌───────────────────▼────────────────────────┐
│                API Backend                 │
│ Incident API / Report API / Search API     │
└───────────────────┬────────────────────────┘
                    │
┌───────────────────▼────────────────────────┐
│              Analysis Engine               │
│ Signal Fusion / Evidence Chain / LLM RCA   │
└───────────────────┬────────────────────────┘
                    │
┌───────────────────▼────────────────────────┐
│              Tool Adapter SDK              │
│ OO / SigNoz / Elastic / Jaeger / Prom      │
└────────────────────────────────────────────┘
```

---

## 3. 分层设计

### 3.1 Presentation Layer

职责：

- 告警工作台
- RCA 报告展示
- 时间线
- 拓扑影响图
- 建议动作
- 人工反馈

### 3.2 API Layer

职责：

- 接收告警事件
- 发起分析任务
- 查询历史 RCA
- 报告搜索
- 反馈闭环

技术选型：

- Go + Chi
- REST + SSE（Server-Sent Events）
- WebSocket（实时事件）

### 3.3 Analysis Engine（核心）

核心职责：

- 多信号融合
- Evidence Ranking
- 异常模式检测
- LLM 推理
- 建议生成
- 置信度评分

子模块：

| Module | Responsibility |
|--------|---------------|
| Incident Context Builder | 构建分析上下文（服务、时间、Trace、变更等） |
| Evidence Retriever | 从各后端获取原始证据 |
| Pattern Matcher | 异常模式匹配 |
| RCA Prompt Builder | 构建 LLM 输入 |
| Recommendation Engine | 生成修复建议 |
| Confidence Scorer | 评估分析置信度 |

### 3.4 Tool Adapter SDK（灵魂）

这是平台长期可扩展性的核心。

```text
adapter/
 ├── openobserve/
 ├── signoz/
 ├── elastic/
 ├── jaeger/
 ├── prometheus/
 └── custom/
```

统一接口：

```go
package adapter

type ToolProvider interface {
    SearchAlerts(ctx context.Context, q AlertQuery) ([]AlertEvent, error)
    SearchLogs(ctx context.Context, q LogQuery) ([]LogRecord, error)
    GetTrace(ctx context.Context, traceID string) (*TraceData, error)
    QueryMetric(ctx context.Context, q MetricQuery) (*MetricSeries, error)
    GetTopology(ctx context.Context, service string) (*TopologyGraph, error)
    GetRecentChanges(ctx context.Context, service string) ([]ChangeEvent, error)
}
```

---

## 4. 数据库设计

### 4.1 incidents

```sql
CREATE TABLE incidents (
  id BIGSERIAL PRIMARY KEY,
  source VARCHAR(64),
  severity VARCHAR(16),
  service_name VARCHAR(128),
  started_at TIMESTAMP,
  status VARCHAR(32),
  created_at TIMESTAMP DEFAULT NOW(),
  updated_at TIMESTAMP DEFAULT NOW()
);
```

### 4.2 rca_reports

```sql
CREATE TABLE rca_reports (
  id BIGSERIAL PRIMARY KEY,
  incident_id BIGINT REFERENCES incidents(id),
  summary TEXT,
  root_cause TEXT,
  confidence NUMERIC(5,2),
  report_json JSONB,
  status VARCHAR(32) DEFAULT 'generated',
  created_at TIMESTAMP DEFAULT NOW(),
  updated_at TIMESTAMP DEFAULT NOW()
);
```

### 4.3 evidence_items

```sql
CREATE TABLE evidence_items (
  id BIGSERIAL PRIMARY KEY,
  report_id BIGINT REFERENCES rca_reports(id),
  evidence_type VARCHAR(32), -- trace, log, metric, change
  score NUMERIC(5,2),
  payload JSONB,
  source_url TEXT,
  created_at TIMESTAMP DEFAULT NOW()
);
```

### 4.4 recommendations

```sql
CREATE TABLE recommendations (
  id BIGSERIAL PRIMARY KEY,
  report_id BIGINT REFERENCES rca_reports(id),
  category VARCHAR(32), -- short_term, mid_term, long_term
  description TEXT,
  priority INTEGER,
  created_at TIMESTAMP DEFAULT NOW()
);
```

### 4.5 feedback

```sql
CREATE TABLE feedback (
  id BIGSERIAL PRIMARY KEY,
  report_id BIGINT REFERENCES rca_reports(id),
  user_id VARCHAR(128),
  rating INTEGER, -- 1-5
  comment TEXT,
  action_taken VARCHAR(32), -- accepted, rejected, modified
  created_at TIMESTAMP DEFAULT NOW()
);
```

---

## 5. API 设计

### 5.1 创建分析

```http
POST /api/v1/incidents/analyze
Content-Type: application/json

{
  "service_name": "payment",
  "time_range": { "start": "...", "end": "..." },
  "trace_id": "optional",
  "alert_source": "openobserve"
}
```

### 5.2 获取报告

```http
GET /api/v1/reports/{id}
```

### 5.3 证据 Drill-down

```http
GET /api/v1/reports/{id}/evidence
```

### 5.4 用户反馈

```http
POST /api/v1/reports/{id}/feedback
Content-Type: application/json

{
  "rating": 4,
  "comment": "根因准确，建议有用",
  "action_taken": "accepted"
}
```

### 5.5 历史搜索

```http
GET /api/v1/reports/search?q=redis+timeout&service=payment
```

### 5.6 告警 Webhook

```http
POST /api/v1/alerts/webhook
Content-Type: application/json

{
  "source": "openobserve",
  "alert_name": "High Error Rate",
  "service": "checkout",
  "severity": "critical",
  "payload": { ... }
}
```

---

## 6. 后端技术选型

| Component | Technology | Reason |
|-----------|-----------|--------|
| Language | Go 1.26.1 | 高性能、强类型、并发友好 |
| HTTP Framework | Chi | 轻量、高性能 |
| Database | SQLite (modernc.org/sqlite) | 嵌入式、零依赖、单文件部署 |
| Migration | golang-migrate | 版本化数据库迁移 |
| Config | Viper | 多格式配置支持 |
| Logging | slog | 结构化日志 |

> **设计决策：** 不引入 PostgreSQL / Redis / ES / OpenSearch 等外部中间件。
> SQLite 的 JSON1 扩展足以支持 JSONB 类型的查询需求。
> 目标是单二进制文件部署，降低运维复杂度。
