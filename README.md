# kasmos [![CI](https://github.com/kastheco/kasmos/actions/workflows/build.yml/badge.svg)](https://github.com/kastheco/kasmos/actions/workflows/build.yml) [![GitHub Release](https://img.shields.io/github/v/release/kastheco/kasmos)](https://github.com/kastheco/kasmos/releases/latest) [![License: BSL 1.1](https://img.shields.io/badge/License-BSL_1.1-blue.svg)](LICENSE.md)

> harness & model-agnostic ai orchestration tool with automated wave-based implementation — powered by scaffolded agent skills, tmux, and git worktrees.

![kasmos screenshot](assets/screenshot.gif)

---

## what it does

kasmos turns your terminal into a multi-agent control center. each task gets its own isolated git worktree and a fresh tmux session at every lifecycle stage: a planner agent writes the implementation plan, an architect can decompose it into coder-ready waves, coder agents execute it wave by wave, and a reviewer agent validates the result — all managed from a single tui.

- **task-centric workflow** — create tasks with name + description, organize them into topics, and track status through the full lifecycle (planning → implementing → reviewing → done)
- **wave orchestration** — task content is split into waves; kasmos automatically runs parallel agents per wave, advancing only when all tasks pass
- **isolated workspaces** — every task gets a dedicated git worktree and tmux session; no branch conflicts, no shared state
- **live agent preview** — the center pane embeds a live terminal so you can watch agents work without leaving kasmos
- **diff + git views** — review changes and git history before merging, right inside the TUI
- **auto-accept mode** — run agents unattended with a background daemon handling permission prompts

---

## installation

#### homebrew

```bash
brew install kastheco/tap/kasmos
```

#### go install

```bash
go install github.com/kastheco/kasmos@latest
```

#### install script

```bash
curl -fsSL https://raw.githubusercontent.com/kastheco/kasmos/main/install.sh | bash
```

installs the `kasmos` binary to `~/.local/bin`. to install with a custom name:

```bash
curl -fsSL https://raw.githubusercontent.com/kastheco/kasmos/main/install.sh | bash -s -- --name kq
```

#### download binary

pre-built binaries for macOS, linux, and windows are on the [releases page](https://github.com/kastheco/kasmos/releases/latest).

---

## prerequisites

- [tmux](https://github.com/tmux/tmux/wiki/Installing)
- [gh](https://cli.github.com/)
- at least one supported AI CLI: **[opencode](https://github.com/sst/opencode)**, [claude code](https://github.com/anthropics/claude-code), [codex](https://github.com/openai/codex), [gemini CLI](https://github.com/google-gemini/gemini-cli), [amp](https://ampcode.com), or [aider](https://aider.chat)

---

## getting started

the cobra command surface is `kas`. if your install produced a `kasmos` binary instead,
substitute `kasmos` in the examples below.

run from within a git repository:

```bash
kas
```

on first run, use the setup wizard to configure your agent harnesses and install skills:

```bash
kas setup
```

the wizard detects installed agent CLIs, lets you assign roles, and scaffolds the
project-local `.kasmos/` files kasmos needs.

---

## usage

```
usage:
  kas [flags]
  kas [command]

available commands:
  setup       configure agent harnesses, install skills, and scaffold project files
  task        manage task lifecycle and content
  serve       start the task store http server (sqlite-backed)
  instance    inspect and control managed agent instances
  audit       query audit events
  tmux        inspect and adopt orphan tmux sessions
  signal      inspect, emit, and process lifecycle signals
  daemon      manage the background daemon
  monitor     inspect daemon health
  status      show a repository status summary
  check       audit scaffold and skill sync health
  reset       reset all stored instances and clean up tmux sessions and worktrees
  debug       print debug information like config paths
  version     print the version number

flags:
  -p, --program string   agent to use for new instances (e.g. 'opencode', 'codex', 'aider --model ...')
  -y, --autoyes          automatically accept all agent prompts (experimental)
  -h, --help             help for kas
```

### keybindings

| key | action |
|-----|--------|
| `n` | new task |
| `/` | search tasks |
| `space` | open context menu |
| `tab` | cycle focus (sidebar → list → preview) |
| `↑ / ↓` | navigate |
| `i` | interactive mode (focus agent pane) |
| `ctrl-q` | exit interactive mode |
| `?` | help |
| `q` | quit |

---

## how it works

1. **tasks** live in the task store — use `kas task list` to inspect metadata and `kas task show <file>` to read stored markdown
2. **topics** group related tasks and act as scheduling boundaries (only one implementing task per topic at a time)
3. **waves** divide implementation into phases — kasmos parses `## Wave N` headers and runs each wave's tasks in parallel
4. **agents** run in isolated tmux sessions with dedicated git worktrees; the TUI shows live output in the preview pane
5. **signals** are emitted into the task-store-backed gateway and processed by the daemon; the legacy `.kasmos/signals/` directory remains as a compatibility path, not the primary control plane
6. **review** is automated — a reviewer agent checks the implementation, and kasmos prompts for merge/PR approval before closing the task

---

## task store

task state is stored in a project-local SQLite database under `<repo-root>/.kasmos/taskstore.db`. `config/config.go:GetConfigDir` anchors `.kasmos/` to the main repo root even when you launch kasmos from a git worktree.

#### managing tasks

use the `kas task` CLI:

```bash
kas task list                          # list all tasks
kas task list --status implementing    # filter by status
kas task show <file>                   # read task content
kas task create <name>                 # create a new task
kas task register <file>               # register a task file from disk
kas task update-content <file>         # update task content (reads stdin)
kas task set-status <file> done --force  # force-override status
kas task transition <file> <event>     # apply FSM event
```

#### optional remote store

for multi-machine access (e.g. over tailscale or a team server), point the
project-local config at an HTTP task store:

```toml
database_url = "http://your-desktop:7433"
```

start the remote server with:

```bash
kas serve --port 7433 --db /path/to/kasmos.db
```

#### run as a systemd service

a unit file is provided in `contrib/kasmosdb.service`:

```bash
just db-service-enable
```

#### run the orchestration daemon as a systemd service

`kas signal emit ...` writes to the signal gateway. a running daemon claims pending
signals and advances task lifecycle state. the filesystem sentinel directory is kept
only for legacy compatibility and local fallback flows.

a user unit is provided in `contrib/kasmos.service`:

```bash
just kasmosd-enable
just doctord
```

or, if you want both the orchestration daemon and task store service in one step:

```bash
just services-enable
```

if you only emit a signal but no daemon is running, the signal stays pending and the
task will not advance until the daemon processes it.

#### rest api

the daemon/task-store HTTP surface is intentionally small. it lives under
`/v1/repos/{project}/...`; some route segments still use legacy `/plans` naming
even though the current CLI is `kas task ...`:

```bash
# daemon/task-store status
curl http://localhost:7433/v1/status

# list registered repos
curl http://localhost:7433/v1/repos

# list all tasks
curl http://localhost:7433/v1/repos/kasmos/plans
```

for filtered task views, use the CLI (`kas task list --status ...`) until the
HTTP surface grows matching server-side filters.

---

## configuration

config lives at `<repo-root>/.kasmos/config.toml`. locate it with:

```bash
kas debug
```

key settings:

```toml
database_url = "http://localhost:7433"  # remote task store (optional)
```

---

## license

[BSL 1.1](LICENSE.md) - converts to [Apache 2.0](https://www.apache.org/licenses/LICENSE-2.0) on the change date. see LICENSE.md for details.
