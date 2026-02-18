# Quickstart: kasmos Development

## Prerequisites

- Go 1.23+
- OpenCode (`opencode` in PATH)
- git

## Bootstrap

```bash
# Initialize Go module
go mod init github.com/user/kasmos

# Install core dependencies
go get github.com/charmbracelet/bubbletea@v2
go get github.com/charmbracelet/lipgloss@v2
go get github.com/charmbracelet/bubbles
go get github.com/charmbracelet/huh
go get github.com/spf13/cobra
go get github.com/muesli/gamut
go get github.com/mattn/go-isatty
go get gopkg.in/yaml.v3

# Create directory structure
mkdir -p cmd/kasmos
mkdir -p internal/{tui,worker,task,persist,setup}
```

## Build and Run

```bash
# Build
go build ./cmd/kasmos

# Run (interactive TUI)
./kasmos

# Run with task source
./kasmos kitty-specs/016-kasmos-agent-orchestrator/

# Run in daemon mode
./kasmos -d --format json

# Setup agent definitions
./kasmos setup
```

## Test

```bash
# All tests
go test ./...

# Verbose with race detection
go test -v -race ./...

# Integration tests (requires opencode in PATH)
KASMOS_INTEGRATION=1 go test ./...

# Specific package
go test ./internal/worker/...
go test ./internal/tui/...
```

## Development Workflow

1. **Worker package first**: Implement `WorkerBackend` interface + `SubprocessBackend`
   before TUI. This can be tested independently.

2. **TUI skeleton second**: Minimal bubbletea app that renders the layout with mock data.
   Verify responsive breakpoints work.

3. **Wire together**: Connect worker events to TUI messages. Spawn real workers.

4. **Task sources third**: Add spec-kitty/GSD adapters after core worker+TUI works.

5. **Polish last**: Overlays, help, AI helpers, persistence, daemon mode.

## Key Files to Read First

| File | Why |
|------|-----|
| `design-artifacts/tui-mockups.md` | See what you're building (12 views) |
| `design-artifacts/tui-layout-spec.md` | Layout math and breakpoints |
| `research/tui-technical.md` Section 1 | WorkerBackend interface contract |
| `research/tui-technical.md` Section 2 | All bubbletea Msg types |
| `research/tui-technical.md` Section 10 | Package structure |
| `design-artifacts/tui-styles.md` | Copy-paste styles.go code |
| `design-artifacts/tui-keybinds.md` | Copy-paste keys.go code |

## Architecture Rules

- **Never block Update()**: All I/O (spawn, read output, persist) goes through `tea.Cmd`
- **Worker -> TUI via messages**: `workerSpawnedMsg`, `workerOutputMsg`, `workerExitedMsg`
- **TUI -> Worker via commands**: `spawnWorkerCmd()`, `killWorkerCmd()`, `readOutputCmd()`
- **One spinner for all running workers**: Single `spinner.Model` in the main Model,
  its `View()` output is reused in every running worker's status cell
- **Layout recalculation on resize only**: `recalculateLayout()` runs in `tea.WindowSizeMsg`,
  not in `View()`
