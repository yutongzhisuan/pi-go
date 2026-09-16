# Hermes → pi-go parity status (2026-09-16)

Branch: `cursor/hermes-sidecar-parity-da3e` (draft PR).

## Closed in this PR (Worker ACP sidecar)

### Docker sandbox (`--sandbox docker`)
- Sidecar applies Hermes-equivalent `TERMINAL_*` env via `apply_sandbox_env` parity (`internal/workersidecar/sandbox`).
- Startup **fail-closed** when the Docker CLI/daemon is unavailable; succeeds when Docker works on Linux.
- Executor children inherit sandbox env; **bash** runs through `docker run --rm` with network-off-by-default, CPU/memory limits, cap-drop, and task workdir bind-mounted to `/workspace`.
- Tests: env wiring, arg assembly, refuse-when-unavailable (skip-if-no-docker integration probe).

### `--local-confined`
- Default + extra deny globs merged (`DEFAULT_LOCAL_DENY_RULES` parity).
- Deny globs enforced on **bash** via `PI_ACP_DENY_RULES` (fnmatch-style, case-insensitive).
- Sidecar starts with `--local-confined` (no longer fail-closed refuse-only).

### Checkpoints / resume
- Mid-run checkpoints emit `checkpoint_id`, `summary`, `fields.step`, empty `resume_blob` (Hermes L1 step milestones).
- Terminal cancel/fail **salvage** fills `resume_blob` from partial `result_text` when useful for Hub uplink.
- `resume_from_checkpoint` + `resume_blob` still prepended into the child goal (Hermes acp_backend behavior).

## Known gaps (documented, tested subset only)

| Area | Hermes | pi-go (this PR) |
|------|--------|-----------------|
| Docker scope | Terminal + file/code tools share container env | **Bash only**; file tools remain host `os.Root` sandbox |
| Deny scope | `approvals.deny` before all approval bypasses | **Bash only**; no Hermes command deobfuscation variants |
| Container lifecycle | Per-session reusable container | **Per bash invocation** `docker run --rm` (disposable, simpler) |
| Checkpoint L2 | `responses_payload` / model session replay | Not ported (out of scope) |

## Unchanged / still deferred

- Hub / swarm-network contract changes.
- `client-daemon` host swap.
- Master planner (`PI_MASTER_PLANNER`) — separate PR track.
- Full Hermes `responses_payload` / `model_sessions` deep resume.

## Verify

```bash
go test ./internal/workersidecar/...
```
