---
work_package_id: WP06
title: tmux Visual Integration
lane: "doing"
dependencies: []
base_branch: main
base_commit: 2e0aad875f49c52020bc757413ccc9c9d19e8a18
created_at: '2026-02-20T08:25:21.455854+00:00'
subtasks: [T030, T031, T032, T033, T034]
phase: implementation
shell_pid: "4116651"
history:
- timestamp: '2026-02-20T14:00:00Z'
  lane: planned
  actor: manager
  action: created work package - tmux border theming and pane titles
---

# WP06: tmux Visual Integration

## Implementation Command

```bash
spec-kitty implement WP06
```

## Objective

Make tmux worker panes look visually integrated with the kasmos TUI by theming tmux pane borders to match the kasmos bubblegum palette, setting worker pane titles, and hiding the tmux status bar. This WP is independent of WP01-05 (browser feature) and touches only `internal/worker/` files.

**Independence note**: This WP has zero overlap with WP01-05 and can run in parallel with any of them.

## Context

### Current State

`TmuxBackend.Init()` creates the parking window and tags the session, but applies no visual theming. The tmux pane border uses the user's default tmux theme, creating a visual disconnect between kasmos (lipgloss-styled) and the worker pane.

### kasmos Palette (from `internal/tui/styles.go`)

```go
colorPurple    = "#7D56F4"  // focused border, running indicator
colorDarkGray  = "#383838"  // unfocused border
colorHotPink   = "#F25D94"  // dialog border, header
colorCream     = "#FFFDF5"  // text
colorMidGray   = "#5C5C5C"  // help text, pending
```

### Existing TmuxCLI Interface (`internal/worker/tmux_cli.go`)

The interface has `SetPaneOption(ctx, paneID, option, value)` for pane-level options (currently used for `remain-on-exit`). It does NOT have a session/window-level `set-option` method needed for pane border styles.

---

## Subtask T030: Add SetOption to TmuxCLI Interface

**Purpose**: Add a session-level `set-option` method to the `TmuxCLI` interface and `tmuxExec` implementation. Distinct from the existing `SetPaneOption` which uses `-p` flag.

**Steps**:

1. Add to the `TmuxCLI` interface in `internal/worker/tmux_cli.go`:

   ```go
   SetOption(ctx context.Context, key, value string) error
   ```

2. Implement on `tmuxExec`:

   ```go
   func (t *tmuxExec) SetOption(ctx context.Context, key, value string) error {
       _, err := t.run(ctx, "set-option", key, value)
       return err
   }
   ```

3. This calls `tmux set-option <key> <value>` (session-level, no `-p` or `-g`).

**Files**: `internal/worker/tmux_cli.go`

**Validation**:
- [ ] `SetOption` compiles and is distinct from `SetPaneOption`
- [ ] Mock implementations in tests updated to include the new method

---

## Subtask T031: Add SetPaneTitle to TmuxCLI Interface

**Purpose**: Add a method to set a tmux pane's title via `select-pane -T`. This enables showing the worker ID and role in the pane border format.

**Steps**:

1. Add to the `TmuxCLI` interface:

   ```go
   SetPaneTitle(ctx context.Context, paneID, title string) error
   ```

2. Implement on `tmuxExec`:

   ```go
   func (t *tmuxExec) SetPaneTitle(ctx context.Context, paneID, title string) error {
       _, err := t.run(ctx, "select-pane", "-t", paneID, "-T", title)
       return err
   }
   ```

**Files**: `internal/worker/tmux_cli.go`

**Validation**:
- [ ] `SetPaneTitle` sets the pane title visible in `pane-border-format`
- [ ] Does not change which pane has focus (no side effect beyond title)

---

## Subtask T032: Apply Palette Theming in TmuxBackend.Init()

**Purpose**: Set tmux pane border styles and border format during backend initialization so all worker panes inherit the kasmos visual theme.

**Steps**:

1. Add theming calls at the end of `TmuxBackend.Init()`, after the parking window is created:

   ```go
   // Theme pane borders to match kasmos palette
   _ = b.cli.SetOption(ctx, "pane-border-style", "fg=#383838")
   _ = b.cli.SetOption(ctx, "pane-active-border-style", "fg=#7D56F4")
   _ = b.cli.SetOption(ctx, "pane-border-lines", "heavy")
   _ = b.cli.SetOption(ctx, "pane-border-format", " #{pane_title} ")
   ```

2. Errors are ignored (best-effort theming). Older tmux versions may not support all options.

3. Matching rationale:
   - `fg=#383838` = `colorDarkGray` = `colorUnfocusBorder` in kasmos
   - `fg=#7D56F4` = `colorPurple` = `colorFocusBorder` in kasmos
   - `heavy` border lines give bolder visual weight matching lipgloss rounded borders
   - `pane-border-format` shows the pane title (set per-worker in T033)

**Files**: `internal/worker/tmux.go`

**Validation**:
- [ ] Pane borders match kasmos focused/unfocused colors
- [ ] `pane-border-format` shows worker title
- [ ] Theming is best-effort (no error propagation)
- [ ] Works with tmux 2.6+ (minimum version from skill)

---

## Subtask T033: Set Pane Title on Spawn

**Purpose**: When a worker pane is created, set its title to show the worker ID and role. This appears in the `pane-border-format` configured in T032.

**Steps**:

1. In `TmuxBackend.Spawn()`, after the pane is created and `remain-on-exit` is set, add:

   ```go
   title := cfg.ID
   if cfg.Role != "" {
       title = fmt.Sprintf("%s %s", cfg.ID, cfg.Role)
   }
   _ = b.cli.SetPaneTitle(ctx, paneID, title)
   ```

2. This renders in the border as e.g., ` w-001 coder `.

**Files**: `internal/worker/tmux.go`

**Validation**:
- [ ] Worker pane border shows "w-001 coder" (or equivalent)
- [ ] Pane with no role shows just the worker ID
- [ ] Title survives pane parking and showing (tmux preserves titles)

---

## Subtask T034: Hide tmux Status Bar

**Purpose**: kasmos has its own status bar at the bottom of the TUI. The tmux status bar is redundant and breaks the visual integration. Hide it during Init().

**Steps**:

1. Add to `TmuxBackend.Init()`, after border theming:

   ```go
   _ = b.cli.SetOption(ctx, "status", "off")
   ```

2. Consider: if the user has a useful tmux status bar, hiding it may be unwelcome. Add a `PreserveStatus bool` field to `TmuxBackend` that skips this call when true. Default to hiding it.

**Files**: `internal/worker/tmux.go`

**Validation**:
- [ ] tmux status bar hidden when kasmos is running in tmux mode
- [ ] Status bar restored on `Cleanup()` (add `_ = b.cli.SetOption(ctx, "status", "on")` to Cleanup)
- [ ] PreserveStatus flag respected if set

---

## Definition of Done

- [ ] `SetOption` and `SetPaneTitle` added to TmuxCLI interface
- [ ] `go build ./internal/worker/` succeeds
- [ ] `go test ./internal/worker/` passes (mock TmuxCLI updated)
- [ ] Pane borders themed with kasmos palette colors
- [ ] Worker pane titles show ID and role
- [ ] tmux status bar hidden (restored on cleanup)
- [ ] All theming is best-effort (errors swallowed, older tmux still works)

## Risks

- **tmux version compatibility**: `pane-border-format` requires tmux 2.3+, `pane-border-lines` requires tmux 3.2+. The `heavy` style may silently fail on older tmux. This is acceptable -- the feature degrades gracefully.
- **User tmux config conflict**: If the user has custom tmux theming, kasmos overrides it for the session. The `Cleanup()` restoration only resets the status bar, not border styles. This is a known limitation.

## Reviewer Guidance

- Verify `SetOption` is session-scoped (no `-p` or `-g` flag) -- it applies to the current session only
- Verify mock TmuxCLI in tests is updated with both new methods
- Verify Cleanup restores the status bar
- Check that the colors match `styles.go` exactly (hex values, not names)
