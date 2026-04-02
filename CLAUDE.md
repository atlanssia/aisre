# CLAUDE.md — AI RCA Workbench Project Rules

## Project Overview

AI RCA Workbench (aisre) 是一个 AI-Native Root Cause Analysis 平台，构建在 OpenObserve / SigNoz / Elastic / Jaeger / Prometheus 等可观测后端之上，提供智能根因分析、证据链压缩和修复建议。

核心价值：**将 TB 级观测数据压缩为人类可执行的下一步行动。**

## Tech Stack

- **Backend:** Go 1.26.1
- **HTTP Router:** Chi (轻量、兼容 net/http)
- **Database:** SQLite (嵌入式，零依赖)
- **Frontend:** React + TypeScript + Tailwind CSS (Phase 2+)
- **Logging:** slog (Go 标准库结构化日志)
- **AI/LLM:** Claude API / OpenAI API, Embedding 模型
- **Note:** 不引入 PostgreSQL / Redis / ES / OpenSearch 等外部依赖，保持轻量部署

## Directory Structure

```
.
├── cmd/
│   └── server/             # Application entrypoint
├── internal/
│   ├── api/                # HTTP handlers + Chi router + middleware
│   ├── contract/           # DTO, request/response, error codes (Contract First)
│   ├── incident/           # Incident business logic
│   ├── analysis/           # Core analysis engine
│   │   ├── service.go      # Service interface
│   │   ├── context_builder.go
│   │   ├── evidence_ranker.go
│   │   ├── recommendation.go
│   │   └── confidence.go
│   ├── report/             # Report management
│   ├── adapter/            # ToolProvider interface + implementations
│   │   ├── provider.go     # ToolProvider interface
│   │   └── openobserve/    # OpenObserve adapter
│   ├── tool/               # Tool orchestration layer
│   ├── prompt/             # Prompt templates & builder
│   ├── feedback/           # Feedback logic
│   ├── store/              # SQLite repository layer
│   │   ├── store.go        # Repo interfaces
│   │   ├── incident_repo.go
│   │   ├── report_repo.go
│   │   └── feedback_repo.go
│   └── testkit/            # Shared test utilities
├── migrations/             # SQLite migrations
├── test/                   # Integration & E2E tests
│   ├── contract/           # Contract validation tests
│   ├── api/                # API handler tests
│   ├── analysis/           # Analysis engine tests
│   ├── adapter/            # Adapter integration tests
│   └── e2e/                # End-to-end tests
├── web/                    # Frontend (React + TypeScript + Tailwind + shadcn/ui)
│   ├── src/
│   │   ├── app/            # App shell, routing
│   │   ├── pages/          # Page-level components
│   │   ├── components/     # Shared components (layout/, rca/)
│   │   ├── features/       # Business domain (incidents/, reports/, feedback/, settings/)
│   │   ├── api/            # API client layer
│   │   ├── hooks/          # Custom React hooks
│   │   ├── lib/            # Utilities
│   │   ├── types/          # TypeScript type definitions
│   │   └── __tests__/      # Component, page, hook, e2e tests
│   ├── public/
│   ├── tailwind.config.ts
│   └── vite.config.ts
├── docs/                   # Design documents
├── configs/                # Configuration files
├── go.mod
└── Makefile
```

## Coding Conventions

### Go Style

- Follow [Effective Go](https://go.dev/doc/effective_go) and [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments)
- Use `golangci-lint` for linting — fix all warnings before committing
- Error handling: wrap errors with `fmt.Errorf("functionName: %w", err)`, never silently ignore
- Use `context.Context` as first parameter in all service/adapter methods
- Interface definition lives in the consuming package, not the implementing package

### Naming

- Package names: lowercase, single word, no underscores
- Files: `snake_case.go`
- Types/Interfaces: `PascalCase`
- Functions/Methods: `PascalCase` (exported), `camelCase` (unexported)
- Constants: `PascalCase` or `UPPER_SNAKE_CASE` for truly constant values
- Database columns: `snake_case`
- API endpoints: `kebab-case` (`/api/v1/rca-reports`)

### Database (SQLite)

- 使用 SQLite 嵌入式数据库，单文件存储
- 不引入 PostgreSQL / Redis / ES / OpenSearch 等外部中间件
- Migration 使用 `golang-migrate` 或内嵌 SQL
- 数据文件位于 `./data/aisre.db`（已加入 .gitignore）
- 使用 `modernc.org/sqlite`（纯 Go 实现，无 CGO 依赖）
- JSON 字段使用 SQLite 的 JSON1 扩展

### Adapter Pattern

All observability backends implement the `ToolProvider` interface defined in `internal/adapter/provider.go`. When adding a new backend:

1. Create a new package under `internal/adapter/<name>/`
2. Implement all `ToolProvider` methods
3. Return structured types from `internal/adapter/`
4. Write integration tests in `test/adapter/`

### Contract First

All DTOs, request/response types, and error codes MUST be defined in `internal/contract/` before implementation:

- `incident.go` — Incident DTOs
- `report.go` — Report DTOs
- `feedback.go` — Feedback DTOs
- `errors.go` — Error codes and constants
- `tool.go` — Tool result DTOs

### Testing (TDD)

Strict TDD: **No implementation before failing tests.**

- Contract tests: `test/contract/` — schema validation
- API tests: `test/api/` — handler-level tests
- Analysis tests: `test/analysis/` — engine logic tests
- Adapter tests: `test/adapter/` — integration with mock servers
- E2E tests: `test/e2e/` — full pipeline tests
- Run all: `make test`
- Shared test utilities: `internal/testkit/`

Coverage targets:
- Contract: 100%
- API handlers: > 90%
- Analysis: > 85%
- Adapter: > 90%

### Database Migrations

- Use migration files in `migrations/` — never manual DDL
- Migration naming: `NNNN_description.up.sql` / `NNNN_description.down.sql`
- Always include `created_at` / `updated_at` timestamps

## Commit Conventions

Format: `type(scope): description`

Types: `feat`, `fix`, `docs`, `refactor`, `test`, `chore`, `ci`

Examples:
- `feat(adapter): add OpenObserve logs search`
- `fix(analysis): correct evidence ranking for trace priority`
- `docs: update architecture design document`

## API Conventions

- REST API with `/api/v1/` prefix
- Use SSE for real-time analysis progress
- JSON request/response bodies
- Standard HTTP status codes
- Error response format: `{"error": "message", "code": "ERROR_CODE"}`

## Architecture Principles

1. **Light platform** — avoid over-orchestration, prefer simple pipelines
2. **Tool First** — backend plugin via ToolProvider interface
3. **AI Compression** — compress data, not display it
4. **Decoupled** — independent from any single observability platform
5. **Fix-ready** —预留 auto-remediation interface

## Important Notes

- Do NOT build logging/trace/metric storage — delegate to observability backends
- Do NOT create full trace UI — link to source platforms for drill-down
- Focus: RCA, Incident Analysis, Alert Intelligence, Human Decision Support
- Always maintain evidence-to-source traceability for audit purposes

## Frontend Conventions

### Tech Stack

- React + TypeScript + TailwindCSS + shadcn/ui
- State: TanStack Query (server state) + Zustand (UI state)
- Build: Vite
- Test: Vitest + Playwright (e2e)

### Design Principles

- **TL;DR First** — 先结论后证据，首屏不滚动即可看到结论
- **Evidence Minimalism** — 只显示 Top 1~3 条关键证据
- **Actionable UI** — 每页必须有下一步建议
- **Progressive Drill-down** — 需要时跳转 OO，不在 Workbench 复刻原始界面
- **Human-in-the-loop** — 反馈入口固定可见

### Red Lines (禁止)

- 不做全量日志控制台
- 不做 Trace explorer
- 不做 Metric dashboard
- 不做全量服务拓扑图
- 只保留：TL;DR + Evidence + Action
