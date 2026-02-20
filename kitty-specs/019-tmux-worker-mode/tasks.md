# Work Packages: Tmux Worker Mode

**Inputs**: Design documents from `kitty-specs/019-tmux-worker-mode/`
**Prerequisites**: plan.md (required), spec.md (6 user stories, 17 FRs, 6 SCs), research.md (8 design decisions), data-model.md (entity definitions)

**Tests**: Required per constitution ("All features must have corresponding tests"). Unit tests use mock TmuxCLI for isolated testing. Integration tests gated behind `KASMOS_INTEGRATION=1`.

**Organization**: 44 fine-grained subtasks (`T001`-`T044`) roll up into 7 work packages (`WP01`-`WP07`). Each work package is independently deliverable. Structure follows the plan's three implementation waves. WP07 is pre-completed (constitution already amended during planning).

**Prompt Files**: Each work package references a matching prompt file in `kitty-specs/019-tmux-worker-mode/tasks/`.

---

## Work Package WP01: TmuxCLI Wrapper Interface (Priority: P0)

**Goal**: Define and implement the `TmuxCLI` interface and `tmuxExec` real implementation that wraps all tmux CLI interactions via `os/exec`. This is the foundation all tmux functionality builds on.
**Independent Test**: `go build ./internal/worker/...` compiles. Unit tests with a mock TmuxCLI pass. Integration tests with real tmux (behind `KASMOS_INTEGRATION=1`) exercise basic pane operations.
**Prompt**: `kitty-specs/019-tmux-worker-mode/tasks/WP01-tmux-cli-wrapper.md`
**Estimated Size**: ~400 lines

### Included Subtasks
- [x] T001 Define TmuxCLI interface, PaneInfo struct, and error types in `internal/worker/tmux_cli.go`
- [x] T002 Implement `tmuxExec` base struct with command execution helper and constructor
- [x] T003 [P] Implement pane lifecycle methods: SplitWindow, KillPane, SelectPane
- [x] T004 [P] Implement pane movement methods: JoinPane (with JoinOpts for both park and show directions), and window management: NewWindow
- [x] T005 Implement pane query methods: ListPanes (with PaneInfo parsing, supports `-s` for session-wide), CapturePane, DisplayMessage, Version
- [x] T006 Implement environment methods: SetEnvironment, ShowEnvironment, UnsetEnvironment, SetPaneOption
- [x] T041 [P] Unit tests for TmuxCLI: mock interface, test `parsePaneList`, test error wrapping in `internal/worker/tmux_cli_test.go`

### Implementation Notes
- All methods shell out to `tmux` via `os/exec.Command`. No Go tmux library.
- All TmuxCLI methods accept `context.Context` as first parameter for timeout/cancellation. The `run` helper should use `exec.CommandContext`.
- The `tmuxExec` struct holds the tmux binary path (resolved via `exec.LookPath` at construction).
- Size parameters are `string` type (not `int`) to support percentage notation (`"50%"`) and absolute columns (`"80"`).
- Format strings for `list-panes` use `#{pane_id} #{pane_pid} #{pane_dead} #{pane_dead_status}` (research.md section 1).
- Environment tagging uses `tmux set-environment` / `show-environment` at session scope with unique-per-worker keys: `KASMOS_PANE_<worker_id>=<pane_id>`. Additional tags: `KASMOS_SESSION_ID`, `KASMOS_PARKING`, `KASMOS_DASHBOARD`. See research.md section 2.
- Pane retention: `SetPaneOption(ctx, paneID, "remain-on-exit", "on")` must be called after pane creation. Without this, dead panes disappear from `list-panes` and `capture-pane` fails. See research.md section 3a.
- Minimum tmux version: 2.6+ (research.md section 7). Version check is advisory (warn, not error).

### Parallel Opportunities
- T003 and T004 can proceed in parallel (different tmux operations, same file).
- WP01 can run in parallel with WP07 (constitution amendment, no shared code).

### Dependencies
- None (starting package).

### Risks & Mitigations
- tmux output format varies across versions -> Pin to documented format strings, test against tmux 3.x+.
- `os/exec` errors are opaque -> Wrap with descriptive error messages including the tmux command that failed.
- tmux "no space for new pane" error on narrow terminal -> Define `IsNoSpace(err)` check that matches tmux stderr containing "no space for new pane". Surface as user-friendly message in spawn failure.

---

## Work Package WP02: TmuxBackend & tmuxHandle Implementation (Priority: P0)

**Goal**: Implement `TmuxBackend` (the `WorkerBackend` interface for tmux) and `tmuxHandle` (the `WorkerHandle` for interactive panes). Extend `WorkerHandle` with `Interactive() bool`. This delivers the core backend that spawns, tracks, and manages worker panes.
**Independent Test**: `TmuxBackend.Spawn()` creates a tagged tmux pane. `tmuxHandle.Interactive()` returns true. `SubprocessBackend` remains unchanged. `go test ./internal/worker/...` passes with mock TmuxCLI.
**Prompt**: `kitty-specs/019-tmux-worker-mode/tasks/WP02-tmux-backend-handle.md`
**Estimated Size**: ~500 lines

### Included Subtasks
- [x] T007 Define TmuxBackend, ManagedPane, PaneStatus, and ReconnectedWorker types in `internal/worker/tmux.go`
- [x] T008 Add `Interactive() bool` to WorkerHandle interface in `internal/worker/backend.go` and implement `Interactive() -> false` on `subprocessHandle` in `internal/worker/subprocess.go`
- [x] T009 Implement `TmuxBackend.Init()`: capture kasmos pane ID AND window ID via `DisplayMessage`, create parking window, set session/parking/dashboard env tags
- [x] T010 Implement `TmuxBackend.Spawn()`: build command, create pane via SplitWindow, tag with worker/session IDs, track in managedPanes
- [x] T011 Implement `tmuxHandle` struct: `Interactive() -> true`, `Stdout() -> nil`, `Wait()` via exitCh, `Kill()`, `PID()`, `NotifyExit()`, `CaptureOutput()`
- [x] T012 Implement pane visibility management: `ShowPane()`, `HidePane()`, `SwapActive()` using JoinPane (both directions)
- [x] T013 Implement `PollPanes()`: list all managed panes, detect dead/missing, return `[]PaneStatus`
- [x] T014 Implement `Reconnect()` (read KASMOS_PANE_* env vars, cross-reference with list-panes -s, clean stale tags via UnsetEnvironment) and `Cleanup()` (kill parking window, kill worker panes, unset all KASMOS_* env vars)
- [x] T042 [P] Unit tests for TmuxBackend with mock TmuxCLI: Spawn, SwapActive, PollPanes, Reconnect, tmuxHandle lifecycle in `internal/worker/tmux_test.go`

### Implementation Notes
- `TmuxBackend` implements `WorkerBackend` interface. Compile-time check: `var _ WorkerBackend = (*TmuxBackend)(nil)`.
- The parking window (`kasmos-parking`) holds non-visible panes. Only one pane is ever visible alongside kasmos.
- `tmuxHandle.Wait()` blocks on `exitCh` channel, closed by `NotifyExit()` (called from TUI tick poller).
- SwapActive sequence: join-pane -d current to parking -> join-pane new from parking to kasmos window -> select-pane new (research.md section 4).
- Pane tagging: `KASMOS_PANE_<worker_id>=<pane_id>` per worker. Session metadata: `KASMOS_SESSION_ID`, `KASMOS_PARKING`, `KASMOS_DASHBOARD`. All at session scope via `set-environment`. See research.md section 2.
- After `SplitWindow` in `Spawn()`, immediately call `SetPaneOption(ctx, paneID, "remain-on-exit", "on")` so dead panes survive for polling and capture.
- Mutex protection for pane operations to prevent race conditions during rapid switching.

### Parallel Opportunities
- T007 and T008 can proceed in parallel (different files: tmux.go vs backend.go/subprocess.go).

### Dependencies
- Depends on WP01 (TmuxCLI interface must exist to inject into TmuxBackend).

### Risks & Mitigations
- join-pane race with rapid switching -> Serialize via mutex, debounce in TUI layer.
- Parking window visible to user -> Name it `kasmos-parking`, document in help text.
- Multiple kasmos instances in same tmux session -> Tag panes with kasmos session ID, only manage matching panes.

---

## Work Package WP03: CLI Flag & Backend Selection (Priority: P0)

**Goal**: Add the `--tmux` CLI flag, implement backend selection logic in `main.go`, validate flag combinations, and add tmux mode state to the TUI Model. After this WP, `kasmos --tmux` creates a TmuxBackend instead of SubprocessBackend.
**Independent Test**: `kasmos --tmux` inside tmux creates a TmuxBackend. `kasmos --tmux` outside tmux shows a clear error. `kasmos --tmux -d` shows mutual exclusivity error.
**Prompt**: `kitty-specs/019-tmux-worker-mode/tasks/WP03-cli-flag-backend-selection.md`
**Estimated Size**: ~350 lines

### Included Subtasks
- [x] T015 Add `--tmux` flag to cobra command in `cmd/kasmos/main.go`
- [x] T016 Implement backend selection logic: `--tmux` flag detection, `$TMUX` environment validation, `NewTmuxBackend()` construction
- [x] T017 Validate `--tmux` and `-d` mutual exclusivity with clear error message (FR-016)
- [x] T018 Add `tmuxMode bool`, `tmuxBackend *worker.TmuxBackend`, `kasmosPaneID string`, and `activePaneID string` fields to TUI Model in `internal/tui/model.go`
- [x] T019 Update `NewModel()` to accept and store tmux mode state; add `backendName()` helper that returns `m.backend.Name()` (e.g., "tmux" or "subprocess"); update status bar rendering in `panels.go` to show backend indicator alongside task source mode (e.g., `mode: spec-kitty [tmux]`) when `tmuxMode` is true. Do NOT modify `modeName()` -- it must continue returning the task source type.

### Implementation Notes
- Backend selection order: `--tmux` flag -> config `TmuxMode` (WP06) -> default subprocess.
- `$TMUX` check: `os.Getenv("TMUX") != ""`. If `--tmux` is set but `$TMUX` is empty, return error with guidance.
- The TmuxBackend needs `Init()` called after creation but before TUI starts (captures kasmos pane ID).
- `main.go` changes are surgical: add flag, add backend construction branch, add validation.
- Model fields added here are populated by tmux initialization in WP04.

### Parallel Opportunities
- None within this WP (sequential flag -> validation -> backend -> model changes).

### Dependencies
- Depends on WP02 (TmuxBackend must be implemented to construct it).

### Risks & Mitigations
- User runs `kasmos --tmux` outside tmux -> Clear error: "tmux mode requires running inside a tmux session. Start one with: tmux new-session -s kasmos"
- TmuxBackend.Init() failure -> Graceful error before TUI starts, not a panic mid-render.

---

## Work Package WP04: Pane Switching & Focus Management (Priority: P1)

**Goal**: Wire tmux pane operations into the TUI. When a worker is selected in the dashboard table, the right-side pane swaps to show that worker's live terminal. Focus automatically moves to the worker pane. Implements FR-006, FR-007, FR-008, and the tmux initialization flow.
**Independent Test**: In tmux mode, spawn a worker. Verify the worker's terminal appears in the right pane. Select a different worker; verify pane swaps. Verify focus moves to the worker pane on selection.
**Prompt**: `kitty-specs/019-tmux-worker-mode/tasks/WP04-pane-switching-focus.md`
**Estimated Size**: ~550 lines

### Included Subtasks
- [x] T020 Define tmux-specific message types in `internal/tui/messages.go`: `paneSwappedMsg`, `paneExitedMsg`, `paneDetectedMsg`, `tmuxInitMsg`
- [x] T021 Implement `tmuxInitCmd()` in `internal/tui/commands.go`: calls `TmuxBackend.Init()`, returns `tmuxInitMsg` with kasmos pane ID, kasmos window ID, and parking window ID
- [x] T022 Implement `paneSwapCmd()` in `internal/tui/commands.go`: calls `TmuxBackend.SwapActive()`, returns `paneSwappedMsg`
- [x] T023 Implement `paneFocusCmd()` in `internal/tui/commands.go`: calls `TmuxCLI.SelectPane()` for worker focus, and dashboard focus return
- [x] T024 Update worker selection handling in `internal/tui/update.go`: on selection change in tmux mode, emit `paneSwapCmd` instead of refreshing viewport content
- [x] T025 Update `renderViewport()` in `internal/tui/panels.go`: in tmux mode with no workers, render placeholder text indicating the right column is reserved for worker panes
- [x] T044 Implement narrow terminal adaptation for tmux mode: when terminal width is below the split threshold, alternate between full-width dashboard and full-width worker pane instead of side-by-side split. Use the existing fullscreen toggle (`f` key) pattern. Detect width on `tea.WindowSizeMsg` and adjust `join-pane` size or skip splitting. See spec.md edge case L124 and research.md section 4.

### Implementation Notes
- `tmuxInitCmd` runs as a `tea.Cmd` from `Init()` when `tmuxMode` is true. Must complete before workers can be spawned.
- `tmuxInitMsg` handler stores `kasmosPaneID`, `kasmosWindowID`, and `parkingWindow` in model state.
- Pane swap on selection change: detect `selectedWorkerID` change in table navigation, fire `paneSwapCmd`.
- First worker spawn: no existing visible pane, so `ShowPane` (not `SwapActive`).
- The viewport in tmux mode is cosmetic (shows status text), not the output viewport. The real output is in the tmux pane.
- SC-002: Pane switch must complete in under 1 second (join-pane + select-pane is typically <100ms).
- SC-005 worker->dashboard return: Standard tmux navigation (prefix + arrow) handles this. Consider adding a kasmos-specific tmux keybind during Init (`bind-key -n M-d select-pane -t <dashboard>`) and showing the hint in the status bar.

### Parallel Opportunities
- T020 and T025 can proceed in parallel (messages.go vs panels.go, no dependency).
- T044 can proceed after T024 (needs pane swap logic to exist).

### Dependencies
- Depends on WP03 (CLI flag and tmuxMode state must exist in Model).

### Risks & Mitigations
- Pane swap during rapid navigation -> Debounce: only fire pane swap after selection is stable for 100ms, or serialize with a "swap in progress" guard.
- tmuxInitCmd failure -> Show error in status bar, disable tmux features, fall back gracefully.
- Viewport rendering in tmux mode -> Keep it simple: static text, not a live updating viewport.

---

## Work Package WP05: Exit Detection, Output Capture & Key Disabling (Priority: P1)

**Goal**: Detect worker exit via tmux pane polling on the existing tick timer. Capture pane output for session ID extraction. Handle externally killed panes. Skip subprocess output reading for interactive handles. Disable AI helper keys in tmux mode. Implements FR-009, FR-011, FR-014, FR-017.
**Independent Test**: Spawn a worker in tmux mode, let it exit. Verify dashboard updates status within 2 seconds. Verify session ID is extracted. Manually kill a worker pane; verify kasmos marks it as killed. Verify Analyze and GenPrompt keys are hidden.
**Prompt**: `kitty-specs/019-tmux-worker-mode/tasks/WP05-exit-detection-output.md`
**Estimated Size**: ~450 lines

### Included Subtasks
- [x] T026 Add tmux pane polling to `tickMsg` handler in `internal/tui/update.go`: call `TmuxBackend.PollPanes()`, emit `paneExitedMsg` for dead panes
- [x] T027 Handle `paneExitedMsg` in update.go: capture pane output via `tmuxHandle.CaptureOutput()`, extract session ID, emit `workerExitedMsg` to reuse existing exit flow
- [x] T028 Handle externally killed panes: when PollPanes reports a pane as missing, emit `workerKilledMsg` for that worker (FR-014)
- [x] T029 Update `workerSpawnedMsg` handler in update.go: check `handle.Interactive()`, skip `readWorkerOutput()` and `waitWorkerCmd()` for interactive handles
- [x] T030 Disable AI helper keys (Analyze, GenPrompt) in `updateKeyStates()` in `internal/tui/keys.go` when `tmuxMode` is active (FR-017)
- [x] T031 Implement auto-focus return: when the focused worker's pane exits, fire `paneFocusCmd` targeting the kasmos dashboard pane (FR-009)

### Implementation Notes
- Tick polling: only runs when `tmuxMode` is true. On each `tickMsg`, call `TmuxBackend.PollPanes()`. This returns `[]PaneStatus` with Dead/Missing flags.
- For dead panes: call `tmuxHandle.CaptureOutput()` (which does `tmux capture-pane -p -t <pane> -S -`), then `extractSessionID()` on the captured text. Reuse existing `extractSessionID` from `commands.go`.
- For missing panes (FR-014): emit `workerKilledMsg` directly.
- `workerSpawnedMsg` handler currently unconditionally calls `readWorkerOutput` and `waitWorkerCmd`. Add conditional: `if !handle.Interactive() { ... }`.
- AI helper disabling: in `updateKeyStates()`, when `m.tmuxMode` is true, set `m.keys.Analyze.SetEnabled(false)` and `m.keys.GenPrompt.SetEnabled(false)`.
- Auto-focus return: on `paneExitedMsg` where the exited pane was the active one, fire `paneFocusCmd` to kasmos pane.

### Parallel Opportunities
- T029 and T030 can proceed in parallel (different files: update.go vs keys.go).
- T028 is independent of T027 (different code paths for dead vs missing).

### Dependencies
- Depends on WP04 (message types and pane commands must exist).

### Risks & Mitigations
- `capture-pane` on a dead pane with large scrollback -> tmux handles this efficiently; content is in-memory.
- Polling overhead -> `tmux list-panes` is fast (<10ms). One call per second is negligible.
- Race between poll and pane operations -> PollPanes uses the existing managed panes map; mutex protects concurrent access.

---

## Work Package WP06: Session Persistence, Config & Reattach (Priority: P2)

**Goal**: Persist the backend mode in session metadata. Add `TmuxMode` to config. Implement reattach logic that infers tmux mode from session file and reconnects to surviving worker panes. Implements FR-002, FR-004, FR-013.
**Independent Test**: Start `kasmos --tmux`, spawn workers, exit kasmos. Restart with `kasmos --attach`; verify it auto-selects tmux backend and reconnects to surviving panes. Set `tmux_mode = true` in config; run `kasmos` inside tmux with no flags; verify tmux mode activates. Run outside tmux; verify fallback to subprocess with notice.
**Prompt**: `kitty-specs/019-tmux-worker-mode/tasks/WP06-session-persistence-config.md`
**Estimated Size**: ~400 lines

### Included Subtasks
- [x] T032 Add `BackendMode string` field to `SessionState` in `internal/persist/schema.go` with JSON tag `"backend_mode,omitempty"`
- [x] T033 Add `TmuxMode bool` field to `Config` in `internal/config/config.go` with TOML tag `"tmux_mode"`
- [x] T034 Update `buildSessionState()` in `internal/tui/model.go` to include `BackendMode` (set to `m.backend.Name()`)
- [x] T035 Update reattach logic in `cmd/kasmos/main.go`: read `BackendMode` from loaded session, auto-select TmuxBackend if "tmux"
- [x] T036 Implement config-based tmux activation in `cmd/kasmos/main.go`: if `cfg.TmuxMode == true` and `$TMUX` is set, enable tmux mode; if `$TMUX` is not set, fall back to subprocess with a notice
- [ ] T037 Implement reattach pane reconnection: call `TmuxBackend.Reconnect()` during `--attach`, restore worker pane mappings, update worker states for dead/surviving panes
- [ ] T043 Add `tmux_mode` boolean toggle to settings form in `internal/tui/settings.go`: new `settingsRowTmuxMode` kind with left/right cycling, wired to `cfg.TmuxMode`, displayed as "tmux mode: on/off"

### Implementation Notes
- `BackendMode` defaults to empty string (backward compatible with existing sessions, treated as "subprocess").
- `buildSessionState()` already exists in model.go. Add one line: `BackendMode: m.backend.Name()`.
- Reattach flow: load session -> check BackendMode -> if "tmux", create TmuxBackend -> Init() -> Reconnect(sessionTag) -> restore workers with pane mappings.
- Config fallback: `if cfg.TmuxMode && os.Getenv("TMUX") == "" { log.Printf("notice: tmux mode configured but not in tmux session, falling back to subprocess"); useTmux = false }`.
- The `--tmux` flag overrides config. Priority: `--tmux` flag > `cfg.TmuxMode` > default (subprocess).
- On reattach, surviving workers get new tmuxHandle instances connected to rediscovered panes. Dead workers get status updated.

### Parallel Opportunities
- T032 and T033 can proceed in parallel (different packages: persist vs config).

### Dependencies
- Depends on WP05 (full tmux TUI integration must work before persistence makes sense).

### Risks & Mitigations
- Stale session file points to dead tmux session -> Reconnect returns empty list; workers treated as killed (same as existing orphan recovery).
- Config file doesn't exist -> `config.Load()` already returns defaults; `TmuxMode` defaults to `false`.
- Reattach races with another kasmos instance -> Existing PID check in main.go prevents this.

---

## Work Package WP07: Constitution Amendment (Priority: P2) - DONE

**Goal**: Update `.kittify/memory/constitution.md` to reflect the dual-mode architecture introduced by tmux worker mode. Amend three principles as identified in plan.md.
**Status**: **Pre-completed** - all three amendments were applied to constitution.md (v2.1.0) during the planning phase. Verified 2026-02-19.
**Prompt**: `kitty-specs/019-tmux-worker-mode/tasks/WP07-constitution-amendment.md`
**Estimated Size**: ~200 lines

### Included Subtasks
- [ ] T038 Amend worker mode principle: "Workers are headless subprocesses" -> "Workers are subprocesses (headless by default, interactive tmux panes when configured)"
- [ ] T039 Amend session continuation principle: "Session continuation over interactivity" -> "Headless by default; interactive via tmux when workflows require it. Session continuation remains available in both modes."
- [ ] T040 Update Go version reference: "Go (1.23+)" -> "Go (1.24+)"

### Implementation Notes
- Constitution file: `.kittify/memory/constitution.md`.
- These are additive amendments - subprocess mode behavior is unchanged.
- Read the existing constitution to find exact text to replace (wording may differ slightly from plan.md quotes).
- Each amendment should be a surgical text replacement, not a rewrite of surrounding content.

### Parallel Opportunities
- WP07 can run in parallel with any other WP (documentation only, no code dependencies).

### Dependencies
- None (documentation-only package).

### Risks & Mitigations
- Constitution wording doesn't match plan.md quotes exactly -> Read the file first, find the actual text, then amend.

---

## Dependency & Execution Summary

- **Sequence**: WP01 -> WP02 -> WP03 -> WP04 -> WP05 -> WP06 (main chain)
- **Parallel**: WP07 is pre-completed (constitution already amended during planning).
- **Parallelization**: After WP03, all remaining WPs are sequential.
- **MVP Scope**: WP01 + WP02 + WP03 + WP04 + WP05 deliver a working tmux mode. WP06 (persistence/config/settings) is polish. WP07 is done.

### Dependency Graph

```
WP01 (TmuxCLI)          WP07 (Constitution)
     |                        |
     v                     (independent)
WP02 (Backend+Handle)
     |
     v
WP03 (CLI+Flag)
     |
     v
WP04 (Pane Switching)
     |
     v
WP05 (Exit Detection)
     |
     v
WP06 (Persistence+Config)
```

---

## Subtask Index (Reference)

| Subtask ID | Summary | Work Package | Priority | Parallel? |
|------------|---------|--------------|----------|-----------|
| T001 | Define TmuxCLI interface, PaneInfo, error types | WP01 | P0 | No |
| T002 | Implement tmuxExec base struct | WP01 | P0 | No |
| T003 | Implement pane lifecycle methods | WP01 | P0 | Yes |
| T004 | Implement JoinPane (both directions) + NewWindow | WP01 | P0 | Yes |
| T005 | Implement pane query methods (+ `-s`, DisplayMessage) | WP01 | P0 | No |
| T006 | Implement environment + pane option methods | WP01 | P0 | No |
| T007 | Define TmuxBackend, ManagedPane, PaneStatus types | WP02 | P0 | Yes |
| T008 | Add Interactive() to WorkerHandle + subprocess impl | WP02 | P0 | Yes |
| T009 | Implement Init with pane+window IDs and env tags | WP02 | P0 | No |
| T010 | Implement TmuxBackend.Spawn() | WP02 | P0 | No |
| T011 | Implement tmuxHandle struct | WP02 | P0 | No |
| T012 | Implement ShowPane/HidePane/SwapActive | WP02 | P0 | No |
| T013 | Implement PollPanes() | WP02 | P0 | No |
| T014 | Implement Reconnect/Cleanup with env tag lifecycle | WP02 | P0 | No |
| T015 | Add --tmux flag to cobra | WP03 | P0 | No |
| T016 | Implement backend selection logic | WP03 | P0 | No |
| T017 | Validate --tmux and -d mutual exclusivity | WP03 | P0 | No |
| T018 | Add tmux state fields to TUI Model | WP03 | P0 | No |
| T019 | Update NewModel + add backendName() + status bar indicator | WP03 | P0 | No |
| T020 | Define tmux message types | WP04 | P1 | Yes |
| T021 | Implement tmuxInitCmd with pane+window+parking IDs | WP04 | P1 | No |
| T022 | Implement paneSwapCmd | WP04 | P1 | No |
| T023 | Implement paneFocusCmd | WP04 | P1 | No |
| T024 | Update selection handling for tmux swap | WP04 | P1 | No |
| T025 | Render tmux placeholder in viewport | WP04 | P1 | Yes |
| T026 | Add tmux polling to tick handler | WP05 | P1 | No |
| T027 | Handle paneExitedMsg | WP05 | P1 | No |
| T028 | Handle externally killed panes | WP05 | P1 | Yes |
| T029 | Skip readWorkerOutput for interactive handles | WP05 | P1 | Yes |
| T030 | Disable AI helper keys in tmux mode | WP05 | P1 | Yes |
| T031 | Auto-focus return on worker exit | WP05 | P1 | No |
| T032 | Add BackendMode to SessionState | WP06 | P2 | Yes |
| T033 | Add TmuxMode to Config | WP06 | P2 | Yes |
| T034 | Update buildSessionState | WP06 | P2 | No |
| T035 | Update reattach logic | WP06 | P2 | No |
| T036 | Config-based tmux activation | WP06 | P2 | No |
| T037 | Reattach pane reconnection | WP06 | P2 | No |
| T038 | Amend worker mode principle | WP07 | P2 | Yes |
| T039 | Amend session continuation principle | WP07 | P2 | Yes |
| T040       | Update Go version reference | WP07 | P2 | Yes |
| T041       | Unit tests for TmuxCLI (parsePaneList, error wrapping) | WP01 | P0 | Yes |
| T042       | Unit tests for TmuxBackend with mock TmuxCLI | WP02 | P0 | Yes |
| T043       | Add tmux_mode toggle to settings form | WP06 | P2 | No |
| T044       | Narrow terminal adaptation for tmux mode | WP04 | P1 | No |
