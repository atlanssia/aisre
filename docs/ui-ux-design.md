# AI RCA Workbench — UI/UX 信息架构

> 版本：v1.0

---

## 1. UI 设计原则

1. **AI 优先** — AI 分析结果是页面焦点，不是辅助
2. **TL;DR 优先** — 先给结论，再给细节
3. **证据极简** — 只展示关键证据，支持 drill-down
4. **动作导向** — 每个页面都有明确的下一步行动
5. **可 Drill-down** — 点击跳转到原始观测平台

---

## 2. 页面结构

### 2.1 首页：Alert Workbench

模块布局：

```text
┌─────────────────────────────────────────────┐
│  Header: Logo / Nav / User / Settings       │
├────────────┬────────────────────────────────┤
│            │  Today's Critical Alerts        │
│  Sidebar   │  ┌──────┐ ┌──────┐ ┌──────┐   │
│            │  │Card 1│ │Card 2│ │Card 3│   │
│  - Alerts  │  └──────┘ └──────┘ └──────┘   │
│  - RCA     ├────────────────────────────────┤
│  - History │  AI Analyzed Events            │
│  - Tools   │  [List of recent analyses]     │
│  - Settings├────────────────────────────────┤
│            │  Top Unstable Services          │
│            │  [Service stability ranking]    │
└────────────┴────────────────────────────────┘
```

首页模块：

| Module | Content |
|--------|---------|
| 今日高危事件 | Critical/High severity alerts |
| AI 已分析事件 | 最近完成的 RCA 分析 |
| 待确认报告 | 需要人工确认的 RCA 报告 |
| 相似重复故障 | 基于历史匹配的重复事件 |
| Top 不稳定服务 | 按故障频率排序的服务 |

---

### 2.2 RCA 报告页（核心页面）

这是产品最重要的页面，采用纵向单栏布局：

```text
┌─────────────────────────────────────────────┐
│  Breadcrumb: Alerts > INC-001 > RCA Report  │
├─────────────────────────────────────────────┤
│                                             │
│  ┌─ TL;DR ────────────────────────────────┐ │
│  │ Root Cause: Redis pool exhausted        │ │
│  │ Impact: checkout, payment               │ │
│  │ Confidence: 93%                         │ │
│  │ Recommendation: Increase pool to 200    │ │
│  └─────────────────────────────────────────┘ │
│                                             │
│  ┌─ Timeline ─────────────────────────────┐ │
│  │ T0  10:23  Anomaly detected            │ │
│  │ T1  10:24  Metric spike (P95 +400%)    │ │
│  │ T2  10:25  Error peak                  │ │
│  │ T3  10:25  Alert triggered             │ │
│  └─────────────────────────────────────────┘ │
│                                             │
│  ┌─ Evidence ─────────────────────────────┐ │
│  │ [Log]   [Span]   [Metric]   [Change]  │ │
│  │ ┌──────────────────────────────────┐   │ │
│  │ │ Key evidence cards (max 4-6)     │   │ │
│  │ │ Click → drill-down to source     │   │ │
│  │ └──────────────────────────────────┘   │ │
│  └─────────────────────────────────────────┘ │
│                                             │
│  ┌─ Blast Radius ─────────────────────────┐ │
│  │ Root (payment) → checkout → cart       │ │
│  └─────────────────────────────────────────┘ │
│                                             │
│  ┌─ Recommendations ──────────────────────┐ │
│  │ Short-term: [Action 1] [Action 2]      │ │
│  │ Mid-term:   [Action 3]                 │ │
│  │ Long-term:  [Action 4]                 │ │
│  └─────────────────────────────────────────┘ │
│                                             │
│  ┌─ Feedback ─────────────────────────────┐ │
│  │ Rating: ★★★★☆                         │ │
│  │ [Accept] [Reject] [Modify]             │ │
│  └─────────────────────────────────────────┘ │
│                                             │
└─────────────────────────────────────────────┘
```

报告页各区域说明：

| Section | Purpose |
|---------|---------|
| TL;DR | 根因、影响、置信度、推荐动作——一句话结论 |
| Timeline | 异常开始 → 指标漂移 → 错误峰值 → 告警触发 |
| Evidence | Log / Span / Metric / Change 四类证据卡片 |
| Blast Radius | 根因节点到受影响服务的传播路径 |
| Recommendations | 短期/中期/长期三层建议 |
| Feedback | 用户对 RCA 的反馈，用于模型优化 |

---

### 2.3 历史复盘页

```text
┌─────────────────────────────────────────────┐
│  Search: [query]  Service: [filter]         │
├─────────────────────────────────────────────┤
│  Similar Incidents to INC-001               │
│  ┌──────────────────────────────────────┐   │
│  │ INC-045  Redis timeout  2024-03-15   │   │
│  │ INC-078  Pool exhaust   2024-05-22   │   │
│  └──────────────────────────────────────┘   │
├─────────────────────────────────────────────┤
│  Postmortem Generator                       │
│  [Generate Postmortem for selected events]  │
└─────────────────────────────────────────────┘
```

---

## 3. 交互规范

### 3.1 Drill-down

所有证据卡片点击后跳转到原始观测平台：

- Log → OpenObserve / SigNoz 日志搜索
- Span → Jaeger Trace 详情
- Metric → Grafana / Prometheus 图表
- Change → CI/CD 平台

### 3.2 实时更新

- 分析进度通过 SSE 推送
- 新告警通过 WebSocket 通知
- 报告生成后自动刷新

### 3.3 反馈闭环

- 每个 RCA 报告都有反馈入口
- 支持 Accept / Reject / Modify 三种操作
- 反馈用于 LLM 模型微调参考
