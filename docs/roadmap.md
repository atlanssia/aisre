# AI RCA Workbench — 路线图与成功指标

> 版本：v1.0

---

## 1. 路线图

### Phase 1：MVP（4~6 周）

目标：验证核心价值 — 告警到 RCA 报告的端到端流程

| Feature | Description | Priority |
|---------|-------------|----------|
| OO Adapter | OpenObserve logs/trace/metrics 查询 | P0 |
| Alert Workbench | 告警接入（Webhook）、列表、优先级 | P0 |
| Smart RCA | 端到端 RCA 分析 pipeline | P0 |
| RCA Report | TL;DR + Timeline + Evidence 展示 | P0 |
| Feedback | 用户反馈收集 | P0 |
| Tool Management | 后端连接配置管理 | P1 |

**MVP 交付物：**
- 可部署的 Go 后端 + 基础 Web UI
- OpenObserve 单后端支持
- 完整的告警 → RCA 报告流程
- 用户反馈闭环

**MVP 里程碑（详见 [PRD](prd.md)）：**

| Milestone | Week | Scope | Status |
|-----------|------|-------|--------|
| M1 | 第1周 | Alert 接入 + Incident API | Done |
| M2 | 第2~3周 | OO Adapter + Logs/Trace/Metrics Tool | Done |
| M3 | 第4周 | RCA Prompt + 报告页 | Done |
| M4 | 第5周 | 历史检索 + 反馈闭环 | Done |
| M5 | 第6周 | 测试/UAT/上线 | Done |

---

### Phase 2：增强（4 周）

目标：提升分析深度和用户体验

| Feature | Description | Priority | Status |
|---------|-------------|----------|--------|
| Similar Incident | 基于 Embedding 的相似事件检索 | P0 | Done |
| Change Correlation | 部署变更关联分析 | P0 | Done |
| Blast Radius | 拓扑爆炸半径可视化 | P0 | Done |
| Prompt Studio | Prompt 模板管理与调试 | P1 | Done |
| Alert Aggregation | 智能告警聚合 | P1 | Done |
| Postmortem | 自动复盘文档生成 | P1 | Done |

**Phase 2 里程碑：**

| Milestone | Week | Scope | Status |
|-----------|------|-------|--------|
| M6 | 第7-8周 | Similar Incident + Embedding | Done |
| M7 | 第8-9周 | Change Correlation | Done |
| M8 | 第9-10周 | Topology / Blast Radius | Done |
| M9 | 第10-11周 | Prompt Studio | Done |
| M10 | 第11-12周 | Alert Aggregation | Done |
| M11 | 第13-14周 | Postmortem Generator | Done |

---

### Phase 3：平台化（4 周）

目标：多后端支持与平台能力

| Feature | Description | Priority |
|---------|-------------|----------|
| SigNoz Adapter | SigNoz 后端接入 | P0 |
| Elastic Adapter | Elasticsearch 后端接入 | P0 |
| Prometheus Adapter | Prometheus 指标查询 | P0 |
| Topology SDK | 自动服务拓扑发现 | P1 |
| Action Interface | 预留修复动作接口 | P1 |
| API Authentication | 用户认证与权限 | P1 |
| Multi-tenancy | 多租户隔离 | P2 |

---

### Phase 4：未来

目标：闭环自动化

| Feature | Description |
|---------|-------------|
| Auto Remediation | 自动执行修复动作 |
| Runbook Execute | Runbook 自动执行 |
| Docker/Systemctl Action | 基础设施操作 |
| Closed-loop Learning | 反馈驱动的模型持续优化 |
| Custom Adapter SDK | 用户自定义后端适配器 |
| Slack/Teams Bot | 即时通讯集成 |
| PagerDuty Integration | 值班系统联动 |

---

## 2. 成功指标

### 核心指标

| Metric | Target | Measurement |
|--------|--------|-------------|
| MTTR | ↓ 50% | 对比使用前后的平均修复时间 |
| 首次 RCA 时间 | ↓ 80% | 从告警到生成 RCA 报告的时间 |
| 告警噪音 | ↓ 60% | 无效/重复告警数量下降 |
| 重复故障复用率 | ↑ 70% | 历史相似事件的复用比例 |
| AI 建议采纳率 | > 40% | 用户 Accept 的建议比例 |

### 过程指标

| Metric | Target | Description |
|--------|--------|-------------|
| RCA 报告准确率 | > 70% | 用户反馈 rating >= 4 的比例 |
| 分析覆盖率 | > 80% | 成功生成报告的告警比例 |
| 证据命中率 | > 60% | 用户点击 drill-down 的比例 |
| 日活跃用户 | 增长趋势 | SRE/DevOps 日常使用 |

### 技术指标

| Metric | Target |
|--------|--------|
| API P99 延迟 | < 500ms（非分析接口） |
| RCA 分析完成时间 | < 60s |
| 系统可用性 | > 99.5% |
| 单次分析成本 | < ¥2（LLM API 调用） |

---

## 3. 里程碑时间线

```text
Week 1      ████████  M1: Alert 接入 + Incident API (Done)
Week 2-3    ████████  M2: OO Adapter + Logs/Trace/Metrics Tool (Done)
Week 4      ████████  M3: RCA Prompt + 报告页 (Done)
Week 5      ████████  M4: 历史检索 + 反馈闭环 (Done)
Week 6      ████████  M5: 测试/UAT/上线 (Done)
Week 7-8    ████████  M6: Similar Incident (Done)
Week 8-9    ████████  M7: Change Correlation (Done)
Week 9-10   ████████  M8: Topology / Blast Radius (Done)
Week 10-11  ████████  M9: Prompt Studio (Done)
Week 11-12  ████████  M10: Alert Aggregation (Done)
Week 13-14  ████████  M11: Postmortem Generator (Done)
Week 15+    ████████  Phase 3: Multi-backend + Platform
Week 19+    ████████  Phase 4: Auto Remediation
```
