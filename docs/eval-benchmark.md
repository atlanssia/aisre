# AI RCA Workbench — Evaluation & Benchmark 手册

> 目标：为 `aisre/internal/eval`、`prompt`、`analysis`、`feedback` 提供可量化、可回归、可持续优化的 RCA 评测体系。
> 原则：Benchmark First、Golden Dataset、Human Feedback Closed Loop、Strict Eval TDD。

---

## 1. 评测体系设计目标

AI RCA 的真正护城河不是生成能力，而是：

> **你能否证明它比上一个版本更准确、更快、更可执行。**

核心目标：

1. **Offline Benchmark First**：每次变更先离线回归
2. **Golden Incident Dataset**：真实故障样本沉淀
3. **Human Feedback Closed Loop**：线上反馈进入训练/Prompt 优化
4. **Version-to-Version Comparison**：模型 / Prompt / Tool 横向对比
5. **Release Gate**：评测不过不允许上线

---

## 2. 目录结构

```text
internal/eval/
├── runner.go       # Benchmark runner
├── scorer.go       # Multi-dimension scorer
├── benchmark.go    # Benchmark definition
├── regression.go   # Regression gate
└── report.go       # Report generation

datasets/golden_incidents/
├── redis_timeout/
│   ├── case.json
│   ├── expected_rca.json
│   ├── evidence/
│   └── feedback_history.json
├── db_connection_leak/
├── oo_stream_backlog/
├── dify_workflow_deadlock/
├── dotnet_grpc_timeout/
└── model_latency_spike/

test/eval/
├── scorer_test.go
├── dataset_schema_test.go
├── regression_gate_test.go
└── benchmark_diff_test.go
```

---

## 3. Golden Incident Dataset（核心资产）

这是长期最大产品壁垒。

每个样本包含：

```text
case.json              — 事件元数据 + 原始证据
expected_rca.json      — 人工确认的 RCA 结论
evidence/              — 原始证据数据（logs, traces, metrics）
feedback_history.json  — 历史反馈记录
```

### Case Schema

```json
{
  "incident_id": "golden-001",
  "service": "payment-api",
  "severity": "P1",
  "evidence": {
    "logs": [],
    "traces": [],
    "metrics": []
  },
  "expected_root_cause": "redis connection pool exhausted"
}
```

要求：

- 必须来自真实历史事故
- 必须人工确认 root cause
- 必须有恢复动作

---

## 4. 评测维度

### 4.1 Root Cause Accuracy

> AI 输出 root_cause 与人工确认根因是否一致

评分：

| Match Level | Score |
|-------------|-------|
| exact match | 1.0 |
| semantic same | 0.8 |
| partial | 0.5 |
| wrong | 0.0 |

### 4.2 Evidence Grounding Score

> 每条结论是否可映射到至少 1 条 ToolResult

```text
grounding_score = grounded_claims / total_claims
```

上线门槛：grounding ≥ 0.9

### 4.3 Actionability Score

> 建议能否立即执行

维度：

- 是否有 immediate action
- 是否指定 owner（SRE / 研发）
- 是否包含 prevention
- 是否风险可控

### 4.4 Confidence Calibration

> 高 confidence 是否真的更准确

统计：

- confidence 0.9+ 的真实准确率
- confidence 0.5 以下的误报率

如果不一致，说明 Prompt 或 scorer 有问题。

### 4.5 Latency

- tool query latency
- prompt render latency
- llm latency
- total RCA latency

SLO：P95 RCA ≤ 15s

---

## 5. 自动评分器

```go
type ScoreResult struct {
    Accuracy        float64
    Grounding       float64
    Actionability   float64
    ConfidenceError float64
    TotalScore      float64
}
```

权重：

```text
Total = 40% accuracy + 25% grounding + 20% actionability + 15% latency
```

---

## 6. Benchmark Runner

职责：

- 批量跑 golden dataset
- 对比不同 prompt version
- 对比不同 model
- 输出 markdown/html 报告

CLI：

```bash
make eval-golden
make eval-prompt-v2
make eval-model-qwen
```

---

## 7. Prompt / Model A-B Benchmark

同一事故样本，对比多个版本输出：

```text
prompt v1 × model A
prompt v2 × model A
prompt v2 × model B
```

输出：score diff, hallucination diff, latency diff, action quality diff

---

## 8. 在线反馈闭环

前端 Human Feedback（Correct / Partial / Wrong）全部进入 `feedback_history.json`，并定期转入 `datasets/golden_incidents/`。

闭环：

```text
Online incidents → human feedback → verified RCA → golden dataset → prompt regression → next release
```

---

## 9. 发布门禁（Release Gate）

上线前必须满足：

| Metric | Threshold |
|--------|-----------|
| golden accuracy | ≥ 85% |
| grounding | ≥ 90% |
| hallucination | ≤ 5% |
| P95 latency | ≤ 15s |
| critical path E2E | 100% pass |

否则禁止切换 active prompt。

---

## 10. 可视化 Benchmark 报告

```text
Prompt Version: v3
Model: qwen3-32b
Accuracy: 88%
Grounding: 93%
Latency P95: 11.2s
Top Failure Modes:
- deploy over-attribution
- db timeout false positive
```

---

## 11. 长期产品壁垒

> Golden Incident Dataset + Benchmark History + Human Feedback Corpus = AI RCA Moat

> **Eval = Product Learning Engine**
