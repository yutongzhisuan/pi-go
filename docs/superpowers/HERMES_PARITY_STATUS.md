# Hermes → pi-go parity status (2026-09-16)

Branch: `cursor/hermes-parity-gaps-f96f` (single draft PR).

## Closed in this PR

### Master planner
- `gateway_*` tools registered on the main pi agent when `PI_MASTER_PLANNER=1` or `pi --master-planner` (interactive + non-interactive).
- Hermes `planner_prompt` ported as `internal/masterplanner/prompt.go` and used as the system prompt when master planner mode is on (unless `--system` overrides).
- Delegation refusal: `PI_DELEGATED_CHILD=1` returns Hermes-shaped `delegated_child_refused` from all eight gateway tools (stub for future `delegate_task` wiring).

### Worker ACP sidecar
- **Cancel**: `acp.run` executes in a cancellable context; `acp.cancel` cancels context and kills the child `pi` process.
- **Progress**: backend throttling + `EnqueueProgress` wired; `--progress-mode`, `--acp-progress-interval-seconds`, env `ACP_PROGRESS_MODE`.
- **Toolsets**: profile `Resolve` applied before spawn; child `pi` gets `PI_ACP_RESOLVED_TOOLSETS` and tool filtering via `tools.FilterByExecutorToolsets`.
- **Model fail-fast**: `runtime.CheckModel` before session start → `status=failed`, `error_code=model_unavailable`.
- **Docker sandbox**: fail-closed refuse at startup (`internal/workersidecar/sandbox`).
- **`--local-confined`**: fail-closed refuse until bash deny globs are enforced (no warning-only path).
- **Checkpoints / resume**: L1 checkpoints on `--checkpoint-every-steps` / `ACP_CHECKPOINT_EVERY_STEPS`; `resume_from_checkpoint` + `resume_blob` prepended to goal.
- **CLI flags**: `--stateless-toolsets`, `--state-root`, progress/checkpoint flags wired through `internal/workersidecar/options`.
- **Long HTTP runs**: `WriteTimeout` disabled on sidecar HTTP server.

## Still deferred (unchanged scope)

- Hub / swarm-network contract changes.
- `client-daemon` host swap.
- Full Docker container-per-task sandbox (Hermes `apply_sandbox_env` execution path).
- `--local-confined` approval deny enforcement on pi-go bash (requires approvals layer).
- Full Hermes `responses_payload` / `model_sessions` / deep checkpoint L2 resume.
- Real `delegate_task` child detection (env stub only until subagent delegate lands).

## Verify

```bash
go test ./internal/workersidecar/... ./internal/masterplanner/...
```
