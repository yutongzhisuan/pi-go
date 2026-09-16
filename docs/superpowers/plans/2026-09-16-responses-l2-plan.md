# Responses L2 (PR1) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Port Hermes Responses structured replay (`split_replay_messages`) and item-level stream events (`turn_output_items` / `response.output_item.added`) into `pi worker-sidecar`, without changing L1 checkpoint resume.

**Architecture:** Extend `internal/workersidecar/responses` to retain raw input items and split them into replay history + current user message; wire backend session construction to inject history; emit item-level events on the Responses path via the smallest progress/event extension that gateway/e2e can observe. Checkpoint `resume_blob` / `resume_summary` behavior stays L1.

**Tech Stack:** Go, existing `internal/workersidecar`, Hermes reference in design doc (do not require hermes-agent checkout).

**Spec:** `docs/superpowers/specs/2026-09-16-hermes-l2-delegate-parity-design.md` (§1 only for this PR).

## Global Constraints

- Repo: `yutongzhisuan/pi-go`; open a PR against `main`.
- Do not invent model-session hot restore across checkpoints.
- Do not change Hub / swarm / client-daemon contracts unless a one-line progress payload extension is required for item JSON.
- Keep empty-assistant fail-closed and existing envelope validation.
- TDD: failing tests before implementation for each task.
- Update `docs/superpowers/HERMES_PARITY_STATUS.md` when done.

---

## File map

| File | Responsibility |
|------|----------------|
| `internal/workersidecar/responses/envelope.go` | Retain raw items; expose replay fields on `ParsedEnvelope` |
| `internal/workersidecar/responses/replay.go` (new) | `SplitReplayMessages`, `TurnOutputItems` |
| `internal/workersidecar/responses/replay_test.go` (new) | Port critical Hermes cases |
| `internal/workersidecar/backend/backend.go` | Use replay history when building the child run; emit item events |
| Progress/RPC types if needed | Carry item JSON without Hub redesign |
| `docs/superpowers/HERMES_PARITY_STATUS.md` | Mark Responses L2 closed |

---

## Task 1: SplitReplayMessages + tests

**Files:** `internal/workersidecar/responses/replay.go`, `replay_test.go`

- [ ] Write failing tests: trailing user → history + user; no trailing user → empty structured user / flatten path; empty input
- [ ] Implement `SplitReplayMessages` matching Hermes semantics
- [ ] `go test ./internal/workersidecar/responses/...` green
- [ ] Commit

## Task 2: TurnOutputItems + tests

**Files:** same package

- [ ] Write failing tests: slice after last user; tool round-trip item shapes; reasoning/message ordering as in Hermes subset we need for SSE
- [ ] Implement `TurnOutputItems`
- [ ] Tests green; commit

## Task 3: Envelope retains raw items

**Files:** `envelope.go`, existing envelope tests

- [ ] Extend `ParsedEnvelope` with raw messages / items needed for split
- [ ] Tests: envelope with multi-turn input still validates; invalid envelope unchanged
- [ ] Commit

## Task 4: Backend injects replay history

**Files:** `backend/backend.go` (+ tests)

- [ ] When envelope present and split yields structured user + history, inject history into child session/prompt path (investigate how child `pi --mode json` receives history today; prefer minimal change)
- [ ] Unit/integration test that non-empty history is applied (mock child or assert constructed args)
- [ ] Commit

## Task 5: Emit item-level stream events

**Files:** backend + rpc/progress as needed

- [ ] On Responses path, emit at least `response.output_item.added` for the assistant message (and tool items if available from TurnOutputItems)
- [ ] Prefer piggybacking existing progress if it can carry structured JSON; else smallest additive field
- [ ] Test that events appear in the progress/event drain
- [ ] Commit

## Task 6: Docs + PR

- [ ] Update `HERMES_PARITY_STATUS.md` (Responses L2 closed; checkpoint still L1; link design)
- [ ] `go test ./internal/workersidecar/...`
- [ ] Open PR titled roughly: `worker-sidecar: Responses L2 replay + item-level SSE`
- [ ] PR body references design §1 and this plan

## Done when

- Unit tests for split/turn_output pass
- Responses path can emit item-level events
- Status doc updated
- PR open against `main`
