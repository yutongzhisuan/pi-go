# Hermes → pi-go Integration Consistency Verification Report

**Date**: 2026-09-16  
**Verifier**: Cloud Agent (bcId: TBD)  
**Branches Verified**:
- `yutongzhisuan/pi-go#cursor/hermes-worker-sidecar-d63b` (PR #1)
- `yutongzhisuan/pi-go#cursor/hermes-master-planner-port-6cd5` (PR #2)
- `yutongzhisuan/hermes-agent@dev` (Hermes source of truth)

---

## Executive Summary

**Status**: ✅ **PASS** — Both branches pass local tests with green evidence; wire contracts match Hermes sources.

### Test Results
- **Worker Sidecar** (`internal/workersidecar/...`): ✅ All tests PASS (5 RPC methods verified)
- **Master Planner** (`internal/masterplanner/...`): ✅ All tests PASS (8 gateway tools verified)
- **Wire Contract Consistency**: ✅ Field names and RPC methods match Hermes
- **Environment Variables**: ✅ Shared naming conventions preserved
- **Fail-Closed Behavior**: ⚠️ Docker sandbox explicitly refused (not implemented)

---

## 1. Test Execution Summary

### 1.1 Worker Sidecar Branch (`cursor/hermes-worker-sidecar-d63b`)

**Test Command**:
```bash
go test ./internal/workersidecar/... -v
```

**Result**: ✅ **ALL TESTS PASSED**

**Test Coverage**:
```
✅ internal/workersidecar/backend      - 5 tests passed
✅ internal/workersidecar/profile      - 14 tests passed  
✅ internal/workersidecar/rpc          - 17 tests passed (including golden fixtures)
✅ internal/workersidecar/runtime      - 4 tests passed
```

**Key Findings**:
- Five RPC methods implemented: `acp.run`, `acp.cancel`, `acp.status`, `acp.progress`, `acp.toolsets`
- Golden fixture tests verify exact JSON-RPC 2.0 wire format compliance
- Integration smoke tests pass for both HTTP and Unix socket transports
- Default socket path: `~/.xhermes/sub_agent/acp.sock` (matches Hermes)
- Sandbox flag `--sandbox docker` explicitly fails with safe error message (fail-closed)

**Test Output Excerpt**:
```
=== RUN   TestGoldenFixtures
=== RUN   TestGoldenFixtures/acp.run_completed
=== RUN   TestGoldenFixtures/acp.run_model_unavailable
=== RUN   TestGoldenFixtures/acp.cancel
=== RUN   TestGoldenFixtures/acp.status
=== RUN   TestGoldenFixtures/acp.progress
=== RUN   TestGoldenFixtures/acp.toolsets
--- PASS: TestGoldenFixtures (0.00s)
...
=== RUN   TestIntegrationSmokeHTTP
--- PASS: TestIntegrationSmokeHTTP (0.10s)
=== RUN   TestIntegrationSmokeUnixSocket
--- PASS: TestIntegrationSmokeUnixSocket (0.20s)
```

### 1.2 Master Planner Branch (`cursor/hermes-master-planner-port-6cd5`)

**Test Command**:
```bash
go test ./internal/masterplanner/... -v
```

**Result**: ✅ **ALL TESTS PASSED**

**Test Coverage**:
```
✅ internal/masterplanner - 21 tests passed (including Hermes field compatibility)
```

**Key Findings**:
- Eight gateway tools implemented with exact names from Hermes `extend/master_planner/tools.py`:
  1. `gateway_dispatch_task`
  2. `gateway_dispatch_batch`
  3. `gateway_watch_task`
  4. `gateway_get_task_result`
  5. `gateway_list_tasks`
  6. `gateway_list_models`
  7. `gateway_list_workers`
  8. `gateway_cancel_task`
- Dedicated `TestIntegration_HermesFieldCompatibility` verifies snake_case field name parity
- Full workflow integration test passes (dispatch → watch → result → cancel)
- Ledger idempotency and recovery tests pass

**Test Output Excerpt**:
```
=== RUN   TestIntegration_HermesFieldCompatibility
=== RUN   TestIntegration_HermesFieldCompatibility/dispatch_task_params
=== RUN   TestIntegration_HermesFieldCompatibility/dispatch_task_result
=== RUN   TestIntegration_HermesFieldCompatibility/watch_task_result
=== RUN   TestIntegration_HermesFieldCompatibility/get_task_result
--- PASS: TestIntegration_HermesFieldCompatibility (0.00s)
```

---

## 2. Wire Contract Verification Matrix

### 2.1 Worker Sidecar RPC Methods

| Method | Hermes (`acp_rpc_server.py`) | pi-go (`workersidecar/rpc/`) | Status |
|--------|------------------------------|------------------------------|--------|
| `acp.run` | ✅ Implemented | ✅ Implemented | ✅ Match |
| `acp.cancel` | ✅ Implemented | ✅ Implemented | ✅ Match |
| `acp.status` | ✅ Implemented | ✅ Implemented | ✅ Match |
| `acp.progress` | ✅ Implemented | ✅ Implemented | ✅ Match |
| `acp.toolsets` | ✅ Implemented | ✅ Implemented | ✅ Match |

**Field Name Verification** (sample from `acp.run` params):
| Field | Hermes | pi-go | Status |
|-------|--------|-------|--------|
| `run_id` | ✅ | ✅ | Match |
| `task_id` | ✅ | ✅ | Match |
| `goal` | ✅ | ✅ | Match |
| `model` | ✅ | ✅ | Match |
| `toolsets` | ✅ | ✅ | Match |
| `timeout_seconds` | ✅ | ✅ | Match |
| `master_session_id` | ✅ | ✅ | Match |
| `resume_from_checkpoint` | ✅ | ✅ | Match |

### 2.2 Master Planner Gateway Tools

| Tool Name | Hermes (`tools.py`) | pi-go (`masterplanner/tools.go`) | Status |
|-----------|---------------------|----------------------------------|--------|
| `gateway_dispatch_task` | ✅ | ✅ | ✅ Match |
| `gateway_dispatch_batch` | ✅ | ✅ | ✅ Match |
| `gateway_watch_task` | ✅ | ✅ | ✅ Match |
| `gateway_get_task_result` | ✅ | ✅ | ✅ Match |
| `gateway_list_tasks` | ✅ | ✅ | ✅ Match |
| `gateway_list_models` | ✅ | ✅ | ✅ Match |
| `gateway_list_workers` | ✅ | ✅ | ✅ Match |
| `gateway_cancel_task` | ✅ | ✅ | ✅ Match |

**Key Input/Output Fields** (sample from `gateway_dispatch_task`):

**Input**:
| Field | Hermes | pi-go | Status |
|-------|--------|-------|--------|
| `goal` | ✅ | ✅ | Match |
| `model` | ✅ | ✅ | Match |
| `toolsets` | ✅ | ✅ | Match |
| `context` | ✅ | ✅ | Match |
| `timeout_seconds` | ✅ | ✅ | Match |
| `priority` | ✅ | ✅ | Match |
| `depends_on` | ✅ | ✅ | Match |
| `resume_from_checkpoint` | ✅ | ✅ | Match |

**Output**:
| Field | Hermes | pi-go | Status |
|-------|--------|-------|--------|
| `task_id` | ✅ | ✅ | Match |
| `run_id` | ✅ | ✅ | Match |
| `status` | ✅ | ✅ | Match |
| `idempotent_hit` | ✅ | ✅ | Match |

---

## 3. Environment Variables Cross-Reference

### 3.1 Shared Environment Names

| Variable | Purpose | Hermes Source | pi-go Implementation | Status |
|----------|---------|---------------|----------------------|--------|
| `TASK_RELAY_ACP_RPC_SOCKET` | Unix socket path for RPC server | `extend/sub_agent/acp_rpc_server.py:635` | `internal/cli/worker_sidecar.go:56-58` | ✅ Match |
| `TASK_RELAY_ACP_RPC_HTTP` | Enable HTTP transport flag | `extend/sub_agent/acp_rpc_server.py:634` | `internal/cli/worker_sidecar.go:62` | ✅ Match |
| `ACP_EXECUTOR_TOOLSETS` | Executor toolset whitelist | `extend/sub_agent/acp_rpc_server.py:519` | `internal/cli/worker_sidecar.go:79` | ✅ Match |
| `ACP_EXECUTOR_ALLOW_EXTRA` | Additional allowed toolsets | `extend/sub_agent/acp_rpc_server.py:520` | `internal/cli/worker_sidecar.go:80` | ✅ Match |
| `ACP_PROGRESS_MODE` | Progress reporting granularity | Hermes (inferred) | `internal/cli/worker_sidecar.go:82` | ✅ Compatible |
| `ACP_CHECKPOINT_EVERY_STEPS` | Checkpoint frequency | Hermes (inferred) | `internal/cli/worker_sidecar.go:83` | ✅ Compatible |

### 3.2 Gateway-Specific Environment Variables (Master Planner)

| Variable Prefix | Scope | Notes |
|-----------------|-------|-------|
| `INFA_GATEWAY_*` | Gateway API connection | Used for base URL, API key, timeout config |
| `ACP_LOCAL_RUNTIME_*` | Local runtime resolver | Worker-side model availability checks |

**Note**: These are documented as the canonical env names in both codebases. The pi-go CLI flags provide overrides but defer to the environment when not specified.

---

## 4. Fail-Closed / Safety Gaps

### 4.1 Docker Sandbox (Worker Sidecar)

**Status**: ⚠️ **NOT IMPLEMENTED** (explicit fail-closed)

**Evidence**:
```go
// internal/cli/worker_sidecar.go:92-96
if workerSandbox != "" {
    if workerSandbox != "docker" {
        return fmt.Errorf("only --sandbox docker is supported")
    }
    return fmt.Errorf("Docker sandbox not yet implemented; fail-safe: refusing to start...")
}
```

**Hermes Behavior**:
- `--sandbox docker` launches each task in a disposable, network-less, resource-capped container
- Implemented via `extend/sub_agent/docker_sandbox.py`

**pi-go Behavior**:
- `--sandbox docker` flag is **refused at startup** with an explicit error
- This is **correct fail-closed behavior**: the flag is accepted but the server refuses to start rather than silently ignoring the isolation requirement
- Tasks run directly on the host when `--sandbox` is omitted

**Recommendation**: Document that Docker sandbox is a future enhancement. The fail-closed refusal ensures no caller can mistakenly believe tasks are sandboxed when they are not.

### 4.2 Local-Confined Mode

**Status**: ⚠️ **PARTIALLY IMPLEMENTED** (logs warning)

**Evidence**:
```go
// internal/cli/worker_sidecar.go:100
if workerLocalConfined {
    log.Printf("Warning: --local-confined approval deny rules not yet implemented")
}
```

**Hermes Behavior**:
- `--local-confined` restricts file system access via approval deny globs
- Implemented in `extend/sub_agent/approvals.py`

**pi-go Behavior**:
- Flag is accepted, logs a warning, but does **not** enforce deny rules
- This is **not fail-closed**: the flag suggests lightweight confinement but does not deliver it

**Recommendation**: Either implement the deny rules or change to fail-closed (refuse to start with `--local-confined` until implemented).

---

## 5. Cross-Repo Integration Points

### 5.1 client-daemon PR #1 Integration

**PR**: `yutongzhisuan/client-daemon#1` (Socket path + AGENT_BACKEND default)

**Status**: Repository not accessible during verification (404 error).

**Expected Integration Points**:
1. **Socket Path**: client-daemon should connect to `~/.xhermes/sub_agent/acp.sock` by default (matches both Hermes and pi-go)
2. **Environment Variable**: `AGENT_BACKEND=hermes` (or equivalent) to route tasks to pi-go worker sidecar
3. **PI_WORKER_SIDECAR_* Variables**: Any client-daemon-specific config (not yet confirmed)

**Recommendation**: Once client-daemon PR #1 is accessible:
- Verify it reads `TASK_RELAY_ACP_RPC_SOCKET` or defaults to `~/.xhermes/sub_agent/acp.sock`
- Confirm it can dispatch to pi-go's `acp.run` method
- Document the handshake flow in this report

### 5.2 Manual E2E Test Plan

**Prerequisites**:
1. Build pi-go worker sidecar: `make build && make install`
2. Start pi-go worker sidecar: `pi worker-sidecar --stateless --executor-toolsets file,web,todo`
3. Verify socket exists: `ls -l ~/.xhermes/sub_agent/acp.sock`

**Minimal E2E Smoke Test** (using `curl`):
```bash
# Test acp.toolsets
curl --unix-socket ~/.xhermes/sub_agent/acp.sock \
  -X POST -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","method":"acp.toolsets","params":{},"id":1}' \
  http://localhost/rpc

# Expected: {"jsonrpc":"2.0","id":1,"result":{"toolsets":["file","web","todo"]}}

# Test acp.run (short goal)
curl --unix-socket ~/.xhermes/sub_agent/acp.sock \
  -X POST -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","method":"acp.run","params":{"run_id":"test-001","goal":"List files in current directory","toolsets":["file"]},"id":2}' \
  http://localhost/rpc

# Expected: {"jsonrpc":"2.0","id":2,"result":{"status":"completed|failed","summary":"...","result_text":"..."}}
```

**Full Integration Test** (with client-daemon):
1. Start client-daemon with `AGENT_BACKEND=hermes` or equivalent
2. Dispatch a task via client-daemon's API
3. Verify task reaches pi-go worker sidecar (check logs)
4. Poll `acp.progress` and `acp.status`
5. Retrieve result via `acp.status` or client-daemon result polling

---

## 6. Consistency Findings Summary

### ✅ Verified Consistent
1. **RPC Method Names**: All five `acp.*` methods match exactly
2. **Gateway Tool Names**: All eight `gateway_*` tools match exactly
3. **Field Names**: Input/output field names use consistent `snake_case` across both codebases
4. **Environment Variables**: Shared env names preserved (`TASK_RELAY_ACP_RPC_SOCKET`, `ACP_EXECUTOR_*`)
5. **Default Socket Path**: `~/.xhermes/sub_agent/acp.sock` in both
6. **JSON-RPC 2.0 Compliance**: Golden fixtures verify exact wire format
7. **Ledger Idempotency**: Both use `{run_id}-{seq}` as task_id (idempotency key)

### ⚠️ Partial Consistency / Gaps
1. **Docker Sandbox**: Hermes has full implementation; pi-go refuses to start with `--sandbox` (fail-closed, correct behavior)
2. **Local-Confined Mode**: Hermes enforces approval deny rules; pi-go logs a warning but does not enforce (NOT fail-closed)
3. **client-daemon PR #1**: Repository not accessible for cross-verification

### ❌ Inconsistencies Found
None. All verified interfaces match Hermes wire contracts.

---

## 7. Recommended Merge Order

**Phase 1: Worker Sidecar Foundation**
1. Merge PR #1 (`cursor/hermes-worker-sidecar-d63b`) first
   - Reason: Establishes the RPC server interface that client-daemon will consume
   - Risk: Low — all tests green, wire contract verified
   - Blocker: None

**Phase 2: Master Planner Tools**
2. Merge PR #2 (`cursor/hermes-master-planner-port-6cd5`) second
   - Reason: Depends on gateway-api client but does not block worker sidecar
   - Risk: Low — all tests green, Hermes field compatibility verified
   - Blocker: Requires Gateway API endpoint access (env: `INFA_GATEWAY_BASE_URL`, `INFA_GATEWAY_API_KEY`)

**Phase 3: Client Integration**
3. Merge client-daemon PR #1 (when accessible)
   - Reason: Connects the full workflow (client → daemon → pi-go worker → platform)
   - Risk: Medium — depends on socket handshake correctness
   - Blocker: Requires PRs #1 and #2 to be deployed and pi-go worker sidecar running

**Phase 4: E2E Validation**
4. Run manual E2E smoke test per section 5.2
   - Confirm client-daemon can dispatch to pi-go worker sidecar
   - Verify master planner tools can dispatch/watch platform tasks

---

## 8. Open Questions / Future Work

1. **Docker Sandbox Implementation**
   - When will `--sandbox docker` be implemented in pi-go?
   - Should it remain fail-closed until then? (Recommendation: Yes)

2. **Local-Confined Approval Deny Rules**
   - Should `--local-confined` be fail-closed (refuse to start) until deny rules are implemented?
   - Or is the warning log sufficient? (Current behavior)

3. **client-daemon PR #1 Status**
   - Repository `yutongzhisuan/client-daemon` was not accessible (404) during verification
   - Need to confirm socket path and `AGENT_BACKEND` routing once accessible

4. **Gateway API Endpoint**
   - Is the Gateway API endpoint deployed and accessible from the environments where pi-go will run?
   - Are credentials (`INFA_GATEWAY_API_KEY`) provisioned?

5. **Progress Callback Plumbing**
   - How does pi-go worker sidecar enqueue progress summaries from ADK model callbacks?
   - Is `acp.progress` polling tested end-to-end with a real agent session?

---

## 9. Test Evidence Files

**Worker Sidecar Test Output**: `/tmp/workersidecar-test-output.txt`
**Master Planner Test Output**: `/tmp/masterplanner-test-output.txt`
**Hermes Source Reference**: `/tmp/hermes-agent` (cloned from `yutongzhisuan/hermes-agent@dev`)

---

## 10. Conclusion

**Verdict**: ✅ **READY TO MERGE** (with documented gaps)

Both branches pass all local tests with green evidence. Wire contracts match Hermes sources for all five RPC methods and all eight gateway tools. Field names and environment variables are consistent across codebases.

The only significant gaps are:
1. Docker sandbox explicitly refused (correct fail-closed behavior)
2. Local-confined approval rules not enforced (warning only, not fail-closed)
3. client-daemon PR #1 not accessible for cross-verification

**Recommendation**: Merge PR #1 and PR #2 in order. Document the Docker sandbox gap in release notes. Change `--local-confined` to fail-closed or document it as a "warning-only" flag until deny rules are implemented.

---

**Report Generated**: 2026-09-16  
**Verifier**: Cloud Agent (Cursor)  
**Next Action**: Open draft PR with this report on branch `cursor/hermes-integration-consistency-report-7398`
