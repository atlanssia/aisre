# Phase 2 Design: Advanced RCA Intelligence

**Goal:** 扩展 Phase 1 MVP 的 3-Stage Pipeline，增加相似事件检索、变更关联、爆炸半径分析、Prompt Studio、告警聚合和 Postmortem 生成六大能力。

**Architecture:** Pipeline Enrichment 模式 — 在现有 3-Stage Pipeline 基础上插入新的数据收集和推理阶段，不替换核心流程。所有新功能作为独立 package 实现，通过 interface 解耦。

**Tech Stack:** Go 1.26.1 + SQLite（JSON1 扩展）+ LLM Client + Embedding Client（独立配置）+ `@xyflow/react`（仅拓扑图）+ 零新外部依赖

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
    incident_id INTEGER NOT NULL,
    service     TEXT NOT NULL DEFAULT '',
    embedding   BLOB NOT NULL,        -- binary-encoded float64 array (encoding/binary)
    model       TEXT NOT NULL DEFAULT 'text-embedding-3-small',
    created_at  TEXT NOT NULL DEFAULT (datetime('now')),
    PRIMARY KEY (incident_id),
    FOREIGN KEY (incident_id) REFERENCES incidents(id)
);
CREATE INDEX idx_embeddings_service ON incident_embeddings(service);
```

**Migration 004: changes**
```sql
CREATE TABLE changes (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    service     TEXT NOT NULL,
    change_type TEXT NOT NULL CHECK(change_type IN ('deploy', 'config', 'feature_flag', 'infra')),
    summary     TEXT NOT NULL,
    author      TEXT,
    timestamp   TEXT NOT NULL,
    metadata    TEXT DEFAULT '{}' CHECK(json_valid(metadata)),
    created_at  TEXT NOT NULL DEFAULT (datetime('now'))
);
CREATE INDEX idx_changes_service_time ON changes(service, timestamp);
CREATE UNIQUE INDEX idx_changes_dedup ON changes(service, change_type, timestamp, summary);
```

**Migration 005: topology**
```sql
CREATE TABLE topology_edges (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    source       TEXT NOT NULL,
    target       TEXT NOT NULL,
    relation     TEXT NOT NULL DEFAULT 'calls' CHECK(relation IN ('calls', 'depends_on', 'publishes')),
    metadata     TEXT DEFAULT '{}' CHECK(json_valid(metadata)),
    updated_at   TEXT NOT NULL DEFAULT (datetime('now'))
);
CREATE UNIQUE INDEX idx_topology_edge ON topology_edges(source, target, relation);
```

**Migration 006: prompt_templates**
```sql
CREATE TABLE prompt_templates (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    name        TEXT NOT NULL UNIQUE,
    stage       TEXT NOT NULL CHECK(stage IN ('context', 'evidence', 'rca', 'summary')),
    system_tpl  TEXT NOT NULL,
    user_tpl    TEXT NOT NULL,
    variables   TEXT DEFAULT '[]' CHECK(json_valid(variables)),
    is_default  BOOLEAN DEFAULT FALSE,
    version     INTEGER DEFAULT 1,
    created_at  TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at  TEXT NOT NULL DEFAULT (datetime('now'))
);
```

**Migration 007: alert_groups**
```sql
CREATE TABLE alert_groups (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    fingerprint TEXT NOT NULL UNIQUE,
    title       TEXT NOT NULL,
    severity    TEXT NOT NULL DEFAULT 'warning' CHECK(severity IN ('critical', 'high', 'medium', 'low', 'info')),
    labels      TEXT DEFAULT '{}' CHECK(json_valid(labels)),
    incident_id INTEGER,
    count       INTEGER DEFAULT 1,
    first_seen  TEXT NOT NULL,
    last_seen   TEXT NOT NULL,
    created_at  TEXT NOT NULL DEFAULT (datetime('now')),
    FOREIGN KEY (incident_id) REFERENCES incidents(id)
);
CREATE INDEX idx_alert_groups_severity_time ON alert_groups(severity, last_seen);
CREATE INDEX idx_alert_groups_incident ON alert_groups(incident_id);
```

**Migration 008: postmortems**
```sql
CREATE TABLE postmortems (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    incident_id INTEGER NOT NULL,
    report_id   INTEGER NOT NULL,
    title       TEXT NOT NULL,
    content     TEXT NOT NULL,
    status      TEXT NOT NULL DEFAULT 'draft' CHECK(status IN ('draft', 'reviewed', 'published')),
    author      TEXT,
    reviewed_by TEXT,
    created_at  TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at  TEXT NOT NULL DEFAULT (datetime('now')),
    FOREIGN KEY (incident_id) REFERENCES incidents(id),
    FOREIGN KEY (report_id) REFERENCES rca_reports(id)
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
2. 调用独立的 Embedding API 生成向量（使用 `EmbeddingClient`，与主 LLM 解耦）
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

**独立的 Embedding 客户端：**
```go
// internal/analysis/embedding_client.go
type EmbeddingConfig struct {
    BaseURL    string  // 独立 endpoint，可与主 LLM 不同
    APIKey     string  // 独立 API key
    Model      string  // e.g. text-embedding-3-small
    Dimensions int     // e.g. 1536
}

type EmbeddingClient struct {
    cfg  EmbeddingConfig
    http *http.Client
}

func NewEmbeddingClient(cfg EmbeddingConfig) *EmbeddingClient
func (c *EmbeddingClient) Embed(ctx context.Context, texts []string) ([][]float64, error)
// 调用 POST {base_url}/embeddings，完全独立于 LLMClient
```

**配置示例：**
```yaml
embedding:
  base_url: "https://api.openai.com/v1"
  api_key: "${EMBEDDING_API_KEY}"
  model: "text-embedding-3-small"
  dimensions: 1536
```

支持环境变量覆盖：`EMBEDDING_BASE_URL`、`EMBEDDING_API_KEY`、`EMBEDDING_MODEL`。

**余弦相似度：** 纯 Go 实现，无外部依赖。

### 5.2 Change Correlation (F2)

**数据来源：** 通过独立的 `ChangeProvider` 接口获取变更事件（Go 接口组合模式）。

**接口设计：**
```go
// adapter/provider.go — Phase 1 ToolProvider 不变

// adapter/change_provider.go — Phase 2 独立接口
type ChangeProvider interface {
    GetChanges(ctx context.Context, q ChangeQuery) ([]ChangeEvent, error)
}

// DTO 在 internal/contract/change.go 定义
type ChangeQuery struct {
    Service     string
    StartTime   string
    EndTime     string
    ChangeTypes []string // deploy, config, feature_flag, infra
}

type ChangeEvent struct {
    ID         int64
    Service    string
    ChangeType string
    Summary    string
    Author     string
    Timestamp  string
    Metadata   map[string]any
}
```

**实现方式：** 在 `internal/adapter/openobserve/` 新增 `ChangeAdapter`，实现 `ChangeProvider` 接口。遵循 CLAUDE.md 规则："接口定义在消费包中"——由 `internal/change/` 定义所需接口，OO adapter 实现。

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
- 使用白名单变量的简单插值语法 `{{.variable_name}}`（非 Go template）
- Dry-run 测试（输入变量值，预览渲染结果）
- 版本管理（更新模板自动 +1 version）

**安全设计（模板注入防护）：**
1. 不使用 `text/template`，实现简单的 `strings.Replace` 变量插值
2. 白名单校验：只允许模板中引用 `variables` 字段声明的变量名
3. 模板内容禁止包含 Go template 指令（`{{` 后非 `.` 开头的直接拒绝）
4. DryRun 使用受限数据对象，不包含 API key 等敏感信息
5. 递归深度限制：单次渲染最多替换 100 个变量引用

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
# 独立 Embedding 配置（与主 LLM 解耦）
embedding:
  base_url: "https://api.openai.com/v1"
  api_key: "${EMBEDDING_API_KEY}"
  model: "text-embedding-3-small"
  dimensions: 1536

# Phase 2 feature flags
features:
  similar_incident:
    enabled: true
    top_k: 5
    similarity_threshold: 0.7
  change_correlation:
    enabled: true
    time_window: "2h"
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

**Feature Flag 接线（在 `cmd/server/main.go`）：**
```go
// 读取 feature flags
features := viper.Sub("features")
// 按需初始化服务
if features.GetBool("similar_incident.enabled") {
    embedClient := analysis.NewEmbeddingClient(embedCfg)
    similarSvc := similar.NewService(embedClient, incidentRepo, reportRepo)
    // 注册路由
}
```

每个 Feature 独立初始化，disabled 时不创建服务、不注册路由。

**环境变量映射：**
- `EMBEDDING_BASE_URL` → `embedding.base_url`
- `EMBEDDING_API_KEY` → `embedding.api_key`
- `EMBEDDING_MODEL` → `embedding.model`
- `FEATURE_SIMILAR_ENABLED` → `features.similar_incident.enabled`
- `FEATURE_CHANGE_ENABLED` → `features.change_correlation.enabled`

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
| Embedding 质量不足 | 相似事件不准 | 混合检索：embedding + keyword + metadata filter（同 service 优先） |
| 拓扑推断不完整 | 爆炸半径遗漏 | 支持手动补充 + 多数据源融合 |
| LLM token 消耗增加 | 成本上升 | 压缩 evidence + 缓存 embedding + 按需生成 |
| SQLite embedding 全表扫描 | 性能瓶颈 | `service` 列索引限制候选集 + embedding 二进制存储（非 JSON） |

## 11. Compatibility Notes

### Alert 端点共存
Phase 1 已有 `POST /api/v1/alerts/webhook`（接收外部告警）。Phase 2 新增 `POST /api/v1/alerts`（内部告警聚合入口）。两者共存：
- `/alerts/webhook` — 保持不变，接收原始告警并创建 Incident
- `/alerts` — 新端点，接收告警后聚合去重到 alert_group，手动 escalate 为 Incident

### Contract-First 合规
所有新 Feature 的 DTO 必须先在 `internal/contract/` 定义：
- `contract/similar.go` — SimilarResult, SimilarQuery
- `contract/change.go` — ChangeEvent, ChangeQuery
- `contract/topology.go` — TopologyGraph, TopologyNode, TopologyEdge, BlastRadius
- `contract/prompt_template.go` — PromptTemplate, PromptTestRequest
- `contract/alert_group.go` — AlertGroup, IncomingAlert, AlertGroupFilter
- `contract/postmortem.go` — Postmortem, UpdatePostmortem
