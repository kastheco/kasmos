# kasmos

Go/bubbletea TUI for orchestrating concurrent OpenCode agent sessions from one terminal dashboard.

## Features

- Full-screen terminal UI built with Bubble Tea v2 and Lip Gloss v2
- Responsive 4-mode layout: `tooSmall`, `narrow`, `standard`, `wide`
- Spawn, kill, restart, and continue OpenCode worker sessions
- Multi-worker dashboard with live output viewport and fullscreen mode
- Worker chain tree rendering (parent/child session relationships)
- Task-aware orchestration from three sources: spec-kitty, GSD markdown, and ad-hoc
- Batch spawn for unassigned tasks
- AI helpers: failure analysis (`a`) and prompt generation (`g`)
- Session persistence to `.kasmos/session.json` with debounced atomic writes
- Resume previous sessions with `--attach`
- Headless daemon mode (`-d`) with human or NDJSON output
- `kasmos setup` for dependency validation and agent scaffolding

## Installation

```sh
go install ./cmd/kasmos
# or
go build ./cmd/kasmos
```

## Quick Start

```sh
# validate deps and scaffold .opencode/agents/*.md
kasmos setup

# run interactive dashboard (ad-hoc mode)
kasmos

# run with a spec-kitty feature directory
kasmos path/to/spec-kitty-feature-dir

# run with a GSD markdown task file
kasmos path/to/tasks.md
```

Basic flow:

1. Start kasmos, optionally pointing at a task source.
2. Spawn workers with `s`, monitor output in the viewport.
3. Continue completed/failed sessions with `c`, restart failed ones with `r`.
4. Reattach later with `kasmos --attach`.

## Usage

```
kasmos [task-source-path] [flags]
```

| Command / Flag | Description |
| --- | --- |
| `kasmos` | Interactive TUI, ad-hoc mode |
| `kasmos <path>` | Auto-detect source (spec-kitty dir or GSD `.md` file) |
| `kasmos setup` | Dependency checks + agent scaffolding |
| `kasmos -d` | Headless daemon mode (human logs) |
| `kasmos -d --format json` | Daemon mode with NDJSON output |
| `kasmos -d --spawn-all <path>` | Spawn all unblocked tasks, exit when complete |
| `kasmos --attach` | Restore session from `.kasmos/session.json` |
| `kasmos --version` | Print version |

## Keybindings

| Key | Action | Context |
| --- | --- | --- |
| `j`/`k`, `down`/`up` | Move selection / scroll | table, tasks, viewport |
| `tab` / `shift+tab` | Next / previous panel | main view |
| `s` | Spawn worker | table / tasks |
| `x` | Kill running worker | table |
| `c` | Continue session | table / fullscreen (exited/failed worker with session ID) |
| `r` | Restart worker | table / fullscreen (failed/killed worker) |
| `b` | Batch spawn dialog | task source mode with unassigned tasks |
| `f` | Toggle fullscreen output | table / viewport |
| `d` / `u` | Half-page down / up | viewport |
| `G` / `g` | Go to bottom / top | viewport |
| `a` | Analyze failed worker (AI) | table (failed worker selected) |
| `g` | Generate task prompt (AI) | table (task source mode) |
| `enter` | Select / confirm | context-dependent |
| `?` | Toggle help | global |
| `q` | Quit (confirms if workers running) | global |
| `ctrl+c` | Force quit | global |
| `esc` | Back / close overlay | global / dialogs |
| `space` | Toggle task selection | batch dialog |

## Task Sources

kasmos detects task sources from the positional argument:

| Input | Source type | Detection |
| --- | --- | --- |
| Directory with `tasks/*.md` | spec-kitty | YAML frontmatter with dependencies, roles |
| `.md` file | GSD | Checkbox lines (`- [ ] task` / `- [x] task`) |
| No argument | ad-hoc | Manual worker spawning only |

## Session Persistence

- File: `.kasmos/session.json`
- Writes are debounced (~1s) and atomic (write to temp, rename)
- Restore with `kasmos --attach` to reload workers and session metadata
- Orphan detection via PID checking on attach

## Daemon Mode

Run headless with `-d` for CI/automation:

- `--format default` (or omit): human-readable log lines
- `--format json`: one JSON object per line (NDJSON)
- `--spawn-all`: auto-spawn all unblocked tasks and exit when all workers complete

Events: `session_start`, `worker_spawn`, `worker_exit`, `worker_kill`, `session_end`

## Architecture

```
cmd/kasmos/          Cobra CLI entry point, flags, setup subcommand
internal/tui/        Bubble Tea model/update/view, layout, keymap, dialogs, daemon logging
internal/worker/     Backend interface, subprocess backend (opencode), output buffering
internal/task/       Source interface + spec-kitty, GSD, ad-hoc adapters
internal/persist/    Session snapshot model and persistence
internal/setup/      Dependency validation and agent scaffold generation
```

## Build and Test

```sh
go build ./cmd/kasmos
go test ./...
go vet ./...
```
