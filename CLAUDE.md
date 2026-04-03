# kasmos

TUI-based multi-agent orchestration IDE. Manages concurrent AI agent sessions (claude, codex, gemini, amp, etc.) in isolated git worktrees + tmux sessions. Each task gets its own branch; the TUI provides unified control over all running agents.

## Key Directories

| Directory | Purpose |
|-----------|---------|
| `app/` | TUI application logic (bubbletea model, input handling, state) |
| `cmd/` | CLI entry points (cobra commands: `kas`, `kas task`, `kas instance`, `kas tmux`) |
| `config/` | Configuration management (`GetConfigDir` repo-root anchoring, repo-rooted `.kasmos/config.toml`, task store wiring, one-time legacy JSON import path, agent profiles) |
| `contracts/` | Shared interfaces and types |
| `daemon/` | Background daemon for auto-accept mode |
| `internal/` | Internal packages (check, clickup, initcmd, mcpclient, opencodesession, sentry) |
| `keys/` | Keybinding definitions |
| `log/` | Structured logging |
| `orchestration/` | Wave/task orchestration engine and prompt generation |
| `session/` | Instance lifecycle, storage, notifications; subpackages: `git/` (worktree ops), `tmux/` (session management) |
| `ui/` | Rendering components (navigation panel, info/audit panes, preview, statusbar, menus, overlays, theme) |
| `web/` | Web UI (public assets + source) |
| `.opencode/` | Scaffolded opencode harness prompts, config, and mirrored skills |
| `.claude/` | Scaffolded claude harness prompts and mirrored skills |
| `.agents/` | Shared project skill sources mirrored into harness-specific directories |

## Standards

Key points:
- Go 1.24+, bubbletea/v2, lipgloss/v2, bubbles/v2
- Tests: testify assertions, table-driven, no real tmux/git/network in tests
- Non-blocking I/O: all I/O in `tea.Cmd` goroutines, results as `tea.Msg`
- Task/config state is project-local under `<repo-root>/.kasmos/` via `config/config.go:GetConfigDir`. `config.toml` is the authoritative live config, `taskstore.db` is the default local store, and `config.json` only survives as a narrow one-time import/migration path handled by `config/config.go:migrateJSONToTOML`.
- Use current task terminology and command names in docs and prompts: `kas task ...`, not legacy `kas plan ...` guidance.
- **Lowercase labels**: all user-visible text (toasts, confirmations, overlay titles, instance list titles) must be lowercase to match the app's aesthetic. No title case or sentence case — e.g. "push changes from 'foo'?" not "Push changes from 'foo'?"
- **Arrow-key navigation in overlays**: use ↑↓ for navigation, not j/k vim bindings. Letter keys should always type into search/filter when present.
- Signals are gateway-backed first. `.kasmos/signals/` still exists for compatibility, but do not document filesystem sentinels as the primary lifecycle path.
- **Daemon runs via systemd.** The kasmos daemon and DB server run as `systemctl --user` services (`kasmos` and `kasmosdb`). Always use `systemctl --user restart kasmos` (not `kas daemon start`). The CLI commands (`kas daemon start/stop`) exist for development and CI only.

## MCP-First Tooling

Always prefer kasmos MCP tools over built-in Claude Code equivalents — they are purpose-built and faster for this codebase.

| Task | Use (kasmos MCP) | Not (built-in) |
|------|-------------------|-----------------|
| search file contents | `mcp__kasmos__grep` | Grep |
| read files | `mcp__kasmos__read_file` | Read |
| find files by pattern | `mcp__kasmos__find_files` | Glob |
| list directory | `mcp__kasmos__list_dir` | Bash `ls` |
| git status | `mcp__kasmos__git_status` | Bash `git status` |
| git diff | `mcp__kasmos__git_diff` | Bash `git diff` |
| git log | `mcp__kasmos__git_log` | Bash `git log` |
| task CRUD | `mcp__kasmos__task_create/show/list/update_content/transition` | Bash `kas task` |
| lifecycle signals | `mcp__kasmos__signal_create` | Bash `touch .kasmos/signals/` |
| instance management | `mcp__kasmos__instance_list/pause/resume/send` | Bash `kas instance` |
| daemon status | `mcp__kasmos__daemon_status` | Bash `kas daemon` |

Built-in tools (Read, Grep, Glob, Bash) are fallback only — use when MCP is unavailable or for operations with no MCP equivalent (e.g., Edit, Write).

## Workflow

Development follows a wave-based plan execution lifecycle. Each agent works only on the specific task it has been assigned — do not expand scope beyond your assigned work package. When `KASMOS_TASK` is set, you are one of several concurrent agents on a shared worktree. `KASMOS_WAVE` identifies your wave, `KASMOS_PEERS` the number of sibling agents. Implement only your assigned task — see your dynamic prompt for specific rules.
