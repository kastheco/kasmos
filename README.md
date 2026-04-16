<p align="center">
  <img src="web/public/logo-full.png" alt="kasmos" width="500" />
</p>

# [![CI](https://github.com/kastheco/kasmos/actions/workflows/build.yml/badge.svg)](https://github.com/kastheco/kasmos/actions/workflows/build.yml) [![GitHub Release](https://img.shields.io/github/v/release/kastheco/kasmos)](https://github.com/kastheco/kasmos/releases/latest) [![License: BSL 1.1](https://img.shields.io/badge/License-BSL_1.1-blue.svg)](LICENSE.md) [![docs](https://img.shields.io/badge/docs-kasmos.kasthe.co-blue)](https://kasmos.kasthe.co)

> mcp-first multi-agent orchestration for git repos: global task store, shared http mcp endpoint, daemon, admin spa, and tui in one tool.

[**docs →** kasmos.kasthe.co](https://kasmos.kasthe.co)

---

![kasmos screenshot](assets/screenshot.gif)

---

## features

- **global task store** — tasks live at `~/.config/kasmos/taskstore.db`, shared across all managed repos via the daemon
- **shared http mcp endpoint** — `kas serve` (or the `kasmosdb` systemd/launchd service) hosts a single mcp server at `http://127.0.0.1:7434/mcp`; all scaffolded harness configs point agents here — no per-agent stdio subprocess needed
- **wave-based orchestration** — planning → architect → implement → review → readiness review lifecycle with per-wave agent concurrency
- **multi-harness support** — works with claude, opencode, codex, and other mcp-aware agents
- **tui + admin spa + daemon** — terminal and browser control surfaces backed by the same service pair
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
go install github.com/kastheco/kasmos/cmd/kas@latest
```

### install script

```bash
curl -fsSL https://raw.githubusercontent.com/kastheco/kasmos/main/install.sh | bash
```

### release asset

prebuilt binaries for macos and linux are on the [releases page](https://github.com/kastheco/kasmos/releases/latest).

the compiled binary is `kas`. `kasmos` remains the project name, module path (`github.com/kastheco/kasmos`), config namespace (`.kasmos/`, `~/.config/kasmos/`), and release archive prefix — only the installed binary artifact is named `kas`.

---

## quick start

from inside a git repo:

```bash
kas serve        # local/dev: start rest api + admin ui + shared http mcp endpoint
kas setup        # scaffold harness configs (.mcp.json, .codex/config.toml, agent prompts) pointing at the shared endpoint
kas              # open the tui
```

open `http://127.0.0.1:7433/admin/` for the admin spa. for always-on operation, run the `kasmosdb` + `kasmos` user services instead of foreground `kas serve` / `kas daemon` commands.

`kas setup` (and `kas reset`) write per-harness mcp config so mcp-aware agents connect to the shared http mcp endpoint (`http://127.0.0.1:7434/mcp`): claude and opencode use `.mcp.json`, codex uses a project-local `.codex/config.toml` with an `[mcp_servers.kasmos]` block (codex cli reads this natively for trusted projects). the endpoint must be running before starting managed agent sessions. `kas check` will warn if the shared endpoint is unreachable or if stale `kas mcp` stdio processes are still running.

> **note:** `kas mcp` (stdio mode) is still available as a fallback for manual or harness-specific use, but it is not the default transport and should not appear in scaffolded configs.

see the [getting started guide](https://kasmos.kasthe.co/docs/getting-started) for a full walkthrough including harness setup and your first task.

---

## mcp server

kasmos exposes its tools via a **shared http mcp endpoint** hosted by `kas serve` (or the `kasmosdb` background service) at `http://127.0.0.1:7434/mcp`. all scaffolded harness configs (written by `kas setup` / `kas reset`) point agents at this single endpoint — no per-agent stdio subprocess is spawned, eliminating the memory overhead of one `kas mcp` process per agent session.

`kas mcp` (stdio) is still available for manual or custom harness use, but it is the fallback transport, not the default. managed agent sessions require the shared http endpoint to be running; if it is down, `kas check` will report the missing endpoint as an error and list any lingering `kas mcp` stdio processes as a warning.

scaffolded agents are expected to use the shared http mcp endpoint for task, signal, and instance operations. `kas task` / `kas signal` are operator and fallback tools, not the default agent path.

tool groups exposed:

| group | tools |
|-------|-------|
| filesystem | `find_files`, `list_dir`, `read_file`, `grep` |
| git | `git_status`, `git_diff`, `git_log` |
| tasks | `task_list`, `task_show`, `task_create`, `task_update_content`, `task_transition` |
| signals | `signal_create` |
| instances | `instance_list`, `instance_pause`, `instance_resume`, `instance_send` |
| daemon | `daemon_status` |

filesystem and git tools are sandboxed to the repo root. see the [mcp server docs](https://kasmos.kasthe.co/docs/mcp-server) for details.

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
- **architecture:** [docs/architecture/README.md](docs/architecture/README.md) — runtime topology: signals, state, and data-flow diagrams
- **contributing:** [CONTRIBUTING.md](CONTRIBUTING.md)
- **license:** [BSL 1.1](LICENSE.md) — converts to [Apache 2.0](https://www.apache.org/licenses/LICENSE-2.0) on the change date
