# Research: 009-standalone-tui-preview

**Date**: 2026-02-12

## Overview

This feature has minimal unknowns — the TUI is already decoupled, all types are known, and the architecture requires no new patterns. One design decision was resolved during planning.

## Decision: Deterministic vs Random Animation

**Decision**: Deterministic cycling (no `rand` crate).

**Rationale**: The animation loop uses a tick counter for WP selection (round-robin through non-Completed WPs) and a modular arithmetic check for the ~15% failure path (`tick_count % 7 == 0` approximates 14.3%). This avoids adding a new dependency for a dev-only feature and produces fully reproducible behavior — useful if debugging a rendering issue triggered at a specific animation state.

**Alternatives considered**:
- `rand` crate for true random WP selection and failure probability. Rejected: adds a transitive dependency tree for a trivial feature; non-reproducible behavior makes TUI bug reproduction harder.

## Decision: Channel Sink for EngineAction

**Decision**: Drop the `mpsc::Receiver` immediately after creating the channel pair.

**Rationale**: The TUI sends `EngineAction` commands via `action_tx.try_send()`. In preview mode, there's no engine to receive them. Dropping the receiver means `try_send()` returns `Err(TrySendError::Closed)`, but the TUI already uses `let _ = app.action_tx.try_send(...)` (confirmed in `crates/kasmos/src/tui/keybindings.rs:149,184,196,211`), so errors are silently discarded. No panic, no log noise.

**Alternatives considered**:
- Spawn a background task that drains the receiver. Rejected: unnecessary — the `let _ =` pattern already handles the closed channel case.
- Keep receiver alive but never read. Rejected: channel buffer would fill up, then `try_send` returns `Full` error — same `let _ =` handles it, but wastes memory for the buffer.

## Decision: Animation Reset Behavior

**Decision**: When all WPs reach `Completed`, pause for 2 seconds showing the completed state, then reset all WPs to initial states and loop.

**Rationale**: The brief completed-state pause lets the developer see the "all done" rendering before the cycle restarts. Immediate reset would flash too fast to observe.

**Alternatives considered**:
- Stop animation when complete (require manual restart). Rejected: defeats the "hands-free preview" goal.
- Randomize initial states on each reset. Rejected: deterministic reset means the same visual sequence repeats, which is better for iterating on specific UI states.
