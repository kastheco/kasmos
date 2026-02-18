---
work_package_id: WP02
title: Launcher Screen View
lane: planned
dependencies: []
subtasks:
- 'ASCII art block-font kasmos text (fits within 80 cols)'
- 'Gradient coloring using gamut from hot pink to purple'
- 'Menu items with key hints (n, f, p, h, r, s, q)'
- 'Each menu item has key hint left, label right, dimmed description below'
- 'Centered layout responsive to terminal width/height'
- 'renderLauncher(width, height int) string function'
- 'Version string shown below branding'
- 'All text lowercase per project convention'
phase: Wave 1 - Foundation
assignee: ''
agent: ''
shell_pid: ''
review_status: ''
reviewed_by: ''
history:
- timestamp: '2026-02-18T00:00:00Z'
lane: done
  agent: planner
  action: Prompt generated via /spec-kitty.tasks
---

# Work Package Prompt: WP02 - Launcher Screen View

## Mission

Create `internal/tui/launcher.go` with the launcher view: centered ASCII "kasmos"
branding, gradient styling, and a 7-item action menu with key hints.

## Scope
### Files to Create / Modify

```text
internal/tui/launcher.go
```

### Technical References

- `internal/tui/styles.go`
- `design-artifacts/tui-styles.md`
- `kitty-specs/017-launcher-dashboard-screen/plan.md`

## Implementation

Build `renderLauncher(width, height int) string` and supporting helpers.

Requirements:
- ASCII art block text for "kasmos" that fits within 80 columns
- Apply line-by-line gradient with gamut from `#F25D94` -> `#7D56F4`
- Render menu entries for keys: `n`, `f`, `p`, `h`, `r`, `s`, `q`
- Each entry includes key hint, label, and dimmed description
- Show version string below branding
- Use `lipgloss.Place()` to center content and recenter on resize
- Maintain correct rendering at minimum terminal size 80x24
- Keep all user-facing strings lowercase

## Verification

- `go test ./internal/tui -run Launcher`
- `go test ./...`
- Manual check in `go run ./cmd/kasmos`: launcher remains centered at 80x24 and recenters on resize
