# AI RCA Workbench — AI RCA Prompt Engineering 手册

> 目标：为 `aisre/internal/prompt`、`analysis`、`report` 提供生产级 Prompt 体系、模板规范、反幻觉策略、评测体系、Prompt TDD 与 PromptOps 手册。
> 核心目标：让 RCA 输出稳定、可解释、低幻觉、可持续优化。

---

## 1. Prompt Engineering 设计目标

RCA Prompt 的目标不是"写一段长提示词"，而是：

> **构建一套可版本化、可测试、可灰度、可回滚的 Prompt Production System。**

核心目标：

1. **Evidence First**：证据优先，禁止无依据猜测
2. **Structured Output First**：先 JSON 契约，再自然语言摘要
3. **Hypothesis before Conclusion**：先假设再收敛
4. **Actionability**：必须输出立即动作 + 修复动作 + 预防动作
5. **Prompt Versioning**：Prompt 视为代码
6. **Prompt TDD**：失败用例先行

---

## 2. Prompt 模块目录

```text
internal/prompt/
├── templates/
│   ├── rca_system_v1.txt
│   ├── rca_summary_v1.txt
│   ├── action_recommendation_v1.txt
│   └── executive_summary_v1.txt
├── builder.go      # Prompt 构建器：evidence compression + template render
├── renderer.go     # 模板渲染引擎
├── validator.go    # 输出 JSON schema 校验
├── version.go      # Prompt 版本管理
├── eval.go         # 评测指标计算
└── prompt_test.go  # 单元测试
```

测试目录：

```text
test/prompt/
├── schema_test.go
├── hallucination_test.go
├── confidence_test.go
├── action_quality_test.go
└── regression_suite_test.go
```

红线：

- Prompt 文件必须版本化（`_v1`, `_v2`, ...）
- 不允许硬编码散落在业务代码
- 模板变更必须走测试

---

## 3. RCA 主 Prompt 架构（6 段结构化 Prompt Band）

```text
1. ROLE
2. TASK
3. EVIDENCE
4. CONSTRAINTS
5. OUTPUT_SCHEMA
6. SELF_CHECK
```

### 3.1 ROLE

```text
You are a senior SRE Root Cause Analyst.
You must follow evidence and avoid unsupported assumptions.
You specialize in distributed systems, microservices, and observability data.
```

强调：

- evidence-first
- no guessing
- confidence calibration

### 3.2 TASK

```text
Analyze the incident and identify the most probable root cause.
Then provide immediate mitigation, long-term fix, and prevention actions.
```

任务必须是正向描述，不使用大量否定句。

### 3.3 EVIDENCE

来自 ToolResult 聚合：

```json
{
  "logs": [...],
  "traces": [...],
  "metrics": [...],
  "timeline": [...],
  "recent_changes": [...]
}
```

必须带：

- top logs
- slowest spans
- blast radius
- recent deploys

原则：

> Prompt 输入必须是"压缩证据"，不是原始海量日志。

### 3.4 CONSTRAINTS（反幻觉核心）

```text
- Never infer missing evidence as fact
- If evidence is insufficient, output uncertainty
- Rank top 3 hypotheses before final conclusion
- Mention contradictory evidence if any
- Every root cause statement must reference at least one evidence item
```

### 3.5 OUTPUT_SCHEMA（生产核心）

必须强制 JSON：

```json
{
  "summary": "",
  "root_cause": "",
  "confidence": 0.0,
  "hypotheses": [
    {
      "description": "",
      "evidence_ids": [],
      "likelihood": 0.0
    }
  ],
  "evidence_ids": [],
  "blast_radius": [],
  "actions": {
    "immediate": [],
    "fix": [],
    "prevention": []
  },
  "uncertainties": []
}
```

> **先结构化 JSON，再前端渲染 TL;DR**

### 3.6 SELF_CHECK

```text
Before finalizing:
1. Verify every conclusion maps to at least one evidence item
2. Check whether contradictory evidence exists
3. Re-score confidence from 0-1
4. Ensure all three action categories have at least one entry
```

---

## 4. Prompt Builder 设计

```go
type PromptInput struct {
    Incident    contract.Incident
    Evidence    []contract.ToolResult
    SimilarRCA  []contract.RCAReport
    TimeWindow  string
    Environment string
}
```

Builder 职责：

- evidence compression（证据压缩）
- top3 selection
- contradictions merge
- template render
- schema inject

---

## 5. Evidence Compression（输入压缩）

> 真正降低 token 和幻觉的关键不在模型，而在输入证据压缩质量。

### Logs

只保留：

- top repeated stack
- first occurrence
- count

### Traces

只保留：

- top 3 slow spans
- upstream/downstream relation

### Metrics

只保留：

- spike point
- baseline delta

### Timeline

只保留：

- first symptom
- deploy event
- alert fired

> 这一步直接影响 RCA 准确率。

---

## 6. Prompt 版本管理（PromptOps）

Prompt 必须像代码一样版本化：

```text
rca_system_v1
rca_system_v2
rca_system_v3
```

数据库记录：

```sql
-- Added to reports table
prompt_version TEXT NOT NULL
model_snapshot TEXT NOT NULL
```

每份 RCA 报告都必须记录：

- model
- prompt_version
- evidence snapshot

便于回归与灰度。

---

## 7. Prompt TDD

原则：

> **先写失败 RCA case，再优化 Prompt。**

### 7.1 Schema Tests

验证：

- JSON 可解析
- 字段完整
- confidence 在 0~1 范围
- actions 三段齐全

### 7.2 Hallucination Tests

固定失败样本：

- 无 deploy 证据却推 deployment 根因
- 无 db trace 却推 DB root cause
- logs 与 trace 冲突时错误选择

### 7.3 Confidence Tests

验证：

- 单证据 → confidence ≤ 0.6
- 多证据一致 → ≥ 0.85
- contradictory evidence → ≤ 0.5

### 7.4 Action Quality Tests

检查：

- 是否立即可执行
- 是否角色明确（SRE / Dev）
- 是否包含 prevention

---

## 8. Golden RCA Regression Suite

长期产品壁垒。固定真实样本：

| Case | Scenario |
|------|----------|
| redis-timeout | Redis 连接池耗尽 |
| db-connection-leak | 数据库连接泄漏 |
| oo-stream-backlog | OpenObserve stream 积压 |
| dify-workflow-deadlock | Dify workflow 死锁 |
| dotnet-grpc-cascade | .NET gRPC 级联超时 |
| model-provider-latency | AI 模型提供方延迟突增 |

每次 Prompt 修改必须全量回归。

---

## 9. 双层 Prompt（管理层摘要）

为前端 Services Health 和管理层报告增加二级 Prompt：

```text
templates/executive_summary_v1.txt
```

输入：

- RCA JSON
- blast radius
- MTTR
- business impact

输出：

- plain language summary
- risk assessment
- follow-up investment suggestions

---

## 10. Prompt 灰度与回滚

支持：

- service-based rollout
- environment rollout
- prompt A/B
- instant rollback

配置：

```yaml
prompt:
  active: rca_system_v2
  canary:
    - dify-worker
    - payment-api
```

---

## 11. 产品级红线

### Prompt 层负责

- reasoning structure
- schema stability
- anti-hallucination
- action generation

### Prompt 层不负责

- tool query
- evidence ranking
- report persistence
- frontend formatting

> **Prompt = Intelligence Policy Layer**
