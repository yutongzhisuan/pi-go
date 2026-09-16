# Hermes → pi-go: Responses L2 + delegate_task parity

Date: 2026-09-16  
Repo: `yutongzhisuan/pi-go`  
Status: Design approved (Path A); awaiting implementation plans

## Goal

Close the remaining **portable** Hermes Worker / planner gaps that are still open after pi-go #7–#12:

1. Responses **structured replay** and **item-level SSE** (docs “L2”; not model-session hot restore).
2. Real **`delegate_task`** spawn with child isolation (`PI_DELEGATED_CHILD`, blocklist, concurrency, depth).

Checkpoint resume stays **L1** (inject `resume_blob` / `resume_summary` into the goal). Hermes Worker ACP itself documents M1 as “no L2 resume” for model sessions; we do **not** invent cross-checkpoint session snapshots.

## Non-goals (entire Path A)

- Hub / swarm-network contract changes
- `client-daemon` host swap / re-embedding Worker
- Modal / SSH backends
- Full Python `approvals.deny` / `DANGEROUS_PATTERNS` deobfuscation UX
- Cross-checkpoint **model session** hot restore

## Delivery plan (three PRs)

| PR | Name | Merge order |
|----|------|-------------|
| **PR1** | Responses L2 (replay + item SSE) | First |
| **PR2** | `delegate_task` core | After PR1 |
| **PR3** | Hermes enhancements (role / depth / list / steer / interrupt / config) | After PR2 |

Each PR is independently reviewable and revertible.

---

## §1 — PR1: Responses L2

### Problem

Today `internal/workersidecar/responses` flattens `responses.v1` `input` into a single user string. Hermes `extend/sub_agent/acp_backend.py` + `responses_payload.py` instead:

- `split_replay_messages`: if the transcript **ends with a user message**, earlier turns become conversation history and the last user turn is the new message; otherwise flatten.
- `turn_output_items`: emit Responses **item-level** stream events (e.g. `response.output_item.added`).

m2-e2e soft-skips these unless `E2E_REQUIRE_RESPONSES_L2=1`.

### Design

1. **Parse**: keep existing envelope validation; additionally retain **raw input items** (not only flattened text).
2. **`split_replay_messages` (Go port)**: same semantics as Hermes; unit tests ported from `test_responses_payload.py` critical cases (trailing user, no trailing user, tool round-trip shapes for `turn_output_items`).
3. **Run**: when replay history is non-empty, inject it into the child pi session / agent history before the current user message. ACP JSON-RPC wire unchanged.
4. **Stream**: on the Responses path, emit `response.output_item.added` (and related item events as needed) via the existing progress / event path so gateway/e2e can observe them. If the current progress frame cannot carry item JSON, add a minimal sidecar→daemon event shape in a follow-up patch **inside PR1** (do not block on Hub proto redesign).
5. **Terminal**: keep assistant `result_text` / empty-output fail-closed from #11–#12.
6. **Checkpoint**: unchanged L1 `_resume_goal`-style behavior.

### Acceptance (PR1)

- [ ] Go unit tests for `split_replay_messages` / `turn_output_items` equivalents
- [ ] ACP/Responses path emits item-level events for a canned tool or message turn
- [ ] Local e2e: with `E2E_REQUIRE_RESPONSES_L2=1`, former soft-skips pass (or document one remaining soft-skip with reason)
- [ ] `HERMES_PARITY_STATUS.md` updated: Responses L2 closed; checkpoint still L1

---

## §2 — PR2: `delegate_task` core

### Problem

Master planner prompt and `PI_DELEGATED_CHILD` refusal exist, but nothing spawns children or sets the env. Hermes `tools/delegate_tool.py` is large; pi-go already has `internal/subagent.Orchestrator` and `tools/subagent.go`.

### Design

**Strategy:** Hermes-compatible **`delegate_task` tool surface** on top of existing Orchestrator — do not rewrite the spawn stack.

#### Tool surface (Hermes schema subset)

- Inputs: `goal` **or** `tasks[]` (each `goal` + optional `context`); optional top-level `context`
- PR2 does **not** expose `role=orchestrator`, `action=list|steer|interrupt` (PR3)
- Over `max_concurrent_children` (default **3**, configurable): return a **tool error** (no silent truncate) — Hermes behavior
- `max_spawn_depth=1`: children **cannot** call `delegate_task` again

#### Isolation (required)

| Mechanism | Behavior |
|-----------|----------|
| `PI_DELEGATED_CHILD=1` | Injected into child process/session env; existing masterplanner gateway_* refusal applies |
| Blocklist | Child must not get: `delegate_task`, `clarify`, master `gateway_*`, and Hermes equivalents (`memory` / `send_message` / `cronjob`) when those tools exist in pi-go |
| Context | **Fresh** child session; only goal + context; no parent transcript |
| Parent view | Parent sees tool call + **summary result** only — not child intermediate tool traces |

#### Execution

1. Validate args → `Orchestrator.SpawnWithInput` (reuse parallel path for `tasks[]`)
2. Each child: env `PI_DELEGATED_CHILD=1`; toolset = parent − blocklist
3. Wait / timeout → aggregate text to parent
4. Child invoking `gateway_*` must receive existing refusal JSON

### Acceptance (PR2)

- [ ] Unit: blocklist, over-concurrency error, depth=1 rejects nested delegate, gateway refusal under `PI_DELEGATED_CHILD`
- [ ] Integration (optional): parent `delegate_task` single goal → child summary; child has no gateway tools
- [ ] `HERMES_PARITY_STATUS.md`: real `delegate_task` spawn marked landed (core)

### Non-goals (PR2)

- Nested orchestrator trees, steer/list/interrupt, full YOLO approval knobs
- Changing Hub remote dispatch (this is **in-process planner/agent** delegation)

---

## §3 — PR3: Hermes enhancements

Built on a stable PR2 leaf `delegate_task`:

| Capability | Behavior |
|------------|----------|
| `role` | `leaf` (default) / `orchestrator`; if `max_spawn_depth` is still 1, orchestrator **degrades to leaf** with a log (Hermes-aligned) |
| `max_spawn_depth` 2–3 | Orchestrator children may spawn leaf grandchildren; document cost scaling |
| `action=list` | List in-process active children (id / goal / status) |
| `action=steer` | Queue guidance to a child (reuse Orchestrator channel if present; else minimal) |
| `action=interrupt` | Cancel/interrupt a named child |
| config | `delegation.max_concurrent_children`, `max_spawn_depth`, `orchestrator_enabled`, optional `subagent_auto_approve` (default deny) |

### Acceptance (PR3)

- [ ] Unit tests for depth degrade, list / steer / interrupt state
- [ ] Docs: `HERMES_PARITY_STATUS.md`, worker-sidecar / masterplanner notes
- [ ] e2e only if cheap; do not block on full platform suite

---

## References

- Hermes: `extend/sub_agent/acp_backend.py`, `responses_payload.py`, `model_sessions.py`
- Hermes: `tools/delegate_tool.py`, `agent/delegation_context.py`
- pi-go: `internal/workersidecar/responses`, `internal/workersidecar/backend`
- pi-go: `internal/subagent`, `internal/tools/subagent.go`, `internal/masterplanner/delegation.go`
- Status: `docs/superpowers/HERMES_PARITY_STATUS.md`

## Open questions for implementers

1. Exact progress/event frame for `output_item.added` if current `acp.progress` text-only — decide inside PR1 with the smallest wire-compatible extension.
2. Whether `delegate_task` replaces or coexists with the existing `subagent` tool name in default toolsets — prefer **coexist**: keep `subagent` for pi-native flows; add `delegate_task` for Hermes prompt parity.
