# AI RCA Workbench — 前端原型设计

> 目标：定义 `aisre/web` 前端信息架构、页面原型、组件分层、交互流，确保 **AI 信息压缩优先，不复刻 OO / SigNoz**。
> 技术栈：React + TypeScript + TailwindCSS + shadcn/ui

---

## 1. 前端设计原则

核心不是"展示更多"，而是：

> **把 RCA 结论压缩成人类 60 秒内可执行的动作。**

5 条设计原则：

1. **TL;DR First**：先结论后证据
2. **Evidence Minimalism**：只显示最关键 1~3 条证据
3. **Actionable UI**：每页必须有下一步建议
4. **Progressive Drill-down**：需要时再跳转 OO
5. **Human-in-the-loop**：反馈入口固定可见

---

## 2. 前端工程结构

```text
web/
├── src/
│   ├── app/                # App shell, routing
│   ├── pages/              # Page-level components
│   │   ├── AlertWorkbench.tsx
│   │   ├── RCAReport.tsx
│   │   ├── History.tsx
│   │   └── ServicesHealth.tsx
│   ├── components/         # Shared components
│   │   ├── layout/
│   │   │   ├── app-shell.tsx
│   │   │   ├── sidebar.tsx
│   │   │   ├── topbar.tsx
│   │   │   └── page-header.tsx
│   │   └── rca/
│   │       ├── tldr-card.tsx
│   │       ├── timeline.tsx
│   │       ├── evidence-list.tsx
│   │       ├── blast-radius.tsx
│   │       ├── action-panel.tsx
│   │       └── feedback-bar.tsx
│   ├── features/           # Business domain modules
│   │   ├── incidents/
│   │   ├── reports/
│   │   ├── feedback/
│   │   └── settings/
│   ├── api/                # API client layer
│   ├── hooks/              # Custom React hooks
│   ├── lib/                # Utilities
│   ├── types/              # TypeScript type definitions
│   └── __tests__/          # Test files
│       ├── pages/
│       ├── components/
│       ├── hooks/
│       └── e2e/
├── public/
├── tailwind.config.ts
├── vite.config.ts
└── package.json
```

---

## 3. 信息架构（IA）

```text
Sidebar
├── Alert Workbench    (首页)
├── RCA Reports        (历史报告)
├── Similar Incidents  (相似事件)
├── Services Health    (服务健康)
├── Prompt Studio      (Prompt 管理，Phase 2)
└── Settings           (系统配置)
```

顶部固定：

- 全局搜索
- 时间范围选择器
- 严重性筛选
- 当前环境标识（prod/staging）

---

## 4. 页面A：Alert Workbench（首页）

### 页面目标

值班 SRE 在 **30 秒内完成事件分诊**。

### 布局

```text
┌──────────────────────────────────────────────┐
│ Search | Severity | TimeRange | Environment  │
├───────────────┬──────────────────────────────┤
│ High Risk     │ AI Suggested Repeated Issues │
│ Incidents     │                              │
├───────────────┴──────────────────────────────┤
│ Incident Stream                               │
│ P1 payment-api redis timeout   RCA Ready      │
│ P2 dify-worker workflow slow   Analyzing      │
└──────────────────────────────────────────────┘
```

### 核心模块

**High Risk Incident Cards**

字段：

- Severity
- Service
- Error summary
- First seen
- Confidence
- Blast radius count

交互：

- 点击进入 RCA Report
- Hover 显示 TL;DR

**Incident Stream**

时间流展示：

- 最新事件
- AI 状态（NEW / ANALYZING / REPORTED）
- 是否重复问题
- 是否已确认

颜色仅用于严重级别，不做复杂图表。

**Repeated Issues Panel**

- 最近 7 天高频根因
- Top unstable services
- 相似历史 RCA 快速入口

---

## 5. 页面B：RCA Report（核心页面）

### 页面目标

让用户 **1 分钟内完成根因理解 + 下一步动作确认**。

### 页面布局

```text
┌──────────────────────────────────────────────┐
│ TL;DR                                         │
│ Redis pool exhausted after deployment          │
│ Confidence 91% | payment-api | blast 3        │
├──────────────────────────────────────────────┤
│ Timeline                                      │
├──────────────────────────────────────────────┤
│ Evidence Chain     │ Recommended Actions      │
├────────────────────┼──────────────────────────┤
│ Blast Radius       │ Human Feedback           │
└──────────────────────────────────────────────┘
```

### 5.1 TL;DR Card（首屏核心）

必须首屏展示：

- 根因一句话
- 影响服务数
- 用户影响等级
- AI 置信度
- 推荐优先动作

> 用户不滚动即可看到结论。

### 5.2 Timeline

时间线只保留 5 类关键节点：

- baseline drift
- first error log
- slow trace span
- deployment/change
- alert fired

每个节点支持：

- 展开摘要
- drill-down 到 OO

### 5.3 Evidence Chain

只展示 Top 3：

1. Most critical log cluster
2. Slowest span
3. Metric anomaly

卡片结构：

```text
[Log] Redis timeout repeated 238x
Reason: same stack pattern
Action: open in OO
```

避免原始日志滚屏。

### 5.4 Recommended Actions（差异化核心）

分三级：

**Immediate（立即止血）**

- 重启异常实例
- 临时扩大连接池

**Fix（根本修复）**

- 优化 pool 配置
- 限流 workflow

**Prevention（长期治理）**

- 增加 baseline regression test
- 容量评估

每条建议附：

- 风险级别
- 预估恢复收益
- 推荐执行人（SRE/研发）

### 5.5 Blast Radius

只画：Root → Impacted services

```text
payment-api
 ├── checkout
 ├── billing
 └── notification
```

### 5.6 Human Feedback

固定底部悬浮，不随页面消失。

按钮：Correct / Partial / Wrong

附：comment input, assign to engineer, create follow-up issue（Phase 2）

---

## 6. 页面C：历史 RCA 报告

```text
Filters + Search
────────────────
Report List
Similarity Cluster
Repeated Root Causes
```

筛选：service, severity, root cause type, date range, confidence

支持 Similarity 排序。

---

## 7. 页面D：Services Health（管理视角）

面向负责人，不做监控复刻。

展示：

- Top unstable services
- MTTR trend
- Repeated incidents
- Most common root causes
- AI confidence trend

强调趋势，不展示细碎 metrics。

---

## 8. 关键组件设计

### Layout Components

```text
components/layout/
├── app-shell.tsx
├── sidebar.tsx
├── topbar.tsx
└── page-header.tsx
```

### RCA Components

```text
components/rca/
├── tldr-card.tsx
├── timeline.tsx
├── evidence-list.tsx
├── blast-radius.tsx
├── action-panel.tsx
└── feedback-bar.tsx
```

---

## 9. API 契约（前后端联调）

### 获取事件列表

```ts
GET /api/v1/incidents

type IncidentCard = {
  id: number
  severity: 'P1' | 'P2' | 'P3'
  service: string
  summary: string
  confidence: number
  status: 'NEW' | 'ANALYZING' | 'REPORTED'
}
```

### 获取 RCA Report

```ts
GET /api/v1/reports/:id
```

与 `internal/contract` 严格对齐。

---

## 10. 状态管理

- **TanStack Query**：服务端状态（incidents, reports, feedback）
- **Zustand**：局部 UI 状态（filters, selected incident, feedback draft）

状态切分：

- filters
- selected incident
- compare reports
- feedback draft

---

## 11. 前端 TDD

目录：`web/src/__tests__/`

### Component Tests

- TL;DR 渲染正确性
- Timeline 排序
- Evidence Top3 截断
- Feedback 按钮状态

### Page Tests

- Incident list filter
- RCA report render
- Empty state
- Loading skeleton

### E2E

- Alert → Report → Feedback 完整链路
- Drilldown 跳转
- History search

覆盖率要求：

- Components > 90%
- Pages > 85%
- E2E critical paths 100%

---

## 12. UI 差异化红线

禁止做成：

- 全量日志控制台
- Trace explorer
- Metric dashboard
- Topology full graph

这些都交给 OO。

Workbench 只保留：

> TL;DR + Evidence + Action

这是产品壁垒核心。
