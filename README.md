<p align="center">
  <img src="web/public/logo-full.png" alt="kasmos" width="500" />
</p>

# kasmos [![CI](https://github.com/kastheco/kasmos/actions/workflows/build.yml/badge.svg)](https://github.com/kastheco/kasmos/actions/workflows/build.yml) [![GitHub Release](https://img.shields.io/github/v/release/kastheco/kasmos)](https://github.com/kastheco/kasmos/releases/latest) [![License: BSL 1.1](https://img.shields.io/badge/License-BSL_1.1-blue.svg)](LICENSE.md)

> mcp-first multi-agent orchestration for git repos: task store, streamable-http mcp server, daemon, worktrees, and tui in one tool.

![kasmos screenshot](assets/screenshot.gif)

---

## what kasmos is now

kasmos is no longer just a tmux-driven terminal ui.

it now combines:

- a **repo-local task store** at `<repo-root>/.kasmos/taskstore.db`
- a **streamable http MCP server** exposed by `kas serve`
- a **rest api** for remote task-store access
- a **setup/scaffold system** that writes per-harness agent configs
- the **tui + daemon** that orchestrate planning, implementation, review, and fixes

the current flow is:

1. run `kas setup` inside a git repo
2. start `kas serve` to expose rest + mcp
3. point your mcp-aware clients at `http://127.0.0.1:7434/mcp` or run `kas mcp` for stdio-only clients
4. use `kas` to manage tasks, instances, and orchestration

today, step 2 is still explicit: `kas setup` does not configure/start the server for you, and launching `kas` does not auto-spawn `kas serve` if it is missing.

---

## install

### homebrew

```bash
brew install kastheco/tap/kasmos
```

### go install

```bash
go install github.com/kastheco/kasmos@latest
ln -sf "$(go env GOPATH)/bin/kasmos" "$(go env GOPATH)/bin/kas"
```

### install script

```bash
curl -fsSL https://raw.githubusercontent.com/kastheco/kasmos/main/install.sh | bash
```

### download release asset

prebuilt release archives are published for macOS and linux on the [releases page](https://github.com/kastheco/kasmos/releases/latest).

the cobra command surface is `kas`. if your install only gives you `kasmos`, either use `kasmos` everywhere or add a `kas` symlink.

---

## prerequisites

- git
- tmux
- [gh](https://cli.github.com/)
- at least one supported harness cli: `opencode`, `claude`, or `codex`

---

## quick start

from inside a git repo:

```bash
kas setup
kas serve --bind 127.0.0.1 --port 7433 --mcp --mcp-port 7434
kas
```

then connect your mcp client to:

```text
http://127.0.0.1:7434/mcp
```

the admin ui is served from the same `kas serve` process at:

```text
http://127.0.0.1:7433/admin/
```

---

## setup wizard

`kas setup` writes project-local config and scaffold files for the harnesses you select.

useful variants:

```bash
kas setup
kas setup --force
kas setup --force --clean
```

- `--clean` ignores the existing `.kasmos/config.toml` and starts from factory defaults
- `--force` overwrites existing scaffolded files

important: `kas setup --force --clean` **does not delete stale files for harnesses you stop using**. it rewrites the selected harness/config paths, but if you are doing a major migration and want a truly clean harness layout, remove stale `.claude/`, `.opencode/`, `.codex/`, or `.agents/skills/` content yourself first.

for already-initialized repos, prefer the built-in backup-first refresh flow instead of manually deleting scaffold state:

```bash
kas reset
cat repos.txt | kas reset --dry-run
cat repos.txt | kas reset --yes
kas reset repos.txt
kas reset --ignore-worktrees repos.txt
```

`kas reset`:

- creates a timestamped backup tarball per repo under `~/kasmos-backups/`
- preserves `.kasmos/config.toml` and `.kasmos/taskstore.db`
- refreshes scaffold/runtime state such as `.agents/`, `.claude/`, `.opencode/`, `.codex/`, `.worktrees/`, `.kasmos/cache`, `.kasmos/signals`, and `.mcp.json`
- re-syncs scaffold files from the current binary and the repo's existing config
- rewrites `.mcp.json` for the local MCP endpoint

to restore one repo from one backup tarball:

```bash
kas restore /path/to/backup.tar.gz /path/to/repo
kas restore --dry-run /path/to/backup.tar.gz /path/to/repo
```

the old instance cleanup behavior is still available as:

```bash
kas reset instances
```

the wizard/scaffolder writes paths like:

- `.kasmos/config.toml`
- `.agents/skills/`
- `.claude/agents/`
- `.opencode/agents/`
- `opencode.jsonc`
- `.codex/AGENTS.md`

after setup, verify scaffold health with:

```bash
kas check -v
```

`kas setup` currently scaffolds harness config only; it does **not** provision or launch the repo's MCP/REST server.

---

## mcp server

`kas serve` starts two surfaces by default:

- rest api on `--port` (default `7433`)
- streamable-http mcp server on `--mcp-port` (default `7434`)

run it locally:

```bash
kas serve --bind 127.0.0.1 --port 7433 --mcp --mcp-port 7434
```

if you want this to be always-on, run `kas serve` under your user service manager or keep it paired with the daemon in your own startup scripts for now.

for stdio-only MCP clients such as OpenClaw, run:

```bash
kas mcp
```

if you do **not** want to launch multiple commands every session, run the server + daemon as user services.

**preferred path (source checkouts):** `just services-enable` detects your OS and wires up the right service manager automatically.

```bash
just services-enable
```

for packaged installs (brew / release binary), follow the manual steps for your platform below.

### linux (systemd)

```bash
mkdir -p ~/.config/systemd/user

cat > ~/.config/systemd/user/kasmosdb.service <<'EOF'
[Unit]
Description=kasmos task store and mcp server
After=network.target

[Service]
Type=simple
ExecStart=%h/.local/bin/kas serve
Restart=on-failure
RestartSec=5
Environment=HOME=%h

[Install]
WantedBy=default.target
EOF

cat > ~/.config/systemd/user/kasmos.service <<'EOF'
[Unit]
Description=kasmos orchestration daemon
After=network.target

[Service]
Type=simple
ExecStart=%h/.local/bin/kas daemon start --foreground
ExecStop=%h/.local/bin/kas daemon stop
Restart=on-failure
RestartSec=5
Environment=HOME=%h

[Install]
WantedBy=default.target
EOF

systemctl --user daemon-reload
systemctl --user enable --now kasmosdb kasmos
```

if your `kas` binary lives somewhere else, replace `%h/.local/bin/kas` with the real path from `command -v kas`.

### macos (launchd)

plist templates are shipped in `contrib/`. install them into `~/Library/LaunchAgents/` and load them with `launchctl`:

```bash
mkdir -p ~/Library/LaunchAgents ~/Library/Logs/kasmos

# render and install the plists (replace the kas path if needed)
KAS=$(command -v kas)
sed "s|__KAS_BIN__|$KAS|g; s|__HOME__|$HOME|g" \
  contrib/com.kasmos.taskstore.plist > ~/Library/LaunchAgents/com.kasmos.taskstore.plist
sed "s|__KAS_BIN__|$KAS|g; s|__HOME__|$HOME|g" \
  contrib/com.kasmos.daemon.plist   > ~/Library/LaunchAgents/com.kasmos.daemon.plist

launchctl bootstrap gui/$(id -u) ~/Library/LaunchAgents/com.kasmos.taskstore.plist
launchctl bootstrap gui/$(id -u) ~/Library/LaunchAgents/com.kasmos.daemon.plist
```

logs land in `~/Library/Logs/kasmos/`. to stop the services:

```bash
launchctl bootout gui/$(id -u) ~/Library/LaunchAgents/com.kasmos.taskstore.plist
launchctl bootout gui/$(id -u) ~/Library/LaunchAgents/com.kasmos.daemon.plist
```

see the [docs](https://kasmos.dev/docs/service-management) for full details on both platforms.

after that, your normal interactive entrypoint can just be:

```bash
kas
```

common flags:

```bash
kas serve --help
```

currently the kasmos mcp server exposes tool groups for:

- filesystem: `find_files`, `list_dir`, `read_file`, `grep`
- git: `git_status`, `git_diff`, `git_log`
- task store: `task_list`, `task_show`, `task_create`, `task_update_content`, `task_transition`
- signals: `signal_create`
- instances: `instance_list`, `instance_pause`, `instance_resume`, `instance_send`
- daemon: `daemon_status`

filesystem and git tools are sandboxed to the current working directory plus the resolved repo root.

---

## how to configure your local mcp client

### generic / claude-style `.mcp.json`

create a project-local `.mcp.json`:

```json
{
  "mcpServers": {
    "kasmos": {
      "type": "http",
      "url": "http://127.0.0.1:7434/mcp"
    }
  }
}
```

that is the simplest option for any client that supports standard `mcpServers` http entries.

### stdio clients (for example OpenClaw)

use the `kas mcp` command as the server process:

```json
{
  "mcpServers": {
    "kasmos": {
      "type": "stdio",
      "command": "kas",
      "args": ["mcp"]
    }
  }
}
```

### opencode local mcp config

`kas setup` / `kas reset` scaffold a local stdio `kasmos` MCP entry into `opencode.jsonc`. each opencode session runs `kas mcp` directly from the correct repo root, so the MCP server always binds to the right project without relying on a running `kas serve` process.

if you need to add or restore it manually, use:

```jsonc
{
  "mcp": {
    "kasmos": {
      "type": "local",
      "command": ["kas", "mcp"],
      "enabled": true
    }
  }
}
```

you can place that in project-local opencode config alongside the scaffolded `opencode.jsonc` content, or in your global opencode config if you want it everywhere.

existing `opencode.jsonc` files that still contain the old remote entry (`type: "remote"`, `url: "http://127.0.0.1:7434/mcp"`) are automatically migrated to the local transport on the next `kas setup` or `kas reset`.

### remote / multi-machine use

if another machine should use the same task store, point it at the rest api in `.kasmos/config.toml`:

```toml
database_url = "http://your-host:7433"
```

then run `kas serve` on the machine that hosts the sqlite db.

---

## current config location

kasmos now anchors project state to the repo root.

- config: `<repo-root>/.kasmos/config.toml`
- task store: `<repo-root>/.kasmos/taskstore.db`
- signals: `<repo-root>/.kasmos/signals/`

show the resolved paths with:

```bash
kas debug
```

---

## useful commands

```bash
kas task list
kas task list --status implementing
kas task show <task-file>
kas task create <name>
kas task register <task-file>
kas task update-content <task-file>
kas task transition <task-file> <event>
kas task set-status <task-file> <status> --force
kas task implement <task-file> [--wave N]
kas task recover <task-file> --action <planner-finished|architect-finished|implement-finished|review-approved|review-changes|advance-review-cycle>

kas serve
kas scaffold sync
kas reset
kas restore <backup.tar.gz> <repo>
kas reset instances
kas daemon start
kas monitor
kas status
```

---

## lifecycle model

kasmos now exposes lifecycle in two layers:

- `status`: coarse task progress (`ready`, `planning`, `implementing`, `reviewing`, `done`, `cancelled`)
- `execution_phase`: operator-facing execution detail persisted alongside the task (`planned`, `architecting`, `wave N running`, `waiting for confirmation`, `fixing round N`, `reviewing round N`)

the tui info pane, navigation panel, and `kas status` all use the same vocabulary.

- a `ready` task with `execution_phase=planned` means planning is complete and implementation has not started yet.
- `architecting` means the architect pass is active before wave execution begins.
- `wave N running` and `waiting for confirmation` describe wave orchestration progress.
- `fixing round N` and `reviewing round N` surface the current review/fix cycle instead of only the top-level status.

for a longer operator guide, see [`docs/lifecycle-operator-guide.md`](docs/lifecycle-operator-guide.md).

---

## manual recovery

manual recovery is supported from both the tui and cli.

- tui: open the selected instance or task context menu, then use the `manage` actions such as `mark planning finished`, `mark architect finished`, and `mark implement finished`
- cli: use `kas task recover <task-file> --action ...`

supported recovery actions:

- `planner-finished`
- `architect-finished`
- `implement-finished`
- `review-approved`
- `review-changes --feedback ...`
- `advance-review-cycle --feedback ...`

the architect completion signal still uses the legacy wire name `elaborator_finished` for compatibility. operators should use `architect-finished` in tui and cli surfaces; `elaborator_finished` is retained on the wire only.

---

## upgrading to v2.0.0

upgrade to the stable release with one of these:

### homebrew

```bash
brew update
brew upgrade kasmos
```

### go install

```bash
go install github.com/kastheco/kasmos@v2.0.0
ln -sf "$(go env GOPATH)/bin/kasmos" "$(go env GOPATH)/bin/kas"
kas version
```

### install script / release asset

re-run the installer or replace your binary with the `v2.0.0` release asset, then confirm:

```bash
kas version
```

### recommended post-upgrade refresh

```bash
cat repos.txt | kas reset --dry-run
cat repos.txt | kas reset --yes
```

for a single repo, run `kas reset` from inside that repo instead.

---

## development

```bash
go test ./...
go build ./...
```

---

## license

[BSL 1.1](LICENSE.md) - converts to [Apache 2.0](https://www.apache.org/licenses/LICENSE-2.0) on the change date.
