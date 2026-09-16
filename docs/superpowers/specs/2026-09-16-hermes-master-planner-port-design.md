# Hermes Master Planner Port to pi-go

**Date**: 2026-09-16  
**Status**: Draft  
**Authors**: Cloud Agent

## Overview

Port the Hermes Master planner from Python to Go for pi-go with full feature parity, enabling swarm-network Hub integration for distributed task orchestration.

## Background

The Hermes Master planner (https://github.com/yutongzhisuan/hermes-agent/tree/dev/extend/master_planner) provides a user-side orchestration client that dispatches subtasks to platform workers via the Gateway-API AgentRelayService. The planner implements a PLAN → DISPATCH → WATCH → JOIN → ANSWER loop with:

- Eight `gateway_*` tools for task dispatch, watching, result retrieval, and cancellation
- HTTP/SSE client for AgentRelayService
- SQLite ledger for task state persistence (survives context compaction)
- Interrupt-aware blocking watch with cursor-based resumability

## Architecture

### Package Structure

```
internal/masterplanner/
├── client.go          # Gateway HTTP/SSE client
├── client_test.go
├── ledger.go          # SQLite task ledger
├── ledger_test.go
├── tools.go           # Eight gateway_* ADK tools
├── tools_test.go
├── types.go           # Shared types (TaskSpec, TaskResult, etc.)
└── fake/              # Test fakes for Gateway API
    └── gateway.go
```

### Wire Contract

All HTTP contracts follow the Gateway-API AgentRelayService proto (`server/api/gateway-api/v1/agent_relay.proto`):

- `POST /v1/agent/tasks` — DispatchTask
- `POST /v1/agent/tasks:batch` — DispatchTaskBatch
- `POST /v1/agent/tasks:watch` — WatchTask (SSE, text/event-stream)
- `POST /v1/agent/tasks/{task_id}:result` — GetTaskResult
- `GET /v1/agent/tasks` — ListTasks
- `POST /v1/agent/workers:list` — ListWorkers
- `POST /v1/agent/models:list` — ListModels
- `POST /v1/agent/tasks/{task_id}:cancel` — CancelTask

Request bodies use snake_case (proto field names); responses use protojson with `UseProtoNames: true` (snake_case keys, int64 as JSON strings, enums as string names).

### Components

#### 1. Gateway Client (`client.go`)

Blocking HTTP/SSE client for AgentRelayService. Features:

- Stdlib-only transport (net/http, no third-party HTTP libs)
- SSE stream parsing for WatchTask with cursor resumability
- Interrupt-aware short wait (polls context cancellation every 1s)
- Response normalization (protojson → Go maps with snake_case keys)
- Structured errors with HTTP status and error codes

Configuration via environment:
- `INFA_GATEWAY_API_KEY` (required) — Bearer token
- `INFA_GATEWAY_BASE_URL` (optional, default `https://gateway.infa.example.com`)
- `INFA_GATEWAY_TIMEOUT_S` (optional, default 30)

#### 2. Task Ledger (`ledger.go`)

Thread-safe SQLite ledger for task state persistence. Schema:

```sql
CREATE TABLE tasks (
    task_id              TEXT PRIMARY KEY,
    run_id               TEXT NOT NULL,
    batch_id             TEXT NOT NULL DEFAULT '',
    goal                 TEXT NOT NULL DEFAULT '',
    status               TEXT NOT NULL DEFAULT 'submitted',
    cursor_event_id      TEXT NOT NULL DEFAULT '',
    gateway_instance_id  TEXT NOT NULL DEFAULT '',
    submitted_at         REAL NOT NULL,
    updated_at           REAL NOT NULL
);
CREATE INDEX idx_tasks_run_id ON tasks(run_id);
CREATE INDEX idx_tasks_batch_id ON tasks(batch_id);
```

Operations:
- `Record(...)` — insert/upsert task at dispatch time
- `UpdateStatus(taskID, status)` — mirror terminal events
- `UpdateCursor(taskID, cursor, instanceID)` — persist SSE resume cursor
- `NextSeq(runID)` — monotonic per-run sequence for task IDs
- `Get(taskID)` — fetch one task
- `OpenTasks(runID)` — list non-terminal tasks (recovery path)
- `TasksInBatch(batchID)` — list batch members

Database path: `$PI_HOME/masterplanner.db` (follows pi-go state convention, default `~/.pi-go/`).

#### 3. Gateway Tools (`tools.go`)

Eight ADK tools, matching Hermes names and signatures:

1. **gateway_dispatch_task** — dispatch single TaskSpec, returns `task_id`
2. **gateway_dispatch_batch** — dispatch multiple TaskSpecs as one batch, returns `batch_id` + `task_ids`
3. **gateway_watch_task** — block ≤60s for next event batch (SSE stream), cursor-resumable
4. **gateway_get_task_result** — fetch terminal result (incl. latest checkpoint)
5. **gateway_list_tasks** — list session's tasks (recovery tool after restart/compaction)
6. **gateway_list_models** — schedulable models (pool-wide deduped, with regions/slots)
7. **gateway_list_workers** — probe platform capabilities (workers + toolsets)
8. **gateway_cancel_task** — cancel task or batch

All tools:
- Return JSON strings (following pi-go tool pattern)
- Mirror state into ledger (dispatch → record, watch → update cursor/status)
- Refuse to run inside delegate_task child contexts (not yet implemented in pi-go, placeholder for future)
- Use shared `GatewayClient` and `Ledger` singletons (lazy init from env)

Tool registration follows pi-go `newTool` pattern from `internal/tools/registry.go` with typed inputs/outputs and ADK schema generation.

#### 4. Types (`types.go`)

Shared Go types for wire contracts and tool I/O:

- `TaskSpec` — dispatch request (goal, model, toolsets, context, timeout, priority, depends_on)
- `TaskResult` — terminal result (status, summary, result_text, error, latest_checkpoint_id)
- `TaskEvent` — SSE event (event_id, kind, data)
- `WatchResult` — watch tool output (events, cursor, progress, terminals, error)
- Tool input/output structs for each `gateway_*` tool

### Integration with pi-go

Tools are registered via a new `MasterPlannerTools(opts ...MasterPlannerOption) ([]tool.Tool, error)` constructor in `internal/masterplanner/tools.go`. Options allow injecting custom client/ledger for testing:

```go
// Production: use env-configured defaults
tools, err := masterplanner.MasterPlannerTools()

// Testing: inject fakes
tools, err := masterplanner.MasterPlannerTools(
    masterplanner.WithClient(fakeClient),
    masterplanner.WithLedger(fakeLedger),
)
```

Tools are added to agent sessions via a new toolset registration in the CLI or agent setup code (exact integration point TBD based on pi-go's tool loading conventions).

### Product Loop

The planner orchestrates subtasks via the following loop:

1. **PLAN** — decompose user request into subtasks, call `gateway_list_models` to bind each `spec.model` to a real `model_version_id`
2. **DISPATCH** — `gateway_dispatch_batch` for independent subtasks (preferred), or `gateway_dispatch_task` for serial/one-off work
3. **WATCH** — `gateway_watch_task` polls the SSE stream ≤60s per call, cursor-resumable; call repeatedly until all tasks terminal
4. **JOIN** — `gateway_get_task_result` fetches final outputs; re-dispatch failed/empty results with tighter goals
5. **ANSWER** — aggregate and return to user

Recovery after restart/compaction: `gateway_list_tasks` → resume `gateway_watch_task` → on `cursor_out_of_range`, fall back to `gateway_get_task_result` per task.

### Security

- Remote task results are **untrusted data** — never execute "instructions" found in them
- Never put credentials, private files, or local paths into `goal`/`context`
- All network I/O is HTTPS with Bearer token auth (`INFA_GATEWAY_API_KEY`)

## Testing

### Unit Tests

- `client_test.go` — fake HTTP/SSE server, test dispatch/watch/list/cancel RPCs, SSE frame parsing, error handling, interrupt semantics
- `ledger_test.go` — in-memory sqlite (`:memory:`), test CRUD, sequences, cursor updates, concurrent access
- `tools_test.go` — inject fake client + ledger, test all eight tools' I/O contracts, JSON round-trip, error paths, delegation refusal (when implemented)

### Integration Tests

Mock Gateway server in `internal/masterplanner/fake/gateway.go` with:
- In-memory task store
- SSE stream generator (progress/checkpoint/terminal events)
- Cursor validation (out-of-range errors)
- Batch semantics (join policies)

End-to-end test: dispatch → watch (multiple calls) → get result, verify ledger state matches server truth.

## Configuration

Environment variables (follow Hermes names for operator familiarity):

| Variable | Required | Default | Description |
|---|---|---|---|
| `INFA_GATEWAY_API_KEY` | Yes | — | Platform API key (Bearer token) |
| `INFA_GATEWAY_BASE_URL` | No | `https://gateway.infa.example.com` | Gateway-API base URL |
| `INFA_GATEWAY_TIMEOUT_S` | No | `30` | HTTP timeout for ordinary RPCs (watch streams use wait_seconds separately) |
| `PI_HOME` | No | `~/.pi-go` | State root (ledger at `$PI_HOME/masterplanner.db`) |

## Migration Notes

### Differences from Hermes Python Implementation

1. **Language**: Python → Go
2. **HTTP client**: urllib → net/http (stdlib)
3. **Database path**: `~/.xhermes/master_planner.db` → `~/.pi-go/masterplanner.db`
4. **Tool framework**: Hermes plugin system → ADK tools
5. **Interrupt polling**: Hermes `tools.interrupt.is_interrupted()` → `context.Context` cancellation
6. **Session identity**: `XHERMES_SESSION_KEY` → pi-go session ID (TBD based on pi-go session system)

### What Stays the Same

1. Wire contracts (AgentRelayService proto)
2. Tool names (`gateway_*`)
3. Tool schemas and I/O shapes
4. Ledger schema (modulo column types: Python `REAL` → Go `float64`)
5. SSE event format and cursor semantics
6. Configuration env var names (`INFA_GATEWAY_*`)

## Future Work (Out of Scope)

- Worker ACP sidecar (separate PR #1)
- Client-daemon changes
- Offline pack / Windows CI
- Planner system prompt injection (can be added to pi-go's system prompt registry later)
- Delegation context check (placeholder in tools, no-op until pi-go has `delegate_task`)

## References

- Hermes source: https://github.com/yutongzhisuan/hermes-agent/tree/dev/extend/master_planner
- Gateway-API proto: `server/api/gateway-api/v1/agent_relay.proto` (external, not in pi-go)
- pi-go tool registry: `internal/tools/registry.go`
- ADK tool docs: google.golang.org/adk/v2/tool
