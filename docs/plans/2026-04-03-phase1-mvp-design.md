# Phase 1 MVP Implementation Design

Date: 2026-04-03

## Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Implementation scope | Phase 1 MVP (M1-M5) | Full end-to-end validation |
| Agent mode | Strict multi-role (Plan/Coder/Review/Test) | Quality assurance |
| UI style | Dark professional (Grafana/Datadog style) | SRE/DevOps audience |
| TDD strictness | Strict (red-green-refactor) | CLAUDE.md requirement |
| LLM provider | OpenAI Compatible API | Unified interface, flexible backend |
| Implementation strategy | Layered progression (M1→M5) | Clean dependency chain |

## Multi-Agent Workflow

```
[Plan Agent] → design implementation strategy
    ↓
[Test Agent] → write failing tests first
    ↓
[Coder Agent] → implement to pass tests
    ↓
[Review Agent] → code quality, security, architecture review
    ↓
[next milestone]
```

| Role | Agent Type | Responsibility |
|------|-----------|----------------|
| Plan | Plan subagent | Analyze, design, break down tasks |
| Test | Main agent (TDD phase) | Write failing tests before implementation |
| Coder | Main agent / general-purpose | Write implementation code |
| Review | code-reviewer / silent-failure-hunter | Quality, security, gap review |
| Explorer | Explore subagent | Search codebase, understand patterns |
| Architect | code-architect subagent | Critical architecture decisions |

## M1: Infrastructure + Alert Ingestion + Incident API

**Goal:** Database, Incident CRUD, Webhook receiver

### Implementation Order (TDD)

1. `go.mod` dependencies (chi, sqlite, migrate, viper)
2. Migration `001_init.up/down.sql` (incidents table)
3. SQLite Store implementation (`internal/store/`)
4. Incident Service (`internal/incident/`)
5. API Handlers + Chi Router (`internal/api/`)
6. Webhook endpoint (`internal/api/webhook.go`)
7. Server entry point (`cmd/server/main.go`)

### API Endpoints

- `POST /api/v1/incidents` — Create incident
- `GET /api/v1/incidents` — List incidents
- `GET /api/v1/incidents/:id` — Get incident
- `POST /api/v1/alerts/webhook` — Receive alert webhook

### Database Schema

```sql
CREATE TABLE incidents (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    source TEXT NOT NULL,
    severity TEXT NOT NULL CHECK(severity IN ('critical','high','medium','low')),
    service_name TEXT NOT NULL,
    title TEXT,
    description TEXT,
    labels TEXT,
    started_at DATETIME NOT NULL,
    status TEXT NOT NULL DEFAULT 'open' CHECK(status IN ('open','analyzing','resolved','closed')),
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
```

## M2: OO Adapter + Tool Layer

**Goal:** OpenObserve backend integration for logs/traces/metrics

### Modules

1. OO HTTP Client (`internal/adapter/openobserve/client.go`)
2. Logs Tool (`internal/adapter/openobserve/logs.go`)
3. Trace Tool (`internal/adapter/openobserve/trace.go`)
4. Metrics Tool (`internal/adapter/openobserve/metrics.go`)
5. Tool Orchestrator (`internal/tool/orchestrator.go`)
6. Mock OO Server (`test/adapter/mock_server.go`)

### Design

- ToolProvider interface already defined
- httptest mock server for testing
- Orchestrator coordinates multi-signal retrieval (log → trace → metric)
- All results normalized to ToolResult

## M3: RCA Engine + LLM Integration

**Goal:** Analysis pipeline: Context → Evidence → LLM RCA → Report

### Modules

1. LLM Client - OpenAI Compatible (`internal/analysis/llm_client.go`)
2. Context Builder (`internal/analysis/context_builder.go`)
3. Evidence Ranker (`internal/analysis/evidence_ranker.go`)
4. RCA Service (`internal/analysis/service.go`)
5. Recommendation Engine (`internal/analysis/recommendation.go`)
6. Confidence Scorer (`internal/analysis/confidence.go`)
7. Prompt Renderer (`internal/prompt/renderer.go`)

### LLM Integration

- OpenAI Compatible API (`/v1/chat/completions`)
- Config: base_url, api_key, model
- SSE streaming for real-time analysis progress

### Database Additions (M3 Migration)

```sql
CREATE TABLE rca_reports (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    incident_id INTEGER NOT NULL REFERENCES incidents(id),
    summary TEXT,
    root_cause TEXT,
    confidence REAL,
    report_json TEXT,
    status TEXT NOT NULL DEFAULT 'generated',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE evidence_items (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    report_id INTEGER NOT NULL REFERENCES rca_reports(id),
    evidence_type TEXT NOT NULL,
    score REAL,
    payload TEXT,
    source_url TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE recommendations (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    report_id INTEGER NOT NULL REFERENCES rca_reports(id),
    category TEXT NOT NULL,
    description TEXT,
    priority INTEGER,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
```

### API Endpoints (M3)

- `POST /api/v1/incidents/:id/analyze` — Trigger analysis
- `GET /api/v1/reports/:id` — Get RCA report
- `GET /api/v1/reports/:id/evidence` — Get evidence items
- `GET /api/v1/reports/:id/recommendations` — Get recommendations

## M4: Frontend + Feedback + Search

**Goal:** React UI, feedback API, history search

### Frontend Pages

| Page | Route | Components |
|------|-------|------------|
| Alert Workbench | `/` | IncidentTable, SeverityBadge, QuickActions |
| RCA Report | `/reports/:id` | RCAHeader, EvidenceCard, ActionPanel |
| History | `/history` | SearchBar, ReportList, FilterPanel |
| Settings | `/settings` | ConnectionForm, PromptEditor |

### UI Design (Dark Professional)

- Background: slate-900/950
- Cards: slate-800 with slate-700 borders
- Status colors: emerald (ok), amber (warning), red (critical)
- Font: mono for data, sans for prose
- shadcn/ui dark theme + custom overrides
- Desktop-first responsive

### Feedback API

- `POST /api/v1/reports/:id/feedback` — Submit feedback
- `GET /api/v1/reports/:id/feedback` — Get feedback history

### Search API

- `GET /api/v1/reports/search?q=&service=&severity=&date_range=` — Search reports

### Database Addition (M4 Migration)

```sql
CREATE TABLE feedback (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    report_id INTEGER NOT NULL REFERENCES rca_reports(id),
    user_id TEXT,
    rating INTEGER CHECK(rating BETWEEN 1 AND 5),
    comment TEXT,
    action_taken TEXT CHECK(action_taken IN ('accepted','rejected','modified')),
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
```

## M5: Testing + Polish + Launch

**Goal:** Comprehensive tests, documentation, deployment readiness

- Contract tests: 100% coverage
- API handler tests: >90%
- Analysis engine tests: >85%
- Adapter tests: >90%
- E2E tests: critical paths
- Golden dataset validation
- Performance benchmarks
- Documentation update

## Acceptance Criteria (Phase 1 MVP)

- [ ] Alert webhook receives and stores incidents
- [ ] Incident list/detail API works
- [ ] OpenObserve adapter queries logs/traces/metrics
- [ ] RCA analysis produces complete report with evidence
- [ ] Report displays TL;DR + evidence + recommendations
- [ ] Feedback collection works
- [ ] History search works
- [ ] All tests pass with target coverage
- [ ] Single binary deployment works
