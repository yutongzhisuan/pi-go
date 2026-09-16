# Hermes → pi-go parity status (2026-09-16)

Design: [`docs/superpowers/specs/2026-09-16-hermes-l2-delegate-parity-design.md`](specs/2026-09-16-hermes-l2-delegate-parity-design.md).

## Closed in PR1 (Responses L2)

### Structured input replay (`split_replay_messages` subset)
- `responses.v1` parsing retains raw `request.input` items and applies `SplitReplayMessages`: trailing user → replay history + current user message; otherwise legacy flatten.
- Executor children receive `PI_ACP_REPLAY_HISTORY` (JSON history items) and a prompt built from the last user turn only; `applyACPReplayHistory` injects formatted prior turns before the current task in `pi --mode json`.
- **Not** cross-checkpoint model-session hot restore — checkpoint resume stays L1 (`resume_blob` / `resume_summary` in goal).

### Item-level SSE (`turn_output_items` / `response.output_item.added`)
- On the Responses envelope path, the sidecar emits `response.output_item.added` events (assistant message + function_call items) via `ResponseEventCallback` → `acp.progress` drain field `response_events` (additive JSON; summaries unchanged).

## Closed in PR2 (`delegate_task` core)

### Hermes-compatible `delegate_task` on Orchestrator
- Tool **`delegate_task`** coexists with **`subagent`**; inputs `goal` or `tasks[]` (+ optional `context`).
- **`max_concurrent_children`**: default **3**, over-limit returns a **tool error** (`PI_DELEGATE_MAX_CONCURRENT` override).
- **`max_spawn_depth=1`**: nested `delegate_task` in a child is refused; children get `PI_DELEGATED_CHILD=1`.
- **Isolation**: child toolset = parent minus blocklist (`delegate_task`, `clarify`, `gateway_*`, Hermes equivalents including pi-go memory tools); fresh child session (goal/context prompt only); parent receives aggregated summary text, not child tool traces.
- Master planner **`gateway_*`** refusal under `PI_DELEGATED_CHILD` (existing) applies to spawned children.

## Carried from prior worker-sidecar parity (#10–#12)

- ACP `result_text` / empty-assistant fail-closed, Docker file-tool remoting, L1 step checkpoints, local-confined deny globs, terminal Responses JSON wrap.

## Still deferred

| Item | Why irreducible in-repo |
|------|-------------------------|
| Hub / swarm-network contract changes | Out of scope (wire owned by platform). |
| `client-daemon` host swap | Out of scope (separate binary). |
| Modal / SSH backends | Product scope. |
| Full Hermes `approvals.deny` / `DANGEROUS_PATTERNS` | Python-specific deobfuscation layer. |
| L2 **model session** hot restore across checkpoints | Explicit non-goal; L1 blob/summary resume only. |
| PR3 delegate enhancements | `role=orchestrator`, depth 2–3, `action=list\|steer\|interrupt`, config knobs — after PR2. |

## Verify

```bash
go test ./internal/tools/... ./internal/masterplanner/...
```
