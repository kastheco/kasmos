# Research: kasmos Agent Orchestrator

**Feature**: 016-kasmos-agent-orchestrator
**Date**: 2026-02-17

## Research Summary

Research was conducted across multiple sessions covering architecture evaluation,
framework selection, TUI design, and technical contract definition. All NEEDS
CLARIFICATION items from the spec have been resolved.

## Decisions

### 1. Framework: Go / bubbletea

**Decision**: Go with bubbletea v2, lipgloss v2, bubbles, huh
**Rationale**: Elm architecture maps naturally to event-driven orchestration. Single binary
distribution. Goroutines for concurrent worker management. charmbracelet ecosystem provides
SSH (wish), forms (huh), and consistent styling out of the box.
**Alternatives considered**:
- Rust / ratatui: More performant but ecosystem friction (no form library equivalent to huh,
  no SSH server equivalent to wish). Async Rust complexity for subprocess management.
- Python / textual: Rich ecosystem but distribution challenges (Python runtime dependency),
  GIL concerns for concurrent worker I/O.

### 2. Worker Execution: Headless subprocesses

**Decision**: Workers spawned via `os/exec.Command("opencode", "run", ...)` with stdout/stderr
piped to Go readers.
**Rationale**: Simplest possible backend. No terminal multiplexer dependency. Output capture
is native Go. Process lifecycle managed by OS. Clean kill via process groups (`Setpgid`).
**Alternatives considered**:
- tmux sessions: More features (scrollback, attach) but adds runtime dependency and complexity.
  Deferred to future TmuxBackend via pluggable interface.
- PTY allocation: Preserves ANSI formatting but adds pty package dependency and complicates
  output parsing. Not needed since workers are non-interactive.

### 3. Session Continuation: --continue -s flag

**Decision**: Use `opencode run --continue -s <session_id>` for follow-up workers.
**Rationale**: Preserves full agent context (files read, decisions made, conversation history)
without requiring interactive access to a running session. The session ID is extracted from
worker output via regex.
**Alternatives considered**:
- Interactive PTY: Would require terminal multiplexer and complex I/O management.
  Overkill for the "review -> fix" workflow.
- Fresh workers with context dump: Loses implicit context. Token-expensive to re-establish.

### 4. Task Sources: Pluggable adapters

**Decision**: Three adapters behind a `Source` interface: SpecKittySource, GsdSource, AdHocSource.
**Rationale**: kasmos serves different planning maturity levels. Formal projects use spec-kitty
(plan.md with WPs). Lightweight projects use GSD (checkbox markdown). Quick tasks use ad-hoc
(manual prompts). The interface is simple (Load, Tasks, Type, Path) so new adapters are trivial.
**Alternatives considered**:
- Single format: Would force all projects into one planning style.
- Plugin-based adapters: Overengineered for 3 built-in sources. Can add later if needed.

### 5. TUI Design: Charm bubblegum aesthetic

**Decision**: Purple primary accent, hot pink headers, rounded borders, gradient title,
context-dependent keybinds, responsive 4-mode layout (too-small/narrow/standard/wide).
**Rationale**: Follows charmbracelet design language (consistent with gum, soft-serve, mods).
The responsive layout ensures usability from 80-column SSH sessions to wide local terminals.
**Research artifacts**: Full design in `design-artifacts/` (4 files, 12 view mockups).

### 6. Testing: Standard library + mock backend

**Decision**: `go test ./...` with table-driven tests. Mock `WorkerBackend` for TUI tests.
Integration tests gated behind `KASMOS_INTEGRATION=1`.
**Rationale**: Standard Go testing is sufficient. testify adds a dependency for marginal
ergonomic gain. Mock backend prevents tests from spawning real opencode processes (which
require API keys and take minutes to run).
**Alternatives considered**:
- testify/assert: Popular but not necessary for this project scope.
- Container-based integration: Overkill for a CLI tool.

## Detailed Research Artifacts

All technical contracts (Go interfaces, message types, JSON schemas, package layout)
are in `kitty-specs/016-kasmos-agent-orchestrator/research/tui-technical.md`.

All visual design (layout, mockups, keybinds, styles) are in `design-artifacts/`.
