# Quickstart: TUI Widget Crate Adoption

**Feature**: 005-tui-widget-crate-adoption
**Date**: 2026-02-12

## Prerequisites

Before starting implementation:

1. **Feature 002** (`ratatui-tui-controller-panel`) must be merged to `master`
2. **Feature 006** (`dependency-upgrade-2026`) must be merged to `master` (upgrades ratatui to 0.30)
3. Verify ratatui version: `grep 'ratatui' crates/kasmos/Cargo.toml` should show `0.30`

## Build & Test (per WP)

After each crate adoption WP:

```bash
# Build check
cargo build -p kasmos

# Full test suite (must pass with zero new failures — SC-005)
cargo test -p kasmos

# Lint check (must produce zero new warnings — SC-006)
cargo clippy -p kasmos -- -D warnings

# Optional: run the TUI to visually verify
cargo run -p kasmos -- start --feature <test-feature>
```

## WP-Specific Verification

### WP01: ratatui-macros

```bash
# Verify macros compile correctly
cargo build -p kasmos

# Verify no behavioral regressions
cargo test -p kasmos

# Verify no clippy warnings from macro usage
cargo clippy -p kasmos -- -D warnings

# Spot check: confirm no remaining verbose Layout::default().direction(...) patterns
# (except where macros genuinely don't apply)
grep -rn 'Layout::default()' crates/kasmos/src/tui/
```

### WP02: tui-logger

```bash
# Verify LogsState, LogEntry, LogLevel are fully removed
grep -rn 'LogsState\|LogEntry\|LogLevel' crates/kasmos/src/tui/

# Verify tui-logger dependency added with tracing-support
grep 'tui-logger' crates/kasmos/Cargo.toml

# Verify tracing integration: init_logger called before subscriber
grep -n 'init_logger\|TuiTracingSubscriberLayer' crates/kasmos/src/

# Run and verify logs appear in tui-logger widget
cargo run -p kasmos -- start --feature <test-feature>
# → Switch to Logs tab (press 3)
# → Verify events appear grouped by target
# → Press 'h' to toggle target selector
# → Use PageUp to enter page mode
```

### WP03: tui-popup

```bash
# Verify tui-popup dependency added
grep 'tui-popup' crates/kasmos/Cargo.toml

# Verify old manual centering code removed
grep -rn 'Clear\|centered.*Rect' crates/kasmos/src/tui/

# Run and trigger a confirmation dialog
cargo run -p kasmos -- start --feature <test-feature>
# → Press Force-Advance on a WP
# → Verify centered popup appears
# → Press 'n' to dismiss, 'y' to confirm
# → Resize terminal while popup is visible — verify it re-centers
```

### WP04: throbber-widgets-tui

```bash
# Verify dependency added
grep 'throbber-widgets-tui' crates/kasmos/Cargo.toml

# Run with Active WPs
cargo run -p kasmos -- start --feature <test-feature>
# → Verify Active WPs show animated spinners
# → Verify non-Active WPs show static badges
# → Verify spinners are synchronized across multiple Active WPs
```

### WP05: tui-nodes

```bash
# Verify dependency added
grep 'tui-nodes' crates/kasmos/Cargo.toml

# Run with a feature that has WP dependencies
cargo run -p kasmos -- start --feature <feature-with-deps>
# → Press 'v' to toggle dependency graph view
# → Verify nodes appear for each WP
# → Verify edges connect dependent WPs
# → Verify node colors match WP state
# → Press 'v' again to return to kanban view
```

## Revertibility Check (SC-009)

After each WP merge, verify independent revertibility:

```bash
# Identify the merge commit
git log --oneline --merges -5

# Dry-run revert (do NOT actually push)
git revert --no-commit <merge-commit-sha>
cargo build -p kasmos
cargo test -p kasmos
git revert --abort
```

## Line Count Verification (SC-007)

After WP02 (tui-logger) is complete:

```bash
# Compare before/after line counts for the Logs tab implementation
# The tui-logger adoption should reduce code by at least 100 lines
# Count lines removed (LogsState, LogEntry, LogLevel, render_logs, filtered_log_entries, etc.)
# vs lines added (TuiWidgetState field, widget construction, key event forwarding)
```
