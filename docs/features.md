# AI RCA Workbench — 核心功能设计

> 版本：v1.0

---

## 1. 功能地图

### L1 一级功能

| # | 功能 | 描述 |
|---|------|------|
| 1 | 告警工作台 | 告警接入、聚合、优先级排序 |
| 2 | 智能 RCA | 核心分析引擎，LLM 推理 |
| 3 | 证据链分析 | 多信号融合与关键证据提取 |
| 4 | 拓扑影响 | 根因节点识别与爆炸半径 |
| 5 | 建议动作 | 短/中/长期修复建议 |
| 6 | 历史复盘 | 相似事件检索与 Postmortem |
| 7 | Tool 管理 | 观测后端配置与管理 |
| 8 | 模型与 Prompt 管理 | LLM 模型配置与 Prompt 模板 |
| 9 | 多后端接入 | Tool Adapter SDK |
| 10 | 系统管理 | 用户、权限、审计 |

---

## 2. 告警工作台

### 2.1 告警接入

支持：

| 方式 | 描述 |
|------|------|
| Webhook | 接收外部告警推送 |
| API Push | 主动推送告警事件 |
| 定时 Pull | 定期从后端拉取告警 |
| 手工分析 | 用户手动发起分析 |

来源：

- OpenObserve Alert
- SigNoz Alert
- Prometheus Alert
- 自定义事件

### 2.2 告警聚合

- 同服务聚合
- 同错误模式聚合
- Trace 聚合
- 时间窗口聚合

### 2.3 智能优先级

评分维度：

| Dimension | Weight | Description |
|-----------|--------|-------------|
| 影响服务数 | 25% | 受影响的服务数量 |
| 用户影响范围 | 25% | 影响的用户比例 |
| 错误率变化 | 20% | 错误率增长幅度 |
| P95 漂移 | 15% | 延迟偏离程度 |
| 历史重复频次 | 15% | 重复出现频率 |

---

## 3. 智能 RCA（核心模块）

### 3.1 分析流程

```text
Alert/Event
→ Context Build       (构建分析上下文)
→ Tool Retrieval      (从各后端获取证据)
→ Evidence Rank       (证据排序)
→ LLM RCA             (LLM 推理根因)
→ Recommendation      (生成建议)
→ Report              (输出报告)
```

### 3.2 上下文构建

输入：

- 服务名
- 时间范围
- Trace ID
- Error Pattern
- 部署版本

扩展上下文：

- 最近变更（部署、配置）
- 历史类似故障
- 下游依赖状态
- SLA 基线

### 3.3 证据链生成

证据优先级排序：

| Priority | Type | Description |
|----------|------|-------------|
| 1 | Error Trace | 错误 Trace 的完整调用链 |
| 2 | Slowest Span | 最慢的 Span |
| 3 | Top Error Log Cluster | 高频错误日志聚类 |
| 4 | P95 Spike | P95 延迟突增 |
| 5 | Deployment Delta | 部署变更差异 |
| 6 | Dependency Failure | 依赖服务故障 |

输出结构：

```json
{
  "evidence_id": "ev_001",
  "type": "trace",
  "importance": 0.92,
  "summary": "redis timeout in payment service",
  "source_url": "http://oo.example.com/trace/xxx",
  "raw_payload": { }
}
```

### 3.4 RCA 输出标准

```json
{
  "summary": "Redis connection pool exhausted",
  "root_cause": "pool max 100 insufficient after deployment v2.3.1",
  "impact": ["checkout", "payment"],
  "evidence": [
    { "evidence_id": "ev_001", "type": "trace", "importance": 0.92 },
    { "evidence_id": "ev_002", "type": "metric", "importance": 0.85 }
  ],
  "recommendations": [
    { "category": "short_term", "action": "Increase pool max to 200" },
    { "category": "mid_term", "action": "Add connection pool monitoring" }
  ],
  "confidence": 0.93,
  "similar_incidents": ["inc_045", "inc_078"]
}
```

---

## 4. 证据链分析

目标：

> 只展示最关键证据，不复刻原始观测界面。

展示内容：

- 1 条关键日志
- 1 个关键 span
- 1 条关键指标漂移
- 1 条变更记录

支持点击 drill-down 到原始平台。

---

## 5. 拓扑影响分析

### 5.1 核心能力

- 根因节点识别
- 上游入口追溯
- 下游受影响节点
- 爆炸半径（Blast Radius）计算

### 5.2 UI 原则

仅展示：

> Root → Impacted Services

避免全量复杂服务地图。

---

## 6. 建议动作引擎

输出三层建议：

### 短期止血

- 重启实例
- 扩容连接池
- 降级 workflow
- 流量切换

### 中期修复

- 索引优化
- 缓存预热
- 超时配置调整
- 限流策略

### 长期治理

- 架构重构
- 容量规划
- SLA 调整
- 混沌工程

---

## 7. 历史复盘系统

### 7.1 相似事件检索

基于：

- 错误摘要 embedding 向量
- Trace Pattern 匹配
- 指标形态相似度
- 服务依赖图匹配

### 7.2 Postmortem 自动生成

包含：

- 故障摘要
- 时间线（T0 → T1 → T2 → Resolution）
- 根因分析
- 临时措施
- 长期优化建议
- Action Items
