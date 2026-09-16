# Worker Sidecar

The Worker ACP sidecar provides JSON-RPC 2.0 compatibility with Hermes Worker agents, allowing existing Hub/task-relay Workers to use pi-go without client code changes.

## Quick Start

### Default (Unix Socket, Stateless)

```bash
pi worker-sidecar --stateless
```

This starts the sidecar on the default Unix socket path (`~/.xhermes/sub_agent/acp.sock`) with stateless mode and the default toolset whitelist (`file`, `web`, `todo`).

### HTTP Mode

```bash
pi worker-sidecar --http --port 9105 --stateless
```

### Custom Toolsets

```bash
pi worker-sidecar --stateless --executor-toolsets file,web,todo
```

## Configuration

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `TASK_RELAY_ACP_RPC_SOCKET` | `~/.xhermes/sub_agent/acp.sock` | Unix socket path |
| `ACP_LOCAL_RUNTIME_BASE_URL` | `http://127.0.0.1:8080/v1` | Local model runtime endpoint |
| `ACP_LOCAL_RUNTIME_API_KEY` | `no-key-required` | Bearer token for local runtime |
| `ACP_ALLOWED_MODELS` | (unset) | Comma-separated allowed model names |

### Command-Line Flags

#### Connection Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--socket` | `~/.xhermes/sub_agent/acp.sock` | Unix socket path (Env: `TASK_RELAY_ACP_RPC_SOCKET`) |
| `--http` | `false` | Use HTTP instead of Unix socket (Env: `TASK_RELAY_ACP_RPC_HTTP=1`) |
| `--host` | `127.0.0.1` | HTTP host address |
| `--port` | `9105` | HTTP port |

#### Execution Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--stateless` | `false` | Run in stateless mode (disposable session, no local state) |
| `--stateless-toolsets` | (empty) | Toolsets for stateless tasks when none requested |
| `--state-root` | (temp) | Directory for ephemeral stateless session store |
| `--workdir-root` | System temp | Parent directory for per-task temp workdirs |

#### Sandbox Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--sandbox` | (none) | Sandbox backend: `docker` (implies `--stateless`) |
| `--sandbox-image` | (default) | Docker image for sandboxed tasks |
| `--sandbox-network` | `false` | Allow container network access |
| `--sandbox-cpu` | (unlimited) | CPU limit for containers (e.g. `2.0`) |
| `--sandbox-memory-mb` | (unlimited) | Memory limit in MB for containers |

#### Confinement Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--local-confined` | `false` | Trusted-task lightweight mode (implies `--stateless`) |
| `--local-confined-extra-deny` | (empty) | Extra deny globs for local-confined mode |

#### Executor Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--executor-toolsets` | `file,web,todo` | Toolset whitelist (Env: `ACP_EXECUTOR_TOOLSETS`) |
| `--executor-allow-extra` | (empty) | Additional toolsets (Env: `ACP_EXECUTOR_ALLOW_EXTRA`) |

#### Progress Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--progress-mode` | `minimal` | Progress granularity: `minimal`, `tools`, `off` (Env: `ACP_PROGRESS_MODE`) |
| `--checkpoint-every-steps` | `0` | Checkpoint every N steps (Env: `ACP_CHECKPOINT_EVERY_STEPS`) |
| `--acp-progress-interval-seconds` | `5.0` | Min seconds between progress frames |

## Methods

The sidecar implements five JSON-RPC 2.0 methods with exact Hermes wire compatibility:

### `acp.run`

Execute a task with a pi-go agent session.

**Request params:**
- `run_id` (required): Unique run identifier
- `task_id`: Task identifier (defaults to `run_id`)
- `goal`: Task goal text
- `toolsets`: Requested toolset list
- `timeout_seconds`: Timeout in seconds (default 600)
- `model`: Model name (must be available in local runtime)
- Additional Hermes-compatible fields

**Response:**
- `status`: Terminal status (`completed`, `failed`, `cancelled`)
- `summary`: Short summary
- `result_text`: Main result text
- `usage`: Token usage stats
- `error`: Error message if failed
- `error_code`: Structured error code (e.g. `model_unavailable`)

### `acp.cancel`

Cancel a running task.

**Request params:**
- `run_id`: Run to cancel
- `reason`: Optional cancellation reason

**Response:**
- `cancelled`: `true` if cancelled, `false` if not found

### `acp.status`

Check if a run is active.

**Request params:**
- `run_id`: Run to check

**Response:**
- `running`: `true` if active

### `acp.progress`

Drain queued progress summaries (FIFO, cleared after read).

**Request params:**
- `run_id`: Run to poll

**Response:**
- `summaries`: Array of progress strings

### `acp.toolsets`

Announce available toolsets.

**Response:**
- `toolsets`: Array of available toolset names
- `detail`: Optional detail message

## Toolset Mapping

The sidecar maps Hermes toolset names to pi-go tools:

| Toolset | pi-go Tools |
|---------|-------------|
| `file` | read, write, edit, grep, find |
| `web` | webfetch, websearch |
| `todo` | task, todo equivalents |

Toolsets not in the allowed list are unavailable. Local-state toolsets (`memory`, `skills`, `session_search`, etc.) are stripped in stateless mode even if explicitly allowed.

## Security Defaults

For untrusted remote tasks:
- Use `--stateless` to ensure disposable workdirs
- Default whitelist: `file,web,todo` (no shell/browser)
- Docker sandbox support (if configured)

Shell-class toolsets excluded by default:
- `terminal`, `code_execution`, `browser`, `computer_use`, `delegation`, `shell`, `bash`

## Model Binding

When a task specifies a `model`:
1. Check allowed list (if `ACP_ALLOWED_MODELS` is set)
2. Query local runtime `/models` endpoint
3. On failure, return `status=failed` with `error_code=model_unavailable`
4. Never fall back to cloud credentials for bound tasks

## Examples

### Basic stateless Worker

```bash
export TASK_RELAY_ACP_RPC_SOCKET=/var/run/acp-worker.sock
pi worker-sidecar --stateless
```

### Custom toolsets and local model

```bash
export ACP_LOCAL_RUNTIME_BASE_URL=http://localhost:11434/v1
export ACP_ALLOWED_MODELS=llama3,codellama
pi worker-sidecar --stateless --executor-toolsets file,web
```

### HTTP with all allowed toolsets

```bash
pi worker-sidecar --http --port 9105 --executor-toolsets file,web,todo,shell --executor-allow-extra memory
```

Note: `shell` will still be excluded by default unless explicitly added; `memory` will be stripped in stateless mode.

## Wire Protocol

Transport: JSON-RPC 2.0 over HTTP POST (body)

Endpoints:
- Unix: Socket file
- HTTP: POST `/rpc` or POST `/`

Socket permissions: `0600` (owner read/write only)

## Differences from Hermes

- Master planner runs on the main `pi` agent (`--master-planner` / `PI_MASTER_PLANNER=1`), not in this subcommand.
- No client-daemon host swap.
- **Docker sandbox**: bash commands run in disposable `docker run --rm` containers; file tools still use the host filesystem sandbox (see parity doc).
- **`--local-confined`**: deny globs apply to the bash tool only (not Hermes full approval stack / deobfuscation).
- See `docs/superpowers/HERMES_PARITY_STATUS.md` for gaps and verify commands.

## Troubleshooting

### "duplicate run_id" error

A run with that ID is already active. Cancel it or use a unique ID.

### "model_unavailable" error

The requested model is not available in the local runtime. Check:
- Local runtime is running
- Model name matches available models
- `ACP_ALLOWED_MODELS` allows it (if set)

### Toolset not available

Check:
- Toolset is in allowed list (`--executor-toolsets`)
- Not a local-state toolset in stateless mode
- Toolset name is correct (see mapping table)

### Docker sandbox unavailable

If startup fails with `docker sandbox unavailable`, the Docker CLI is missing or the daemon is not reachable. The sidecar **does not** fall back to unsandboxed execution when `--sandbox docker` is set. Install/start Docker or omit `--sandbox`.

### Local-confined deny hit

When a bash command matches a deny glob, the tool returns `BLOCKED: command matches deny rule ...`. This is guardrail mode for trusted internal tasks, not a security boundary — use `--sandbox docker` for untrusted work.

## Limitations

- **Docker sandbox**: bash-only; per-invocation containers; see `HERMES_PARITY_STATUS.md`.
- **Local-confined**: bash-only deny matching (subset of Hermes `approvals.deny`).
- **Checkpoint L2**: goal prepend + L1 step checkpoints; no full model-session replay.
- **Master planner**: runs on main `pi`, not this subcommand.

Use `--stateless` with the default toolset whitelist for secure remote task execution without shell tools.

## Testing & Validation

### Integration Smoke Tests

Run the included integration tests to verify the full RPC flow:

```bash
# Run integration tests (includes HTTP and Unix socket)
cd internal/workersidecar/rpc
go test -v -run TestIntegrationSmoke

# Run all tests including golden fixtures
go test -v ./...
```

The integration tests validate:
- ✅ `acp.toolsets` → returns allowed toolset list
- ✅ `acp.run` → executes with Hermes-shaped result
- ✅ `acp.progress` → drains queued summaries
- ✅ `acp.status` → reports run state
- ✅ `acp.cancel` → cancels active run
- ✅ Duplicate `run_id` → returns proper error
- ✅ Golden fixture field compliance

### Manual E2E with Real Worker

To test with an actual `task-relay` Worker (`acp-remote` client):

#### 1. Start the sidecar

```bash
# Unix socket (default path for drop-in compatibility)
pi worker-sidecar --stateless

# Or HTTP for easier debugging
pi worker-sidecar --http --port 9105 --stateless
```

#### 2. Configure Worker to point at the socket

For Unix socket mode (recommended):
```bash
export TASK_RELAY_ACP_RPC_SOCKET=~/.xhermes/sub_agent/acp.sock
```

For HTTP mode:
```bash
export TASK_RELAY_ACP_RPC_HTTP=1
export TASK_RELAY_ACP_RPC_HOST=127.0.0.1
export TASK_RELAY_ACP_RPC_PORT=9105
```

#### 3. Run Worker with test task

From the Worker (`acp-remote`) side:

```python
# Example: task-relay Worker test
import asyncio
from acp_remote import RemoteAcpBackend

async def test_sidecar():
    backend = RemoteAcpBackend(
        socket_path="~/.xhermes/sub_agent/acp.sock"  # or HTTP endpoint
    )
    
    # Test toolsets announcement
    toolsets = await backend.rpc_call("acp.toolsets", {})
    print(f"Available toolsets: {toolsets['toolsets']}")
    
    # Run a simple task
    result = await backend.rpc_call("acp.run", {
        "run_id": "test-123",
        "goal": "List files in the current directory",
        "toolsets": ["file"],
        "timeout_seconds": 60
    })
    
    print(f"Status: {result['status']}")
    print(f"Summary: {result['summary']}")
    print(f"Result: {result['result_text']}")
    
asyncio.run(test_sidecar())
```

#### 4. Verify wire compatibility checklist

- [ ] `acp.toolsets` returns array matching `--executor-toolsets`
- [ ] `acp.run` returns with `status`, `summary`, `result_text`, `usage` fields
- [ ] `acp.run` with invalid model returns `error_code: model_unavailable`
- [ ] `acp.progress` drains summaries (poll during long run)
- [ ] `acp.status` reports `running: true` for active run
- [ ] `acp.cancel` stops active run and returns `cancelled: true`
- [ ] Duplicate `run_id` returns JSON-RPC error (code -32000)
- [ ] All field names match Hermes exactly (no camelCase drift)

#### 5. Test with Hub integration (optional)

If you have access to a Hub instance:

1. Configure node to use this sidecar
2. Submit task through Hub → Worker → sidecar chain
3. Verify task completes and results propagate back
4. Check progress updates arrive at Hub
5. Test cancel from Hub UI

### Debugging

Enable verbose logging:
```bash
# Start with logging
pi worker-sidecar --stateless 2>&1 | tee sidecar.log

# In another terminal, monitor socket
ss -lx | grep acp.sock  # Unix socket
netstat -tlnp | grep 9105  # HTTP mode
```

Check RPC calls:
```bash
# Unix socket test
echo '{"jsonrpc":"2.0","method":"acp.toolsets","id":1}' | \
  socat - UNIX-CONNECT:$HOME/.xhermes/sub_agent/acp.sock

# HTTP test  
curl -X POST http://127.0.0.1:9105/rpc \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","method":"acp.toolsets","id":1}'
```

Expected response:
```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "result": {
    "toolsets": ["file", "web", "todo"]
  }
}
```
