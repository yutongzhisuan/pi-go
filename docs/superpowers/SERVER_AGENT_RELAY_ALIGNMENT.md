# Gateway API Agent Relay Alignment

This document maps the Gateway API's AgentRelayService proto contract to pi-go's Master planner implementation.

## Source of Truth

- **Proto**: `api/gateway-api/v1/agent_relay.proto` (AgentRelayService) in https://github.com/yutongzhisuan/server
- **Domain notes**: `AGENTS.md` in server repo (task_relay.v1 / agent_relay_model)
- **Local stack**: `deployments/dev/docker-compose.yaml` — Gateway API on localhost:18082

## Environment Variables

- `INFA_GATEWAY_BASE_URL`: Base URL for Gateway API (default: `http://localhost:18082`)
- `INFA_GATEWAY_API_KEY`: Optional API key for authentication

## RPC to Implementation Mapping

| Proto RPC | HTTP Method | HTTP Path | pi-go Client Method | Status |
|-----------|-------------|-----------|---------------------|--------|
| CreateTask | POST | `/v1/agent/tasks` | `Client.CreateTask` | ✅ Implemented |
| BatchCreateTasks | POST | `/v1/agent/tasks:batch` | `Client.BatchCreateTasks` | ✅ Implemented |
| ListTasks | GET | `/v1/agent/tasks` | `Client.ListTasks` | ✅ Implemented |
| WatchTasks (SSE) | POST | `/v1/agent/tasks:watch` | `Client.WatchTasks` | ✅ Implemented |
| SubmitTaskResult | POST | `/v1/agent/tasks/{task_id}:result` | `Client.SubmitTaskResult` | ✅ Implemented |
| CancelTask | POST | `/v1/agent/tasks/{task_id}:cancel` | `Client.CancelTask` | ✅ Implemented |
| ListWorkers | POST | `/v1/agent/workers:list` | `Client.ListWorkers` | ✅ Implemented |
| ListModels | POST | `/v1/agent/models:list` | `Client.ListModels` | ✅ Implemented |

## Type Mappings

### TaskStatus Enum

Proto values map to Go constants with snake_case JSON encoding:

| Proto | Go Constant | JSON Value |
|-------|-------------|------------|
| TASK_STATUS_PENDING | TaskStatusPending | "PENDING" |
| TASK_STATUS_RUNNING | TaskStatusRunning | "RUNNING" |
| TASK_STATUS_COMPLETED | TaskStatusCompleted | "COMPLETED" |
| TASK_STATUS_FAILED | TaskStatusFailed | "FAILED" |
| TASK_STATUS_CANCELLED | TaskStatusCancelled | "CANCELLED" |

### TaskEventKind Enum

| Proto | Go Constant | JSON Value |
|-------|-------------|------------|
| TASK_EVENT_KIND_CREATED | TaskEventKindCreated | "CREATED" |
| TASK_EVENT_KIND_STARTED | TaskEventKindStarted | "STARTED" |
| TASK_EVENT_KIND_PROGRESS | TaskEventKindProgress | "PROGRESS" |
| TASK_EVENT_KIND_COMPLETED | TaskEventKindCompleted | "COMPLETED" |
| TASK_EVENT_KIND_FAILED | TaskEventKindFailed | "FAILED" |
| TASK_EVENT_KIND_CANCELLED | TaskEventKindCancelled | "CANCELLED" |
| TASK_EVENT_KIND_HEARTBEAT | TaskEventKindHeartbeat | "HEARTBEAT" |

### Request/Response Fields

All JSON field names use snake_case as per protojson encoding:

#### CreateTaskRequest
```json
{
  "worker_id": "string",
  "model_id": "string",
  "prompt": "string",
  "priority": 1,
  "metadata": {"key": "value"},
  "timeout": 3600,
  "max_retries": 3,
  "context": {"key": "value"}
}
```

#### CreateTaskResponse
```json
{
  "task_id": "string",
  "status": "PENDING",
  "created_at": "2026-09-16T05:46:00Z"
}
```

#### Task
```json
{
  "task_id": "string",
  "worker_id": "string",
  "model_id": "string",
  "prompt": "string",
  "status": "PENDING",
  "priority": 1,
  "result": "string",
  "error": "string",
  "metadata": {"key": "value"},
  "context": {"key": "value"},
  "created_at": "2026-09-16T05:46:00Z",
  "started_at": "2026-09-16T05:47:00Z",
  "completed_at": "2026-09-16T05:48:00Z",
  "retry_count": 0
}
```

#### TaskEvent
```json
{
  "event_id": "string",
  "task_id": "string",
  "kind": "CREATED",
  "message": "string",
  "data": "string",
  "timestamp": "2026-09-16T05:46:00Z",
  "sequence_id": 1
}
```

## SSE (Server-Sent Events) Handling

The `WatchTasks` endpoint uses SSE for real-time task event streaming.

### SSE Frame Format

```
event: task_event
data: {"event_id":"evt-1","task_id":"task-1","kind":"CREATED",...}

```

### SSE Error Frames

Special error events with error codes:

| Error Code | Go Constant | Meaning |
|------------|-------------|---------|
| `cursor_out_of_range` | SSEErrorCodeCursorOutOfRange | Requested event sequence is out of range |
| `slow_consumer` | SSEErrorCodeSlowConsumer | Client is not consuming events fast enough |
| `unknown` | SSEErrorCodeUnknown | Unknown error occurred |

Error event format:
```
event: error
data: {"code":"cursor_out_of_range","message":"Requested sequence ID is too old"}

```

### Client Implementation

The `Client.WatchTasks` method returns a `TaskEventStream`:

```go
stream, err := client.WatchTasks(ctx, &WatchTasksRequest{
    WorkerID: "worker-1",
    Status: TaskStatusPending,
})
defer stream.Close()

for {
    select {
    case event := <-stream.Events:
        // Handle event
    case err := <-stream.Errors:
        // Handle error
    case <-ctx.Done():
        return
    }
}
```

## Testing

### Unit Tests

All client methods are tested against a fake gateway implementation:

```bash
go test ./internal/masterplanner/...
```

Tests cover:
- All RPC methods
- JSON field naming (snake_case)
- Enum values
- SSE streaming
- Error handling
- Timeout behavior

### Integration Tests

To test against a real Gateway API instance:

```bash
export INFA_GATEWAY_BASE_URL=http://localhost:18082
export INFA_GATEWAY_API_KEY=your-api-key
go test -tags=integration ./internal/masterplanner/...
```

## Local Development Stack

The server repository provides a docker-compose setup for local development:

### Prerequisites

1. Clone the server repository (requires access):
   ```bash
   git clone https://github.com/yutongzhisuan/server.git
   cd server
   ```

2. Navigate to the development deployment:
   ```bash
   cd deployments/dev
   ```

### Starting the Stack

```bash
docker compose up -d
```

This starts:
- Gateway API on `http://localhost:18082`
- Hub service (task orchestration)
- PostgreSQL (task storage)
- Other dependencies

### Verifying the Stack

```bash
# Check Gateway API health
curl http://localhost:18082/health

# List available models
curl -H "Authorization: Bearer $INFA_GATEWAY_API_KEY" \
  http://localhost:18082/v1/agent/models:list

# Create a test task
curl -X POST http://localhost:18082/v1/agent/tasks \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $INFA_GATEWAY_API_KEY" \
  -d '{"model_id":"test-model","prompt":"test"}'
```

### Connecting pi-go

```bash
export INFA_GATEWAY_BASE_URL=http://localhost:18082
export INFA_GATEWAY_API_KEY=test-api-key

# Run pi-go with Master planner
go run cmd/pi/main.go --gateway-api
```

## Known Limitations

1. **Server Repository Access**: The server repository (https://github.com/yutongzhisuan/server) is private and requires authentication to access the proto files and docker-compose setup.

2. **Production API Key**: No live production Gateway API key is available for testing against a production environment.

3. **Proto File**: The actual proto file (`api/gateway-api/v1/agent_relay.proto`) could not be directly examined, so this implementation is based on the HTTP API contract and common protobuf/gRPC conventions.

4. **Worker Sidecar**: This document focuses on the Master planner client implementation. Worker sidecar expectations and Hub/task-relay worker interactions are not covered in detail.

## Future Work

1. **Proto Validation**: Once the server repository is accessible, validate all field names, types, and enums against the actual proto definition.

2. **Integration Tests**: Add integration tests that run against the local docker-compose stack.

3. **Worker Implementation**: Implement the worker sidecar that receives and executes tasks from the Gateway API.

4. **Metrics**: Add instrumentation for task creation, completion, failures, and latency.

5. **Retries**: Implement automatic retry logic with exponential backoff for failed requests.

6. **Connection Pooling**: Optimize HTTP client for high-throughput task submission.

## Related Files

- Implementation: `internal/masterplanner/`
  - `types.go` - Type definitions and enums
  - `client.go` - HTTP client implementation
  - `fake.go` - Fake gateway for testing
  - `client_test.go` - Unit tests

## References

- Gateway API Server: https://github.com/yutongzhisuan/server
- Proto File: `api/gateway-api/v1/agent_relay.proto`
- Domain Docs: `AGENTS.md` in server repo
- Local Stack: `deployments/dev/docker-compose.yaml` in server repo
