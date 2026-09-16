# Hermes → pi-go parity status (2026-09-16)

Branch: `cursor/responses-l2-replay-sse-f9e1` (PR1 — Responses L2 replay + item SSE).

Design: [`docs/superpowers/specs/2026-09-16-hermes-l2-delegate-parity-design.md`](specs/2026-09-16-hermes-l2-delegate-parity-design.md) §1.

## Closed in PR1 (Responses L2)

### Structured input replay (`split_replay_messages` subset)
- `responses.v1` parsing retains raw `request.input` items and applies `SplitReplayMessages`: trailing user → replay history + current user message; otherwise legacy flatten.
- Executor children receive `PI_ACP_REPLAY_HISTORY` (JSON history items) and a prompt built from the last user turn only; `applyACPReplayHistory` injects formatted prior turns before the current task in `pi --mode json`.
- **Not** cross-checkpoint model-session hot restore — checkpoint resume stays L1 (`resume_blob` / `resume_summary` in goal).

### Item-level SSE (`turn_output_items` / `response.output_item.added`)
- On the Responses envelope path, the sidecar emits `response.output_item.added` events (assistant message + function_call items) via `ResponseEventCallback` → `acp.progress` drain field `response_events` (additive JSON; summaries unchanged).

## Carried from prior worker-sidecar parity (#10–#12)

- ACP `result_text` / empty-assistant fail-closed, Docker file-tool remoting, L1 step checkpoints, local-confined deny globs, terminal Responses JSON wrap.

## Still deferred

| Item | Why irreducible in-repo |
|------|-------------------------|
| Hub / swarm-network contract changes | Out of scope (wire owned by platform). `response_events` on `acp.progress` is the minimal sidecar extension for item JSON. |
| `client-daemon` host swap | Out of scope (separate binary). |
| Modal / SSH backends | Product scope. |
| Full Hermes `approvals.deny` / `DANGEROUS_PATTERNS` | Python-specific deobfuscation layer. |
| L2 **model session** hot restore across checkpoints | Explicit non-goal; L1 blob/summary resume only. |
| Real `delegate_task` spawn (PR2) | `PI_DELEGATED_CHILD` refusal exists; spawn path lands in PR2. |
| PR3 delegate enhancements | list / steer / interrupt / orchestrator depth — after PR2. |

## Verify

```bash
go test ./internal/workersidecar/...
```
