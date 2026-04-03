# Phase 2 Design: Advanced RCA Intelligence

**Goal:** 扩展 Phase 1 MVP 的 3-Stage Pipeline，增加相似事件检索、变更关联、爆炸半径分析、Prompt Studio、告警聚合和 Postmortem 生成六大能力。

**Architecture:** Pipeline Enrichment 模式 — 在现有 3-Stage Pipeline 基础上插入新的数据收集和推理阶段，不替换核心流程。所有新功能作为独立 package 实现，通过 interface 解耦。

**Tech Stack:** Go 1.26.1 + SQLite（JSON1 扩展）+ 已有 LLM Client + `@xyflow/react`（仅拓扑图）+ 零新外部依赖

---

## 1. Feature Overview

| # | Feature | Package | 里程碑 | 优先级 |
|---|---------|---------|--------|--------|
| F1 | Similar Incident Retrieval | `internal/similar/` | M6 (W7-8) | P0 |
| F2 | Change Correlation | `internal/change/` | M7 (W8-9) | P0 |
| F3 | Blast Radius Analysis | `internal/topology/` | M8 (W9-10) | P1 |
| F4 | Prompt Studio | `internal/promptstudio/` | M9 (W10-11) | P1 |
| F5 | Alert Aggregation | `internal/alertgroup/` | M10 (W11-12) | P2 |
| F6 | Postmortem Generator | `internal/postmortem/` | M11 (W13-14) | P2 |

## 2. Architecture

### 2.1 Pipeline Enrichment

Phase 1 Pipeline: `Alert → Context → Evidence → RCA`

Phase 2 Pipeline: `Alert → [AlertGroup] → Context → [SimilarRCA] → [Changes] → Evidence → [Topology] → RCA → [Postmortem]`

方括号为 Phase 2 新增阶段，每个阶段是可选的（feature flag 控制）。

### 2.2 Package Dependencies

```
internal/similar/     → internal/store/, internal/analysis/
internal/change/      → internal/adapter/ (ToolProvider)
internal/topology/    → internal/adapter/ (ToolProvider), internal/analysis/
internal/promptstudio/→ internal/store/, internal/analysis/
internal/alertgroup/  → internal/store/, internal/incident/
internal/postmortem/  → internal/store/, internal/analysis/
```

### 2.3 Design Principles

- **SQLite-only storage** — 所有新数据存 SQLite，不引入向量数据库
- **Embedding via LLM API** — 相似度计算使用 LLM embedding endpoint（OpenAI-compatible `/v1/embeddings`）
- **零新 Go 依赖** — 使用标准库 + 已有 deps 实现
- **Feature flags** — 每个 Feature 通过 config 开关控制

## 3. Data Model

### 3.1 New Tables (Migrations 003-008)

**Migration 003: incident_embeddings**
```sql
CREATE TABLE incident_embeddings (
    incident_id TEXT NOT NULL,
    embedding   BLOB NOT NULL,        -- JSON-encoded float64 array
    model       TEXT NOT NULL DEFAULT 'text-embedding-3-small',
    created_at  TEXT NOT NULL DEFAULT (datetime('now')),
    PRIMARY KEY (incident_id)
);
```

**Migration 004: changes**
```sql
CREATE TABLE changes (
    id          TEXT PRIMARY KEY,     -- UUID
    service     TEXT NOT NULL,
    change_type TEXT NOT NULL,        -- deploy, config, feature_flag, infra
    summary     TEXT NOT NULL,
    author      TEXT,
    timestamp   TEXT NOT NULL,
    metadata    TEXT DEFAULT '{}',    -- JSON
    created_at  TEXT NOT NULL DEFAULT (datetime('now'))
);
CREATE INDEX idx_changes_service_time ON changes(service, timestamp);
```

**Migration 005: topology**
```sql
CREATE TABLE topology_edges (
    id           TEXT PRIMARY KEY,
    source       TEXT NOT NULL,
    target       TEXT NOT NULL,
    relation     TEXT NOT NULL DEFAULT 'calls',  -- calls, depends_on, publishes
    metadata     TEXT DEFAULT '{}',
    updated_at   TEXT NOT NULL DEFAULT (datetime('now'))
);
CREATE UNIQUE INDEX idx_topology_edge ON topology_edges(source, target, relation);
```

**Migration 006: prompt_templates**
```sql
CREATE TABLE prompt_templates (
    id          TEXT PRIMARY KEY,
    name        TEXT NOT NULL UNIQUE,
    stage       TEXT NOT NULL,         -- context, evidence, rca
    system_tpl  TEXT NOT NULL,
    user_tpl    TEXT NOT NULL,
    variables   TEXT DEFAULT '[]',     -- JSON array of variable names
    is_default  BOOLEAN DEFAULT FALSE,
    version     INTEGER DEFAULT 1,
    created_at  TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at  TEXT NOT NULL DEFAULT (datetime('now'))
);
```

**Migration 007: alert_groups**
```sql
CREATE TABLE alert_groups (
    id          TEXT PRIMARY KEY,
    fingerprint TEXT NOT NULL UNIQUE,  -- dedup key
    title       TEXT NOT NULL,
    severity    TEXT NOT NULL DEFAULT 'warning',
    labels      TEXT DEFAULT '{}',     -- JSON
    incident_id TEXT,                  -- nullable, linked later
    count       INTEGER DEFAULT 1,
    first_seen  TEXT NOT NULL,
    last_seen   TEXT NOT NULL,
    created_at  TEXT NOT NULL DEFAULT (datetime('now')),
    FOREIGN KEY (incident_id) REFERENCES incidents(id)
);
CREATE INDEX idx_alert_groups_fingerprint ON alert_groups(fingerprint);
```

**Migration 008: postmortems**
```sql
CREATE TABLE postmortems (
    id          TEXT PRIMARY KEY,
    incident_id TEXT NOT NULL,
    report_id   TEXT NOT NULL,
    title       TEXT NOT NULL,
    content     TEXT NOT NULL,         -- Markdown
    status      TEXT NOT NULL DEFAULT 'draft',  -- draft, reviewed, published
    author      TEXT,
    reviewed_by TEXT,
    created_at  TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at  TEXT NOT NULL DEFAULT (datetime('now')),
    FOREIGN KEY (incident_id) REFERENCES incidents(id),
    FOREIGN KEY (report_id) REFERENCES reports(id)
);
```

## 4. API Endpoints

### 4.1 Similar Incident (F1)

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/v1/incidents/{id}/similar` | 查询相似事件 |
| POST | `/api/v1/incidents/{id}/embed` | 触发 embedding 计算 |

### 4.2 Change Correlation (F2)

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/v1/changes` | 列出变更（支持 service/time 范围过滤） |
| GET | `/api/v1/incidents/{id}/changes` | 获取关联变更 |

### 4.3 Blast Radius / Topology (F3)

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/v1/topology` | 获取服务拓扑图 |
| GET | `/api/v1/incidents/{id}/blast-radius` | 获取爆炸半径 |

### 4.4 Prompt Studio (F4)

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/v1/prompts` | 列出所有模板 |
| GET | `/api/v1/prompts/{id}` | 获取模板详情 |
| POST | `/api/v1/prompts` | 创建模板 |
| PUT | `/api/v1/prompts/{id}` | 更新模板 |
| POST | `/api/v1/prompts/{id}/test` | 测试模板（dry-run） |

### 4.5 Alert Aggregation (F5)

| Method | Path | Description |
|--------|------|-------------|
| POST | `/api/v1/alerts` | 接收告警 |
| GET | `/api/v1/alert-groups` | 列出告警组 |
| GET | `/api/v1/alert-groups/{id}` | 告警组详情 |
| POST | `/api/v1/alert-groups/{id}/escalate` | 升级为 Incident |

### 4.6 Postmortem (F6)

| Method | Path | Description |
|--------|------|-------------|
| POST | `/api/v1/incidents/{id}/postmortem` | 生成 Postmortem |
| GET | `/api/v1/postmortems` | 列出所有 Postmortem |
| GET | `/api/v1/postmortems/{id}` | Postmortem 详情 |
| PUT | `/api/v1/postmortems/{id}` | 更新 Postmortem |

## 5. Feature Design Details

### 5.1 Similar Incident Retrieval (F1)

**核心算法：**
1. 新 Incident 创建时，提取 service + error pattern + metric anomaly 特征
2. 调用 LLM embedding API 生成向量（复用 `LLMClient`，新增 `Embed()` 方法）
3. 与 SQLite 中已有 embedding 计算余弦相似度
4. 返回 Top-5 相似事件，附带相似度分数

**接口：**
```go
// internal/similar/service.go
type Service interface {
    FindSimilar(ctx context.Context, incidentID string, topK int) ([]SimilarResult, error)
    ComputeEmbedding(ctx context.Context, incidentID string) error
}

type SimilarResult struct {
    IncidentID  string  `json:"incident_id"`
    Similarity  float64 `json:"similarity"`
    Summary     string  `json:"summary"`
    RootCause   string  `json:"root_cause"`
}
```

**LLMClient 扩展：**
```go
// 在 llm_client.go 新增
func (c *LLMClient) Embed(ctx context.Context, texts []string) ([][]float64, error)
// 调用 POST /v1/embeddings
```

**余弦相似度：** 纯 Go 实现，无外部依赖。

### 5.2 Change Correlation (F2)

**数据来源：** 通过 `ToolProvider` 接口扩展获取变更事件。

**接口扩展：**
```go
// 在 adapter/provider.go 新增方法
type ToolProvider interface {
    SearchLogs(ctx context.Context, q LogQuery) ([]LogRecord, error)
    GetTrace(ctx context.Context, traceID string) (*TraceData, error)
    QueryMetric(ctx context.Context, q MetricQuery) (*MetricSeries, error)
    // Phase 2 新增
    GetChanges(ctx context.Context, q ChangeQuery) ([]ChangeEvent, error)
}

type ChangeQuery struct {
    Service   string
    StartTime string
    EndTime   string
    ChangeTypes []string // deploy, config, feature_flag, infra
}

type ChangeEvent struct {
    ID          string
    Service     string
    ChangeType  string
    Summary     string
    Author      string
    Timestamp   string
    Metadata    map[string]any
}
```

**关联逻辑：**
1. 在 Evidence 收集阶段后，查询 Incident 时间窗口内的变更
2. LLM 在 RCA Prompt 中接收变更列表，判断是否为根因
3. 高置信度变更自动标记为 Evidence

### 5.3 Blast Radius Analysis (F3)

**拓扑数据来源：**
- 优先从 Trace 数据中自动推断（分析 span 调用关系）
- 次要支持手动导入（API）
- 未来可接入 Service Mesh 数据

**接口：**
```go
// internal/topology/service.go
type Service interface {
    GetTopology(ctx context.Context) (*TopologyGraph, error)
    ComputeBlastRadius(ctx context.Context, service string, depth int) ([]string, error)
    InferFromTraces(ctx context.Context, traces []adapter.TraceData) error
}

type TopologyGraph struct {
    Nodes []TopologyNode `json:"nodes"`
    Edges []TopologyEdge `json:"edges"`
}
```

**Blast Radius 计算：** 从故障服务出发，BFS 遍历拓扑图，收集所有下游服务。

**前端可视化：** 使用 `@xyflow/react` 渲染服务拓扑图，故障节点红色高亮，受影响节点橙色。

### 5.4 Prompt Studio (F4)

**功能：**
- CRUD 管理 Prompt 模板
- 支持 Go template 语法变量插值
- Dry-run 测试（输入变量值，预览渲染结果）
- 版本管理（更新模板自动 +1 version）

**接口：**
```go
// internal/promptstudio/service.go
type Service interface {
    List(ctx context.Context) ([]PromptTemplate, error)
    Get(ctx context.Context, id string) (*PromptTemplate, error)
    Create(ctx context.Context, tpl PromptTemplate) (*PromptTemplate, error)
    Update(ctx context.Context, id string, tpl PromptTemplate) (*PromptTemplate, error)
    DryRun(ctx context.Context, id string, vars map[string]string) (string, error)
}
```

**与现有 Prompt Builder 集成：** `prompt.Builder` 在构建时查询活跃模板，覆盖默认模板。

### 5.5 Alert Aggregation (F5)

**聚合策略：**
1. 接收告警时计算 fingerprint（基于 labels 去重）
2. 相同 fingerprint 的告警合并到同一 alert_group
3. 维护 count、first_seen、last_seen
4. 支持手动 escalate 为 Incident

**接口：**
```go
// internal/alertgroup/service.go
type Service interface {
    Ingest(ctx context.Context, alert IncomingAlert) (*AlertGroup, error)
    List(ctx context.Context, filter AlertGroupFilter) ([]AlertGroup, error)
    Get(ctx context.Context, id string) (*AlertGroup, error)
    Escalate(ctx context.Context, id string) (*contract.Incident, error)
}
```

**告警去重 fingerprint：** 将 labels 按字典序排序后 SHA256 哈希。

### 5.6 Postmortem Generator (F6)

**生成逻辑：**
1. 读取 Incident + RCA Report + Timeline + Feedback
2. 调用 LLM 生成 Markdown 格式的 Postmortem
3. 包含：Summary、Timeline、Root Cause、Actions Taken、Lessons Learned
4. 支持 draft → reviewed → published 状态流转

**接口：**
```go
// internal/postmortem/service.go
type Service interface {
    Generate(ctx context.Context, incidentID string) (*Postmortem, error)
    List(ctx context.Context) ([]Postmortem, error)
    Get(ctx context.Context, id string) (*Postmortem, error)
    Update(ctx context.Context, id string, input UpdatePostmortem) (*Postmortem, error)
}
```

## 6. Frontend Pages

| Page | Route | Feature |
|------|-------|---------|
| Similar Incidents Panel | `/incidents/:id` (tab) | F1: 相似事件列表 |
| Change Timeline | `/incidents/:id` (tab) | F2: 变更时间线 |
| Topology View | `/topology` | F3: 服务拓扑图 |
| Prompt Studio | `/settings/prompts` | F4: Prompt 编辑器 |
| Alert Groups | `/alerts` | F5: 告警聚合列表 |
| Postmortem Editor | `/postmortems/:id` | F6: Markdown 编辑器 |

**拓扑图组件：** `@xyflow/react`，仅在 Topology View 页面使用，不影响其他页面。

## 7. Configuration

```yaml
# Phase 2 feature flags
features:
  similar_incident:
    enabled: true
    embedding_model: "text-embedding-3-small"
    top_k: 5
    similarity_threshold: 0.7
  change_correlation:
    enabled: true
    time_window: "2h"  # look back window before incident
  topology:
    enabled: true
    infer_from_traces: true
  prompt_studio:
    enabled: true
  alert_aggregation:
    enabled: true
    dedup_window: "5m"
  postmortem:
    enabled: true
    default_status: "draft"
```

## 8. Milestone Timeline

| 里程碑 | 周次 | Features | 交付物 |
|--------|------|----------|--------|
| M6 | W7-8 | F1 Similar Incident | embedding 计算、相似度检索、前端 tab |
| M7 | W8-9 | F2 Change Correlation | ToolProvider 扩展、变更关联、前端 tab |
| M8 | W9-10 | F3 Blast Radius | 拓扑推断、BFS 爆炸半径、拓扑图页面 |
| M9 | W10-11 | F4 Prompt Studio | CRUD API、模板编辑器、dry-run |
| M10 | W11-12 | F5 Alert Aggregation | 告警接入、聚合去重、escalate |
| M11 | W13-14 | F6 Postmortem | LLM 生成、Markdown 编辑器、状态流转 |

每个 Milestone 遵循：
1. Design review (1 day)
2. TDD implementation (contract tests → adapter → service → API → frontend)
3. Independent audit (design compliance check)
4. Merge + integration test

## 9. Pre-wired Fields (Phase 1 已预留)

以下字段在 Phase 1 已定义但未使用，Phase 2 直接启用：

| 字段 | 位置 | Phase 2 用途 |
|------|------|-------------|
| `PromptInput.SimilarRCA` | `internal/prompt/builder.go` | F1: 注入相似事件到 RCA Prompt |
| `RCAOutput.BlastRadius` | `internal/analysis/llm_client.go` | F3: 爆炸半径输出 |
| `EvidenceItem.Type: "change"` | `internal/contract/tool.go` | F2: 变更证据类型 |
| `Actions.Prevention` | `internal/analysis/llm_client.go` | F6: Postmortem Lessons Learned 来源 |

## 10. Risk & Mitigation

| Risk | Impact | Mitigation |
|------|--------|------------|
| Embedding 质量不足 | 相似事件不准 | 混合检索：embedding + keyword + metadata filter |
| 拓扑推断不完整 | 爆炸半径遗漏 | 支持手动补充 + 多数据源融合 |
| LLM token 消耗增加 | 成本上升 | 压缩 evidence + 缓存 embedding + 按需生成 |
| SQLite embedding 全表扫描 | 性能瓶颈 | 限制候选集（同 service 优先）+ 定期清理 |
