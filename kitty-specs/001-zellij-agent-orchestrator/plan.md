# Implementation Plan: Feature 001 — Zellij Agent Orchestrator

## Goal
Build a Rust-based orchestrator that uses Zellij to manage concurrent AI coding agent sessions with dynamic KDL layouts, prompt injection, automatic completion detection, and wave-based progression.

## Key Architecture Decisions

| Decision | Rationale |
|----------|-----------|
| KDL layout with `split_direction="vertical"` for 3-column structure | Zellij's canonical pattern: left controller pane, right container with nested splits for agent grid |
| Prompt injection via stdin pipe (`cat prompt.md \| opencode --prompt "context:"`) | OpenCode has no `--prompt-file` flag; stdin pipe (PR #1230) is the supported mechanism |
| Completion detection via filesystem watcher on WP frontmatter `lane` field | spec-kitty stores WP state in YAML frontmatter of MD files; no DB, pure filesystem |
| Pane commands via `zellij action` CLI (WriteCharsToPaneId, FocusTerminalPane) | Zellij 0.41+ exposes targeted pane actions by PaneId for precise orchestration |
| spec-kitty lane transitions via `spec-kitty agent tasks move-task` | Agent subcommand interface for programmatic lane management |
| Pane ID discovery via KDL `name` attribute + `zellij action list-panes` parsing | Panes support `name="WP01"` attribute; `list-panes` maps names to IDs |
| Controller commands via named pipe (FIFO) at `.kasmos/cmd.pipe` | Avoids stdin conflicts with controller's OpenCode session |
| Partial wave failure: block dependent WPs, allow operator `force-advance` | Conservative default prevents cascading failures; operator retains control |
| Completion detection uses `IN_CLOSE_WRITE` + 200ms debounce + read-retry | Prevents partial reads from concurrent agent writes |
| Controller pane in layout, excluded from capacity accounting | Total panes = agent panes + 1 controller. Default max 8 agent panes. |
| V1 = layout orchestration only (no Ratatui dashboard) | Fastest path to core value; dashboard deferred to follow-up WP |

## FR Coverage Matrix

| FR | Description | WP(s) | Tasks |
|----|-------------|--------|-------|
| FR-001 | Read WP specs and dependency graphs | WP01, WP02 | 1.1, 2.1–2.3 |
| FR-002 | Generate valid Zellij KDL layouts | WP03 | 3.1–3.8 |
| FR-003 | Create 3-column Zellij session | WP03, WP05 | 3.2, 5.1 |
| FR-004 | Launch OpenCode TUI per WP pane with prompt via stdin | WP04, WP05 | 4.5, 5.2 |
| FR-005 | Generate WP-specific prompt files | WP04 | 4.1–4.4 |
| FR-006 | Auto-detect WP completion | WP06 | 6.1–6.7 |
| FR-007 | Manual WP state transitions | WP08 | 8.3–8.5 |
| FR-008 | Configurable wave progression | WP07 | 7.1–7.3 |
| FR-009 | Persist orchestration state to disk | WP09 | 9.1–9.4 |
| FR-010 | Handle agent crashes gracefully | WP10 | 10.1–10.2 |
| FR-011 | Validate dependency graph, reject cycles | WP02 | 2.5 |
| FR-012 | View orchestration status command | WP08 | 8.5 |
| FR-013 | Pane focus and zoom operations | WP05, WP08 | 5.5, 8.6 |
| FR-014 | Clean up sessions and temp files | WP05, WP10 | 5.6, 10.3–10.5 |
| FR-015 | Concurrent WP execution with capacity limit | WP07 | 7.4 |
| FR-016 | Integrate with spec-kitty lane management | WP06 | 6.2, 6.7 |
| FR-017 | Generate post-run summary report | WP11 | 11.7 |

## Wave Assignment

| Wave | Work Packages | Dependencies | Parallelism |
|------|--------------|--------------|-------------|
| 1 | WP01, WP02 | None | Full parallel |
| 2 | WP03, WP04 | WP01+WP02 | Full parallel |
| 3 | WP05, WP06 | WP05: WP01+WP03+WP04; WP06: WP01+WP02 | Full parallel |
| 4 | WP07, WP08 | WP07: WP01+WP02+WP05+WP06; WP08: WP01+WP05 | Full parallel |
| 5 | WP09, WP10 | WP09: WP01+WP05+WP06; WP10: WP05+WP06+WP07+WP08 | Full parallel |
| 6 | WP11 | All | Sequential |

## Work Packages

### Phase 1: Foundation

#### WP01 — Core Types & Configuration
- 1.1 Define orchestration data model (OrchestrationRun, WorkPackage, Wave, WPState, Config)
- 1.2 Config loading from CLI args + env + optional TOML
- 1.3 Error types via thiserror (ConfigError, ZellijError, SpecKittyError, StateError, PaneError)
- 1.4 State machine transitions with guard clauses
- 1.5 Logging setup via tracing crate with RUST_LOG
- **FR**: FR-001, FR-011 | **Deps**: none

#### WP02 — Spec Parser & Dependency Graph
- 2.1 Parse kitty-specs feature directory structure
- 2.2 Extract YAML frontmatter from WP markdown files
- 2.3 Build dependency DAG (HashMap adjacency list)
- 2.4 Topological sort via Kahn's algorithm
- 2.5 Cycle detection with actionable error messages
- 2.6 Compute wave groups from topological layers
- **FR**: FR-001, FR-011 | **Deps**: none

### Phase 2: Layout & Prompt Generation

#### WP03 — KDL Layout Generator
- 3.1 KDL layout template engine
- 3.2 3-column layout: controller left (40%), agent grid right (60%)
- 3.3 Adaptive grid sizing (cols=ceil(sqrt(n)), rows=ceil(n/cols))
- 3.4 Embed pane commands with stdin pipe pattern
- 3.5 Per-pane cwd to WP worktree
- 3.6 KDL name attribute per pane for ID discovery
- 3.7 Write KDL to .kasmos/layout.kdl
- 3.8 Validate generated KDL syntax
- **FR**: FR-002, FR-003 | **Deps**: WP01, WP02

#### WP04 — Prompt File Generator
- 4.1 Prompt template with WP description, scope, context
- 4.2 Dependency context and file references
- 4.3 AGENTS.md content inclusion
- 4.4 Write prompts to .kasmos/prompts/
- 4.5 Shell wrapper scripts for stdin pipe
- 4.6 Validate OpenCode in PATH
- **FR**: FR-005 | **Deps**: WP01, WP02

### Phase 3: Session Management & Detection

#### WP05 — Zellij Session Manager
- 5.1 Create Zellij session from KDL layout
- 5.2 Launch initial wave panes
- 5.3 Pane ID discovery (list-panes + name matching)
- 5.4 Pane lifecycle (open, close, restart)
- 5.5 Focus/zoom operations
- 5.6 Session attach/detach handling
- 5.7 Prompt injection failure detection
- **FR**: FR-003, FR-004, FR-013, FR-014 | **Deps**: WP01, WP03, WP04

#### WP06 — Completion Detector
- 6.1 Filesystem watcher with IN_CLOSE_WRITE
- 6.2 Parse WP frontmatter lane transitions
- 6.3 Read-retry with 200ms debounce
- 6.4 Git activity detection
- 6.5 File marker detection
- 6.6 Signal deduplication
- 6.7 Emit events via tokio::sync::mpsc
- **FR**: FR-006, FR-016 | **Deps**: WP01, WP02

### Phase 4: Wave Engine & Commands

#### WP07 — Wave Engine
- 7.1 Wave progression logic
- 7.2 Continuous mode (auto-launch on dependency resolution)
- 7.3 Wave-gated mode (operator confirmation)
- 7.4 Capacity limiting (max 8 agent panes, excludes controller)
- 7.5 Partial wave failure policy
- **FR**: FR-008, FR-015 | **Deps**: WP01, WP02, WP05, WP06

#### WP08 — Controller Commands
- 8.1 FIFO command input (.kasmos/cmd.pipe)
- 8.2 Command grammar parsing
- 8.3–8.9 Commands: restart, pause, status, focus/zoom, abort, force-advance, retry
- **FR**: FR-007, FR-012, FR-013 | **Deps**: WP01, WP05

### Phase 5: Persistence & Resilience

#### WP09 — State Persistence
- 9.1 Serialize state to .kasmos/state.json
- 9.2 Atomic write (tmp + rename) on every transition
- 9.3 State reconciliation decision table for reattach
- 9.4 Stale state detection
- **FR**: FR-009 | **Deps**: WP01, WP05, WP06

#### WP10 — Error Handling & Cleanup
- 10.1 Pane crash detection (poll list-panes every 5s)
- 10.2 WP state to Failed on crash
- 10.3 Graceful shutdown sequence
- 10.4 Signal handling (SIGINT, SIGTERM)
- 10.5 Artifact cleanup
- **FR**: FR-010, FR-014 | **Deps**: WP05, WP06, WP07, WP08

### Phase 6: Integration & CLI

#### WP11 — CLI Entry Point & Integration
- 11.1 `kasmos launch <feature>`
- 11.2 `kasmos status [<feature>]`
- 11.3 `kasmos attach <feature>`
- 11.4 `kasmos stop [<feature>]`
- 11.5 Wire all modules with error propagation
- 11.6 End-to-end integration test
- 11.7 Generate post-run summary report (.kasmos/report.md)
- **FR**: FR-001–FR-017 | **Deps**: all
