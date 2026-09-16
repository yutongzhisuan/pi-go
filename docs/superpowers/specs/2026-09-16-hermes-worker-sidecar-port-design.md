# Design: Hermes Worker Sidecar Port into pi-go

Date: 2026-09-16  
Repo: `yutongzhisuan/pi-go` (fork of `dimetron/pi-go`)  
Source of truth for wire contract: `yutongzhisuan/hermes-agent` `dev` → `extend/sub_agent`  
Status: Approved in design review (architecture, components/wire, flow/errors/acceptance)

## 1. Goal

Port the Hermes **Worker ACP sidecar** into pi-go with **exact JSON-RPC wire parity**, so existing Hub / task-relay Workers keep working without client changes. Execution uses pi-go’s agent runtime (tools, optional sandbox, local model binding) instead of Hermes Python ACP sessions.

This is **phase 1** of a larger Hermes→pi-go migration. Master planner and `client-daemon` host swap are out of scope for this spec.

## 2. Non-goals (explicit)

- Master planner (`extend/master_planner` / `gateway_*` tools)
- Changing `swarm-network` Hub/Worker wire contracts
- Changing `client-daemon` (compute-node host that today launches `hermes serve` and ticket-proxies gateway JSON-RPC)
- Replacing or wrapping the official ACP stdio server (`pi acp-server` / `internal/acp/server`) as this sidecar
- Cloud provider credential fallback when a task-bound model is unavailable
- Offline pack / Windows CI / AOS packaging work

## 3. Context: what client-daemon is (and is not)

`yutongzhisuan/client-daemon` is a Go compute-node daemon (Gin local REST, PCN, runtimes, tasks). Per `docs/hermes-agent-integration.md`, it hosts Hermes as a child process and proxies Hermes **gateway** JSON-RPC over a ticketed WebSocket to the desktop UI.

It does **not** implement the Worker sidecar methods `acp.run` / `acp.cancel` / `acp.status` / `acp.progress` / `acp.toolsets`. Those live in Hermes `extend/sub_agent/acp_rpc_server.py` and are consumed by task-relay Workers (`acp-remote`). After this sidecar is stable in pi-go, a later change can point the node host at pi-go; that is a separate project.

## 4. Architecture

```text
Master / Hub
  → task-relay Worker (existing acp-remote client)
      → Unix domain socket (default) or HTTP loopback JSON-RPC
          → pi-go Worker sidecar (new)
              → disposable / constrained pi-go agent session
                 (tools + optional Docker sandbox + optional local model)
```

### Boundaries

1. **Wire surface** must match Hermes `acp_rpc_server` method names, params, and result shapes. Worker zero-change is a success criterion.
2. **Execution kernel** is in-process pi-go agent/runtime. Do not pretend official ACP-over-stdio is this sidecar.
3. **Security defaults** for untrusted remote tasks: `--stateless` + executor whitelist `file,web,todo` (no shell/browser). Docker sandbox and `--executor-allow-extra` are operator opt-in.
4. **Model binding**: if `acp.run` carries `model`, resolve against local OpenAI-compatible runtime; on failure return `failed` + `error_code=model_unavailable` with no silent model substitution.

## 5. Package layout

| Package / entry | Responsibility |
|-----------------|----------------|
| `internal/workersidecar/rpc` | JSON-RPC 2.0 transport (Unix socket / HTTP), five methods, `runs` + `progress` buffers |
| `internal/workersidecar/backend` | Map run params → pi-go session; progress/checkpoint/cancel; Hermes-shaped results |
| `internal/workersidecar/profile` | Executor whitelist, intersection with requested toolsets, announce list for `acp.toolsets` |
| `internal/workersidecar/runtime` | Local model allowlist + `/models` probe for task-bound models |
| CLI `pi worker-sidecar` (preferred) or `cmd/pi-worker-sidecar` | Flags/env parity with Hermes sidecar |

Do **not** place this under `internal/acp/server` (that package is the IDE ACP agent surface).

## 6. Wire protocol

Transport: JSON-RPC 2.0 over HTTP POST body (Hermes uses aiohttp `UnixSite` / TCPSite with POST `/rpc` and `/`). Default listen: Unix socket. Optional `--http` on loopback.

### Socket path compatibility

- Env: `TASK_RELAY_ACP_RPC_SOCKET` (same as Hermes)
- Hermes default path: `~/.xhermes/sub_agent/acp.sock`
- pi-go may document an alternate default under `~/.pi-go/...`, but **must** honor `TASK_RELAY_ACP_RPC_SOCKET` and should default to the Hermes path when unset so existing Workers connect without reconfiguration
- HTTP mode: env `TASK_RELAY_ACP_RPC_HTTP=1` and `--http` / `--host` / `--port` (Hermes defaults `127.0.0.1:9105`)

Socket file mode: owner read/write only (`0600`); parent dir created `0700`; stale socket unlinked before bind.

### Methods

#### `acp.run`

**Params (field names fixed):**

| Field | Notes |
|-------|--------|
| `run_id` | Required; must not collide with an active run |
| `task_id` | Default to `run_id` if empty |
| `attempt` | Default `1` |
| `goal` | Task goal text |
| `params` | Object or empty |
| `context` | Object or null |
| `toolsets` | String list (requested) |
| `timeout_seconds` | Default `600` |
| `first_progress_seconds` | Optional |
| `trace_context` | Optional |
| `resume_from_checkpoint` | Optional |
| `resume_blob` | Optional |
| `model` | Optional; if set, local runtime binding required |
| `master_session_id` | Optional |

**Result (field names fixed):**

| Field | Notes |
|-------|--------|
| `status` | Terminal status string (same vocabulary as Hermes backend) |
| `summary` | Short summary |
| `result_text` | Main result text |
| `fields` | Object |
| `usage` | Usage object as produced by backend |
| `error` | Error message when failed/cancelled |
| `error_code` | Optional structured code (e.g. `model_unavailable`) |
| `checkpoint` | Optional last checkpoint `{checkpoint_id, summary, fields, resume_blob}` |

Behavior notes matching Hermes:

- Create progress bucket for `run_id` before execute
- `acp.run` awaits completion (synchronous RPC from Worker’s perspective)
- Remove from `runs` in `finally`; progress buffer may still be drained once after return

#### `acp.cancel`

Params: `run_id`, optional `reason`.  
Result: `{ "cancelled": true }` if active; `{ "cancelled": false, "reason": "run not found" }` otherwise. Sets cancel event on the active run.

#### `acp.status`

Params: `run_id`.  
Result: `{ "running": false }` if unknown; `{ "running": <not done> }` if active.

#### `acp.progress`

Params: `run_id`.  
Result: `{ "summaries": [ ... ] }` — **drain** queued summaries (clear after read). If run is gone and buffer empty after drain, drop the bucket.

#### `acp.toolsets`

Params: none (or ignored).  
Result: `{ "toolsets": [ ... ] }` from executor profile announce list; if no profile, `{ "toolsets": null, "detail": "..." }` matching Hermes shape.

### JSON-RPC errors

| Condition | Code |
|-----------|------|
| Parse error | `-32700` |
| Method not found | `-32601` |
| Uncaught handler error | `-32000` + message |

Business failures (timeout, cancel, model unavailable, tool/sandbox failure) are returned inside the **`acp.run` result**, not as JSON-RPC errors, except for invalid params that Hermes raises as handler exceptions (e.g. missing `run_id`, duplicate `run_id`) which surface as `-32000`.

## 7. Components

### 7.1 Executor profile (`profile`)

- Default whitelist: `file`, `web`, `todo`
- Shell-class toolsets excluded by default: `terminal`, `code_execution`, `browser`, `computer_use`, `delegation`, and other Hermes `SHELL_CLASS_TOOLSETS` equivalents
- Resolve: `intersection(requested, allowed)`; if empty → empty tool list (no widening)
- CLI/env: `--executor-toolsets` / `ACP_EXECUTOR_TOOLSETS` (replace); `--executor-allow-extra` / `ACP_EXECUTOR_ALLOW_EXTRA` (add)
- Stateless layer: always strip local-state toolsets (memory, skills, session_search, cron, messaging, etc.) even if operator added them to the whitelist
- Stateful (no `--stateless`) path: do not apply executor profile (Hermes behavior for local trusted use)

### 7.2 Toolset → pi-go tool mapping

Announce names stay Hermes toolset names. Implementation maps to pi-go built-ins:

| Toolset | pi-go tools (initial mapping) |
|---------|--------------------------------|
| `file` | read / write / edit / grep / find (and equivalents already in pi-go) |
| `web` | webfetch / websearch |
| `todo` | task / todo (pi-go equivalents) |

If a mapped tool is missing in pi-go, that toolset is unavailable and must be logged; **never** silently expand to shell/browser to compensate. Mapping table is part of the acceptance docs and may be extended only by explicit whitelist additions.

### 7.3 Backend session (`backend`)

- Build disposable workdir / session when `--stateless` (or sandbox / local-confined implying stateless)
- Attach tools after profile resolve
- Optional Docker sandbox: per-task disposable container; default no network; CPU/memory flags parity with Hermes where practical
- Progress callback appends non-empty summaries to the run’s progress queue (respect progress interval / mode flags)
- Checkpoint callback stores last checkpoint for optional result field
- Cancel event aborts the session cooperatively
- Cleanup transcript/workdir on stateless end

Exact pi-go APIs used to spawn a headless one-shot agent session are an implementation detail for the plan; the design requires a clear adapter boundary so RPC/tests can fake the backend.

### 7.4 Local runtime (`runtime`)

Env (Hermes-compatible names):

| Env | Default | Meaning |
|-----|---------|---------|
| `ACP_LOCAL_RUNTIME_BASE_URL` | `http://127.0.0.1:8080/v1` | OpenAI-compatible base |
| `ACP_LOCAL_RUNTIME_API_KEY` | `no-key-required` | Bearer for local runtime |
| `ACP_ALLOWED_MODELS` | unset | Optional comma allowlist |

When `model` is set: check allowlist (if set) → `GET {base}/models` → on miss or unreachable, fail fast with `error_code=model_unavailable`. On hit, bind session to that local endpoint (custom/OpenAI-compatible provider), never fall back to cloud credentials for bound tasks. When `model` is absent, use sidecar default model/provider unchanged.

### 7.5 CLI flags (parity set)

Must support (names aligned with Hermes):

- `--http`, `--socket`, `--host`, `--port`
- `--stateless`, `--stateless-toolsets`, `--state-root`, `--workdir-root`
- `--sandbox docker`, `--sandbox-image`, `--sandbox-network`, `--sandbox-cpu`, `--sandbox-memory-mb`
- `--local-confined`, `--local-confined-extra-deny`
- `--executor-toolsets`, `--executor-allow-extra`
- `--progress-mode`, `--checkpoint-every-steps`, `--acp-progress-interval-seconds`

`--sandbox` and `--local-confined` imply `--stateless`.

## 8. Execution flow (`acp.run`)

1. Validate `run_id`; reject duplicate active id.
2. Allocate progress bucket.
3. Resolve toolsets via profile (and stateless filters when applicable).
4. If `model` present, probe local runtime; on failure return result with `status=failed`, `error_code=model_unavailable` without starting a session.
5. Create session/workdir; apply sandbox if configured.
6. Run agent; enqueue progress; capture checkpoints; honor cancel.
7. Build Hermes-shaped result; remove from `runs`; leave progress drainable once more.

## 9. Testing and acceptance

### Unit / integration tests

- JSON-RPC: five methods, parse error, method not found
- Duplicate `run_id`, missing `run_id`
- Progress drain during run and once after completion; bucket cleanup
- Cancel: `acp.cancel` then terminal run result reflects cancel
- Profile intersection; empty intersection → empty tools; no widen
- Stateless stripping of local-state toolsets
- `model_unavailable` path without session start
- Golden fixtures: request/response JSON compared to Hermes samples (checked into `internal/workersidecar/testdata/`)

### System acceptance

- Existing task-relay Worker (`acp-remote`) completes one run → progress polls → terminal result **without Worker code changes**, pointing at this sidecar socket
- Documented start command + env table + toolset mapping table in repo docs (this spec plus a short operator README under `docs/` or package comment)

### Done definition (phase 1)

All of the above green; Master planner and client-daemon host swap still deferred.

## 10. Delivery order (broader migration)

1. **This spec** — Worker sidecar in `yutongzhisuan/pi-go`
2. Master planner port (separate design)
3. Optional: `client-daemon` host swap from Hermes gateway to pi-go process (separate design)

## 11. Risks and open implementation notes (resolved policy)

| Topic | Decision |
|-------|----------|
| Default socket path | Prefer Hermes path when env unset for drop-in; document any pi-go-specific alternate |
| Status string vocabulary | Match Hermes backend terminal statuses used by Hub today; capture in golden fixtures from Hermes |
| Responses / `@responses` progress semantics | Port progress summary text behavior needed for Worker polling; full OpenAI Responses replay is in-scope only insofar as Hermes sidecar exposes it through progress/result fields today |
| pi-go sandbox maturity | If Docker sandbox hooks are incomplete, ship `--stateless` + profile first; sandbox flag must either work or fail closed with a clear error (no silent disable) |

## 12. References

- Hermes: `extend/sub_agent/acp_rpc_server.py`, `acp_backend.py`, `executor_profile.py`, `local_runtime.py`, `README.md` on `yutongzhisuan/hermes-agent` `dev`
- pi-go: existing `internal/acp/server` (IDE ACP — do not overload), agent/tools/subagent packages
- client-daemon: `docs/hermes-agent-integration.md` (host role only)
