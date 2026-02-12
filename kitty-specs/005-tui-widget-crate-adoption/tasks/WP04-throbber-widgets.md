---
work_package_id: "WP04"
subtasks:
  - "T018"
  - "T019"
  - "T020"
  - "T021"
  - "T022"
title: "Adopt throbber-widgets-tui — Animated Activity Indicators"
phase: "Phase 2 - Crate Adoptions"
lane: "planned"
assignee: ""
agent: ""
shell_pid: ""
review_status: ""
reviewed_by: ""
dependencies: ["WP01"]
history:
  - timestamp: "2026-02-12T00:00:00Z"
    lane: "planned"
    agent: "system"
    shell_pid: ""
    action: "Prompt generated via /spec-kitty.tasks"
---

# Work Package Prompt: WP04 – Adopt throbber-widgets-tui — Animated Activity Indicators

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
spec-kitty implement WP04 --base WP01
```

Depends on WP01 (ratatui-macros). New code should use macro syntax.

---

## Objectives & Success Criteria

1. Add `throbber-widgets-tui` 0.10.x as a dependency.
2. Add a shared `ThrobberState` to `DashboardState`, ticked every 250ms in `on_tick()` (AD-3).
3. Active WPs in the Dashboard display animated spinners that cycle through frames each tick (FR-008, US3-AC1).
4. Non-Active WPs display static state badges (FR-009, US3-AC2).
5. Multiple Active WPs show **synchronized** spinners sharing the same animation frame (US3-AC3).
6. **SC-004**: Animation updates at least once per second (250ms tick × 4+ frames per cycle).
7. **SC-005/SC-006**: `cargo test` and `cargo clippy` pass with zero regressions.

## Context & Constraints

- **Architecture Decision AD-3** (plan.md): Single shared ThrobberState — all Active WPs render the same frame.
- **Research R-5** (research.md): ThrobberState API, available throbber sets, integration pattern.
- **data-model.md**: ThrobberState added to DashboardState, ticked unconditionally every 250ms.
- **Spec US3**: Animated spinner on Active WPs, static badges on non-Active.
- **Current code**: `render_dashboard()` in app.rs (lines ~784-907) renders WPs as `ListItem` with `Span` text. Active WPs show elapsed time as `Span::styled(elapsed_str, ...)`. The spinner should be added **before** the WP ID in each Active WP's line.

### Files in scope

| File | Changes |
|------|---------|
| `crates/kasmos/Cargo.toml` | Add throbber-widgets-tui dependency |
| `crates/kasmos/src/tui/app.rs` | DashboardState gains ThrobberState, on_tick, render_dashboard updated |

---

## Subtasks & Detailed Guidance

### Subtask T018 – Add `throbber-widgets-tui` dependency to Cargo.toml

- **Purpose**: Introduce the animated spinner widget crate.
- **Steps**:
  1. Add to `crates/kasmos/Cargo.toml` `[dependencies]`:
     ```toml
     throbber-widgets-tui = "0.10"
     ```
  2. Run `cargo check -p kasmos` to verify resolution.
- **Files**: `crates/kasmos/Cargo.toml`
- **Notes**: throbber-widgets-tui 0.10.x requires ratatui ^0.30 (confirmed in R-1).

### Subtask T019 – Add `ThrobberState` to `DashboardState`

- **Purpose**: Track animation frame position for synchronized spinners.
- **Steps**:
  1. Add field to `DashboardState`:
     ```rust
     /// Shared spinner state for Active WP indicators.
     /// Ticked on App::on_tick() every 250ms.
     pub throbber_state: throbber_widgets_tui::ThrobberState,
     ```
  2. Update `Default` impl for `DashboardState`:
     ```rust
     impl Default for DashboardState {
         fn default() -> Self {
             Self {
                 focused_lane: 0,
                 selected_index: 0,
                 scroll_offsets: [0; 4],
                 throbber_state: throbber_widgets_tui::ThrobberState::default(),
             }
         }
     }
     ```
- **Files**: `crates/kasmos/src/tui/app.rs`

### Subtask T020 – Tick `ThrobberState` in `App::on_tick()`

- **Purpose**: Advance the spinner animation frame every 250ms.
- **Steps**:
  1. Update `App::on_tick()` (currently a placeholder):
     ```rust
     pub fn on_tick(&mut self) {
         self.dashboard.throbber_state.calc_next();
     }
     ```
  2. This is called every 250ms from the event loop in `tui/mod.rs`.
  3. Per data-model.md: ThrobberState is ticked **unconditionally** regardless of active tab (so animation is smooth when switching back to Dashboard).
- **Files**: `crates/kasmos/src/tui/app.rs`

### Subtask T021 – Render Throbber for Active WPs, static badge for non-Active

- **Purpose**: Replace the static WP ID display with animated spinners for Active work packages.
- **Steps**:
  1. The current `render_dashboard()` method constructs WP items as `ListItem` with `Line::from(spans)`. The spinner needs to be **prepended** to each Active WP's line.

  2. **Challenge**: `Throbber` is a `StatefulWidget` that needs `render_stateful_widget()`, but the current code uses `List` (which renders `ListItem`s). Two approaches:

     **Approach A — Inline the throbber character manually**:
     Instead of using the `Throbber` widget directly, compute the current frame character from the throbber set and prepend it as a `Span`:
     ```rust
     use throbber_widgets_tui::BRAILLE_SIX;
     
     // In the WP item rendering loop:
     let status_span = if wp.state == WPState::Active {
         let frame_idx = self.dashboard.throbber_state.index()
             .unwrap_or(0) as usize % BRAILLE_SIX.symbols.len();
         span!(Style::default().fg(Color::Yellow); "{} ", BRAILLE_SIX.symbols[frame_idx])
     } else {
         // Static state badge
         let (badge, color) = match wp.state {
             WPState::Pending => ("○", Color::DarkGray),
             WPState::Paused => ("‖", Color::Yellow),
             WPState::ForReview => ("◈", Color::Cyan),
             WPState::Completed => ("✓", Color::Green),
             WPState::Failed => ("✗", Color::Red),
             WPState::Active => unreachable!(),
         };
         span!(Style::default().fg(color); "{} ", badge)
     };
     ```
     Then prepend `status_span` to the spans vec for each WP.

     **Approach B — Use `Throbber` widget in a custom layout per row**:
     This requires splitting each list row into a throbber cell + text cell, which is significantly more complex and would change the rendering architecture.

     **Recommended**: Approach A — simpler, maintains List-based rendering, achieves synchronized animation via shared state.

  3. Check `ThrobberState` API: The `index()` method may return an `Option<i32>` or similar. Verify in the crate source.
     - Alternative: `throbber_state.normalize(&throbber_set)` returns the current symbol directly. Check if this method exists.

  4. Update the span construction in the WP rendering loop (inside `render_dashboard()`, around line 849):
     ```rust
     // Current code builds spans starting with wp.id
     // Prepend the status_span before the wp.id span
     let mut spans = vec![status_span];
     spans.push(/* existing wp.id span */);
     spans.push(Span::raw(" "));
     spans.push(Span::raw(&wp.title));
     // ... rest of existing span construction
     ```

- **Files**: `crates/kasmos/src/tui/app.rs`
- **Notes**:
  - Use macro syntax from WP01 for all new text construction.
  - The `BRAILLE_SIX` set has 6 symbols → at 250ms tick, full rotation is 1.5s. This meets SC-004 (at least once per second).
  - Alternative throbber set: `DOT` is also clean. Use `BRAILLE_SIX` as the default.
  - The empty lane "(empty)" ListItem should NOT get a spinner — only actual WP items.

### Subtask T022 – Update dashboard rendering tests

- **Purpose**: Ensure the throbber integration doesn't break existing dashboard tests.
- **Steps**:
  1. **`test_dashboard_renders_wps_in_correct_lanes`** (line ~1267): This test renders the dashboard and checks for WP IDs in the output buffer. The spinner character will appear before WP IDs for Active WPs. The test should still pass since it checks `rendered.contains("WP01")` etc. — the WP ID is still in the rendered output. Verify this.

  2. **`test_resize_reflow_render_does_not_panic`** (line ~1113): Renders the full TUI. Should still work. Verify.

  3. **`test_event_loop_hot_paths_stay_non_blocking_under_load`** (line ~1164): Calls `handle_event()` repeatedly. The `on_tick()` is not called in this test, so ThrobberState stays at default. Should be fine.

  4. Consider adding a targeted test:
     ```rust
     #[test]
     fn test_throbber_state_advances_on_tick() {
         let (tx, _rx) = mpsc::channel(4);
         let mut app = App::new(create_test_run(1), tx);
         let initial = app.dashboard.throbber_state.index();
         app.on_tick();
         let after_tick = app.dashboard.throbber_state.index();
         assert_ne!(initial, after_tick, "ThrobberState should advance on tick");
     }
     ```
- **Files**: `crates/kasmos/src/tui/app.rs` (tests module)

---

## Risks & Mitigations

| Risk | Likelihood | Mitigation |
|------|-----------|------------|
| ThrobberState API differs from R-5 | Medium | Check crate source for exact methods (index, calc_next) |
| Spinner character width issues in List | Low | BRAILLE_SIX uses single Unicode chars — should be 1-cell wide |
| render_stateful_widget not usable in List context | N/A | Approach A avoids StatefulWidget — uses raw character lookup |
| Performance: frame lookup per WP per render | Negligible | Index lookup is O(1), WP count is small |

## Review Guidance

- **Synchronized spinners**: Run with 2+ Active WPs — all should show the same animation frame simultaneously.
- **Static badges**: Non-Active WPs should show a static badge character (○ ‖ ◈ ✓ ✗), NOT a spinner.
- **State transition**: Change a WP from Active → Paused — spinner should immediately become a static ‖ badge.
- **Tick verification**: Observe spinner animation — should cycle smoothly without flickering.
- **Tests pass**: `cargo test -p kasmos` — existing dashboard tests plus new throbber test.

## Activity Log

- 2026-02-12T00:00:00Z – system – lane=planned – Prompt created.
