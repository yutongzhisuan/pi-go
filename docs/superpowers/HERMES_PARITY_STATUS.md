# Hermes → pi-go parity status (2026-09-16)

Branch: `cursor/docker-file-remoting-parity-f711` (draft PR after merged #9).

## Closed in this PR

### Docker file-tool remoting (worker sidecar)
- Executor children with an active per-run container (`PI_WORKER_DOCKER_*`) route **read / write / edit** (via sandbox I/O), **grep** (`docker exec … rg`), and **find** (`docker exec … find`) through the container filesystem at `/workspace`, aligned with Hermes terminal-env file routing.
- **`PI_WORKER_SANDBOX_DOCKER=1`** fail-closed when the session container id is missing (bash + file tools refuse instead of falling back to host).
- Bind-mounted task workdir remains the source of truth; container-only paths outside `/workspace` are still out of scope.

### `--local-confined` hardline floor
- Portable Hermes **hardline** subset (rm root/home, mkfs, dd→block dev, fork bomb, shutdown/reboot at command position) runs **before** fnmatch deny globs on executor bash.
- Not full Hermes `approvals.deny` (Python command-position deobfuscation for all dangerous patterns).

### Responses / checkpoint depth (L2 partial)
- Extended `responses.v1` parse: `response_id`, echo `model`, `limits.max_result_bytes`, `request` echo.
- Terminal **`result_text`** wrapped as OpenAI Responses `object: "response"` JSON when the envelope was present (Hermes `_wrap_responses` subset).
- **Still irreducible:** no `split_replay_messages` / tool-call item replay, no `model_sessions` restore, no L2 checkpoint blobs in pi-go session store.

## Carried from #9 (unchanged)

- Per-run Docker container + bash via `docker exec`.
- L1 step checkpoints, envelope user-message extraction, local-confined fnmatch deny globs.

## Still deferred

- Hub / swarm-network contract changes.
- `client-daemon` host swap.
- Full Hermes file-tool parity (Modal/SSH backends, container mirror paths, persistent cross-process container).
- Real `delegate_task` child detection (env stub only).

## Verify

```bash
go test ./internal/workersidecar/... ./internal/tools/...
```
