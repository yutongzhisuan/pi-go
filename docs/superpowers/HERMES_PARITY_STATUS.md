# Hermes → pi-go parity status (2026-09-16)

Branch: `cursor/acp-result-text-parity-3ed7` (draft PR on `main` after squash-merge #10 / `6b1f546`).

## Closed in this PR

### ACP `result_text` / `summary` from assistant output
- `collectResults` no longer returns on `message_end` before `proc.Wait()`; it merges streamed deltas with the child pi `--mode json` accumulated stdout.
- Completed runs set `result_text` and `summary` from assistant text when present. Missing assistant text after a clean child exit yields `failed` / `empty_assistant_output` (not a duration-only `Completed in …` success shape).
- `responses.v1` wrap still emits OpenAI Responses JSON, but `output_text` is assistant text (not a duration fallback).

### Hardline deny depth (portable subset)
- Extended Hermes `HARDLINE_PATTERNS` subset: protected `rm` targets (`/etc`, `/usr`, …), redirect-to-block-device, `kill -1`, `init 0/6`, `systemctl poweroff|reboot|halt|kexec`, `telinit 0/6`.
- Command-position anchor (`_CMDPOS`) now includes `;`, `|`, and `&` separators (Hermes-aligned).
- Still **not** full Python `approvals.deny` deobfuscation or the full `DANGEROUS_PATTERNS` ask/yolo layer.

## Carried from #10 (unchanged)

- Docker file-tool remoting (`read` / `write` / `edit` / `grep` / `find`) via per-run container + bind-mounted workdir.
- L1 step checkpoints, envelope user-message extraction, local-confined fnmatch deny globs.
- Terminal `result_text` Responses JSON wrap when `responses.v1` envelope is present.

## Still deferred

| Item | Why irreducible in-repo |
|------|-------------------------|
| Hub / swarm-network contract changes | Out of scope (wire owned by platform). |
| `client-daemon` host swap | Out of scope (separate binary). |
| Modal / SSH backends, cross-process persistent containers | Product scope; not a sidecar-only patch. |
| Full Hermes `approvals.deny` / `DANGEROUS_PATTERNS` | Python-specific deobfuscation and approval UX; pi-go ships hardline + fnmatch deny only. |
| L2 `model_sessions` / tool-item replay | pi-go session store has no Hermes `split_replay_messages` / checkpoint blob model; faking replay would break resume semantics. L1 goal/blob/summary resume remains. |
| Real `delegate_task` child detection | pi-go has no `delegate_task` spawn path yet; `PI_DELEGATED_CHILD` is wired for master-planner refusal but nothing sets it on children until delegate_task lands. |

## Verify

```bash
go test ./internal/workersidecar/... ./internal/tools/...
```
