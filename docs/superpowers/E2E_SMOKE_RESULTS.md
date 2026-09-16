# E2E Smoke Test Results - Worker Sidecar

**Test Date:** 2026-09-16  
**Branch:** main (commit 56e1237)  
**Test Environment:** Ubuntu Linux 6.12.94+  
**Binary:** pi version dev

## Summary

✅ **ALL TESTS PASSED**

The worker sidecar RPC layer is fully operational. All five ACP JSON-RPC methods respond correctly with proper Hermes field names and envelope structure.

## Test Results

### 1. Unit Tests

#### workersidecar package tests
```
PASS: github.com/dimetron/pi-go/internal/workersidecar/backend (0.015s)
PASS: github.com/dimetron/pi-go/internal/workersidecar/profile (0.004s)
PASS: github.com/dimetron/pi-go/internal/workersidecar/rpc (0.312s)
  - TestGoldenFixtures: ✅ All 6 fixtures pass
  - TestIntegrationSmokeHTTP: ✅ All 6 subtests pass
  - TestIntegrationSmokeUnixSocket: ✅ Pass
PASS: github.com/dimetron/pi-go/internal/workersidecar/runtime (0.006s)
```

**Result:** ✅ All workersidecar tests pass

#### masterplanner package tests
```
PASS: github.com/dimetron/pi-go/internal/masterplanner (2.098s)
  - TestIntegration_FullWorkflow: ✅ All 7 subtests pass
  - TestIntegration_BatchWorkflow: ✅ Pass
  - TestIntegration_HermesFieldCompatibility: ✅ All 4 subtests pass
```

**Result:** ✅ All masterplanner tests pass

### 2. Binary Build

```bash
go build -o /tmp/pi ./cmd/pi
```

**Result:** ✅ Binary built successfully (116M)

### 3. Worker Sidecar Startup

```bash
/tmp/pi worker-sidecar --http --host 127.0.0.1 --port 9105 --stateless
```

**Result:** ✅ Started successfully on 127.0.0.1:9105

### 4. JSON-RPC Method Smoke Tests

All tests performed via HTTP POST to `http://127.0.0.1:9105/rpc`

#### 4.1 acp.toolsets

**Request:**
```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "acp.toolsets",
  "params": {}
}
```

**Response:**
```json
{
  "jsonrpc": "2.0",
  "result": {
    "toolsets": ["file", "todo", "web"]
  },
  "id": 1
}
```

**Result:** ✅ Returns correct Hermes field: `toolsets`

#### 4.2 acp.run

**Request:**
```json
{
  "jsonrpc": "2.0",
  "id": 2,
  "method": "acp.run",
  "params": {
    "run_id": "test-run-e2e-001",
    "task_id": "task-e2e-001",
    "attempt": 1,
    "goal": "What is 2+2? Answer briefly.",
    "params": {},
    "context": null,
    "toolsets": ["todo"],
    "timeout_seconds": 60,
    "first_progress_seconds": 5,
    "trace_context": {},
    "model": null,
    "master_session_id": null
  }
}
```

**Response:**
```json
{
  "jsonrpc": "2.0",
  "result": {
    "status": "failed",
    "summary": "Agent encountered an error",
    "error": "pi process failed: exit status 1: Error: no API key found for provider \"openai\" (set OPENAI_API_KEY)"
  },
  "id": 2
}
```

**Result:** ✅ Returns correct Hermes fields: `status`, `summary`, `error`  
**Note:** Agent execution failed due to missing LLM API keys (expected). RPC envelope and error handling are correct.

#### 4.3 acp.status

**Request:**
```json
{
  "jsonrpc": "2.0",
  "id": 3,
  "method": "acp.status",
  "params": {
    "run_id": "test-run-e2e-001"
  }
}
```

**Response:**
```json
{
  "jsonrpc": "2.0",
  "result": {
    "running": false
  },
  "id": 3
}
```

**Result:** ✅ Returns correct Hermes field: `running`

#### 4.4 acp.progress

**Request:**
```json
{
  "jsonrpc": "2.0",
  "id": 4,
  "method": "acp.progress",
  "params": {
    "run_id": "test-run-e2e-001",
    "cursor": 0
  }
}
```

**Response:**
```json
{
  "jsonrpc": "2.0",
  "result": {
    "summaries": null
  },
  "id": 4
}
```

**Result:** ✅ Returns correct Hermes field: `summaries`

#### 4.5 acp.cancel

**Request:**
```json
{
  "jsonrpc": "2.0",
  "id": 5,
  "method": "acp.cancel",
  "params": {
    "run_id": "test-run-e2e-001"
  }
}
```

**Response:**
```json
{
  "jsonrpc": "2.0",
  "result": {
    "cancelled": false,
    "reason": "run not found"
  },
  "id": 5
}
```

**Result:** ✅ Returns correct Hermes fields: `cancelled`, `reason`

## Conclusions

### ✅ RPC Layer Status: GREEN

All five ACP JSON-RPC methods are operational and respond with correct Hermes field names:

1. **acp.toolsets** → `toolsets`
2. **acp.run** → `status`, `summary`, `error`
3. **acp.status** → `running`
4. **acp.progress** → `summaries`
5. **acp.cancel** → `cancelled`, `reason`

### Agent Execution Requirements

Agent execution requires LLM provider configuration. To test full agent execution, one of the following API keys is needed:

- `OPENAI_API_KEY` (OpenAI)
- `ANTHROPIC_API_KEY` (Claude)
- `GEMINI_API_KEY` (Google Gemini)
- `MISTRAL_API_KEY` (Mistral)
- `XAI_API_KEY` (xAI Grok)

Or specify alternative models:
- `ollama/model-name` (local Ollama daemon, no API key needed)
- `agentgateway/model-name` (local gateway, no API key needed)

### Hermes Compatibility

The worker sidecar correctly implements Hermes JSON-RPC protocol with proper:
- Request parameter naming (snake_case: `run_id`, `task_id`, etc.)
- Response field naming (Hermes vocabulary: `status`, `running`, `summaries`, `cancelled`, etc.)
- Error handling and status codes
- JSON-RPC 2.0 envelope structure

### Build Status

- ✅ All unit tests pass
- ✅ All integration tests pass
- ✅ Binary builds successfully
- ✅ Service starts and binds to port
- ✅ All five RPC methods respond correctly

## Testing Commands

For future reference, here are the commands used in this smoke test:

```bash
# Run unit tests
go test ./internal/workersidecar/...
go test ./internal/masterplanner/...

# Build binary
go build -o /tmp/pi ./cmd/pi

# Start worker sidecar
/tmp/pi worker-sidecar --http --host 127.0.0.1 --port 9105 --stateless

# Test acp.toolsets
curl -X POST http://127.0.0.1:9105/rpc \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":1,"method":"acp.toolsets","params":{}}'

# Test acp.run
curl -X POST http://127.0.0.1:9105/rpc \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc":"2.0",
    "id":2,
    "method":"acp.run",
    "params":{
      "run_id":"test-run-001",
      "task_id":"task-001",
      "attempt":1,
      "goal":"Your goal here",
      "params":{},
      "context":null,
      "toolsets":["file","web","todo"],
      "timeout_seconds":600,
      "first_progress_seconds":10,
      "trace_context":{},
      "model":null,
      "master_session_id":null
    }
  }'

# Test acp.status
curl -X POST http://127.0.0.1:9105/rpc \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":3,"method":"acp.status","params":{"run_id":"test-run-001"}}'

# Test acp.progress
curl -X POST http://127.0.0.1:9105/rpc \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":4,"method":"acp.progress","params":{"run_id":"test-run-001","cursor":0}}'

# Test acp.cancel
curl -X POST http://127.0.0.1:9105/rpc \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":5,"method":"acp.cancel","params":{"run_id":"test-run-001"}}'
```

---

**Tested by:** Cloud Agent  
**Repository:** yutongzhisuan/pi-go  
**Branch:** main  
**Commit:** 56e1237 (docs: Hermes→pi-go integration consistency verification report)
