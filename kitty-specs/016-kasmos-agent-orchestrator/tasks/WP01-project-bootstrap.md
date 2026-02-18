---
work_package_id: WP01
title: Project Bootstrap + CLI Entry Point
lane: done
dependencies: []
subtasks:
- go.mod with all dependencies
- cmd/kasmos/main.go with cobra root command
- Minimal tea.Program that renders placeholder
- go build ./cmd/kasmos compiles
phase: Wave 1 - Core TUI + Worker Lifecycle
assignee: ''
agent: ''
shell_pid: ''
review_status: ''
reviewed_by: ''
history:
- timestamp: '2026-02-17T00:00:00Z'
  lane: planned
  agent: planner
  action: Prompt generated via /spec-kitty.tasks
- timestamp: '2026-02-18T05:39:40.145695064+00:00'
  lane: doing
  actor: manager
  shell_pid: '14003'
  action: 'transition active (Launching Wave 1 parallel pair: WP01 (bootstrap) + WP02 (worker backend))'
- timestamp: '2026-02-18T06:08:46.604948124+00:00'
  lane: done
  actor: manager
  shell_pid: '401658'
  action: 'transition done (Verified: 18/18 PASS. Build, vet, runtime all clean.)'
---

# Work Package Prompt: WP01 - Project Bootstrap + CLI Entry Point

## Mission

Create the kasmos Go project from scratch: go.mod with all dependencies, the cobra
CLI entry point, and a minimal bubbletea program that compiles and runs. This is
the foundation every other WP builds on.

## Scope

### Files to Create

```
go.mod                     # Module declaration + all dependencies
cmd/kasmos/main.go         # Entry point: cobra root command + tea.Program setup
```

### Technical References

- `kitty-specs/016-kasmos-agent-orchestrator/research/tui-technical.md` Section 10
  (Package Structure, Key Dependencies)
- `.kittify/memory/constitution.md` (Go 1.23+, bubbletea v2, lipgloss v2)

## Implementation

### go.mod

```
module github.com/user/kasmos

go 1.23
```

Dependencies (from tui-technical.md Section 10):
- `github.com/charmbracelet/bubbletea` v2 (the TUI framework)
- `github.com/charmbracelet/bubbles` (table, viewport, spinner, help, textinput, list)
- `github.com/charmbracelet/lipgloss` v2 (styling)
- `github.com/charmbracelet/huh` (form dialogs)
- `github.com/muesli/gamut` (gradient colors)
- `github.com/mattn/go-isatty` (terminal detection for daemon mode)
- `github.com/spf13/cobra` (CLI command structure)
- `gopkg.in/yaml.v3` (WP frontmatter parsing)

Run `go mod tidy` after creating go.mod to resolve exact versions.

### cmd/kasmos/main.go

Set up cobra with a root command that:
1. Parses flags (placeholder for now: `--version`)
2. Creates a minimal bubbletea Model (just stores width/height)
3. Initializes `tea.NewProgram(model, tea.WithAltScreen())` 
4. Runs the program
5. Exits cleanly on quit

The minimal Model should:
- Handle `tea.WindowSizeMsg` (store dimensions)
- Handle `tea.KeyMsg` for `q` and `ctrl+c` (quit)
- Render a centered placeholder: "kasmos v0.1.0 - press q to quit"
- Return `tea.Quit` on quit keys

Signal handling setup (from tui-technical.md Section 8):
```go
ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
defer stop()
```

Pass the context to `tea.NewProgram` via `tea.WithContext(ctx)`.

### What NOT to Do

- Do NOT implement any TUI styling, layout, or panels (that is WP03)
- Do NOT implement the worker backend (that is WP02)
- Do NOT add the `setup` subcommand yet (that is WP10)
- Keep the model minimal -- just enough to prove the scaffold works

## Acceptance Criteria

1. `go build ./cmd/kasmos` produces a binary without errors
2. Running `./kasmos` shows a terminal UI with the placeholder text
3. Pressing `q` or `ctrl+c` exits cleanly
4. `go vet ./...` reports no issues
5. Terminal resize updates dimensions (no crash)
