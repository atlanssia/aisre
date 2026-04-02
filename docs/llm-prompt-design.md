# AI RCA Workbench — LLM 与 Prompt 设计

> 版本：v1.0

---

## 1. Prompt Pipeline

RCA 分析采用三段式 Prompt Pipeline：

```text
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│  Context Prompt │ →  │ Evidence Prompt │ →  │   RCA Prompt    │
│                 │    │                 │    │                 │
│ 构建分析上下文   │    │ 整理证据链      │    │ 推理根因 + 建议  │
└─────────────────┘    └─────────────────┘    └─────────────────┘
```

---

## 2. Context Prompt

目标：将原始告警信息转化为结构化分析上下文。

输入：

- 告警元数据（服务名、时间、严重级别）
- 服务拓扑信息
- 历史故障摘要
- SLA 基线

输出：

- 结构化的 IncidentContext

```text
你是可观测性分析助手。将以下原始告警信息整理为结构化的分析上下文。

输入：
- 告警信息：{alert_json}
- 服务依赖：{topology}
- 历史故障：{history}

输出 JSON 格式：
{
  "service": "...",
  "anomaly_type": "...",
  "time_window": { "start": "...", "end": "..." },
  "affected_services": [],
  "context_signals": []
}
```

---

## 3. Evidence Prompt

目标：从多个数据源获取的证据中，筛选和排序关键证据。

输入：

- 日志片段（top error cluster）
- Trace 数据（error trace, slowest span）
- 指标数据（P95 spike, error rate）
- 变更记录（deployment delta）

```text
你是 SRE 证据分析专家。从以下多源数据中，提取最关键的证据。

日志：
{logs_summary}

Trace：
{trace_summary}

指标：
{metrics_summary}

变更：
{changes_summary}

对每条证据打分（0-1），并说明为什么重要。按重要性排序。

输出 JSON 数组：
[
  {
    "evidence_id": "ev_001",
    "type": "trace|log|metric|change",
    "score": 0.92,
    "summary": "一句话描述",
    "detail": "详细描述",
    "source_url": "原始链接"
  }
]
```

---

## 4. RCA Prompt

目标：基于上下文和证据，推理根因并生成建议。

```text
你是资深 SRE RCA 专家。

基于以下信息进行根因分析：

## 上下文
{context}

## 证据链（按重要性排序）
{evidence}

## 分析要求
请输出以下内容：

1. **Summary（一句话总结）**
   简洁描述故障本质

2. **Root Cause（根因）**
   最可能的根因，包含技术细节

3. **Impact（影响范围）**
   受影响的服务和用户范围

4. **Evidence（证据引用）**
   引用上述证据 ID，说明每条证据如何支持你的推理

5. **Recommendations（建议动作）**
   - 短期止血：立即可执行的缓解措施
   - 中期修复：根因修复方案
   - 长期治理：防止再次发生的系统性改进

6. **Confidence（置信度）**
   0-1 的置信度评分，说明评分理由

输出 JSON 格式。
```

---

## 5. Postmortem Prompt

用于自动生成复盘文档：

```text
你是 SRE 复盘专家。基于以下故障信息生成 Postmortem 文档。

## 故障信息
{incident_details}

## RCA 报告
{rca_report}

## 时间线
{timeline}

请生成包含以下部分的 Postmortem：
1. 故障摘要
2. 时间线（含 UTC 时间）
3. 根因分析
4. 临时措施
5. 长期优化建议
6. Action Items（含负责人建议）
```

---

## 6. 模型策略

### 6.1 模型分层

| Task | Model Type | Examples |
|------|-----------|----------|
| 摘要 / 分类 | 小模型（快速、低成本） | Claude Haiku, GPT-4o-mini |
| RCA 推理 | 大模型（高质量推理） | Claude Sonnet/Opus, GPT-4 |
| Embedding | Embedding 模型 | text-embedding-3-small |
| 相似事件匹配 | 向量检索 + 小模型 | Embedding + cosine similarity |

### 6.2 模型配置

```json
{
  "models": {
    "summarize": {
      "provider": "anthropic",
      "model": "claude-haiku-4-5-20251001",
      "max_tokens": 1024
    },
    "rca": {
      "provider": "anthropic",
      "model": "claude-sonnet-4-6",
      "max_tokens": 4096
    },
    "embedding": {
      "provider": "openai",
      "model": "text-embedding-3-small",
      "dimensions": 1536
    }
  }
}
```

### 6.3 Token 预算

| Stage | Max Input Tokens | Max Output Tokens |
|-------|-----------------|-------------------|
| Context Build | 4K | 1K |
| Evidence Rank | 8K | 2K |
| RCA Reasoning | 12K | 4K |
| Postmortem | 8K | 3K |

---

## 7. 质量保障

### 7.1 幻觉检测

- 证据引用验证：确保 RCA 输出中引用的 evidence_id 存在
- 数值合理性检查：置信度、评分在合理范围内
- 服务名验证：确保提到的服务在拓扑中存在

### 7.2 反馈闭环

- 用户反馈（Accept/Reject）作为模型优化信号
- 低评分报告自动标记用于人工审核
- 定期用高质量反馈构建 eval 数据集
