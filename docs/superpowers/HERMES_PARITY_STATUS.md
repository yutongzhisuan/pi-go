# Hermes → pi-go parity status (2026-09-16)

Branch: `cursor/worker-sidecar-parity-gaps-a664` (PR #9).

## Closed in this PR (worker-sidecar gaps after #8)

### Docker sandbox (partial Hermes parity)
- `--sandbox docker` **fail-closed** when Docker/Podman CLI or daemon is unavailable (`CheckDockerAvailable` on Linux).
- Per **run** reusable container (`docker run -d` + `sleep infinity`), workdir bind-mounted at `/workspace`, destroyed on run end/cancel.
- Executor child **bash** runs via `docker exec` into that container (`PI_WORKER_DOCKER_*` env), not per-invocation `docker run --rm` when the session container is active.
- Legacy `TERMINAL_*` / `docker run --rm` path remains in `internal/tools/terminal_docker.go` for non-session callers only.
- **Remaining gap:** file/code tools use host `os.Root` on the bind-mounted workdir (same task files as `/workspace`; not Hermes terminal-env file routing).

### `--local-confined`
- Sidecar starts with `--local-confined`; Hermes `DEFAULT_LOCAL_DENY_RULES` (+ extras) via `PI_ACP_DENY_RULES`.
- Bash deny **before** execution; normalized/fnmatch variants (NFKC, line continuations, backslash/empty-quote/IFS, segment split).
- Not full Hermes `approvals.deny` (command-position deobfuscation, hardline floor).

### Checkpoint / resume (L1 + envelope)
- L1: step checkpoints (empty `resume_blob`), terminal salvage, `resume_from_checkpoint` / `resume_blob` / `params.resume_summary`.
- `params.responses.v1` envelope → user message; malformed → `invalid_responses_payload`.
- **L2 gaps:** no Responses object as `result_text`, no `split_replay_messages`, no `model_sessions` restore (Hermes M1 deferral).

## Still deferred

- Hub / swarm-network contract changes.
- `client-daemon` host swap.
- Full Hermes file-tool remoting inside Docker.
- Real `delegate_task` child detection (env stub only).

## Verify

```bash
go test ./internal/workersidecar/... ./internal/tools/...
```
