<p align="center">
  <img src="web/public/logo-full.png" alt="kasmos" width="500" />
</p>

# [![CI](https://github.com/kastheco/kasmos/actions/workflows/build.yml/badge.svg)](https://github.com/kastheco/kasmos/actions/workflows/build.yml) [![GitHub Release](https://img.shields.io/github/v/release/kastheco/kasmos)](https://github.com/kastheco/kasmos/releases/latest) [![License: BSL 1.1](https://img.shields.io/badge/License-BSL_1.1-blue.svg)](LICENSE.md) [![docs](https://img.shields.io/badge/docs-kasmos.kasthe.co-blue)](https://kasmos.kasthe.co)

> mcp-first multi-agent orchestration for git repos: task store, stdio mcp server, daemon, worktrees, and tui in one tool.

[**docs →** kasmos.kasthe.co](https://kasmos.kasthe.co)

---

![kasmos screenshot](assets/screenshot.gif)

---

## features

- **global task store** — tasks live at `~/.config/kasmos/taskstore.db`, shared across all managed repos via the daemon
- **stdio mcp server** — `kas mcp` exposes filesystem, git, task, signal, and instance tools over stdio; agents connect via `.mcp.json`
- **wave-based orchestration** — planning → architect → implement → review lifecycle with per-wave agent concurrency
- **multi-harness support** — works with claude, opencode, codex, and other mcp-aware agents
- **tui + daemon** — interactive tui for task management plus a headless daemon for automated orchestration
- **git worktree isolation** — each task runs in its own branch and worktree; merges are handled at review time
- **cross-platform services** — systemd (linux) and launchd (macos) for always-on operation; see [service management docs](https://kasmos.kasthe.co/docs/service-management)

---

## install

### homebrew

```bash
brew install kastheco/tap/kasmos
```

### go install

```bash
go install github.com/kastheco/kasmos@latest
```

### install script

```bash
curl -fsSL https://raw.githubusercontent.com/kastheco/kasmos/main/install.sh | bash
```

### release asset

prebuilt binaries for macos and linux are on the [releases page](https://github.com/kastheco/kasmos/releases/latest).

the primary command surface is `kas`; if your install only provides `kasmos`, add a `kas` symlink or use `kasmos` in its place.

---

## quick start

from inside a git repo:

```bash
kas setup        # scaffold harness configs (.mcp.json, agent prompts)
kas              # open the tui
```

`kas setup` writes `.mcp.json` so mcp-aware agents (claude, opencode, codex) automatically connect to the kasmos stdio mcp server.

see the [getting started guide](https://kasmos.kasthe.co/docs/getting-started) for a full walkthrough including harness setup and your first task.

---

## mcp server

`kas mcp` starts a stdio mcp server that agents connect to via `.mcp.json`. `kas serve` is an optional http surface for the admin web ui and rest api.

tool groups exposed:

| group | tools |
|-------|-------|
| filesystem | `find_files`, `list_dir`, `read_file`, `grep` |
| git | `git_status`, `git_diff`, `git_log` |
| tasks | `task_list`, `task_show`, `task_create`, `task_update_content`, `task_transition` |
| signals | `signal_create` |
| instances | `instance_list`, `instance_pause`, `instance_resume`, `instance_send` |
| daemon | `daemon_status` |

filesystem and git tools are sandboxed to the repo root. `kas setup` writes the `.mcp.json` that registers the server; see the [mcp server docs](https://kasmos.kasthe.co/docs/mcp-server) for details.

---

## useful commands

```bash
kas setup                              # scaffold project config and harness files
kas serve                              # start rest api + admin web ui
kas task list                          # list all tasks
kas task create <name>                 # create a new task
kas task show <task-file>              # show task details
kas task implement <task-file>         # start orchestration for a task
kas task transition <task-file> <event># advance task lifecycle
kas reset                              # refresh scaffold state (with backup)
kas check -v                           # verify scaffold health
kas status                             # show daemon and server status
```

see the [cli reference](https://kasmos.kasthe.co/docs/cli-reference) for the full command surface.

---

## links

- **documentation:** [kasmos.kasthe.co](https://kasmos.kasthe.co)
- **contributing:** [CONTRIBUTING.md](CONTRIBUTING.md)
- **license:** [BSL 1.1](LICENSE.md) — converts to [Apache 2.0](https://www.apache.org/licenses/LICENSE-2.0) on the change date
