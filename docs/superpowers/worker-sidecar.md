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

| Flag | Default | Description |
|------|---------|-------------|
| `--socket` | From env or Hermes default | Unix socket path |
| `--http` | `false` | Use HTTP instead of Unix socket |
| `--host` | `127.0.0.1` | HTTP host address |
| `--port` | `9105` | HTTP port |
| `--stateless` | `false` | Run in stateless mode |
| `--executor-toolsets` | `file,web,todo` | Toolset whitelist (comma-separated) |
| `--executor-allow-extra` | (empty) | Additional toolsets to allow |
| `--workdir-root` | System temp | Root directory for run workdirs |

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

- No master planner integration
- No client-daemon host swap
- Sandbox configuration may differ
- Progress/checkpoint semantics follow pi-go agent behavior

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
