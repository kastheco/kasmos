# kasmos Constitution

> Updated: 2026-02-17
> Version: 2.0.0

## Purpose

This constitution captures the technical standards and governance rules for kasmos,
a TUI-based agent orchestrator for managing concurrent AI coding sessions.
All features and pull requests should align with these principles.

## Technical Standards

### Languages and Frameworks

- **Go** (1.23+)
- **bubbletea** v2 for TUI (Elm architecture: Model/Update/View)
- **lipgloss** v2 for terminal styling
- **bubbles** for TUI components (table, viewport, textinput, list, spinner, help)
- **huh** for form dialogs
- **cobra** for CLI command structure
- **OpenCode** as the sole AI agent harness (`opencode run` for headless workers)

### Testing Requirements

- Use `go test ./...` for all testing
- All features must have corresponding tests
- Standard library `testing` package; table-driven tests for parsers and state machines
- Mock `WorkerBackend` for TUI tests (no real subprocess spawning in unit tests)
- Integration tests gated behind `KASMOS_INTEGRATION=1` env var
- No hard coverage target, but untested features are not considered complete

### Performance and Scale

- TUI must remain responsive at all times - never block the Update loop
- Support orchestrating many concurrent workers without degradation
- Worker output reading must be async (goroutines + channels, surfaced as tea.Msg)
- Minimize unnecessary allocations in hot paths (output buffer ring, not unbounded slices)

### Architecture Principles

- **No manager AI agent** - the TUI is the orchestrator. Zero token cost for orchestration.
- **Workers are headless subprocesses** - spawned via `opencode run`, output captured via Go pipes.
- **Session continuation over interactivity** - `opencode run --continue -s <id>` preserves context without PTY allocation.
- **Pluggable WorkerBackend interface** - SubprocessBackend (MVP), TmuxBackend (future).
- **Three task source adapters** - spec-kitty (plan.md/WP frontmatter), GSD (checkbox markdown), ad-hoc (manual prompts).
- **Daemon mode** - same Model/Update loop, no View rendering (`WithoutRenderer()`).

### Deployment and Constraints

- **Linux**: Primary platform (full support)
- **macOS**: Secondary platform (best-effort support)
- **Runtime dependencies**: OpenCode and git must be installed and in PATH
- Distributed as a single binary (standard `go install` or goreleaser workflow)

## Governance

### Amendment Process

Any team member can propose amendments via pull request. Changes are discussed
and merged following standard PR review process.

### Compliance Validation

Code reviewers validate compliance during PR review. Constitution violations
should be flagged and addressed before merge.

### Exception Handling

Exceptions discussed case-by-case with team. Strong justification required.
Consider updating constitution if exceptions become common.
