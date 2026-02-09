# Work Packages: Zellij Agent Orchestrator

**Feature**: 001-zellij-agent-orchestrator
**Generated**: 2026-02-09
**Total Work Packages**: 11
**Total Subtasks**: 69

## Inputs

- [spec.md](spec.md) — Feature specification (17 FRs, 6 user stories)
- [plan.md](plan.md) — Implementation plan (11 WPs, 6 waves)

## Prerequisites

- Rust 2024 edition toolchain
- Zellij installed and in PATH
- OpenCode installed and in PATH
- spec-kitty CLI v0.14.1+
- Git

## Organization

Work packages are organized into 6 waves based on dependency analysis:

| Wave | Work Packages | Theme |
|------|--------------|-------|
| 1 | WP01, WP02 | Foundation — core types + spec parser |
| 2 | WP03, WP04 | Generation — KDL layouts + prompts |
| 3 | WP05, WP06 | Runtime — session manager + completion detector |
| 4 | WP07, WP08 | Control — wave engine + commands |
| 5 | WP09, WP10 | Resilience — persistence + error handling |
| 6 | WP11 | Integration — CLI entry point + report |

---

## Phase 1: Foundation

### Work Package WP01: Core Types & Configuration (Priority: P1)

**Goal**: Define the orchestration data model, configuration loading, error types, state machine, and logging infrastructure that all other WPs depend on.

**Independent Test**: Instantiate all core types, verify state machine transitions (valid and invalid), load config from defaults/env/TOML, and confirm tracing output with RUST_LOG=debug.

**Prompt**: [`tasks/WP01-core-types-config.md`](tasks/WP01-core-types-config.md)

#### Included Subtasks

- [x] [T001] [P] Define orchestration data model (OrchestrationRun, WorkPackage, Wave, WPState, Config)
- [x] [T002] Config loading from CLI args + env + optional TOML
- [x] [T003] Error types via thiserror (ConfigError, ZellijError, SpecKittyError, StateError, PaneError, WaveError)
- [x] [T004] WPState machine transitions with guard clauses
- [x] [T005] Logging setup via tracing crate with RUST_LOG

#### Implementation Notes

- Start by defining the WPState enum and transition rules — this is the backbone for WP05-WP10
- Config struct should use clap for CLI args with env var fallbacks and optional TOML override
- Error types should cover all subsystems; use thiserror derive macros with context messages
- State machine guards prevent invalid transitions (e.g., Completed→Active is illegal)

#### Parallel Opportunities

- T001 (data model) can be developed independently of T002 (config) and T003 (errors)

#### Dependencies

None — this is a root work package.

#### Risks & Mitigations

- **Risk**: Data model doesn't accommodate future needs → **Mitigation**: Design with `#[non_exhaustive]` on enums, use builder pattern for Config
- **Risk**: State machine too rigid → **Mitigation**: Include Paused state and explicit transition table

---

### Work Package WP02: Spec Parser & Dependency Graph (Priority: P1)

**Goal**: Parse kitty-specs feature directories, extract WP metadata from YAML frontmatter, build a dependency DAG, detect cycles, and compute wave groupings.

**Independent Test**: Parse a sample feature directory with 4-5 WP markdown files containing YAML frontmatter, build the DAG, run topological sort, verify wave grouping matches expected output. Test with a cyclic dependency to confirm rejection.

**Prompt**: [`tasks/WP02-spec-parser-dep-graph.md`](tasks/WP02-spec-parser-dep-graph.md)

#### Included Subtasks

- [x] [T006] [P] Parse kitty-specs feature directory structure
- [x] [T007] [P] Extract YAML frontmatter from WP markdown files
- [x] [T008] Build dependency DAG (HashMap adjacency list)
- [x] [T009] Topological sort via Kahn's algorithm
- [x] [T010] Cycle detection with actionable error messages
- [x] [T011] Compute wave groups from topological layers

#### Implementation Notes

- Feature dir structure: `kitty-specs/{feature}/tasks/WPxx-slug.md` with YAML frontmatter
- Frontmatter contains: work_package_id, dependencies (array of WP IDs), title, lane
- DAG uses HashMap<String, Vec<String>> — key is WP ID, value is list of dependencies
- Kahn's algorithm: maintain in-degree count, process zero-in-degree nodes, detect cycles via remaining nodes
- Wave groups = nodes at same depth in topological ordering

#### Parallel Opportunities

- T006 (dir parsing) and T007 (frontmatter extraction) are independent file operations

#### Dependencies

None — this is a root work package.

#### Risks & Mitigations

- **Risk**: Frontmatter format varies → **Mitigation**: Strict YAML schema validation with clear error messages
- **Risk**: Large dependency graphs slow sorting → **Mitigation**: Unlikely at expected scale (<50 WPs), but Kahn's is O(V+E)

---

## Phase 2: Generation

### Work Package WP03: KDL Layout Generator (Priority: P1)

**Goal**: Generate valid Zellij KDL layout files with a 3-column structure (controller left, adaptive agent grid right), embedded pane commands for OpenCode launch, and per-pane naming for ID discovery.

**Independent Test**: Generate a KDL layout for 4 agent panes, parse the output with kdl crate to verify syntax, check that controller pane exists at 40% width, agent grid has correct row/column structure, each pane has name attribute and command node.

**Prompt**: [`tasks/WP03-kdl-layout-generator.md`](tasks/WP03-kdl-layout-generator.md)

#### Included Subtasks

- [ ] [T012] KDL layout template engine using kdl v5 crate
- [ ] [T013] 3-column layout: controller left (40%), agent grid right (60%)
- [ ] [T014] Adaptive grid sizing (cols=ceil(sqrt(n)), rows=ceil(n/cols))
- [ ] [T015] Embed pane commands with stdin pipe pattern
- [ ] [T016] Per-pane cwd to WP worktree path
- [ ] [T017] KDL name attribute per pane for ID discovery
- [ ] [T018] Write KDL to .kasmos/layout.kdl
- [ ] [T019] [P] Validate generated KDL syntax

#### Implementation Notes

- KDL v5 crate API: build KdlDocument, add KdlNode entries, serialize to string
- Zellij layout structure: `layout { ... }` containing `pane split_direction="vertical" { ... }`
- Controller pane: `pane size="40%" name="controller" command="opencode"`
- Agent container: `pane size="60%" split_direction="horizontal" { ... }` with nested rows
- Each row: `pane split_direction="vertical" { ... }` containing column panes
- Pane command: `command "bash"` with `args ["-c" "cat /path/to/prompt.md | opencode -p 'context:'"]`

#### Parallel Opportunities

- T019 (validation) is independent once layout generation exists

#### Dependencies

Depends on WP01 (core types for WorkPackage struct) and WP02 (dependency graph for wave-based pane selection).

#### Risks & Mitigations

- **Risk**: KDL v5 API differs from docs → **Mitigation**: Write validation test (T019) early as integration check
- **Risk**: Complex grid math for odd pane counts → **Mitigation**: Handle remainder row separately (fewer panes than cols)

---

### Work Package WP04: Prompt File Generator (Priority: P1)

**Goal**: Generate work-package-specific prompt files containing WP description, scope, dependency context, and AGENTS.md content, plus shell wrapper scripts for stdin pipe injection into OpenCode.

**Independent Test**: Generate prompts for 3 WPs with varying dependency depths, verify each contains the correct WP metadata, dependency context from upstream WPs, AGENTS.md content, and that shell wrapper scripts are executable and correctly reference prompt paths.

**Prompt**: [`tasks/WP04-prompt-file-generator.md`](tasks/WP04-prompt-file-generator.md)

#### Included Subtasks

- [ ] [T020] [P] Prompt template struct with WP description, scope, context
- [ ] [T021] Dependency context injection (upstream WP summaries)
- [ ] [T022] AGENTS.md content inclusion
- [ ] [T023] Write prompts to .kasmos/prompts/WPxx.md
- [ ] [T024] Shell wrapper scripts for stdin pipe
- [ ] [T025] [P] Validate OpenCode binary in PATH

#### Implementation Notes

- Prompt template should be a Rust struct with `render() -> String` method
- Include WP title, description, subtask list, scope boundaries, constraints
- Dependency context: for each upstream WP, include a brief summary of what it provides
- AGENTS.md: read from project root, include verbatim in prompt
- Shell wrapper: `#!/bin/bash\ncat /path/to/.kasmos/prompts/WPxx.md | opencode -p "context:"`
- Wrappers go in .kasmos/scripts/WPxx.sh, must be chmod +x

#### Parallel Opportunities

- T020 (template struct) and T025 (OpenCode validation) are independent

#### Dependencies

Depends on WP01 (WorkPackage struct) and WP02 (dependency graph for context injection).

#### Risks & Mitigations

- **Risk**: Prompt too long for OpenCode stdin → **Mitigation**: Keep prompts focused, truncate if >10K chars with warning
- **Risk**: AGENTS.md missing → **Mitigation**: Graceful fallback with warning log

---

## Phase 3: Runtime

### Work Package WP05: Zellij Session Manager (Priority: P1)

**Goal**: Create and manage Zellij sessions from KDL layouts, discover pane IDs, handle pane lifecycle operations, and manage session attach/detach.

**Independent Test**: Create a Zellij session from a test KDL layout, verify pane IDs are discovered correctly, open/close/restart a pane, verify focus/zoom operations work, detach and reattach verifying session persistence.

**Prompt**: [`tasks/WP05-zellij-session-manager.md`](tasks/WP05-zellij-session-manager.md)

#### Included Subtasks

- [ ] [T026] Create Zellij session from KDL layout
- [ ] [T027] Launch initial wave panes
- [ ] [T028] Pane ID discovery (list-panes + name matching)
- [ ] [T029] Pane lifecycle (open, close, restart)
- [ ] [T030] Focus/zoom operations
- [ ] [T031] Session attach/detach handling
- [ ] [T032] Prompt injection failure detection

#### Implementation Notes

- Session creation: `zellij --layout .kasmos/layout.kdl --session kasmos-{feature}`
- Pane discovery: `zellij action list-panes` returns tab-separated output; parse name→ID mapping
- Pane commands: `zellij action write-chars-to-pane-id {id} "text"`, `zellij action focus-terminal-pane {id}`
- Detect existing session: `zellij list-sessions` and check for `kasmos-{feature}`
- Prompt injection failure: check pane exit code after launch, retry once

#### Parallel Opportunities

- Most subtasks are sequential (session must exist before pane operations)

#### Dependencies

Depends on WP01 (types), WP03 (KDL layout), WP04 (prompt files + shell wrappers).

#### Risks & Mitigations

- **Risk**: Zellij CLI output format changes → **Mitigation**: Version check at startup, parse defensively
- **Risk**: Race condition on pane discovery after session creation → **Mitigation**: Retry with backoff (3 attempts, 500ms)

---

### Work Package WP06: Completion Detector (Priority: P1)

**Goal**: Monitor filesystem events to detect WP completion via spec-kitty lane transitions, git activity, and file markers, with debouncing and deduplication, emitting events to the wave engine.

**Independent Test**: Set up filesystem watcher on a temp directory, write a WP markdown file with lane: "planned", modify it to lane: "done", verify detection within 2 seconds. Test debouncing by writing rapidly. Test deduplication with repeated events.

**Prompt**: [`tasks/WP06-completion-detector.md`](tasks/WP06-completion-detector.md)

#### Included Subtasks

- [ ] [T033] Filesystem watcher using notify crate (IN_CLOSE_WRITE)
- [ ] [T034] Parse WP frontmatter lane transitions
- [ ] [T035] Read-retry with 200ms debounce + 3 retry attempts
- [ ] [T036] [P] Git activity detection
- [ ] [T037] [P] File marker detection
- [ ] [T038] Signal deduplication
- [ ] [T039] Emit events via tokio::sync::mpsc

#### Implementation Notes

- notify crate: use RecommendedWatcher with EventKind::Modify(ModifyKind::Data(DataChange::Content))
- On event: debounce 200ms, then read frontmatter, parse lane field
- Lane transitions that signal completion: "for_review" or "done"
- Git detection: watch for new commits in WP worktree (.git/refs/heads/)
- File marker: watch for .done or .complete file in WP worktree root
- mpsc channel: CompletionEvent { wp_id, method: AutoDetect|Manual, timestamp }

#### Parallel Opportunities

- T036 (git) and T037 (file marker) are independent detection methods

#### Dependencies

Depends on WP01 (types) and WP02 (WP metadata for path resolution).

#### Risks & Mitigations

- **Risk**: notify crate misses events under high I/O → **Mitigation**: Periodic polling fallback (every 30s)
- **Risk**: Partial YAML read during write → **Mitigation**: Read-retry with 200ms delay, 3 attempts

---

## Phase 4: Control

### Work Package WP07: Wave Engine (Priority: P1)

**Goal**: Implement wave progression logic supporting both continuous and wave-gated modes, with capacity limiting and partial wave failure handling.

**Independent Test**: Create a mock orchestration with 3 waves, simulate WP completions, verify continuous mode auto-advances, wave-gated mode pauses for confirmation, capacity limit queues excess WPs, and failed WPs block only direct dependents.

**Prompt**: [`tasks/WP07-wave-engine.md`](tasks/WP07-wave-engine.md)

#### Included Subtasks

- [ ] [T040] Wave progression logic
- [ ] [T041] Continuous mode (auto-launch on dependency resolution)
- [ ] [T042] Wave-gated mode (operator confirmation)
- [ ] [T043] Capacity limiting (max 8 agent panes)
- [ ] [T044] Partial wave failure policy

#### Implementation Notes

- Wave engine receives CompletionEvents from mpsc channel
- On completion: check if all deps for pending WPs are met, launch eligible ones
- Continuous: launch immediately when deps resolve (up to capacity)
- Wave-gated: wait for all current wave WPs to complete, then prompt operator
- Capacity: track active pane count, queue excess, launch from queue on pane close
- Failure policy: mark WP as Failed, check dependents — only those listing failed WP in deps are blocked

#### Parallel Opportunities

- All subtasks are tightly coupled (sequential implementation recommended)

#### Dependencies

Depends on WP01 (types), WP02 (dependency graph), WP05 (session manager for pane launch), WP06 (completion events).

#### Risks & Mitigations

- **Risk**: Race between completion detection and pane launch → **Mitigation**: Single-threaded event loop via tokio::select!
- **Risk**: Wave-gated confirmation never arrives → **Mitigation**: Timeout with reminder message to controller

---

### Work Package WP08: Controller Commands (Priority: P2)

**Goal**: Implement FIFO-based command input from the controller pane, supporting restart, pause, status, focus/zoom, abort, force-advance, and retry commands.

**Independent Test**: Create the FIFO, write commands to it, verify each command is parsed correctly and triggers the expected action (mock pane operations). Test invalid commands produce clear error messages.

**Prompt**: [`tasks/WP08-controller-commands.md`](tasks/WP08-controller-commands.md)

#### Included Subtasks

- [ ] [T045] FIFO command input (.kasmos/cmd.pipe)
- [ ] [T046] Command grammar parsing
- [ ] [T047] [P] Restart command
- [ ] [T048] [P] Pause command
- [ ] [T049] [P] Status command
- [ ] [T050] [P] Focus/zoom commands
- [ ] [T051] [P] Abort command
- [ ] [T052] [P] Force-advance command
- [ ] [T053] [P] Retry command

#### Implementation Notes

- FIFO: `nix::mkfifo(".kasmos/cmd.pipe", 0o600)`, spawn tokio task reading lines
- Grammar: `<command> [<wp_id>] [<args>]` — e.g., `restart WP03`, `status`, `focus WP05`
- Each command maps to an async handler function
- Commands modify OrchestrationRun state and trigger ZellijSessionManager actions
- Status command: format current state as table (WP, State, Pane, Duration, Wave)

#### Parallel Opportunities

- T047-T053 (individual commands) are independent once T045-T046 (FIFO + parser) exist

#### Dependencies

Depends on WP01 (types) and WP05 (session manager for pane operations).

#### Risks & Mitigations

- **Risk**: FIFO blocks if no reader → **Mitigation**: Open FIFO with O_NONBLOCK, use tokio async I/O
- **Risk**: Command while state is transitioning → **Mitigation**: Acquire state lock before executing command

---

## Phase 5: Resilience

### Work Package WP09: State Persistence (Priority: P2)

**Goal**: Serialize orchestration state to disk with atomic writes, support state reconciliation on reattach, and detect stale state.

**Independent Test**: Serialize state, kill process, restart, verify state loads correctly. Test atomic write by checking no partial writes occur. Test stale detection with outdated timestamp.

**Prompt**: [`tasks/WP09-state-persistence.md`](tasks/WP09-state-persistence.md)

#### Included Subtasks

- [ ] [T054] Serialize OrchestrationRun to .kasmos/state.json
- [ ] [T055] Atomic write (tmp + rename)
- [ ] [T056] State reconciliation decision table for reattach
- [ ] [T057] [P] Stale state detection

#### Implementation Notes

- Serialize: serde_json::to_string_pretty(&state), write to .kasmos/state.json
- Atomic: write to .kasmos/state.json.tmp, then std::fs::rename to .kasmos/state.json
- Reconciliation: on reattach, load state, check each WP against live pane status
  - Running pane + Running state → continue
  - Missing pane + Running state → mark as Crashed
  - Pane exists + Completed state → verify (pane might be stale)
- Stale: compare state file mtime against session start time

#### Parallel Opportunities

- T057 (stale detection) is independent of the write path

#### Dependencies

Depends on WP01 (types to serialize), WP05 (session manager for pane verification), WP06 (completion events trigger state updates).

#### Risks & Mitigations

- **Risk**: State file corruption during crash → **Mitigation**: Atomic write ensures either old or new state, never partial
- **Risk**: State diverges from reality → **Mitigation**: Reconciliation on every reattach

---

### Work Package WP10: Error Handling & Cleanup (Priority: P2)

**Goal**: Detect pane crashes, handle graceful shutdown with signal handling, and clean up artifacts on exit.

**Independent Test**: Simulate a pane crash (kill a Zellij pane), verify detection within 10 seconds and state update. Send SIGTERM, verify graceful shutdown sequence completes. Verify .kasmos/ artifacts are cleaned on normal exit.

**Prompt**: [`tasks/WP10-error-handling-cleanup.md`](tasks/WP10-error-handling-cleanup.md)

#### Included Subtasks

- [ ] [T058] Pane crash detection (poll list-panes every 5s)
- [ ] [T059] WP state to Failed on crash
- [ ] [T060] Graceful shutdown sequence
- [ ] [T061] Signal handling (SIGINT, SIGTERM)
- [ ] [T062] [P] Artifact cleanup

#### Implementation Notes

- Crash detection: tokio interval (5s), run `zellij action list-panes`, compare against expected panes
- Missing pane → WP state = Failed, emit event to wave engine
- Graceful shutdown: stop watchers → close FIFO → persist state → close panes → kill session
- Signals: tokio::signal::ctrl_c() for SIGINT, tokio::signal::unix::signal(SIGTERM) for SIGTERM
- Cleanup: remove .kasmos/layout.kdl, .kasmos/prompts/, .kasmos/scripts/, .kasmos/cmd.pipe (keep state.json + report.md)

#### Parallel Opportunities

- T062 (artifact cleanup) is independent

#### Dependencies

Depends on WP05 (pane operations), WP06 (watcher lifecycle), WP07 (wave engine notifications), WP08 (command system for abort).

#### Risks & Mitigations

- **Risk**: Crash detection races with normal pane close → **Mitigation**: Check WP state before marking as crashed
- **Risk**: Signal during shutdown causes double cleanup → **Mitigation**: AtomicBool shutdown flag, check before each step

---

## Phase 6: Integration

### Work Package WP11: CLI Entry Point & Integration (Priority: P1)

**Goal**: Wire all modules together into the kasmos CLI with launch/status/attach/stop subcommands, end-to-end integration test, and post-run summary report generation.

**Independent Test**: Run `kasmos launch` with a test feature directory, verify full lifecycle from layout generation through wave execution to completion. Run `kasmos status` and verify output. Run `kasmos stop` and verify graceful shutdown. Verify post-run report content.

**Prompt**: [`tasks/WP11-cli-integration.md`](tasks/WP11-cli-integration.md)

#### Included Subtasks

- [ ] [T063] `kasmos launch <feature>` command
- [ ] [T064] `kasmos status [<feature>]` command
- [ ] [T065] `kasmos attach <feature>` command
- [ ] [T066] `kasmos stop [<feature>]` command
- [ ] [T067] Wire all modules with anyhow error propagation
- [ ] [T068] End-to-end integration test
- [ ] [T069] Generate post-run summary report

#### Implementation Notes

- Use clap with subcommands: `kasmos {launch|status|attach|stop} [args]`
- `launch`: read specs → build DAG → generate layout → generate prompts → create session → start wave engine → start watchers → start command reader
- `status`: load state.json, format and display
- `attach`: check session exists, reattach, reconcile state
- `stop`: send abort to running orchestration, wait for graceful shutdown
- Integration test: use tempdir with mock WP specs, mock zellij binary (shell script), verify state transitions
- Report: markdown table with columns: WP, Duration, Completion Method, Status

#### Parallel Opportunities

- T064-T066 (status/attach/stop commands) can be developed in parallel once T063 (launch) establishes the wiring pattern

#### Dependencies

Depends on ALL previous work packages (WP01-WP10).

#### Risks & Mitigations

- **Risk**: Integration reveals interface mismatches → **Mitigation**: Define trait interfaces in WP01, implement against them
- **Risk**: Mock Zellij insufficient for integration test → **Mitigation**: Design mock to cover core commands (list-sessions, list-panes, new-session)

---

## Dependency & Execution Summary

### Sequence

```
Wave 1: WP01 ──┬── Wave 2: WP03 ──┬── Wave 3: WP05 ──┬── Wave 4: WP07 ──┬── Wave 5: WP09 ──┬── Wave 6: WP11
                │                  │                   │                  │                  │
        WP02 ──┘           WP04 ──┘           WP06 ──┘           WP08 ──┘           WP10 ──┘
```

### Parallelization

- **Wave 1**: WP01 + WP02 (fully parallel, no deps)
- **Wave 2**: WP03 + WP04 (fully parallel, both depend on Wave 1)
- **Wave 3**: WP05 + WP06 (fully parallel, WP05 needs Wave 1+2, WP06 needs Wave 1)
- **Wave 4**: WP07 + WP08 (fully parallel, both need Wave 1-3)
- **Wave 5**: WP09 + WP10 (fully parallel, both need Wave 3+)
- **Wave 6**: WP11 (sequential, depends on all)

### MVP Scope

**WP01 + WP02 + WP03 + WP04 + WP05** = minimal launchable orchestrator (can create sessions and launch panes, but no auto-detection or wave progression). Add WP06 + WP07 for auto-detection and wave engine.

## Subtask Index

| ID | Summary | WP | Priority | Parallel? |
|----|---------|-----|----------|-----------|
| T001 | Define orchestration data model | WP01 | P1 | Yes |
| T002 | Config loading (CLI + env + TOML) | WP01 | P1 | No |
| T003 | Error types via thiserror | WP01 | P1 | No |
| T004 | WPState machine transitions | WP01 | P1 | No |
| T005 | Logging setup via tracing | WP01 | P1 | No |
| T006 | Parse kitty-specs directory | WP02 | P1 | Yes |
| T007 | Extract YAML frontmatter | WP02 | P1 | Yes |
| T008 | Build dependency DAG | WP02 | P1 | No |
| T009 | Topological sort (Kahn's) | WP02 | P1 | No |
| T010 | Cycle detection | WP02 | P1 | No |
| T011 | Compute wave groups | WP02 | P1 | No |
| T012 | KDL layout template engine | WP03 | P1 | No |
| T013 | 3-column layout structure | WP03 | P1 | No |
| T014 | Adaptive grid sizing | WP03 | P1 | No |
| T015 | Embed pane commands (stdin pipe) | WP03 | P1 | No |
| T016 | Per-pane cwd to worktree | WP03 | P1 | No |
| T017 | KDL name attribute per pane | WP03 | P1 | No |
| T018 | Write KDL to .kasmos/ | WP03 | P1 | No |
| T019 | Validate KDL syntax | WP03 | P1 | Yes |
| T020 | Prompt template struct | WP04 | P1 | Yes |
| T021 | Dependency context injection | WP04 | P1 | No |
| T022 | AGENTS.md inclusion | WP04 | P1 | No |
| T023 | Write prompts to .kasmos/prompts/ | WP04 | P1 | No |
| T024 | Shell wrapper scripts | WP04 | P1 | No |
| T025 | Validate OpenCode in PATH | WP04 | P1 | Yes |
| T026 | Create Zellij session from KDL | WP05 | P1 | No |
| T027 | Launch initial wave panes | WP05 | P1 | No |
| T028 | Pane ID discovery | WP05 | P1 | No |
| T029 | Pane lifecycle (open/close/restart) | WP05 | P1 | No |
| T030 | Focus/zoom operations | WP05 | P1 | No |
| T031 | Session attach/detach | WP05 | P1 | No |
| T032 | Prompt injection failure detection | WP05 | P1 | No |
| T033 | Filesystem watcher (notify crate) | WP06 | P1 | No |
| T034 | Parse frontmatter lane transitions | WP06 | P1 | No |
| T035 | Read-retry with debounce | WP06 | P1 | No |
| T036 | Git activity detection | WP06 | P1 | Yes |
| T037 | File marker detection | WP06 | P1 | Yes |
| T038 | Signal deduplication | WP06 | P1 | No |
| T039 | Emit events via mpsc | WP06 | P1 | No |
| T040 | Wave progression logic | WP07 | P1 | No |
| T041 | Continuous mode | WP07 | P1 | No |
| T042 | Wave-gated mode | WP07 | P1 | No |
| T043 | Capacity limiting | WP07 | P1 | No |
| T044 | Partial wave failure policy | WP07 | P1 | No |
| T045 | FIFO command input | WP08 | P2 | No |
| T046 | Command grammar parsing | WP08 | P2 | No |
| T047 | Restart command | WP08 | P2 | Yes |
| T048 | Pause command | WP08 | P2 | Yes |
| T049 | Status command | WP08 | P2 | Yes |
| T050 | Focus/zoom commands | WP08 | P2 | Yes |
| T051 | Abort command | WP08 | P2 | Yes |
| T052 | Force-advance command | WP08 | P2 | Yes |
| T053 | Retry command | WP08 | P2 | Yes |
| T054 | Serialize state to JSON | WP09 | P2 | No |
| T055 | Atomic write (tmp + rename) | WP09 | P2 | No |
| T056 | State reconciliation on reattach | WP09 | P2 | No |
| T057 | Stale state detection | WP09 | P2 | Yes |
| T058 | Pane crash detection | WP10 | P2 | No |
| T059 | WP state to Failed on crash | WP10 | P2 | No |
| T060 | Graceful shutdown sequence | WP10 | P2 | No |
| T061 | Signal handling (SIGINT/SIGTERM) | WP10 | P2 | No |
| T062 | Artifact cleanup | WP10 | P2 | Yes |
| T063 | kasmos launch command | WP11 | P1 | No |
| T064 | kasmos status command | WP11 | P1 | Yes |
| T065 | kasmos attach command | WP11 | P1 | Yes |
| T066 | kasmos stop command | WP11 | P1 | Yes |
| T067 | Wire modules with error propagation | WP11 | P1 | No |
| T068 | End-to-end integration test | WP11 | P1 | No |
| T069 | Post-run summary report | WP11 | P1 | No |
