# Task Breakdown: Spec-Kitty Feature Browser

**Feature**: 022-spec-kitty-feature-browser
**Date**: 2026-02-20
**Work Packages**: 7 (WP01-WP07)
**Total Subtasks**: 38

## Overview

This breakdown decomposes the feature browser into 5 work packages following the bubbletea Elm architecture: types/scanner foundation, model state wiring, View rendering, Update interaction logic, and tests. WP03 (rendering) and WP04 (interaction) are parallelizable since bubbletea's View and Update functions are independent.

All new code goes in a single new file `internal/tui/browser.go` (plus `browser_test.go` for tests). Integration touches `model.go`, `update.go`, `launcher.go`, and `keys.go` for wiring.

## Dependency Graph

```
WP01 (types/scanner)
  |
  v
WP02 (state/wiring)                WP06 [P] (tmux theming)
  |                                 WP07 [P] (interactive opencode)
  +----+----+
  |         |
  v         v
WP03 [P]  WP04 [P]     <-- parallel
  |         |
  +----+----+
       |
       v
     WP05 (tests)
```

**Parallel opportunity**: WP03 and WP04 can execute simultaneously after WP02 completes. WP06 and WP07 are fully independent of WP01-05 (they touch `internal/worker/`, not `internal/tui/`) and can run at any time in parallel.

**Reference note**: Line numbers in WP prompt files are guidance only. If source files shift, use the quoted code context and symbols as the authoritative insertion points.

---

## Phase 1: Foundation

### WP01 - Core Types, Scanner, and Phase Detection

**Prompt**: `tasks/WP01-core-types-scanner.md`
**Summary**: Define the type system (FeaturePhase, FeatureEntry, lifecycleAction), implement the filesystem scanner that discovers features in `kitty-specs/`, classify phases by file existence, and provide utility functions (parse directory names, filter entries).
**Priority**: P0 (foundation - everything depends on this)
**Dependencies**: none
**Estimated prompt size**: ~400 lines

**Subtasks**:
- [x] T001: Define FeaturePhase enum with String() and phaseBadge() style helper
- [x] T002: Define FeatureEntry struct (Number, Slug, Dir, Phase, WPCount)
- [x] T003: Define lifecycleAction struct and actionsForPhase() mapping
- [x] T004: Implement parseFeatureDir() - extract number and slug from directory name
- [x] T005: Implement scanFeatures() - glob, classify phases, sort by number descending
- [x] T006: Implement filterFeatures() - case-insensitive substring match on slugs

**Implementation sketch**:
1. Create `internal/tui/browser.go` with package declaration and imports
2. Define types (T001-T003) at top of file
3. Implement pure functions (T004-T006) below types
4. All functions are pure (no Model receiver) for testability

**Risks**:
- Phase detection must match spec exactly: spec.md-only = SpecOnly, plan.md present but no WPs = PlanReady, WP*.md present = TasksReady
- parseFeatureDir must handle edge cases (no number prefix, single-segment names)

---

## Phase 2: Integration

### WP02 - Browser Model State and Launcher Wiring

**Prompt**: `tasks/WP02-model-state-launcher-wiring.md`
**Summary**: Add browser state fields to the Model struct, implement open/close helpers, add the `b` key handler to the launcher, add the menu item, and wire the browser into the View and Update dispatch chains.
**Priority**: P0 (integration - connects browser to existing launcher)
**Dependencies**: WP01
**Estimated prompt size**: ~350 lines

**Subtasks**:
- [x] T007: Add browser state fields to Model struct in model.go
- [x] T008: Implement openFeatureBrowser() and closeFeatureBrowser() methods
- [x] T009: Add `b` key case to updateLauncherKeys() in update.go
- [x] T010: Add "browse features" menu item to launcherMenuItems in launcher.go
- [x] T011: Add showFeatureBrowser dispatches in View() and Update()
- [x] T012: Update updateKeyStates() to handle browser overlay state

**Implementation sketch**:
1. Add fields to Model struct (T007)
2. Wire open/close helpers (T008) - follow restorePicker pattern
3. Add `b` key in updateLauncherKeys (T009) - follows `r`/`s` pattern
4. Add menu item (T010) - single line addition
5. Add dispatch checks (T011) - follows showRestorePicker/showHistory pattern
6. Update key states (T012) - add browser to overlayActive check

**Risks**:
- View dispatch order matters - showFeatureBrowser must be checked before showRestorePicker
- updateKeyStates overlayActive check must include showFeatureBrowser

---

## Phase 3: UI Implementation (parallel)

### WP03 - Browser Rendering (View)

**Prompt**: `tasks/WP03-browser-rendering.md`
**Summary**: Implement all View-side rendering for the feature browser: the backdrop dialog frame, feature entry lines with phase badges and selection highlights, inline tree expansion with ASCII tree chars, filter textinput display, and the empty state.
**Priority**: P1 (rendering - visual layer)
**Dependencies**: WP02
**Estimated prompt size**: ~400 lines
**Parallel**: Can run simultaneously with WP04

**Subtasks**:
- [x] T013: Implement renderFeatureBrowser() main structure (backdrop dialog, header, list, help bar)
- [x] T014: Implement feature entry line rendering (number, slug, phase badge, selection highlight)
- [x] T015: Implement inline tree expansion rendering (ASCII tree chars, action selection)
- [x] T016: Implement filter textinput rendering at bottom of dialog
- [x] T017: Implement empty state rendering ("no features found" with hint)

**Implementation sketch**:
1. renderFeatureBrowser() builds the dialog using dialogStyle + renderWithBackdrop (T013)
2. Feature lines: right-aligned number, slug, phase badge styled per phase (T014)
3. When expanded: insert action lines with `|--` and `'--` tree chars (T015)
4. Filter: show textinput.View() at bottom when active (T016)
5. Empty: centered message with suggestion to press `f` (T017)

**Risks**:
- ASCII tree chars must use only UTF-8 safe characters per AGENTS.md encoding rule
- Phase badge colors must match existing kasmos palette (colorMidGray, colorLightBlue, colorGreen)

---

### WP04 - Browser Interaction Logic (Update)

**Prompt**: `tasks/WP04-browser-interaction.md`
**Summary**: Implement all Update-side interaction logic for the feature browser: the update dispatcher, list navigation with scroll management, feature selection routing (dashboard load vs sub-menu expansion), action selection with spawn dialog pre-fill, filter mode, and back navigation.
**Priority**: P1 (interaction - behavior layer)
**Dependencies**: WP02
**Estimated prompt size**: ~450 lines
**Parallel**: Can run simultaneously with WP03

**Subtasks**:
- [ ] T018: Implement updateFeatureBrowser() dispatcher (route by filter/actions state)
- [ ] T019: Implement list navigation (j/k/up/down, visible window clamping, `f` shortcut from empty state)
- [ ] T020: Implement feature selection (Enter/right -> dashboard load or sub-menu expand)
- [ ] T021: Implement action selection (Enter/right -> spawn dialog with prefill)
- [ ] T022: Implement filter mode (/ activate, keystrokes filter, Enter/Esc confirm/clear)
- [ ] T023: Implement back navigation (Esc/left -> collapse sub-menu or close browser)

**Implementation sketch**:
1. Dispatcher checks featureFilterActive first, then featureActionsOpen, then normal list mode (T018)
2. Navigation: j/k/up/down change featureSelectedIdx within featureFiltered bounds (T019)
3. Enter on tasks-ready: call DetectSourceType + swapTaskSource + transitionFromLauncher (T020)
4. Enter on non-ready: set featureActionsOpen=true, populate actions (T020)
5. Enter on action: build prompt string, call transitionFromLauncher + openSpawnDialogWithPrefill (T021)
6. Filter: / -> featureFilterActive=true, focus textinput; update -> recompute featureFiltered (T022)
7. Esc: context-dependent collapse/close (T023)

**Risks**:
- Selection routing must correctly distinguish tasks-ready features (direct dashboard load) from others (sub-menu)
- Filter must recompute featureFiltered on every keystroke and clamp featureSelectedIdx
- swapTaskSource requires constructing a SpecKittySource from the feature directory

---

## Phase 4: Quality

### WP05 - Tests

**Prompt**: `tasks/WP05-tests.md`
**Summary**: Write comprehensive table-driven tests for all browser pure functions: filesystem scanning, phase detection, action mapping, directory name parsing, and filter matching. Uses temp directories for filesystem tests.
**Priority**: P2 (quality gate)
**Dependencies**: WP03, WP04
**Estimated prompt size**: ~400 lines

Dependency rationale: WP05 tests are exclusively for WP01 pure functions and branches from WP04 (`--base WP04`). WP03 rendering code is not needed (WP02 stubs remain). The WP03 dependency is a sequencing gate ensuring the full browser is implemented before the quality phase, not a code dependency.

**Subtasks**:
- [ ] T024: Test scanFeatures() with temp directory structures at various phases
- [ ] T025: Test phase detection accuracy (spec-only, plan-ready, tasks-ready, invalid dirs)
- [ ] T026: Test actionsForPhase() returns correct actions per phase
- [ ] T027: Test parseFeatureDir() with normal and edge-case directory names
- [ ] T028: Test filterFeatures() (case-insensitive, empty query, no matches)
- [ ] T029: Benchmark scanFeatures() with 50 features to validate NFR-001 (<200ms)

**Implementation sketch**:
1. Create `internal/tui/browser_test.go`
2. Helper function to create temp kitty-specs directory with configurable features
3. Table-driven test for scanFeatures with multiple scenarios (T024-T025)
4. Table-driven test for actionsForPhase covering all phases (T026)
5. Table-driven test for parseFeatureDir edge cases (T027)
6. Table-driven test for filterFeatures (T028)
7. Benchmark scanFeatures for 50-feature dataset and missing-dir error-path coverage (T029)

**Risks**:
- Filesystem tests need temp directories - use t.TempDir() and proper cleanup
- scanFeatures uses relative paths from CWD - tests must chdir or use absolute paths

---

## Phase 5: tmux Integration (independent)

### WP06 - tmux Visual Integration

**Prompt**: `tasks/WP06-tmux-visual-integration.md`
**Summary**: Theme tmux pane borders to match the kasmos bubblegum palette, set worker pane titles (shown in border format), and hide the tmux status bar. Adds `SetOption` and `SetPaneTitle` methods to the `TmuxCLI` interface.
**Priority**: P1 (visual polish)
**Dependencies**: none (independent of WP01-05, touches only `internal/worker/`)
**Estimated prompt size**: ~250 lines

**Subtasks**:
- [x] T030: Add SetOption to TmuxCLI interface (session-level set-option)
- [x] T031: Add SetPaneTitle to TmuxCLI interface (select-pane -T wrapper)
- [ ] T032: Apply kasmos palette theming in TmuxBackend.Init()
- [ ] T033: Set pane title on spawn with worker ID and role
- [ ] T034: Hide tmux status bar (restore on cleanup)

**Implementation sketch**:
1. Extend TmuxCLI interface with two new methods (T030-T031)
2. Call theming options in Init() after parking window creation (T032)
3. Set title per pane in Spawn() after remain-on-exit (T033)
4. Hide/restore status bar in Init()/Cleanup() (T034)

**Risks**:
- `pane-border-lines heavy` requires tmux 3.2+ (degrades gracefully on older tmux)
- Overrides user's tmux theme for the session (acceptable tradeoff)

---

### WP07 - Interactive opencode for tmux Workers

**Prompt**: `tasks/WP07-interactive-opencode.md`
**Summary**: Switch the tmux backend from `opencode run` (headless) to `opencode` (interactive TUI). Uses `--prompt` flag instead of positional args. Adds WorkDir support via tmux `-c` flag. Subprocess backend stays headless.
**Priority**: P0 (bug fix -- current tmux workers are non-interactive contrary to design intent)
**Dependencies**: none (independent of WP01-06, modifies only backend arg construction)
**Estimated prompt size**: ~300 lines

**Subtasks**:
- [x] T035: Separate buildArgs for subprocess (opencode run) vs tmux (opencode interactive)
- [ ] T036: Add WorkDir support to tmux SplitWindow via -c flag
- [ ] T037: Update buildArgs tests for both backends
- [ ] T038: Update architecture documentation to reflect dual-mode design

**Implementation sketch**:
1. Rewrite TmuxBackend.buildArgs() to omit "run", use --prompt flag (T035)
2. Add Dir field to SplitOpts, wire through Spawn (T036)
3. Table-driven tests for both arg builders (T037)
4. Update architecture.md and constitution.md (T038)

**Risks**:
- `--variant` and `-f` flags not available in interactive opencode (silently dropped)
- Long prompts via --prompt need testing (460+ line WP bodies)

---

## Summary Table

| WP | Title | Subtasks | Est. Lines | Dependencies | Parallel |
|----|-------|----------|-----------|--------------|----------|
| WP01 | Core Types, Scanner, Phase Detection | T001-T006 (6) | ~400 | none | - |
| WP02 | Model State and Launcher Wiring | T007-T012 (6) | ~350 | WP01 | - |
| WP03 | Browser Rendering (View) | T013-T017 (5) | ~400 | WP02 | [P] with WP04 |
| WP04 | Browser Interaction Logic (Update) | T018-T023 (6) | ~450 | WP02 | [P] with WP03 |
| WP05 | Tests | T024-T029 (6) | ~400 | WP03, WP04 | - |
| WP06 | tmux Visual Integration | T030-T034 (5) | ~250 | none | [P] with all |
| WP07 | Interactive opencode for tmux Workers | T035-T038 (4) | ~300 | none | [P] with all |

**MVP scope**: WP01-04 delivers the feature browser. WP05 adds tests. WP06-07 add tmux visual integration and fix the interactive worker mode.

**Recommended execution**:
1. WP01 (serial)
2. WP02 (serial, depends on WP01)
3. WP03 + WP04 + WP06 + WP07 (all parallel -- WP03/04 touch tui/, WP06/07 touch worker/)
4. WP05 (serial, depends on WP03 + WP04)
