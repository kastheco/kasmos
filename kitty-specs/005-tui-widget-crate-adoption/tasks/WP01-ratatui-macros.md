---
work_package_id: "WP01"
subtasks:
  - "T001"
  - "T002"
  - "T003"
  - "T004"
  - "T005"
title: "Adopt ratatui-macros — Layout & Text Macro Migration"
phase: "Phase 1 - Foundation"
lane: "doing"
assignee: ""
agent: "coder"
shell_pid: ""
review_status: ""
reviewed_by: ""
dependencies: []
history:
  - timestamp: "2026-02-12T00:00:00Z"
    lane: "planned"
    agent: "system"
    shell_pid: ""
    action: "Prompt generated via /spec-kitty.tasks"
---

# Work Package Prompt: WP01 – Adopt ratatui-macros — Layout & Text Macro Migration

## ⚠️ IMPORTANT: Review Feedback Status

**Read this first if you are implementing this task!**

- **Has review feedback?**: Check the `review_status` field above. If it says `has_feedback`, scroll to the **Review Feedback** section immediately.
- **You must address all feedback** before your work is complete.
- **Mark as acknowledged**: When you understand the feedback and begin addressing it, update `review_status: acknowledged` in the frontmatter.

---

## Review Feedback

> **Populated by `/spec-kitty.review`** – Reviewers add detailed feedback here when work needs changes.

*[This section is empty initially.]*

---

## Implementation Command

```bash
spec-kitty implement WP01
```

No dependencies — this is the foundation work package.

---

## Objectives & Success Criteria

1. Add `ratatui-macros` 0.7.x to the kasmos crate dependencies.
2. Migrate **all** verbose `Layout::default().direction(...).constraints(...)` patterns to `vertical![]` / `horizontal![]` macros.
3. Migrate **all** verbose `Line::from(vec![Span::styled(...), ...])` patterns to `line![]` / `span![]` macros.
4. Clean up imports — add macro imports, remove now-unused ratatui imports.
5. **Zero regressions**: `cargo build -p kasmos`, `cargo test -p kasmos`, and `cargo clippy -p kasmos -- -D warnings` must all pass (SC-005, SC-006).
6. **Zero runtime behavior changes**: This is a purely syntactic migration with zero functional impact (FR-010, FR-014).

## Context & Constraints

- **Architecture Decision AD-2** (plan.md): ratatui-macros is adopted first so all subsequent WP code uses macro syntax from day one.
- **Research R-6** (research.md): Detailed migration patterns with before/after examples.
- **Spec FR-010**: System MUST adopt `ratatui-macros` (`line![]`, `span![]`, `constraint![]`) across TUI source files.
- **Spec US4-AC3**: All existing tests pass without modification after migration.
- **Spec US4-AC4**: `cargo clippy` produces zero new warnings.
- **Constitution**: ratatui-macros is compile-time only — zero runtime overhead, no render loop impact.

### Files in scope

Only `crates/kasmos/src/tui/app.rs` contains layout and text construction code. Other TUI files (`keybindings.rs`, `event.rs`, `mod.rs`) contain no widget rendering and do NOT need macro migration.

---

## Subtasks & Detailed Guidance

### Subtask T001 – Add `ratatui-macros` dependency to Cargo.toml

- **Purpose**: Introduce the macro crate as a project dependency.
- **Steps**:
  1. Open `crates/kasmos/Cargo.toml`
  2. Add to `[dependencies]`:
     ```toml
     ratatui-macros = "0.7"
     ```
  3. Run `cargo check -p kasmos` to verify the dependency resolves correctly with ratatui 0.30.
- **Files**: `crates/kasmos/Cargo.toml`
- **Notes**: The crate requires `ratatui-core ^0.1.0` and `ratatui-widgets ^0.3.0` which are part of ratatui 0.30's modularized crates. This should resolve automatically.

### Subtask T002 – Migrate Layout construction patterns to macros

- **Purpose**: Replace verbose `Layout::default().direction(...).constraints([...]).split(area)` with concise `vertical![...].areas(area)` / `horizontal![...].areas(area)` syntax.
- **Steps**: Locate and replace each Layout pattern in `crates/kasmos/src/tui/app.rs`:

  **Site 1 — `render_review()` (around line 479)**:
  ```rust
  // BEFORE:
  let panes = Layout::default()
      .direction(Direction::Horizontal)
      .constraints([Constraint::Percentage(40), Constraint::Percentage(60)])
      .split(area);
  // AFTER:
  let [list_pane, detail_pane] = horizontal![==40%, ==60%].areas(area);
  ```
  Then replace `panes[0]` with `list_pane` and `panes[1]` with `detail_pane`.

  **Site 2 — `render_dashboard()` vertical split (around line 786-789)**:
  ```rust
  // BEFORE:
  let vert_chunks = Layout::default()
      .direction(Direction::Vertical)
      .constraints([Constraint::Min(0), Constraint::Length(1)])
      .split(area);
  // AFTER:
  let [kanban_area, hint_area] = vertical![*=0, ==1].areas(area);
  ```
  Then replace `vert_chunks[0]` with `kanban_area` and `vert_chunks[1]` with `hint_area`.

  **Site 3 — `render_dashboard()` horizontal columns (around line 795-803)**:
  ```rust
  // BEFORE:
  let columns = Layout::default()
      .direction(Direction::Horizontal)
      .constraints([
          Constraint::Percentage(25),
          Constraint::Percentage(25),
          Constraint::Percentage(25),
          Constraint::Percentage(25),
      ])
      .split(kanban_area);
  // AFTER:
  let columns = horizontal![==25%, ==25%, ==25%, ==25%].areas(kanban_area);
  ```
  Note: `columns` here is used as an array with indexed access `columns.iter().zip(...)`. The `areas()` call returns an array, so the iteration pattern may need slight adjustment.

  **Site 4 — `render()` main layout (around line 965-968)**:
  ```rust
  // BEFORE:
  let chunks = Layout::default()
      .direction(Direction::Vertical)
      .constraints(constraints)
      .split(area);
  ```
  This one uses a dynamic `constraints` vec (conditional on notifications). The macro syntax requires a compile-time-known number of elements, so this site may need to remain as-is OR be split into two branches:
  ```rust
  // Option A: Keep as-is (dynamic constraint count not supported by macros)
  // Option B: Two branches
  let (tab_area, notif_area, body_area) = if has_notifications {
      let [tab, notif, body] = vertical![==3, ==1, *=0].areas(area);
      (tab, Some(notif), body)
  } else {
      let [tab, body] = vertical![==3, *=0].areas(area);
      (tab, None, body)
  };
  ```
  Choose whichever is cleaner. Option B is preferred as it fully uses macros.

- **Files**: `crates/kasmos/src/tui/app.rs`
- **Parallel?**: Yes — can proceed concurrently with T003.
- **Notes**: After replacing `chunks[N]` / `panes[N]` with named bindings, update all downstream references. The compiler will catch any mismatches.

### Subtask T003 – Migrate Line/Span text construction patterns to macros

- **Purpose**: Replace verbose `Line::from(vec![Span::styled(...), Span::raw(...)])` with `line![...]` / `span![]` macros.
- **Steps**: Locate and replace text construction patterns throughout `crates/kasmos/src/tui/app.rs`. There are many sites — here are the key patterns:

  **Pattern A — Styled + Raw spans**:
  ```rust
  // BEFORE:
  Line::from(vec![
      Span::styled("ID:    ", Style::default().fg(Color::DarkGray)),
      Span::styled(&wp.id, Style::default().fg(Color::Cyan).add_modifier(Modifier::BOLD)),
  ])
  // AFTER:
  line![
      span!(Style::default().fg(Color::DarkGray); "ID:    "),
      span!(Style::default().fg(Color::Cyan).add_modifier(Modifier::BOLD); "{}", &wp.id)
  ]
  ```

  **Pattern B — Simple raw text lines**:
  ```rust
  // BEFORE:
  Line::from(Span::raw("some text"))
  // AFTER:
  line!["some text"]
  ```

  **Pattern C — Tab titles** (around line 971-974):
  ```rust
  // BEFORE:
  let titles: Vec<Line> = Tab::titles()
      .iter()
      .map(|t| Line::from(Span::raw(*t)))
      .collect();
  // AFTER:
  let titles: Vec<Line> = Tab::titles().iter().map(|t| line![*t]).collect();
  ```

  **Pattern D — Empty lines**:
  ```rust
  // BEFORE:
  Line::from("")
  // AFTER:
  line![""]
  ```

  **Pattern E — Notification bar spans** (render_notification_bar method):
  ```rust
  // BEFORE:
  let mut spans: Vec<Span> = vec![Span::raw("  ")];
  // ... push styled spans ...
  let bar = Paragraph::new(Line::from(spans));
  ```
  This dynamically builds a span list — macro replacement may be partial. Convert the final `Line::from(spans)` construction where feasible, but dynamic span building may need to stay as-is.

  **Pattern F — Log rendering** (render_logs method):
  Convert the log line construction in the render loop. Note: WP02 will completely replace `render_logs()`, so only migrate if it doesn't conflict. If in doubt, leave `render_logs()` unchanged since WP02 will remove it entirely.

  **Guidance**: Focus on static text construction patterns. Leave dynamic/conditional span building as-is where macros don't cleanly apply. The goal is readability improvement, not 100% macro coverage.

- **Files**: `crates/kasmos/src/tui/app.rs`
- **Parallel?**: Yes — can proceed concurrently with T002.
- **Notes**: The `render_review()`, `render_notification_bar()`, `render_dashboard()`, and `render()` methods have the most text construction. `render_logs()` will be removed by WP02 — migrate it only if the changes are clean.

### Subtask T004 – Update imports

- **Purpose**: Add macro imports and remove now-unused ratatui imports.
- **Steps**:
  1. Add at the top of `crates/kasmos/src/tui/app.rs`:
     ```rust
     use ratatui_macros::{line, span, vertical, horizontal};
     ```
  2. Review the existing ratatui imports (lines 12-16 of app.rs):
     ```rust
     use ratatui::layout::{Constraint, Direction, Layout, Rect};
     use ratatui::style::{Color, Modifier, Style};
     use ratatui::text::{Line, Span};
     use ratatui::widgets::{Block, Borders, List, ListItem, Paragraph, Tabs};
     ```
  3. After macro migration:
     - `Direction` may no longer be needed if all `Layout::default().direction(...)` calls are replaced
     - `Layout` may still be needed if any dynamic constraint sites remain
     - `Line` and `Span` are still needed (macros produce these types, and they're used in type annotations)
     - `Constraint` may still be needed for dynamic constraint sites
  4. Run `cargo clippy -p kasmos -- -D warnings` to catch any unused import warnings.
- **Files**: `crates/kasmos/src/tui/app.rs`
- **Notes**: `cargo clippy` with `-D warnings` will flag unused imports, making this easy to verify.

### Subtask T005 – Verify build, test, and clippy pass

- **Purpose**: Confirm zero regressions after the macro migration.
- **Steps**:
  1. `cargo build -p kasmos` — must compile without errors
  2. `cargo test -p kasmos` — all existing tests pass (SC-005)
  3. `cargo clippy -p kasmos -- -D warnings` — zero new warnings (SC-006)
  4. Spot check: `grep -rn 'Layout::default()' crates/kasmos/src/tui/` — should show zero results or only sites where macros genuinely don't apply (document exceptions)
- **Files**: N/A (verification step)
- **Notes**: The key render tests (`test_resize_reflow_render_does_not_panic`, `test_dashboard_renders_wps_in_correct_lanes`) validate that layout changes produce correct visual output.

---

## Risks & Mitigations

| Risk | Likelihood | Mitigation |
|------|-----------|------------|
| Macro expansion differs from manual Layout | Low | Existing render tests catch visual regressions |
| Unused import warnings from removed patterns | Medium | `cargo clippy -D warnings` flags them (T004) |
| Dynamic constraint sites can't use macros | Low | Leave as-is with branched approach (T002 Site 4) |

## Review Guidance

- **Key checkpoint**: Run `grep -rn 'Layout::default()' crates/kasmos/src/tui/` — result should be empty or justified.
- **Verify tests pass**: `cargo test -p kasmos` with no new failures.
- **Code readability**: The migrated code should be noticeably more readable than the original.
- **No behavioral changes**: This is syntactic only — no state transitions, no new features, no removed features.

## Activity Log

- 2026-02-12T00:00:00Z – system – lane=planned – Prompt created.
