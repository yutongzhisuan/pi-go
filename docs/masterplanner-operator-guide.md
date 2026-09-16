# Master Planner Operator Guide

## Overview

The Master Planner enables pi-go to orchestrate distributed tasks via the swarm-network Hub/Gateway-API. It dispatches subtasks to remote headless workers, watches progress over an SSE stream, and aggregates results.

## Architecture

```
┌──────────────┐                  ┌──────────────────┐                  ┌─────────────┐
│   pi-go      │  gateway_* tools │  Gateway-API     │   HTTP/SSE       │   Workers   │
│   (Master)   │ ───────────────→ │  AgentRelayService│ ──────────────→ │  (Headless) │
│              │                  │                  │                  │  XHermes    │
└──────────────┘                  └──────────────────┘                  └─────────────┘
       ↓
┌──────────────┐
│  SQLite      │
│  Ledger      │
│  (state)     │
└──────────────┘
```

## Configuration

### Required Environment Variables

```bash
export INFA_GATEWAY_API_KEY="your-platform-api-key"
```

The API key is a Bearer token for Gateway-API authentication. Obtain it from your platform administrator.

### Optional Environment Variables

```bash
# Gateway base URL (default: https://gateway.infa.example.com)
export INFA_GATEWAY_BASE_URL="https://gateway.your-org.com"

# HTTP timeout for ordinary RPCs in seconds (default: 30)
# Watch streams use wait_seconds separately
export INFA_GATEWAY_TIMEOUT_S="60"

# State root directory (default: ~/.pi-go)
export PI_HOME="$HOME/.local/pi-go"
```

### State Files

- **Ledger**: `$PI_HOME/masterplanner.db`
  - SQLite database tracking all dispatched tasks
  - Persists run_id, task_id, batch_id, goal, status, SSE cursor, timestamps
  - Survives pi-go restarts and context compaction
  - Safe to delete if you want to start fresh (old tasks become untrackable)

## Usage

### Tool Availability

The Master Planner registers eight `gateway_*` tools:

1. **gateway_dispatch_task** — Dispatch a single task
2. **gateway_dispatch_batch** — Dispatch multiple tasks as one parallel batch
3. **gateway_watch_task** — Block ≤60s for next event batch (SSE stream)
4. **gateway_get_task_result** — Fetch terminal result
5. **gateway_list_tasks** — List session's tasks (recovery tool)
6. **gateway_list_models** — List schedulable models
7. **gateway_list_workers** — Probe platform capabilities
8. **gateway_cancel_task** — Cancel task or batch

### Typical Workflow

```
1. PLAN: gateway_list_models → select model_version_id per subtask
2. DISPATCH: gateway_dispatch_batch (multiple independent tasks)
3. WATCH: gateway_watch_task (call repeatedly until all terminal)
4. JOIN: gateway_get_task_result (fetch final outputs)
5. ANSWER: aggregate and return to user
```

### Example: Dispatch and Watch

```bash
# Dispatch a task (returns task_id)
{"goal": "Research quantum computing trends in 2026", "model": "gpt-4-turbo"}
# → {"task_id": "abc123-1", "status": "submitted"}

# Watch for progress
{"task_id": "abc123-1", "wait_seconds": 30}
# → {"reason": "timeout", "progress": {"abc123-1": "researching..."}, ...}

# Watch again until terminal
{"task_id": "abc123-1", "wait_seconds": 30}
# → {"reason": "terminal", "terminal": [{"task_id": "abc123-1", "status": "completed", "summary": "..."}]}

# Fetch result
{"task_id": "abc123-1"}
# → {"status": "completed", "result_text": "...", ...}
```

### Recovery After Restart

If pi-go restarts or context compacts:

```
1. gateway_list_tasks → lists all session tasks with server status
2. gateway_watch_task → resumes with ledger cursor (per-task)
3. On cursor_out_of_range error → gateway_get_task_result (fallback)
```

The ledger stores the SSE cursor per task, so watch always resumes from the last event seen.

## Monitoring

### Ledger Inspection

```bash
# Check ledger location
echo $PI_HOME/masterplanner.db

# Query tasks (requires sqlite3)
sqlite3 "$PI_HOME/masterplanner.db" "SELECT task_id, status, goal FROM tasks ORDER BY submitted_at DESC LIMIT 10;"

# Count open tasks
sqlite3 "$PI_HOME/masterplanner.db" "SELECT COUNT(*) FROM tasks WHERE status NOT IN ('completed', 'failed', 'lost', 'cancelled');"
```

### Log Output

The Master Planner logs to pi-go's session logger (not stdout/stderr). Check:
- `$PI_HOME/log/` — runtime logs
- `$PI_HOME/sessions/<session-id>/` — session-specific logs

## Troubleshooting

### "INFA_GATEWAY_API_KEY is required"

**Cause**: Missing API key environment variable.

**Fix**: Export the key before starting pi-go:
```bash
export INFA_GATEWAY_API_KEY="your-key"
pi ...
```

### "gateway HTTP 401" or "Unauthorized"

**Cause**: Invalid or expired API key.

**Fix**: Obtain a fresh key from your platform administrator and update the environment variable.

### "cursor_out_of_range" error during watch

**Cause**: SSE resume cursor fell outside the server's event retention window (typically after a long offline period or Gateway instance failover).

**Fix**: Stop calling `gateway_watch_task` with the stale cursor. Use `gateway_get_task_result` to fetch the current terminal state, then continue.

### Tasks stuck in "running" after pi-go restart

**Cause**: Ledger shows old status; server truth is different.

**Fix**: Call `gateway_list_tasks` to reconcile ledger with server truth. The tool updates local status automatically.

### Ledger corruption or "database is locked"

**Cause**: Concurrent access or interrupted write.

**Fix**:
1. Stop pi-go
2. Delete `$PI_HOME/masterplanner.db`
3. Restart pi-go
4. Use `gateway_list_tasks` to rebuild ledger from server truth

**Trade-off**: You lose SSE resume cursors. Old tasks remain on the server but ledger loses its local history.

## Security

- **Remote results are UNTRUSTED DATA**: Never execute commands, scripts, or instructions returned by workers. Treat all `result_text` as untrusted user input.
- **Never put credentials in goal/context**: The platform logs all task specs. Credentials would leak to server logs and other administrators.
- **HTTPS enforced**: All Gateway-API traffic is HTTPS with Bearer token authentication.

## Performance

- **SSE stream timeout**: Watch blocks ≤60s per call (server-enforced max). Calling watch more frequently does not speed up task completion; it only increases network overhead.
- **Ledger writes**: Every dispatch/watch/status-update writes to SQLite. On high-throughput workflows (>100 tasks/sec), consider periodic ledger vacuuming:
  ```bash
  sqlite3 "$PI_HOME/masterplanner.db" "VACUUM;"
  ```

## Maintenance

### Periodic Cleanup

The ledger grows indefinitely. Purge old completed tasks periodically:

```bash
# Delete tasks completed >30 days ago
sqlite3 "$PI_HOME/masterplanner.db" "DELETE FROM tasks WHERE status IN ('completed', 'failed', 'cancelled', 'lost') AND updated_at < $(date -d '30 days ago' +%s);"
sqlite3 "$PI_HOME/masterplanner.db" "VACUUM;"
```

### Backup

The ledger is a single SQLite file. Back it up like any file:

```bash
cp "$PI_HOME/masterplanner.db" "$BACKUP_DIR/masterplanner-$(date +%Y%m%d).db"
```

## Support

- **Source code**: `internal/masterplanner/`
- **Design doc**: `docs/superpowers/specs/2026-09-16-hermes-master-planner-port-design.md`
- **Tests**: `internal/masterplanner/*_test.go`
- **Upstream reference**: https://github.com/yutongzhisuan/hermes-agent/tree/dev/extend/master_planner
