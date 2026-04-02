# AI RCA Workbench

> AI-Native Root Cause Analysis Workbench — 可观测数据之上的智能诊断决策层

## What is it

AI RCA Workbench 构建在 OpenObserve / SigNoz / Elastic / Jaeger / Prometheus 等可观测后端之上，提供：

- **智能根因分析** — 多信号融合 + LLM 推理，自动定位根因
- **证据链压缩** — 从 TB 级数据中提取关键证据，呈现给人类
- **修复建议生成** — 短期止血、中期修复、长期治理三层建议
- **历史复盘** — 相似事件检索、自动 Postmortem 生成
- **拓扑影响** — 爆炸半径可视化，根因节点识别

核心价值主张：

> **把 TB 级观测数据压缩为人类可执行的下一步行动。**

## Architecture

```
┌────────────────────────────────────────────┐
│                Web UI Portal               │
│ Alert Workbench / RCA Report / Timeline    │
└───────────────────┬────────────────────────┘
                    │
┌───────────────────▼────────────────────────┐
│                API Backend (Go)             │
│ Incident API / Report API / Search API      │
└───────────────────┬────────────────────────┘
                    │
┌───────────────────▼────────────────────────┐
│              Analysis Engine                │
│ Signal Fusion / Evidence Chain / LLM RCA    │
└───────────────────┬────────────────────────┘
                    │
┌───────────────────▼────────────────────────┐
│              Tool Adapter SDK               │
│ OO / SigNoz / Elastic / Jaeger / Prom       │
└────────────────────────────────────────────┘
```

## Tech Stack

| Layer | Technology |
|-------|-----------|
| Backend | Go 1.26.1 + Chi |
| Database | SQLite (embedded, zero-dependency) |
| Frontend | React + TypeScript + Tailwind CSS |
| AI/LLM | Claude API / OpenAI API |

> **设计哲学：** 不引入 PostgreSQL / Redis / ES / OpenSearch 等外部中间件，保持单二进制部署。

## Quick Start

```bash
# Clone
git clone https://github.com/atlanssia/aisre.git
cd aisre

# Build
make build

# Run
make dev

# Test
make test
```

## Documentation

Design documents are in the `docs/` directory:

| Document | Description |
|----------|------------|
| [Whitepaper](docs/whitepaper.md) | Product vision, positioning, value proposition |
| [Architecture](docs/architecture.md) | System architecture, layers, database design |
| [Features](docs/features.md) | Core feature specifications (L1-L3) |
| [UI/UX Design](docs/ui-ux-design.md) | Information architecture, page structure |
| [LLM & Prompt](docs/llm-prompt-design.md) | Prompt pipeline, model strategy |
| [Prompt Engineering](docs/prompt-engineering.md) | Production PromptOps, anti-hallucination, TDD |
| [Roadmap](docs/roadmap.md) | Phased roadmap and success metrics |
| [PRD](docs/prd.md) | Product requirements for MVP (Phase 1) |
| [Backend Design](docs/backend-design.md) | HLD/LLD, contract, TDD, sprint plan |
| [Frontend Design](docs/frontend-design.md) | Page prototypes, components, IA, state management |
| [OO Adapter Spec](docs/oo-adapter-spec.md) | OpenObserve SDK contract, query protocol, TDD |
| [Prompt Engineering](docs/prompt-engineering.md) | Production PromptOps, anti-hallucination, TDD |
| [Eval & Benchmark](docs/eval-benchmark.md) | Golden dataset, release gate, feedback loop |

## Roadmap

| Phase | Scope | Timeline |
|-------|-------|----------|
| Phase 1 (MVP) | OO Adapter, Alert Workbench, RCA Report | 4-6 weeks |
| Phase 2 | Similar Incident, Blast Radius, Prompt Studio | 4 weeks |
| Phase 3 | SigNoz/Elastic Adapter, Topology SDK | 4 weeks |
| Phase 4 | Auto Remediation, Closed-loop Learning | Future |

## License

See [LICENSE](LICENSE).
