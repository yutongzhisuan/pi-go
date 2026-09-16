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

## Closed in PR3 (Hermes delegate enhancements)

### Role, depth, and management actions
- **`role`**: `leaf` (default) / `orchestrator`. When `max_spawn_depth` is **1**, orchestrator requests **degrade to leaf** with a session-visible notice hook.
- **`max_spawn_depth` 2–3** (config `delegation.max_spawn_depth`, env `PI_DELEGATE_MAX_SPAWN_DEPTH`): orchestrator children (`PI_DELEGATE_ROLE=orchestrator`) may call `delegate_task` to spawn **leaf** grandchildren; cost scales with pool concurrency × depth.
- **`action=list`**: in-process active children (`id`, truncated `goal`, `status`) via Orchestrator tracking.
- **`action=steer`**: queues guidance on the Orchestrator (`agent_id` + `message`); subprocess consumption is future work.
- **`action=interrupt`**: cancels a running child via Orchestrator `Cancel`.

### Config knobs (`~/.pi-go/config.json` → `delegation`)
| Field | Default | Env override |
|-------|---------|----------------|
| `max_concurrent_children` | 3 | `PI_DELEGATE_MAX_CONCURRENT` |
| `max_spawn_depth` | 1 | `PI_DELEGATE_MAX_SPAWN_DEPTH` |
| `orchestrator_enabled` | false | `PI_DELEGATE_ORCHESTRATOR_ENABLED` |
| `subagent_auto_approve` | false (deny) | `PI_SUBAGENT_AUTO_APPROVE` |

Child env: `PI_DELEGATED_CHILD=1`, `PI_DELEGATE_DEPTH`, `PI_DELEGATE_ROLE` (orchestrator children retain `delegate_task` in the toolset when depth allows).

### Master planner / worker-sidecar notes
- Master planner: unchanged `gateway_*` refusal for any `PI_DELEGATED_CHILD` process; nested delegation is in-process planner only (not worker-sidecar RPC).
- Worker-sidecar: no Hub/swarm changes; delegate trees remain the interactive planner + Orchestrator path.

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
| Steer delivery into running child pi sessions | Queued on Orchestrator only; no stdin inject yet. |
| Full YOLO approval UX | `subagent_auto_approve` config stub only (default deny). |

## Verify

```bash
go test ./internal/tools/... ./internal/subagent/... ./internal/masterplanner/...
```
