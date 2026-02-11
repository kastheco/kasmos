# Work Packages: Ratatui TUI Controller Panel

**Inputs**: Design documents from `kitty-specs/002-ratatui-tui-controller-panel/`
**Prerequisites**: plan.md (architecture, channel topology), spec.md (8 user stories, 25 FRs), research.md (async patterns), data-model.md (App/Tab/Notification types)

**Tests**: Required by constitution. All features must have corresponding tests. Feature completion requires passing `cargo test`. Test coverage is provided in WP10.

**Organization**: 62 subtasks (`T001`–`T062`) roll up into 10 work packages (`WP01`–`WP10`). Each WP is independently deliverable.

---

## Work Package WP01: TUI Foundation & Dependencies (Priority: P0)

**Goal**: Establish the TUI module skeleton with ratatui/crossterm deps, terminal lifecycle, event loop, App state, and keybinding framework.
**Independent Test**: `cargo build` succeeds with TUI module compiled. App struct instantiates. Terminal enters/exits alternate screen cleanly.
**Prompt**: `tasks/WP01-tui-foundation.md`
**Estimated Size**: ~400 lines

### Included Subtasks
- [x] T001 Add ratatui + crossterm + futures-util dependencies to Cargo.toml
- [x] T002 Create `tui/mod.rs` — terminal init/teardown, panic hook, async event loop skeleton
- [x] T003 Create `tui/event.rs` — Event enum and crossterm EventStream wrapper with tokio integration
- [x] T004 Create `tui/app.rs` — App struct, Tab enum, per-tab state structs, Notification types
- [x] T005 Create `tui/keybindings.rs` — keymap definitions for tab switching, vim navigation, quit

### Implementation Notes
- Event loop uses `tokio::select!` with three branches: crossterm events, watch channel, tick interval
- App struct holds `OrchestrationRun` snapshot, per-tab UI state, `mpsc::Sender<EngineAction>`
- Panic hook must restore terminal state before unwinding
- Skeleton renders a placeholder "kasmos TUI" message — real tabs come in WP03+

### Dependencies
- None (starting package)

### Risks & Mitigations
- crossterm EventStream requires `futures-util` for `.next().fuse()` — ensure dep is added
- Raw mode must be exited on all code paths (panic hook + graceful shutdown)

---

## Work Package WP02: Engine Integration — Watch Channel & Review States (Priority: P0)

**Goal**: Add `tokio::sync::watch` broadcasting from engine to TUI, extend WPState with ForReview variant, add Approve/Reject EngineActions, update completion detector, and emit review-ready events for automation.
**Independent Test**: Engine broadcasts state changes via watch channel. ForReview state transitions work in state machine. Approve/Reject actions handled correctly. Review-ready events fire when WPs enter ForReview.
**Prompt**: `tasks/WP02-engine-integration.md`
**Estimated Size**: ~450 lines

### Included Subtasks
- [x] T006 Add `watch::Sender<OrchestrationRun>` to WaveEngine, broadcast after every state mutation
- [x] T007 Add `ForReview` variant to `WPState` enum, update state machine with new transitions
- [x] T008 Add `Approve(String)` and `Reject { wp_id, relaunch }` to `EngineAction`, implement handlers
- [x] T009 Update completion detector to distinguish `for_review` from `done` lane transitions
- [x] T010 Wire watch channel creation in `launch.rs` — create channel, pass tx to engine
- [x] T011 Spawn TUI task in `launch.rs`, re-export `tui` module in `lib.rs`
- [x] T056 Emit review-ready events when WPs enter ForReview and enqueue review runner jobs

### Implementation Notes
- WaveEngine constructor gains `watch_tx: watch::Sender<OrchestrationRun>`
- After `handle_completion()` and `handle_action()`, clone run state and send via watch_tx
- ForReview transitions: Active→ForReview, ForReview→Completed (approve), ForReview→Active (reject+relaunch), ForReview→Paused (reject+hold; manual resume/restart)
- Completion detector: when task file lane=`for_review`, emit CompletionEvent with a new `review` method (not `done`)
- Engine should emit a review-ready signal (or directly enqueue a review job) whenever a WP transitions to ForReview

### Dependencies
- Depends on WP01 (TUI module must exist to spawn)

### Risks & Mitigations
- Adding ForReview to WPState affects serialization — ensure serde snake_case matches
- Existing tests must still pass — ForReview is additive, no existing transitions broken

---

## Work Package WP03: Dashboard Tab — Kanban Board (Priority: P1) MVP

**Goal**: Render the kanban dashboard with WPs grouped by lane, navigable with vim keys, showing state badges and wave info.
**Independent Test**: Dashboard displays WPs in correct lanes, navigation with h/j/k/l works, wave separators visible.
**Prompt**: `tasks/WP03-dashboard-tab.md`
**Estimated Size**: ~450 lines

### Included Subtasks
- [x] T012 Create `tui/tabs/mod.rs` — Tab rendering dispatch, tab header bar widget
- [x] T013 Create `tui/tabs/dashboard.rs` — 4-column kanban layout (planned/doing/for_review/done)
- [x] T014 [P] Create `tui/widgets/mod.rs` and `tui/widgets/wp_card.rs` — WP card widget (id, title, state badge, wave, elapsed time)
- [x] T015 Implement WP-to-lane grouping — partition work_packages by WPState into lane columns
- [x] T016 Implement dashboard navigation — h/l between lanes, j/k within lane, scroll for overflow
- [x] T017 Render wave separators and progress summary (X/Y WPs complete, current wave indicator)

### Parallel Opportunities
- T014 (wp_card widget) can be developed in parallel with T013 (dashboard layout)

### Dependencies
- Depends on WP01 + WP02

### Risks & Mitigations
- Lane column widths must adapt to terminal size — use ratatui Constraint::Percentage
- Empty lanes should render with placeholder text, not collapse

---

## Work Package WP04: Action Buttons & WP Control Dispatch (Priority: P1) MVP

**Goal**: Render contextual action buttons for selected WP and dispatch EngineActions on activation.
**Independent Test**: Selecting a failed WP shows Restart/Retry/Force-Advance buttons. Pressing keybind sends correct EngineAction.
**Prompt**: `tasks/WP04-action-buttons.md`
**Estimated Size**: ~350 lines

### Included Subtasks
- [x] T018 Create `tui/widgets/action_buttons.rs` — horizontal button bar rendered below selected WP
- [x] T019 Implement state-based action filtering — map WPState to valid action set per plan table
- [x] T020 Wire action key dispatch — on keybind (R/P/F/T/A), construct and send EngineAction via action_tx
- [x] T021 Implement wave advance UI — "Advance Wave" button at wave boundary in wave-gated mode
- [x] T022 Add confirmation for destructive actions (Force-Advance) — inline yes/no prompt

### Dependencies
- Depends on WP02 (EngineAction channel) + WP03 (dashboard for WP selection context)

### Risks & Mitigations
- Confirmation dialog must not block the event loop — use a `pending_confirmation: Option<EngineAction>` state

---

## Work Package WP05: Notification Bar (Priority: P1)

**Goal**: Persistent notification strip across all tabs showing review/failure/input-needed counts with jump-to navigation.
**Independent Test**: When a WP transitions to Failed, notification bar updates. Pressing `n` jumps to the failed WP in Dashboard.
**Prompt**: `tasks/WP05-notification-bar.md`
**Estimated Size**: ~350 lines

### Included Subtasks
- [x] T023 Create `tui/widgets/notification_bar.rs` — persistent bar at top of frame, rendered before tab content
- [x] T024 Implement notification diffing — on watch update, compare previous vs current run to detect state changes
- [x] T025 Render notification counts per type with WP identifiers, visually distinguish review/failure/input-needed
- [x] T026 Implement notification jump — `n` key cycles through notifications, switches to relevant tab, focuses WP
- [x] T027 Auto-dismiss notifications when WP leaves triggering state (e.g., failure resolved by restart)

### Dependencies
- Depends on WP01 + WP02

### Risks & Mitigations
- Notification IDs must be stable across state updates to avoid spurious add/remove cycles
- Use WP id + notification kind as dedup key

---

## Work Package WP06: Review Tab (Priority: P1)

**Goal**: Dedicated review view for WPs in for_review state with approve/reject/request-changes workflow and automated tiered review execution.
**Independent Test**: WP entering ForReview appears in Review tab. Approve moves to Completed. Reject with relaunch triggers restart. Automated review runs and surfaces results/errors.
**Prompt**: `tasks/WP06-review-tab.md`
**Estimated Size**: ~450 lines

### Included Subtasks
- [x] T028 Create `tui/tabs/review.rs` — split layout: review queue list (left) + detail pane (right)
- [x] T029 List all WPs with `WPState::ForReview`, show title, time in review, wave
- [x] T030 Implement approve action (key `a`) — send `EngineAction::Approve(wp_id)`
- [x] T031 Implement reject action (key `r`) — prompt auto-relaunch vs hold, send `EngineAction::Reject`
- [x] T032 Implement request-changes action (key `c`) — keep in ForReview, mark for manual edits, show re-review option
- [x] T033 Display review context — read review feedback section from WP task file
- [x] T057 Add ReviewRunner service with configurable trigger mode (`slash` or `prompt`)
- [x] T058 Implement slash mode injection for reviewer command (default `/kas:verify`) in target pane
- [x] T059 Implement prompt mode tiered review execution via opencode (default model `openai/gpt-5.3-codex`, reasoning high)
- [x] T060 Persist ReviewResult (status/findings/mode/timestamps) and display in Review tab detail pane

### Dependencies
- Depends on WP02 (ForReview state, Approve/Reject actions) + WP03 (tab framework)

### Risks & Mitigations
- Reading WP task files for review context requires filesystem access from TUI — keep reads on tick, not per-frame
- Request-changes is TUI-only state — no engine action needed, just UI flag
- Review automation policy defaults to `auto_then_manual_approve`; auto-mark-done remains opt-in

---

## Work Package WP07: Logs Tab (Priority: P2)

**Goal**: Scrollable, filterable log viewer for orchestration events.
**Independent Test**: State transitions generate log entries. Filter with `/` narrows visible entries. Auto-scroll follows new entries.
**Prompt**: `tasks/WP07-logs-tab.md`
**Estimated Size**: ~350 lines

### Included Subtasks
- [ ] T034 Create `tui/tabs/logs.rs` — scrollable list of LogEntry items with timestamp + level badge
- [ ] T035 Implement log capture — convert state diffs (on watch update) to LogEntry items
- [ ] T036 Implement text filter — `/` activates filter input mode, Esc exits, real-time filtering
- [ ] T037 Implement auto-scroll — follow tail by default, pause on manual scroll up, `G` to resume
- [ ] T038 Apply log level styling — color-coded by Info (dim), Warn (yellow), Error (red)

### Dependencies
- Depends on WP01 + WP02

### Risks & Mitigations
- Log list can grow unbounded — cap at 10,000 entries with FIFO eviction
- Filter input mode must capture all keys (including vim nav) until Esc

---

## Work Package WP08: Input-Needed Signal Detection (Priority: P2)

**Goal**: Detect agent `.input-needed` marker files, surface notifications, enable focus/zoom to agent pane.
**Independent Test**: Creating `.input-needed` in a WP worktree surfaces notification within 2s. Activating notification zooms correct pane.
**Prompt**: `tasks/WP08-input-needed-signals.md`
**Estimated Size**: ~300 lines

### Included Subtasks
- [x] T039 Implement `.input-needed` marker file polling — check WP worktree paths on each tick (~1s)
- [x] T040 Surface InputNeeded notifications in notification bar with agent's message text from marker file
- [x] T041 Implement focus/zoom action — on notification activation, call `SessionManager.zoom_pane(wp_id)`
- [x] T042 Auto-clear InputNeeded notifications when agent removes marker file

### Dependencies
- Depends on WP05 (notification bar) + WP02 (SessionManager access)

### Risks & Mitigations
- Polling worktree paths requires knowing worktree locations from OrchestrationRun.work_packages[].worktree_path
- SessionManager.zoom_pane() may fail if pane crashed — show error in notification bar

---

## Work Package WP09: FIFO Compatibility & Terminal Lifecycle (Priority: P2)

**Goal**: Ensure FIFO commands work alongside TUI, handle terminal resize/mouse, orchestration end state, and empty state.
**Independent Test**: FIFO `restart WP01` while TUI is active updates display. Terminal resize reflows layout. Mouse clicks on tabs work.
**Prompt**: `tasks/WP09-fifo-compat-lifecycle.md`
**Estimated Size**: ~400 lines

### Included Subtasks
- [x] T043 Verify FIFO commands produce state changes visible in TUI via watch channel
- [x] T044 Handle concurrent TUI + FIFO input without data races — both send to same action_rx
- [x] T045 Implement terminal resize handling — graceful reflow on crossterm Resize event
- [x] T046 Implement mouse support — click on tab headers, click on WP cards, scroll wheels
- [x] T047 Handle orchestration termination — show final state summary with exit code, allow quit
- [x] T048 Implement empty/no-run state — guidance message when no orchestration is active

### Dependencies
- Depends on WP03 + WP07 (core tabs must exist for FIFO verification and resize testing)

### Risks & Mitigations
- FIFO + TUI concurrency: mpsc channels are thread-safe, no additional locking needed
- Mouse click target detection requires tracking rendered widget positions

---

## Work Package WP10: Test, Compatibility, and Performance Gates (Priority: P1)

**Goal**: Satisfy constitution-required tests and validate notification/performance/review-automation success criteria.
**Independent Test**: `cargo test` passes; notification audit, latency checks, and review automation fallback tests pass thresholds.
**Prompt**: `tasks/WP10-validation-gates.md`
**Estimated Size**: ~350 lines

### Included Subtasks
- [ ] T049 Add unit tests for ForReview transitions (approve, reject+relaunch, reject+hold->Paused)
- [ ] T050 Add unit tests for contextual action availability by WP state
- [ ] T051 Add integration parity tests for FIFO vs TUI command outcomes
- [ ] T052 Add integration tests for input-needed notification lifecycle
- [ ] T053 Add notification delivery audit test (emitted IDs == surfaced IDs)
- [ ] T054 Add synthetic 50-WP latency test and assert SC-005 thresholds
- [ ] T055 Add final validation gate documentation (`cargo test` required before done)
- [ ] T061 Add integration tests for review runner mode selection/fallback (`slash` failure -> `prompt`)
- [ ] T062 Add integration tests for persisted review results and `for_review` lifecycle visibility after restart

### Implementation Notes
- Tests use `ratatui::backend::TestBackend` for UI assertions
- FIFO parity tests spawn both input sources concurrently
- Latency test instruments event loop with histogram metrics
- Notification audit test hooks state broadcaster to capture all emitted events and verify bar coverage
- Review automation tests should mock slash injection and opencode prompt runner to validate deterministic outcomes

### Dependencies
- Depends on WP02 (ForReview state machine), WP04 (action buttons), WP05 (notification bar), WP06 (review tab), WP08 (input-needed signals), WP09 (FIFO compat)

### Risks & Mitigations
- Synthetic load test may be flaky on slow CI — use deterministic clock and set generous timeout buffers
- Test backend rendering may differ from real terminal — validate core logic, not pixel-perfect layout

---

## Dependency & Execution Summary

```
Wave 1 (Foundation):       WP01 ──┐
                           WP02 ──┤ (WP02 depends on WP01)
                                  │
Wave 2 (Core Views):       WP03 ──┤ depends WP01+WP02
                           WP04 ──┤ depends WP02+WP03
                           WP05 ──┤ depends WP01+WP02
                           WP07 ──┤ depends WP01+WP02
                                  │
Wave 3 (Advanced):         WP06 ──┤ depends WP02+WP03
                           WP08 ──┤ depends WP05+WP02
                           WP09 ──┤ depends WP03+WP07
                                  │
Wave 4 (Validation):       WP10 ──┘ depends WP02+WP04+WP05+WP06+WP08+WP09
```

**Parallelization**:
- Wave 2: WP03, WP05, WP07 can run in parallel (all depend only on WP01+WP02)
- WP04 depends on WP03 (needs dashboard WP selection), so starts after WP03
- Wave 3: WP06, WP08, WP09 can run in parallel once their deps complete
- Wave 4: WP10 runs last (tests all prior WP implementations)

**MVP Scope**: WP01 + WP02 + WP03 + WP04 = functional dashboard with WP control

---

## Subtask Index (Reference)

| Subtask | Summary | WP | Priority | Parallel? |
|---------|---------|-----|----------|-----------|
| T001 | Add ratatui/crossterm/futures deps | WP01 | P0 | No |
| T002 | TUI mod.rs — terminal lifecycle + event loop | WP01 | P0 | No |
| T003 | Event enum + EventStream wrapper | WP01 | P0 | No |
| T004 | App struct + Tab + per-tab state types | WP01 | P0 | No |
| T005 | Keybindings — tab switch, vim nav, quit | WP01 | P0 | No |
| T006 | watch::Sender in WaveEngine + broadcast | WP02 | P0 | No |
| T007 | ForReview WPState + state machine | WP02 | P0 | No |
| T008 | Approve/Reject EngineAction + handlers | WP02 | P0 | No |
| T009 | Completion detector for_review distinction | WP02 | P0 | No |
| T010 | Wire watch channel in launch.rs | WP02 | P0 | No |
| T011 | Spawn TUI + re-export tui module | WP02 | P0 | No |
| T012 | tabs/mod.rs — tab dispatch + header bar | WP03 | P1 | No |
| T013 | Dashboard 4-column kanban layout | WP03 | P1 | No |
| T014 | wp_card widget | WP03 | P1 | Yes |
| T015 | WP-to-lane grouping logic | WP03 | P1 | No |
| T016 | Dashboard h/j/k/l navigation | WP03 | P1 | No |
| T017 | Wave separators + progress summary | WP03 | P1 | No |
| T018 | action_buttons widget | WP04 | P1 | No |
| T019 | State-based action filtering | WP04 | P1 | No |
| T020 | Action key dispatch via action_tx | WP04 | P1 | No |
| T021 | Wave advance UI button | WP04 | P1 | No |
| T022 | Confirmation dialog for destructive actions | WP04 | P1 | No |
| T023 | notification_bar widget | WP05 | P1 | No |
| T024 | Notification diffing on state update | WP05 | P1 | No |
| T025 | Notification counts + visual distinction | WP05 | P1 | No |
| T026 | Jump-to-notification (`n` key) | WP05 | P1 | No |
| T027 | Auto-dismiss notifications | WP05 | P1 | No |
| T028 | review.rs — split layout | WP06 | P1 | No |
| T029 | List ForReview WPs | WP06 | P1 | No |
| T030 | Approve action | WP06 | P1 | No |
| T031 | Reject action (relaunch/hold) | WP06 | P1 | No |
| T032 | Request-changes action | WP06 | P1 | No |
| T033 | Display review feedback context | WP06 | P1 | No |
| T034 | logs.rs — scrollable list | WP07 | P2 | No |
| T035 | Log capture from state diffs | WP07 | P2 | No |
| T036 | Text filter (`/` mode) | WP07 | P2 | No |
| T037 | Auto-scroll + resume | WP07 | P2 | No |
| T038 | Log level styling | WP07 | P2 | No |
| T039 | .input-needed file polling | WP08 | P2 | No |
| T040 | InputNeeded notification display | WP08 | P2 | No |
| T041 | Focus/zoom pane action | WP08 | P2 | No |
| T042 | Auto-clear on marker removal | WP08 | P2 | No |
| T043 | FIFO state change visibility in TUI | WP09 | P2 | No |
| T044 | Concurrent TUI + FIFO input handling | WP09 | P2 | No |
| T045 | Terminal resize reflow | WP09 | P2 | No |
| T046 | Mouse support | WP09 | P2 | No |
| T047 | Orchestration termination state | WP09 | P2 | No |
| T048 | Empty/no-run state | WP09 | P2 | No |
| T049 | Unit tests: ForReview transitions | WP10 | P1 | No |
| T050 | Unit tests: contextual action availability | WP10 | P1 | No |
| T051 | Integration tests: FIFO vs TUI parity | WP10 | P1 | No |
| T052 | Integration tests: input-needed lifecycle | WP10 | P1 | No |
| T053 | Notification delivery audit test | WP10 | P1 | No |
| T054 | Synthetic 50-WP latency test | WP10 | P1 | No |
| T055 | Validation gate documentation | WP10 | P1 | No |
| T056 | Emit review-ready events on ForReview transition | WP02 | P0 | No |
| T057 | ReviewRunner service with slash/prompt modes | WP06 | P1 | No |
| T058 | Slash mode command injection (`/kas:verify`) | WP06 | P1 | No |
| T059 | Prompt mode tiered review via opencode | WP06 | P1 | No |
| T060 | Persist + render ReviewResult details | WP06 | P1 | No |
| T061 | Integration tests for slash->prompt fallback | WP10 | P1 | No |
| T062 | Integration tests for review result persistence | WP10 | P1 | No |
