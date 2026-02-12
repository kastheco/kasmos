# Feature Specification: TUI Widget Crate Adoption

**Feature Branch**: `005-tui-widget-crate-adoption`
**Created**: 2026-02-11
**Status**: Draft
**Base**: `master` (after feature 002 merges)
**Input**: Refactor the kasmos TUI to adopt 5 community ratatui crates, replacing hand-rolled implementations with battle-tested community widgets and adding a WP dependency graph visualization.

## Clarifications

### Session 2026-02-11

- Q: Should all 6 evaluated crates be included? → A: No. Defer `tachyonfx` (rendering overhead risk) and `tui-widget-list` (refactoring working code for marginal gain). Adopt 5: `tui-logger`, `tui-popup`, `throbber-widgets-tui`, `ratatui-macros`, `tui-nodes`.
- Q: Should `tui-nodes` be added for WP dependency visualization? → A: Yes. Use it to render a node graph showing WP dependencies within a feature, visible from the Dashboard tab.
- Q: Should this be a new feature or additional WPs on 002? → A: New feature branching from `master`, not from 002's branch. Won't be planned/tasked until 002 merges.
- Q: Should each crate adoption be independently mergeable? → A: Yes. Structure WPs so each crate can land or be reverted without affecting the others.

### Session 2026-02-12

- Q: Is feature 006 (ratatui 0.30 upgrade) expected to land before this feature's tui-nodes WP? → A: Yes. 006 lands first; the tui-nodes WP depends on 006 being merged.
- Q: Should tui-logger's buffer size be configured to match the current 10k entry cap? → A: No. Use tui-logger's default buffer size; no explicit cap needed.
- Q: What key should toggle between kanban and dependency graph views in Dashboard? → A: `v` for "view toggle".

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Enhanced Log Viewer with Per-Target Filtering (Priority: P1)

An operator opens the Logs tab and sees orchestration events organized by their tracing target (e.g., `kasmos::engine`, `kasmos::tui`, `kasmos::review`). The operator can toggle which log levels are captured vs displayed per target, scroll through history with page mode, and filter without losing context. This replaces the previous manual log list with a richer, zero-setup experience powered by `tui-logger`.

**Why this priority**: The Logs tab is the highest-value replacement — eliminates ~150-200 LOC of hand-rolled log management while adding significant new functionality (per-target filtering, level toggling, page mode) that the manual implementation lacked.

**Independent Test**: Can be tested by launching kasmos with tracing enabled, verifying log events appear in the tui-logger widget, toggling target visibility, and using page mode to scroll through history.

**Acceptance Scenarios**:

1. **Given** kasmos is running with active orchestration, **When** the operator opens the Logs tab, **Then** orchestration events appear in the tui-logger smart widget grouped by tracing target with timestamps.
2. **Given** the Logs tab is active, **When** the operator uses the target selector to hide a specific target, **Then** events from that target disappear from the log view without affecting other targets.
3. **Given** the Logs tab has accumulated many events, **When** the operator enters page mode (PageUp), **Then** they can scroll through history without new events disrupting their position.
4. **Given** the operator adjusts the captured log level for a target to Warn, **When** new Debug/Info events arrive for that target, **Then** they are silently discarded and do not appear in the log view.
5. **Given** the previous hand-rolled LogsState code existed, **When** this adoption is complete, **Then** the manual `Vec<LogEntry>`, filter string, scroll offset management code, and 10k-entry cap logic are fully removed (tui-logger manages its own default buffer).

---

### User Story 2 - Polished Confirmation Dialogs (Priority: P2)

When the operator triggers a destructive or significant action (Force-Advance, Abort), a centered popup dialog appears asking for confirmation. The popup auto-sizes to its content, has a styled border and title, and can be dismissed with `y`/`n`/`Esc`. This replaces the manual `Clear` + `Rect` centering code with `tui-popup`.

**Why this priority**: Replaces fragile manual popup rendering with a maintained widget that handles centering, sizing, and border styling automatically. Also reusable for future dialogs (review detail panels, help screens).

**Independent Test**: Can be tested by triggering confirmation-required actions and verifying the popup renders centered with correct content, responds to y/n/Esc, and dismisses cleanly.

**Acceptance Scenarios**:

1. **Given** the operator presses Force-Advance on a WP, **When** the confirmation dialog appears, **Then** it renders as a centered popup with a title, action description, and y/n prompt.
2. **Given** a confirmation popup is visible, **When** the operator presses `y`, **Then** the action executes and the popup dismisses.
3. **Given** a confirmation popup is visible, **When** the operator presses `n` or `Esc`, **Then** the popup dismisses without executing the action.
4. **Given** the terminal is resized while a popup is visible, **When** the next frame renders, **Then** the popup re-centers itself in the new terminal dimensions.
5. **Given** the previous hand-rolled dialog code existed, **When** this adoption is complete, **Then** the manual centering calculation, `Clear` widget usage, and `Paragraph`-in-block popup code is replaced by `tui-popup` calls.

---

### User Story 3 - Animated Activity Indicators on Active Work Packages (Priority: P3)

When a work package is in the Active state, the dashboard displays an animated spinner next to it instead of a static icon. The spinner provides immediate visual feedback that the WP is running and the system is responsive. Paused, Failed, and Completed WPs retain static badges.

**Why this priority**: Small but impactful UX improvement that makes the dashboard feel alive. Active WPs with static icons look identical to stalled ones; spinners differentiate running from stuck.

**Independent Test**: Can be tested by placing WPs in Active state and verifying the spinner animates on each tick, while non-Active WPs show static badges.

**Acceptance Scenarios**:

1. **Given** a WP is in `Active` state, **When** the dashboard renders, **Then** an animated spinner appears next to the WP name that cycles through frames on each render tick.
2. **Given** a WP transitions from `Active` to `Paused`, **When** the next frame renders, **Then** the spinner is replaced by a static paused badge.
3. **Given** multiple WPs are Active simultaneously, **When** the dashboard renders, **Then** all Active WPs show synchronized spinner animation.
4. **Given** the render tick interval is 250ms, **When** the spinner animates, **Then** the animation is visually smooth (at least 4 distinct frames per cycle).

---

### User Story 4 - Reduced Layout Boilerplate with Macros (Priority: P4)

Developers working on the kasmos TUI use concise macro syntax (`line![]`, `span![]`, `constraint![]`) instead of verbose builder patterns for layout and text construction. This is a developer-experience improvement that makes the TUI code more readable and maintainable.

**Why this priority**: Lowest user-facing impact but improves code quality across all TUI files. Zero runtime overhead (compile-time macros).

**Independent Test**: Can be tested by running `cargo build` and `cargo test` after the migration, verifying all existing functionality is preserved.

**Acceptance Scenarios**:

1. **Given** the TUI source files use verbose `Layout::default().direction(...).constraints([...])` patterns, **When** this adoption is complete, **Then** layout declarations use `constraint![]` macro syntax.
2. **Given** the TUI source files construct `Line` and `Span` objects manually, **When** this adoption is complete, **Then** text construction uses `line![]` and `span![]` macros where applicable.
3. **Given** the macro migration is complete, **When** `cargo test` is run, **Then** all existing tests pass without modification.
4. **Given** the macro migration is complete, **When** `cargo clippy` is run, **Then** zero new warnings are introduced.

---

### User Story 5 - WP Dependency Graph Visualization (Priority: P2)

An operator selects a feature in the Dashboard and toggles a dependency graph view that renders all work packages as nodes connected by directed edges representing their dependency relationships. The graph makes it immediately clear which WPs are blocking others, which are independent, and what the critical path is through the feature. This is powered by `tui-nodes`.

**Why this priority**: WP dependencies are currently implicit (listed as text in task files). A visual graph gives the operator instant understanding of parallelism opportunities, blocking chains, and overall feature structure — especially valuable for features with 8+ WPs.

**Independent Test**: Can be tested by loading a feature with known dependencies (e.g., feature 002 with 10 WPs and documented dependency edges) and verifying the graph renders all nodes and connections correctly, with state-based coloring.

**Acceptance Scenarios**:

1. **Given** an orchestration run is active with WPs that have declared dependencies, **When** the operator activates the dependency graph view, **Then** each WP renders as a labeled node and dependency relationships render as directed edges between nodes.
2. **Given** the dependency graph is visible, **When** a WP transitions state (e.g., Pending→Active, Active→Completed), **Then** the node's visual styling updates to reflect the new state (e.g., color change, badge update).
3. **Given** a feature has WPs with no dependencies (fully parallel), **When** the graph renders, **Then** those WPs appear as disconnected nodes without incoming edges.
4. **Given** a feature has a linear dependency chain (WP01→WP02→WP03), **When** the graph renders, **Then** the chain is visually clear with directed edges flowing in dependency order.
5. **Given** the terminal is narrow, **When** the graph renders with many WPs, **Then** nodes are laid out without overlapping and the graph remains readable (scrollable if necessary).

---

### Edge Cases

- What if `tui-logger` version is incompatible with the project's `tracing` subscriber stack? The integration layer must compose with existing tracing subscribers without replacing them.
- What if `tui-popup` auto-sizing produces a popup larger than the terminal? The popup must clamp to terminal dimensions.
- What if the throbber tick rate doesn't align with the existing 250ms render tick? The spinner must degrade gracefully (slower animation is acceptable; flickering is not).
- What if `ratatui-macros` macro expansion conflicts with existing imports or naming? Compilation errors must be caught by CI before merge.
- What if any adopted crate has a breaking release during development? Pin exact versions in Cargo.toml.
- What if a feature has circular dependencies declared between WPs? The graph renderer must detect cycles and render them visually (e.g., highlighted edges) rather than entering an infinite layout loop.
- What if a feature has 20+ WPs? The node graph layout must remain readable — either via auto-scaling, scrolling, or collapsing completed nodes.
- What if `tui-nodes` requires ratatui 0.30 but the project is still on 0.29? The tui-nodes adoption WP must be sequenced after feature 006 (dependency upgrade) or use a compatible version.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST replace the hand-rolled `LogsState`/`Vec<LogEntry>` log viewer with `tui-logger`'s `TuiLoggerSmartWidget`.
- **FR-002**: System MUST integrate `tui-logger` with the existing `tracing` subscriber stack via the `tracing-support` feature, without removing or replacing other tracing layers.
- **FR-003**: System MUST expose per-target log level toggling (captured and displayed levels) in the Logs tab via the tui-logger target selector widget.
- **FR-004**: System MUST support page-mode scrollback in the Logs tab via tui-logger's built-in page navigation (PageUp/PageDown/Escape).
- **FR-005**: System MUST replace the hand-rolled confirmation dialog (`Clear` + centered `Rect` + `Paragraph`) with `tui-popup`'s `Popup` widget.
- **FR-006**: System MUST render confirmation popups centered with auto-sizing, styled borders, and title text.
- **FR-007**: System MUST support `y`/`n`/`Esc` key handling for popup confirmation/dismissal, preserving existing behavior.
- **FR-008**: System MUST render animated spinners via `throbber-widgets-tui` on work packages in `Active` state in the Dashboard tab.
- **FR-009**: System MUST display static state badges (not spinners) for work packages in non-Active states (Pending, Paused, Failed, Completed, ForReview).
- **FR-010**: System MUST adopt `ratatui-macros` (`line![]`, `span![]`, `constraint![]`) across TUI source files, replacing verbose builder patterns.
- **FR-011**: System MUST render a WP dependency graph via `tui-nodes` showing all work packages as nodes and their dependency relationships as directed edges.
- **FR-012**: System MUST color-code dependency graph nodes by WP state (e.g., distinct colors for Pending, Active, Completed, Failed, ForReview).
- **FR-013**: System MUST use the `v` key to toggle between the kanban lane view and the dependency graph view in the Dashboard tab.
- **FR-014**: System MUST preserve all existing TUI functionality after each crate adoption — no behavioral regressions.
- **FR-015**: Each crate adoption MUST be independently mergeable — reverting one adoption must not break the others.

### Key Entities

- **TuiLoggerSmartWidget**: The tui-logger dual-pane widget (target selector + log view) that replaces LogsState rendering.
- **Popup**: The tui-popup centered dialog widget that replaces manual confirmation rendering.
- **Throbber**: The throbber-widgets-tui animated spinner widget rendered per-Active-WP.
- **ThrobberState**: Per-WP animation state tracking spinner frame position.
- **NodeGraph**: The tui-nodes graph widget that renders WP dependency relationships as a directed node graph.
- **NodeLayout**: Per-WP node render configuration (position, label, styling based on WP state).
- **Connection**: A directed edge between two WP nodes representing a dependency relationship.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: The Logs tab renders orchestration events with per-target filtering within 1 second of feature startup.
- **SC-002**: Log viewer supports at least 5 distinct tracing targets with independent level control.
- **SC-003**: All confirmation dialogs render centered and auto-sized without manual coordinate calculation in application code.
- **SC-004**: Active work packages display animated indicators that visually update at least once per second.
- **SC-005**: The full test suite (`cargo test`) passes after each individual crate adoption with zero new failures.
- **SC-006**: `cargo clippy` produces zero new warnings after each individual crate adoption.
- **SC-007**: The Logs tab implementation is reduced by at least 100 lines of code compared to the hand-rolled version.
- **SC-008**: The dependency graph correctly renders all WP nodes and dependency edges for a feature with at least 10 WPs.
- **SC-009**: Each crate adoption can be reverted via a single `git revert` of its merge commit without breaking other adoptions.

## Assumptions

- Feature 002 (`ratatui-tui-controller-panel`) is fully merged to `master` before this feature begins implementation.
- The current `tracing` subscriber setup in kasmos supports adding additional layers (tui-logger's `TuiTracingSubscriberLayer`).
- All 5 adopted crates (`tui-logger` 0.18.x, `tui-popup` 0.7.x, `throbber-widgets-tui` 0.10.x, `ratatui-macros` 0.7.x, `tui-nodes` 0.10.x) require `ratatui 0.30` in their latest versions. Feature 006 (dependency upgrade) is confirmed to merge first, so all adoptions target ratatui 0.30 with latest crate versions. (Research finding — originally only tui-nodes was known to require 0.30.)
- The existing 250ms render tick is sufficient for throbber animation (4+ frames per cycle at 250ms = 1 second per full rotation, which is acceptable).
- `tui-popup` from the `joshka/tui-widgets` mono-repo is published on crates.io and usable as a standard dependency.
- `ratatui-macros` is a compile-time-only dependency with zero runtime overhead.
