# Master Planner Integration Testing

This document describes how to run integration and E2E tests for the Master Planner to verify Hermes wire compatibility.

## Quick Start

```bash
# Run all tests (unit + integration)
cd /workspace
go test ./internal/masterplanner/...

# Run only integration tests
go test ./internal/masterplanner/... -run TestIntegration -v

# Skip integration tests (fast mode)
go test ./internal/masterplanner/... -short
```

## Integration Tests

The integration test suite (`internal/masterplanner/integration_test.go`) verifies:

1. **Full workflow**: PLAN → DISPATCH → WATCH → JOIN → ANSWER
2. **Batch operations**: Parallel task dispatch and watch
3. **Ledger recovery**: State persistence across restarts
4. **Hermes field compatibility**: All tool param/result field names match Hermes `tools.py`

### Fake Gateway

Tests use a fake Gateway (`internal/masterplanner/fake/gateway.go`) that:
- Mimics AgentRelayService HTTP + SSE behavior
- Returns Hermes-shaped payloads (protojson with snake_case keys)
- Supports dispatch, watch (SSE), get_result, list_*, and cancel
- Allows injecting progress/checkpoint/terminal events for testing

### What's Tested

✅ **Dispatch single task**
- Request: `task_id`, `goal`, `model`, etc. match Hermes spec
- Response: `task_id`, `status`, `idempotent_hit` fields

✅ **Dispatch batch**
- Request: `specs[]`, `batch_id`, `join_policy`
- Response: `batch_id`, `count`

✅ **Watch task (SSE)**
- Stream: `event: message`, `data: {...}` frames
- Events: `event_id`, `kind` (TASK_EVENT_KIND_*), task-specific data
- Progress events: `progress_summary`
- Checkpoint events: `checkpoint.summary`
- Terminal events: `result.status`, `result.summary`
- Cursor-based resumability

✅ **Get task result**
- Response: `task_id`, `status`, `summary`, `result_text`, `latest_checkpoint_id`, `error`

✅ **List tasks**
- Response: `tasks[]` with `task_id`, `status`, `goal`

✅ **List models**
- Response: `models[]` with `model_version_id`, `display_name`, `node_count`, `available_slots`, `regions`

✅ **List workers**
- Response: `workers[]` with `worker_id`, `toolsets`, `status`

✅ **Cancel task/batch**
- Response: `cancelled` boolean

✅ **Ledger operations**
- Record, UpdateStatus, UpdateCursor, NextSeq, Get, OpenTasks, TasksInBatch

## End-to-End Testing Against Real Gateway

To run tests against a live Gateway-API deployment:

### 1. Set Environment Variables

```bash
export INFA_GATEWAY_API_KEY="your-platform-api-key"
export INFA_GATEWAY_BASE_URL="https://gateway.your-org.com"  # optional
export INFA_GATEWAY_TIMEOUT_S="60"  # optional
export REAL_GATEWAY_TEST=1
```

### 2. Run E2E Test

```bash
go test ./internal/masterplanner/... -run TestIntegration_RealGateway -v
```

### 3. What It Tests

The `TestIntegration_RealGateway` test:
- ✅ Verifies connectivity with `ListModels`
- ✅ Logs available models from the real Gateway
- 📝 Optionally dispatches a real task (commented out by default to avoid spurious tasks)

To enable full E2E dispatch → watch → result workflow, uncomment the `dispatch_real_task` subtest in `integration_test.go`.

### 4. Expected Output

```
=== RUN   TestIntegration_RealGateway
=== RUN   TestIntegration_RealGateway/connectivity_check
    integration_test.go:XYZ: Found 5 models on real Gateway
    integration_test.go:XYZ:   Model 1: gpt-4-turbo
    integration_test.go:XYZ:   Model 2: claude-opus-4
    ...
--- PASS: TestIntegration_RealGateway (0.5s)
```

### 5. Manual E2E Workflow

For manual verification, use the pi-go CLI with real Gateway:

```bash
# Configure Gateway
export INFA_GATEWAY_API_KEY="your-key"
export INFA_GATEWAY_BASE_URL="https://gateway.your-org.com"

# Start pi-go
pi

# In the session, the Master Planner tools are available:
# - gateway_list_models
# - gateway_dispatch_task
# - gateway_watch_task
# - gateway_get_task_result
# - gateway_list_tasks
# - gateway_list_workers
# - gateway_cancel_task
```

Example interaction:

```
> List available models
[agent calls gateway_list_models]
> Dispatch a research task to the first available model
[agent calls gateway_dispatch_task with model_version_id from list]
> Watch the task until it completes
[agent calls gateway_watch_task repeatedly until terminal]
> Show me the final result
[agent calls gateway_get_task_result]
```

## Continuous Integration

### Pre-merge Checks

The following tests MUST pass before merging:

```bash
# All unit tests
go test ./internal/masterplanner/... -short

# All integration tests (fake Gateway)
go test ./internal/masterplanner/... -run TestIntegration -v

# Field compatibility verification
go test ./internal/masterplanner/... -run TestIntegration_HermesFieldCompatibility -v
```

### Post-merge Smoke Tests

After deployment to staging/production:

```bash
# Verify connectivity to real Gateway
REAL_GATEWAY_TEST=1 go test ./internal/masterplanner/... -run TestIntegration_RealGateway -v

# Optional: full E2E workflow (uncomment dispatch_real_task in code)
# Creates a real task on the platform
```

## Hermes Wire Compatibility

The integration tests verify exact field name compatibility with Hermes `tools.py`:

| Tool | Verified Fields |
|------|----------------|
| `gateway_dispatch_task` | goal, model, toolsets, params, context, timeout_seconds, priority, depends_on, resume_from_checkpoint, resume_summary |
| `gateway_dispatch_batch` | specs, batch_id, join_policy |
| `gateway_watch_task` | task_id, batch_id, wait_seconds, since_event_id |
| `gateway_get_task_result` | task_id |
| `gateway_list_tasks` | batch_id, status, master_session_id |
| `gateway_list_models` | region |
| `gateway_list_workers` | require_toolsets |
| `gateway_cancel_task` | task_id, batch_id, reason |

All response fields match Hermes wire shapes:
- Snake_case keys (protojson `UseProtoNames: true`)
- Enum names as strings (`TASK_STATUS_COMPLETED`, not integers)
- Int64 as JSON strings (protojson default)

## Troubleshooting

### Integration tests fail with "connection refused"

**Cause**: Fake Gateway not starting.

**Fix**: Check for port conflicts. The fake uses `httptest.NewServer` which auto-assigns a port.

### Real Gateway test fails with 401 Unauthorized

**Cause**: Invalid or missing `INFA_GATEWAY_API_KEY`.

**Fix**: Verify the key is correct and has not expired:
```bash
curl -H "Authorization: Bearer $INFA_GATEWAY_API_KEY" \
     $INFA_GATEWAY_BASE_URL/v1/agent/models:list
```

### Field compatibility test fails

**Cause**: Type definition changed without updating tests.

**Fix**: The test documents the Hermes wire contract. If a field name changes, update both the type and the test. **Do NOT change field names** without coordinating with the Hermes/Hub team — this breaks wire compatibility.

### Watch test times out

**Cause**: Fake Gateway not sending events, or client not reading stream.

**Fix**: Check that:
1. Events are added to task before watch: `gw.AddProgressEvent(taskID, "...")`
2. Fake's `handleWatch` is flushing after each frame
3. Client's SSE parser is not blocking on context deadline

## References

- Hermes tools.py: https://github.com/yutongzhisuan/hermes-agent/tree/dev/extend/master_planner/tools.py
- Gateway-API proto: `server/api/gateway-api/v1/agent_relay.proto` (external)
- Fake Gateway: `internal/masterplanner/fake/gateway.go`
- Integration tests: `internal/masterplanner/integration_test.go`
