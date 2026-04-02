# M2: OpenObserve Adapter + Tool Layer
> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Connect to real OpenObserve instance for multi-signal evidence retrieval.

**Architecture:** OO HTTP client + SQL query builder + Result mapper. Tool orchestrator in parallel multi-tool execution.

**Tech Stack:** Go 1.26.1, net/http (stdlib) + SQLite

---

## Task Summary

| Task | Files | Description |
|------|------|-------------|
| 1 | client.go + client_test.go | OO HTTP Client Core (auth, request/response, error mapping) |
| 2 | logs.go + logs_test.go | Logs search via SQL |
| 3 | traces.go + traces_test.go | Trace search via SQL |
| 4 | metrics.go + metrics_test.go | Metric query via SQL aggregation |
| 5 | drilldown.go + drilldown_test.go | OO UI drill-down URL generation |
| 6 | orchestrator.go + orchestrator_test.go | Multi-signal tool orchestration |
| 7 | Tool impls | logs_tool.go + trace_tool.go + metrics_tool.go + tests | Tool interface adapters |
| 8 | Integration Tests | mock_server.go + integration tests |

## Status

Completed.
