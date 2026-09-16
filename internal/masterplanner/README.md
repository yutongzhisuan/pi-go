# Master Planner - Gateway API Client

Client implementation for the Gateway API's AgentRelayService, enabling task orchestration and agent coordination.

## Overview

The Master planner provides a Go client for interacting with the Gateway API's task relay service. It handles:

- Task creation and batch submission
- Task listing and filtering
- Real-time task event streaming (SSE)
- Task result submission
- Task cancellation
- Worker and model discovery

## Installation

```bash
go get github.com/dimetron/pi-go/internal/masterplanner
```

## Quick Start

```go
package main

import (
    "context"
    "fmt"
    "log"
    
    "github.com/dimetron/pi-go/internal/masterplanner"
)

func main() {
    client := masterplanner.NewClient(
        masterplanner.WithBaseURL("http://localhost:18082"),
        masterplanner.WithAPIKey("your-api-key"),
    )
    
    resp, err := client.CreateTask(context.Background(), &masterplanner.CreateTaskRequest{
        WorkerID: "worker-1",
        ModelID:  "gpt-4",
        Prompt:   "Analyze this code",
        Priority: masterplanner.TaskPriorityNormal,
    })
    if err != nil {
        log.Fatal(err)
    }
    
    fmt.Printf("Task created: %s\n", resp.TaskID)
}
```

## Configuration

### Environment Variables

- `INFA_GATEWAY_BASE_URL`: Gateway API base URL (default: `http://localhost:18082`)
- `INFA_GATEWAY_API_KEY`: API key for authentication

### Client Options

```go
client := masterplanner.NewClient(
    masterplanner.WithBaseURL("https://api.example.com"),
    masterplanner.WithAPIKey("sk-..."),
    masterplanner.WithHTTPClient(customHTTPClient),
)
```

## Usage Examples

### Create a Task

```go
resp, err := client.CreateTask(ctx, &masterplanner.CreateTaskRequest{
    WorkerID: "worker-1",
    ModelID:  "gpt-4",
    Prompt:   "Write a function to sort an array",
    Priority: masterplanner.TaskPriorityHigh,
    Metadata: map[string]string{
        "user_id": "user-123",
        "session": "sess-456",
    },
    Timeout:    3600,
    MaxRetries: 3,
})
```

### Batch Create Tasks

```go
resp, err := client.BatchCreateTasks(ctx, &masterplanner.BatchCreateTasksRequest{
    Tasks: []*masterplanner.CreateTaskRequest{
        {ModelID: "gpt-4", Prompt: "Task 1"},
        {ModelID: "claude-3", Prompt: "Task 2"},
        {ModelID: "gemini-pro", Prompt: "Task 3"},
    },
})
```

### List Tasks

```go
resp, err := client.ListTasks(ctx, &masterplanner.ListTasksRequest{
    WorkerID: "worker-1",
    Status:   masterplanner.TaskStatusPending,
    PageSize: 50,
})

for _, task := range resp.Tasks {
    fmt.Printf("Task %s: %s\n", task.TaskID, task.Status)
}
```

### Watch Task Events (SSE)

```go
stream, err := client.WatchTasks(ctx, &masterplanner.WatchTasksRequest{
    WorkerID: "worker-1",
})
if err != nil {
    log.Fatal(err)
}
defer stream.Close()

for {
    select {
    case event := <-stream.Events:
        if event == nil {
            return
        }
        fmt.Printf("Event: %s - %s\n", event.Kind, event.Message)
        
    case err := <-stream.Errors:
        if err != nil {
            log.Printf("Stream error: %v\n", err)
            return
        }
        
    case <-ctx.Done():
        return
    }
}
```

### Submit Task Result

```go
resp, err := client.SubmitTaskResult(ctx, "task-123", &masterplanner.SubmitTaskResultRequest{
    TaskID: "task-123",
    Result: "Function successfully implemented",
})
```

### Cancel a Task

```go
resp, err := client.CancelTask(ctx, "task-123", &masterplanner.CancelTaskRequest{
    TaskID: "task-123",
    Reason: "User requested cancellation",
})
```

### List Workers

```go
resp, err := client.ListWorkers(ctx, &masterplanner.ListWorkersRequest{
    Status:   "active",
    PageSize: 100,
})

for _, worker := range resp.Workers {
    fmt.Printf("Worker %s: %s\n", worker.WorkerID, worker.Status)
}
```

### List Models

```go
resp, err := client.ListModels(ctx, &masterplanner.ListModelsRequest{
    Provider: "openai",
    PageSize: 50,
})

for _, model := range resp.Models {
    fmt.Printf("Model %s: %s (%d tokens)\n", 
        model.ModelID, model.DisplayName, model.ContextWindow)
}
```

## Types

### Task Statuses

```go
const (
    TaskStatusPending   TaskStatus = "PENDING"
    TaskStatusRunning   TaskStatus = "RUNNING"
    TaskStatusCompleted TaskStatus = "COMPLETED"
    TaskStatusFailed    TaskStatus = "FAILED"
    TaskStatusCancelled TaskStatus = "CANCELLED"
)
```

### Task Event Kinds

```go
const (
    TaskEventKindCreated    TaskEventKind = "CREATED"
    TaskEventKindStarted    TaskEventKind = "STARTED"
    TaskEventKindProgress   TaskEventKind = "PROGRESS"
    TaskEventKindCompleted  TaskEventKind = "COMPLETED"
    TaskEventKindFailed     TaskEventKind = "FAILED"
    TaskEventKindCancelled  TaskEventKind = "CANCELLED"
    TaskEventKindHeartbeat  TaskEventKind = "HEARTBEAT"
)
```

### Task Priorities

```go
const (
    TaskPriorityLow    TaskPriority = 0
    TaskPriorityNormal TaskPriority = 1
    TaskPriorityHigh   TaskPriority = 2
)
```

## Error Handling

The client returns standard Go errors. HTTP errors include the status code and response body:

```go
resp, err := client.CreateTask(ctx, req)
if err != nil {
    // Check for specific error types
    if strings.Contains(err.Error(), "status=401") {
        log.Fatal("Authentication failed")
    }
    log.Fatal(err)
}
```

### SSE Error Codes

```go
const (
    SSEErrorCodeCursorOutOfRange = "cursor_out_of_range"
    SSEErrorCodeSlowConsumer     = "slow_consumer"
    SSEErrorCodeUnknown          = "unknown"
)
```

## Testing

The package includes a fake gateway for testing:

```go
func TestMyCode(t *testing.T) {
    fg := masterplanner.NewFakeGateway()
    defer fg.Close()
    
    client := masterplanner.NewClient(
        masterplanner.WithBaseURL(fg.URL()),
    )
    
    // Add test data
    fg.AddWorker(&masterplanner.Worker{
        WorkerID: "test-worker",
        Status:   "active",
    })
    
    // Test your code
    resp, err := client.ListWorkers(ctx, &masterplanner.ListWorkersRequest{})
    // ...
}
```

## Architecture

```
┌─────────────┐
│   pi-go     │
│ Application │
└──────┬──────┘
       │
       │ Client API
       │
┌──────▼──────────┐
│ Master Planner  │
│     Client      │
└──────┬──────────┘
       │
       │ HTTP/SSE
       │
┌──────▼──────────┐
│  Gateway API    │
│ AgentRelay Svc  │
└──────┬──────────┘
       │
       │
┌──────▼──────────┐
│   Task Hub /    │
│   Worker Pool   │
└─────────────────┘
```

## Performance Considerations

1. **Connection Pooling**: The default HTTP client reuses connections. For high throughput, consider tuning `http.Transport.MaxIdleConns`.

2. **Timeouts**: Default request timeout is 30s. SSE streams use 5m timeout. Adjust via `WithHTTPClient()`.

3. **Batch Operations**: Use `BatchCreateTasks` for bulk submissions to reduce round trips.

4. **SSE Buffering**: Events are buffered with capacity 10. Slow consumers may lose events or receive `slow_consumer` errors.

## Alignment with Gateway API

This implementation aligns with:
- Proto: `api/gateway-api/v1/agent_relay.proto` (AgentRelayService)
- HTTP paths and methods as defined in the proto service
- JSON field naming (snake_case per protojson)
- Enum values match proto definitions

See [SERVER_AGENT_RELAY_ALIGNMENT.md](../../docs/superpowers/SERVER_AGENT_RELAY_ALIGNMENT.md) for detailed mapping.

## Local Development

Start the local Gateway API stack:

```bash
cd server/deployments/dev
docker compose up -d
```

Point the client at localhost:

```bash
export INFA_GATEWAY_BASE_URL=http://localhost:18082
export INFA_GATEWAY_API_KEY=test-api-key
```

## Contributing

When making changes:

1. Ensure all tests pass: `go test ./internal/masterplanner/...`
2. Update types and enums to match proto changes
3. Maintain snake_case JSON field naming
4. Add tests for new functionality
5. Update alignment documentation

## License

See repository root LICENSE file.
