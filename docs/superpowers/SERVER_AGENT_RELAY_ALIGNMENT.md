# Server AgentRelayService Alignment

This document describes the alignment between pi-go's `internal/masterplanner` package and the authoritative Gateway API contract defined in `server/api/gateway-api/v1/agent_relay.proto`.

## RPC ↔ HTTP ↔ Client Method ↔ Tool Mapping

| Proto RPC | HTTP Method & Path | Client Method | Hermes Tool |
|-----------|-------------------|---------------|-------------|
| `DispatchTask` | `POST /v1/agent/tasks` | `DispatchTask` | `gateway_dispatch_task` |
| `DispatchTaskBatch` | `POST /v1/agent/tasks:batch` | `DispatchBatch` | `gateway_dispatch_batch` |
| `WatchTask` | `POST /v1/agent/tasks:watch` (SSE) | `Watch` | `gateway_watch_task` |
| `GetTaskResult` | `POST /v1/agent/tasks/{task_id}:result` | `GetTaskResult` | `gateway_get_task_result` |
| `ListTasks` | `GET /v1/agent/tasks` | `ListTasks` | `gateway_list_tasks` |
| `ListWorkers` | `POST /v1/agent/workers:list` | `ListWorkers` | `gateway_list_workers` |
| `ListAgentModels` | `POST /v1/agent/models:list` | `ListAgentModels` | `gateway_list_models` |
| `CancelTask` | `POST /v1/agent/tasks/{task_id}:cancel` | `CancelTask` | `gateway_cancel_task` |

## TaskStatus Enum Mapping

| Proto Value | JSON Wire Value | Go Constant |
|-------------|----------------|-------------|
| `TASK_STATUS_UNSPECIFIED` | `"TASK_STATUS_UNSPECIFIED"` | `TaskStatusUnspecified` (not defined) |
| `TASK_STATUS_PENDING` | `"TASK_STATUS_PENDING"` | `TaskStatusPending` |
| `TASK_STATUS_RUNNING` | `"TASK_STATUS_RUNNING"` | `TaskStatusRunning` |
| `TASK_STATUS_COMPLETED` | `"TASK_STATUS_COMPLETED"` | `TaskStatusCompleted` |
| `TASK_STATUS_FAILED` | `"TASK_STATUS_FAILED"` | `TaskStatusFailed` |
| `TASK_STATUS_LOST` | `"TASK_STATUS_LOST"` | `TaskStatusLost` |
| `TASK_STATUS_CANCELLED` | `"TASK_STATUS_CANCELLED"` | `TaskStatusCancelled` |

**Note:** The client uses snake_case normalization internally (`pending`, `running`, `completed`, `failed`, `lost`, `cancelled`) and converts to/from proto enum names as needed.

## TaskEventKind Enum Mapping

| Proto Value | JSON Wire Value | Client String | Description |
|-------------|----------------|---------------|-------------|
| `TASK_EVENT_KIND_UNSPECIFIED` | `"TASK_EVENT_KIND_UNSPECIFIED"` | `"event"` | Unspecified event |
| `TASK_EVENT_KIND_STATUS` | `"TASK_EVENT_KIND_STATUS"` | `"status"` | Status change |
| `TASK_EVENT_KIND_PROGRESS` | `"TASK_EVENT_KIND_PROGRESS"` | `"progress"` | Progress update |
| `TASK_EVENT_KIND_TERMINAL` | `"TASK_EVENT_KIND_TERMINAL"` | `"terminal"` | Task completed/failed/lost/cancelled |
| `TASK_EVENT_KIND_CHECKPOINT` | `"TASK_EVENT_KIND_CHECKPOINT"` | `"checkpoint"` | Intermediate checkpoint |
| `TASK_EVENT_KIND_AGGREGATE` | `"TASK_EVENT_KIND_AGGREGATE"` | `"aggregate"` | Batch aggregation result |

**Implementation:** `taskEventKindType()` in `client.go` normalizes proto enum names and numeric values to lowercase client strings.

## ContextPayload Oneof

The proto defines a `oneof payload` with three variants:

| Variant | Type | JSON Field | Usage |
|---------|------|-----------|--------|
| `inline` | `string` | `"inline"` | Plain text, recommended < 64 KiB |
| `inline_gzip` | `InlineGzip` | `"inline_gzip"` | Compressed in-band delivery |
| `ref` | `ContextRef` | `"ref"` | Out-of-band URI fetch by worker |

### InlineGzip Structure

```go
type InlineGzip struct {
    GzipData []byte `json:"gzip_data"`
    SHA256   string `json:"sha256"`  // over decompressed plaintext
}
```

### ContextRef Structure

```go
type ContextRef struct {
    URI             string `json:"uri"`
    SHA256          string `json:"sha256"`
    ContentEncoding string `json:"content_encoding,omitempty"`  // "" or "gzip"
    Signature       string `json:"signature,omitempty"`
}
```

**Implementation:** `encodeContext()` in `client.go` automatically uses `inline_gzip` for payloads > 48 KiB.

## Key Type Alignments

### TaskSpec

All proto fields are mapped in `internal/masterplanner/types.go`:

- `task_id`, `goal`, `params`, `context`, `toolsets`, `model` — core fields
- `target_worker`, `callback_topic`, `priority`, `depends_on`, `aggregate_key` — routing/DAG
- `timeout_seconds`, `queue_timeout_seconds`, `max_attempts`, `first_progress_seconds` — timeouts
- `min_resources`, `trace_context` — scheduling & tracing
- `allowed_worker_ids`, `deny_worker_ids` — ACL enforcement
- `resume_from_checkpoint` — client extension (not in proto)

### TaskResult

Proto fields mapped:

- `task_id`, `status`, `summary`, `result_text`, `error` — core result
- `fields`, `usage` — structured metadata & metrics
- `started_at`, `completed_at`, `worker_id`, `schema_version` — execution metadata
- `batch_id`, `latest_checkpoint_id`, `attempt`, `max_attempts` — batch/retry context
- `result_truncated` — indicates relay size limit hit

### DispatchTaskResponse

Proto fields mapped:

- `task_id`, `batch_id`, `callback_topic`, `status` — identification
- `idempotent_hit`, `existing_result` — idempotency detection
- `attempt` — retry counter

### Worker & Model Types

- `WorkerInfo` — full worker node metadata (status, toolsets, resources, load, announce/heartbeat times)
- `WorkerResources` — CPU, memory, GPU, disk, network profile
- `WorkerLoad` — current utilization (running tasks, CPU%, memory%)
- `AgentModel` — pool-wide schedulable model (model_version_id, display_name, node_count, available_slots, regions)

### SSE Error Frames

When the watch stream encounters an error, it emits an `event: error` SSE frame with Kratos error JSON in the data field.

#### CursorOutOfRange

Emitted when `since_event_id` has expired from the Hub's ring buffer.

```json
{
  "code": "FAILED_PRECONDITION",
  "reason": "cursor_out_of_range",
  "message": "Requested event cursor 12345 is no longer available",
  "metadata": {
    "requested_since_event_id": 12345,
    "oldest_available_event_id": 50000,
    "newest_event_id": 67890
  }
}
```

**Recovery:** Call `gateway_list_tasks` to get current state, then resume watch from `newest_event_id` or fall back to `gateway_get_task_result` per task.

#### SlowConsumer

Emitted when the client cannot keep up with the event stream rate.

```json
{
  "code": "RESOURCE_EXHAUSTED",
  "reason": "slow_consumer",
  "message": "Client cannot keep up with event stream",
  "metadata": {
    "delivered_event_id": 60000,
    "newest_event_id": 67890
  }
}
```

**Recovery:** Resume watch from `delivered_event_id + 1` or use a shorter `wait_seconds` to process events in smaller batches.

**Implementation:** `normalizeSSEError()` in `client.go` parses error frames and extracts metadata into the `WatchResult.Error` map.

## BatchPolicy CompletionMode

| Proto Enum Value | JSON Wire Value | Tool Input String |
|-----------------|----------------|-------------------|
| `COMPLETION_MODE_UNSPECIFIED` | `"COMPLETION_MODE_UNSPECIFIED"` | `""` |
| `COMPLETION_MODE_ALL` | `"COMPLETION_MODE_ALL"` | `"all"` |
| `COMPLETION_MODE_ANY` | `"COMPLETION_MODE_ANY"` | `"any"` |
| `COMPLETION_MODE_MAJORITY` | `"COMPLETION_MODE_MAJORITY"` | `"majority"` |
| `COMPLETION_MODE_THRESHOLD` | `"COMPLETION_MODE_THRESHOLD"` | `"threshold"` |

**Implementation:** `joinPolicyToCompletionMode()` in `client.go` converts tool string input to proto enum names.

## Hermes Gateway Tool Flow

The eight `gateway_*` tools implement the master planner's five-phase workflow:

### Phase 1: PLAN
- Tool: `gateway_list_models`
- Purpose: Discover schedulable models; bind each `spec.model` to a valid `model_version_id`

### Phase 2: DISPATCH
- Tools: `gateway_dispatch_task`, `gateway_dispatch_batch`
- Purpose: Submit single task or parallel batch; returns `task_id` / `batch_id`

### Phase 3: WATCH
- Tool: `gateway_watch_task`
- Purpose: SSE stream watch for progress/checkpoint/terminal events; cursor-resumable

### Phase 4: JOIN
- Tool: `gateway_get_task_result`
- Purpose: Fetch terminal result; re-dispatch failed/empty results as needed

### Phase 5: ANSWER
- Internal: Aggregate results and return to user

### Recovery After Restart/Compaction
- Tool: `gateway_list_tasks`
- Purpose: Query ledger + Gateway for open tasks; resume watch from last cursor
- Fallback: On `cursor_out_of_range`, use `gateway_get_task_result` per task

## Local Development

The Gateway API is served by the swarm-network Hub. For local testing:

```bash
# If you have access to the server repo:
cd server/deployments/dev
docker compose up -d gateway-api

# Configure pi-go client:
export INFA_GATEWAY_BASE_URL=http://localhost:18082
export INFA_GATEWAY_API_KEY=<test-key>

# Test with masterplanner integration tests:
go test -v ./internal/masterplanner/...
```

**Note:** No live production API key is available for pi-go open-source testing. The `fake/gateway.go` mock server provides a full in-memory implementation for unit tests.

## Alignment Checklist

- ✅ All 8 proto RPCs mapped to HTTP paths and client methods
- ✅ `TaskStatus` enum includes `LOST` status
- ✅ `TaskEventKind` enum mapped (STATUS, PROGRESS, TERMINAL, CHECKPOINT, AGGREGATE)
- ✅ `TaskSpec` includes all proto fields (target_worker, min_resources, trace_context, ACL, timeouts, max_attempts)
- ✅ `TaskResult` includes all proto fields (fields, usage, timestamps, worker_id, attempt, result_truncated)
- ✅ `ContextPayload` oneof with inline, inline_gzip (with gzip_data + sha256), ref
- ✅ `InlineGzip`, `ContextRef`, `ResourceRequirements`, `TraceContext` types defined
- ✅ `TaskUsage`, `TaskFields`, `Metric`, `KeyValue` types defined
- ✅ `TaskCheckpoint`, `AggregateResult`, `BatchPolicy` types defined
- ✅ `WorkerInfo`, `WorkerResources`, `WorkerLoad`, `AgentModel` types defined
- ✅ `CursorOutOfRange`, `SlowConsumer` SSE error types defined
- ✅ `DispatchTaskRequest/Response`, `DispatchTaskBatchRequest/Response` types match proto
- ✅ `ListAgentModels` method exists (aliased from `ListModels`)
- ✅ Watch filter `oneof` (topic/batch_id/task_id) + `since_event_id` supported
- ✅ SSE error frames documented and handled
- ✅ All masterplanner tests pass

## Known Limitations

1. **No live Gateway API key** — pi-go open-source cannot test against production Gateway
2. **Worker sidecar out of scope** — ACP sidecar for workers is tracked separately
3. **Client-daemon host changes out of scope** — not part of this alignment
4. **Guessing forbidden** — implementation strictly follows attached proto; no invented APIs

## References

- Proto source of truth: `uploads/agent_relay.proto` (from `yutongzhisuan/server` → `api/gateway-api/v1/agent_relay.proto`)
- Client implementation: `internal/masterplanner/client.go`
- Type definitions: `internal/masterplanner/types.go`
- Hermes tools: `internal/masterplanner/tools.go`
- Task ledger: `internal/masterplanner/ledger.go`
- Tests: `internal/masterplanner/client_test.go`, `internal/masterplanner/integration_test.go`
- Mock server: `internal/masterplanner/fake/gateway.go`
