---
work_package_id: WP04
title: Detail View
lane: done
dependencies:
- WP03
subtasks:
- T018
- T019
- T020
phase: Phase 3 - Views & Actions
assignee: ''
agent: ''
shell_pid: ''
review_status: ''
reviewed_by: ''
history:
- timestamp: '2026-02-13T03:53:23Z'
  lane: planned
  agent: system
  shell_pid: ''
  action: Prompt generated via /spec-kitty.tasks
---

# Work Package Prompt: WP04 - Detail View

## Review Feedback Status

**Read this first if you are implementing this task!**

- **Has review feedback?**: Check the `review_status` field above. If it says `has_feedback`, scroll to the **Review Feedback** section immediately.
- **You must address all feedback** before your work is complete.
- **Mark as acknowledged**: When you understand the feedback and begin addressing it, update `review_status: acknowledged` in the frontmatter.

---

## Review Feedback

> **Populated by `/spec-kitty.review`** -- Reviewers add detailed feedback here when work needs changes.

*[This section is empty initially.]*

---

## Markdown Formatting
Wrap HTML/XML tags in backticks: `` `<div>` ``, `` `<script>` ``
Use language identifiers in code blocks: ````rust`, ````bash`

---

## Objectives & Success Criteria

- Implement the feature detail view showing individual work packages with lane status, dependencies, and wave assignments
- Detail view is accessible by pressing Enter on a feature in the list view
- Pressing Esc returns to the list view with selection preserved
- WP table shows: ID, Title, Lane (colored), Wave, Dependencies
- Lane colors: planned=gray, doing=yellow, for_review=blue, done=green
- Scrollable table for features with many WPs

## Context & Constraints

- **Plan**: `kitty-specs/010-hub-tui-navigator/plan.md` (AD-002: Hub Module Structure)
- **Spec**: `kitty-specs/010-hub-tui-navigator/spec.md` (FR-003, User Story 1 acceptance scenarios 2, 5)
- **Data Model**: `kitty-specs/010-hub-tui-navigator/data-model.md` (FeatureDetail, WPSummary, HubView)
- **Dependencies**: WP03 (hub app core with list view and navigation)

### Key Architectural Decisions

- `FeatureDetail` is lazily loaded when the operator drills into a feature
- WP frontmatter is parsed on-demand (not during list scan) for title, lane, wave, dependencies
- Detail view is a separate render path in `App::render()` based on `HubView::Detail`

## Subtasks & Detailed Guidance

### Subtask T018 - Define FeatureDetail and WPSummary types

- **Purpose**: Create types for the expanded feature view.
- **Steps**:
  1. Add to `crates/kasmos/src/hub/scanner.rs`:

```rust
/// Summary of a single work package, parsed from WP frontmatter.
#[derive(Debug, Clone)]
pub struct WPSummary {
    /// e.g., "WP01"
    pub id: String,
    /// WP title for display
    pub title: String,
    /// planned / doing / for_review / done
    pub lane: String,
    /// Wave assignment (0 if not set)
    pub wave: usize,
    /// WP IDs this depends on
    pub dependencies: Vec<String>,
}

/// Expanded view of a single feature.
#[derive(Debug, Clone)]
pub struct FeatureDetail {
    /// The feature being detailed
    pub feature: FeatureEntry,
    /// Individual WP states
    pub work_packages: Vec<WPSummary>,
}
```

  2. Implement `pub fn load_detail(feature: &FeatureEntry) -> FeatureDetail` that:
     a. Scans `feature.feature_dir/tasks/` for `WPxx-*.md` files
     b. Parses full frontmatter from each (work_package_id, title, lane, wave, dependencies)
     c. Sorts WPs by ID
     d. Returns `FeatureDetail` with the feature and WP list

- **Files**: `crates/kasmos/src/hub/scanner.rs`
- **Parallel?**: No (T019 and T020 depend on this)
- **Notes**: The frontmatter parsing needs to extract more fields than the list scan (which only needs `lane`). Use a more complete frontmatter struct:

```rust
#[derive(serde::Deserialize)]
struct DetailFrontmatter {
    work_package_id: Option<String>,
    title: Option<String>,
    lane: Option<String>,
    wave: Option<usize>,
    dependencies: Option<Vec<String>>,
}
```

Gracefully handle missing fields with defaults.

### Subtask T019 - Implement detail view rendering

- **Purpose**: Render the WP table for a selected feature.
- **Steps**:
  1. In `crates/kasmos/src/hub/app.rs`, add a `detail` field: `pub detail: Option<FeatureDetail>`
  2. In `App::render()`, when `self.view == HubView::Detail { index }`:
     a. Load detail if not cached (or if index changed)
     b. Render a header with feature name and overall status
     c. Render a table with columns: ID | Title | Lane | Wave | Dependencies
     d. Color the Lane column: planned=`Color::DarkGray`, doing=`Color::Yellow`, for_review=`Color::Blue`, done=`Color::Green`
     e. Use `ratatui::widgets::Table` with `Row` items
  3. Layout: header (feature name), table area, footer (keybinding hints)

- **Files**: `crates/kasmos/src/hub/app.rs`
- **Parallel?**: Yes (can proceed once T018 types exist)
- **Notes**: Use `ratatui::widgets::Table` with `Constraint::Percentage` or `Constraint::Length` for column widths. The table should be scrollable if there are more WPs than fit on screen.

**Example detail view layout**:
```
+--------------------------------------------------+
| Feature: 001-my-feature  [2/5 done]              |
|--------------------------------------------------|
| ID   | Title              | Lane       | Wave | Deps  |
|------|--------------------|-----------:|------|-------|
| WP01 | Setup & Env        | done       |  0   |       |
| WP02 | Core Models        | done       |  0   |       |
| WP03 | API Endpoints      | doing      |  1   | WP02  |
| WP04 | Frontend Views     | planned    |  1   | WP02  |
| WP05 | Polish             | planned    |  2   | WP03,WP04 |
|--------------------------------------------------|
| Esc:back  Enter:action  r:refresh                |
+--------------------------------------------------+
```

### Subtask T020 - Implement detail view navigation

- **Purpose**: Handle keyboard events in the detail view.
- **Steps**:
  1. In `crates/kasmos/src/hub/keybindings.rs`, add handling for `HubView::Detail`:
     - `Esc` -> return to list view: `app.view = HubView::List; app.detail = None;`
     - `j/k` or `Down/Up` -> scroll through WP rows (if table is scrollable)
     - `Enter` -> placeholder for action dispatch (WP05/WP06 will implement)
     - `r` -> manual refresh (same as list view)
  2. When transitioning from list to detail (Enter in list view):
     - Set `app.view = HubView::Detail { index: app.selected }`
     - Load the detail: `app.detail = Some(scanner::load_detail(&app.features[app.selected]))`
  3. When returning from detail to list (Esc):
     - Preserve `app.selected` (don't reset it)
     - Clear `app.detail` to free memory

- **Files**: `crates/kasmos/src/hub/keybindings.rs`, `crates/kasmos/src/hub/app.rs`
- **Parallel?**: Yes (can proceed once T018 types exist)
- **Notes**: The detail is loaded synchronously since it only parses a few files. If performance is a concern, it can be moved to `spawn_blocking` later.

## Test Strategy

- **Unit tests**: Test `load_detail()` with filesystem fixtures (tempdir with WP files)
- **Manual testing**: Navigate to a feature with tasks, verify WP table renders correctly
- **Edge cases**: Feature with no tasks (show "No work packages" message), feature with 20+ WPs (scrollable)

## Risks & Mitigations

- **Large WP counts**: Use scrollable table widget -- ratatui handles this natively with `StatefulWidget`
- **Frontmatter parsing**: Graceful degradation for missing/malformed fields

## Review Guidance

- Verify `FeatureDetail` and `WPSummary` match data-model.md
- Verify lane colors are correct and visually distinct
- Verify Esc preserves list selection
- Verify detail loading handles missing tasks directory

## Activity Log

- 2026-02-13T03:53:23Z - system - lane=planned - Prompt created.
- 2026-02-13T12:00:00Z - release opencode agent - lane=done - Acceptance validation passed
