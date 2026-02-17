# 014 - Architecture Pivot Evaluation: Research

> Verified technical findings for architecture evaluation.
> All claims cite documentation sources.
> Researched: 2026-02-17

## R-001: Zellij Plugin API — Pane Lifecycle Events

**Source**: https://zellij.dev/documentation/plugin-api-events

The plugin API provides **event-driven** pane lifecycle observability via subscription:

| Event | What it provides | Permission |
|-------|-----------------|------------|
| `PaneUpdate` | Info on ALL active panes: title, command, exit code | `ReadApplicationState` |
| `CommandPaneOpened` | Terminal pane ID + context dict when a command pane opens | `ReadApplicationState` |
| `CommandPaneExited` | Pane ID + exit code when command inside pane exits (pane stays open) | `ReadApplicationState` |
| `CommandPaneReRun` | Pane ID + exit code when user re-runs command (e.g., presses Enter) | `ReadApplicationState` |
| `PaneClosed` | Pane ID when any pane in the session is closed | `ReadApplicationState` |
| `EditPaneOpened` / `EditPaneExited` | Editor pane lifecycle | `ReadApplicationState` |
| `TabUpdate` | All tab info (name, position, pane counts, swap layout info) | `ReadApplicationState` |
| `SessionUpdate` | Active sessions of current version on the machine | `ReadApplicationState` |
| `ListClients` | Connected clients, their focused pane, running command/plugin | `ReadApplicationState` |
| `FileSystemCreate/Read/Update/Delete` | File change notifications in Zellij's CWD | None |

**Key finding**: `PaneUpdate` + `CommandPaneOpened` + `CommandPaneExited` + `PaneClosed` together provide **complete pane lifecycle observability** — exactly what kasmos lacks. No polling needed; these are push events delivered to the plugin's `update()` method.

**Key finding**: `CommandPaneExited` fires when the command inside a pane exits but the pane remains open. This means a plugin can detect when an agent process finishes without the pane disappearing. The event includes the numeric exit code.

## R-002: Zellij Plugin API — Pane Management Commands

**Source**: https://zellij.dev/documentation/plugin-api-commands

Comprehensive pane management via plugin commands:

| Command | What it does | Permission |
|---------|-------------|------------|
| `open_command_pane` | Open command pane (tiled) | `RunCommands` |
| `open_command_pane_floating` | Open command pane (floating) | `RunCommands` |
| `open_command_pane_near_plugin` | Open command pane in plugin's tab (tiled) | `RunCommands` |
| `open_command_pane_floating_near_plugin` | Open command pane in plugin's tab (floating) | `RunCommands` |
| `open_command_pane_in_place` | Replace focused pane with command pane | `RunCommands` |
| `open_command_pane_background` | Open hidden/background command pane | `RunCommands` |
| `rerun_command_pane` | Re-run command in existing pane | `RunCommands` |
| `close_terminal_pane` | Close terminal pane by ID | `ChangeApplicationState` |
| `close_plugin_pane` | Close plugin pane by ID | `ChangeApplicationState` |
| `close_multiple_panes` | Close multiple panes at once | `ChangeApplicationState` |
| `focus_terminal_pane` | Focus pane by ID (switches tab/layer) | `ChangeApplicationState` |
| `rename_terminal_pane` | Rename pane UI title by ID | `ChangeApplicationState` |
| `hide_pane_with_id` / `show_pane_with_id` | Suppress/unsuppress panes | `ChangeApplicationState` |
| `open_terminal` + variants | Open terminal panes (no command) | `OpenTerminalsOrPlugins` |
| `run_command` | Run command in background (result via event) | `RunCommands` |
| `write_to_pane_id` / `write_chars_to_pane_id` | Write to specific pane's STDIN | `WriteToStdin` |

**Key finding**: `open_command_pane_near_plugin` + `close_terminal_pane` + `rename_terminal_pane` + `focus_terminal_pane` provide everything kasmos needs for worker pane management — and they work by pane ID, not by name or focus state.

**Key finding**: `new_tabs_with_layout` accepts a **stringified KDL layout** and applies it to the session. This means layout generation could remain in Rust (compiled into the plugin) or be passed via pipe. `dump_session_layout` serializes the current layout back to KDL.

**Key finding**: `run_command` runs a command in the background on the host machine and returns results via `RunCommandResult` event. This enables non-pane command execution (e.g., git operations, file checks).

## R-003: Zellij Plugin API — Inter-Plugin Communication (Pipes)

**Source**: https://zellij.dev/documentation/plugin-pipes

Pipes are unidirectional communication channels to/from plugins:

- **CLI → Plugin**: `zellij pipe` CLI command sends messages to plugins. Supports STDIN streaming with backpressure.
- **Plugin → Plugin**: `pipe_message_to_plugin` command sends messages to other plugins (by URL or internal ID). Target plugin is auto-launched if not running.
- **Plugin → CLI**: `cli_pipe_output` writes to the STDOUT of a CLI pipe.
- **Backpressure**: Plugins can `block_cli_pipe_input` / `unblock_cli_pipe_input` to control flow.
- **Pipe lifecycle method**: Plugins implement `fn pipe(&mut self, pipe_message: PipeMessage) -> bool` to receive messages.

`PipeMessage` contains:
- `source`: `PipeSource::Cli(input_pipe_id)` or `PipeSource::Plugin(source_plugin_id)`
- `name`: pipe name (user-provided or random UUID)
- `payload`: optional arbitrary string content
- `args`: optional string→string dictionary
- `is_private`: whether directed specifically at this plugin

**Key finding**: Pipes + `write_to_pane_id` together can replace the message-log pane system. The plugin could receive structured messages via pipes from the manager agent (through `zellij pipe` CLI calls), and could write structured data to agent pane STDIN.

**Key finding**: `pipe_message_to_plugin` with `zellij:OWN_URL` destination allows a plugin to launch new instances of itself with different configurations — useful for multi-role scenarios.

## R-004: Zellij Plugin API — Workers for Async Tasks

**Source**: https://zellij.dev/documentation/plugin-api-workers

Since WASM/WASI threads are not stable, plugins use workers for async:

- Workers implement `ZellijWorker` trait with `on_message(message: String, payload: String)`.
- Registered via `register_worker!` macro with a namespace.
- Plugin sends to worker via `post_message_to("worker_namespace", ...)`.
- Worker sends back to plugin via `post_message_to_plugin(...)` → received as `CustomMessage` event.

**Key finding**: Workers are the ONLY async mechanism. All long-running operations (file watching, polling, network-equivalent tasks) must go through workers. There is no tokio, no async/await, no threads.

## R-005: Zellij Plugin API — Filesystem Access

**Source**: https://zellij.dev/documentation/plugin-api-file-system

Three mapped paths:
- `/host` — CWD of last focused terminal, or Zellij start folder
- `/data` — per-plugin shared folder (created on load, deleted on unload)
- `/tmp` — system temp directory

Additional commands:
- `change_host_folder` (requires `FullHdAccess`) — change `/host` to arbitrary path
- `scan_host_folder` — performance workaround because WASI filesystem scanning is "extremely slow"

**Key finding**: Filesystem access is possible but mediated. With `FullHdAccess` permission, a plugin can access any path via `change_host_folder`, but this changes the `/host` mapping globally for that plugin instance. Reading spec files, task files, and config requires either:
1. Starting Zellij in the repo root (so `/host` = repo root), OR
2. Using `change_host_folder` to point at the repo, OR
3. Using `run_command` to read files via host commands (e.g., `cat`)

**Key finding**: `scan_host_folder` exists specifically because WASI filesystem traversal is slow. This is a yellow flag for operations that scan many files (e.g., task file detection, spec resolution).

## R-006: Zellij Plugin API — WASM Runtime Constraints

**Source**: https://docs.rs/zellij-tile/latest/zellij_tile/ (inferred from API design + WASI target)

| Constraint | Impact |
|-----------|--------|
| Target: `wasm32-wasip1` | No native code, no C FFI, limited crate ecosystem |
| No tokio | All async via Zellij workers (message-passing) |
| No TCP/UDP sockets | Cannot host MCP stdio server, HTTP server, or any network listener |
| No threads (stable) | Workers are the workaround |
| Mapped filesystem only | `/host`, `/data`, `/tmp` — not arbitrary paths without `FullHdAccess` |
| Slow filesystem scanning | `scan_host_folder` exists as a workaround |
| WASM binary size | Can grow large with complex logic (serde, etc.) |
| Plugin state | Lives in WASM memory; lost on plugin unload/reload |
| State persistence | Must manually serialize to `/data` filesystem |

**Key finding**: The showstopper for Option A (full plugin) is **no TCP/stdio server capability**. kasmos's MCP server (`rmcp` crate, stdio transport) cannot run inside a WASM plugin. AI agents (OpenCode/Crush) connect to MCP servers via stdio — a WASM plugin has no way to expose this interface.

## R-007: Zellij Plugin API — Permissions System

**Source**: https://zellij.dev/documentation/plugin-api-permissions

Granular permission model — plugin requests permissions on load, user approves:

| Permission | Needed for |
|-----------|------------|
| `ReadApplicationState` | Pane/tab/session events, clipboard events |
| `ChangeApplicationState` | Pane management (open/close/focus/rename), tab operations, mode switching |
| `RunCommands` | Command panes, background commands |
| `WriteToStdin` | Writing to pane STDIN |
| `OpenTerminalsOrPlugins` | Opening terminal panes |
| `OpenFiles` | Opening editor panes |
| `FullHdAccess` | Arbitrary filesystem access |
| `Reconfigure` | Changing Zellij config at runtime |
| `MessageAndLaunchOtherPlugins` | Sending pipes to other plugins |
| `ReadCliPipes` | Receiving CLI pipe messages |
| `InterceptInput` | Intercepting user keypresses |
| `StartWebServer` | Controlling Zellij web server |

**Key finding**: A kasmos plugin would need: `ReadApplicationState`, `ChangeApplicationState`, `RunCommands`, `WriteToStdin`, `OpenTerminalsOrPlugins`, `FullHdAccess`, `ReadCliPipes`, `MessageAndLaunchOtherPlugins`. This is a broad permission surface but plugins can request all needed permissions upfront.

## R-008: OpenCode — Extensibility Model

**Source**: https://github.com/anomalyco/opencode, https://opencode.ai/docs

### OpenCode Status
- **Active and thriving**: 106k stars, 10.4k forks, 757 contributors, 9,380 commits, v1.2.6 (Feb 16, 2026).
- TypeScript monorepo (50.8% TS), MIT licensed.
- Maintained by Anomaly (anomalyco). NOT the archived opencode-ai/opencode Go project (which became Crush/charmbracelet).
- Client/server architecture: TUI is one client; desktop app, web, and IDE extensions are others.

### Architecture
- **Client/server split**: `opencode serve` runs a headless HTTP server with OpenAPI 3.1 spec. The TUI connects to it. Multiple clients can connect simultaneously.
- **TypeScript/Bun runtime**: Monorepo with packages/ directory.
- **Built-in agents**: `build` (full access), `plan` (read-only), `general` (subagent), `explore` (read-only subagent), plus system agents (compaction, title, summary).
- **MCP support**: Consume external MCP servers via stdio, HTTP, SSE transports.

### Extension Points Available

1. **Plugin system** (https://opencode.ai/docs/plugins):
   - JS/TS modules loaded from `.opencode/plugins/` (project) or `~/.config/opencode/plugins/` (global), or npm packages.
   - Plugins receive context: `{ project, client, $, directory, worktree }`.
   - Hook into events: `tool.execute.before`, `tool.execute.after`, `session.idle`, `session.created`, `session.compacted`, `file.edited`, `message.updated`, `shell.env`, etc.
   - Can register **custom tools** with Zod schemas available to the AI alongside built-in tools.
   - Can modify behavior (e.g., intercept tool calls, inject env vars, add compaction context).
   - TypeScript support with `@opencode-ai/plugin` types.

2. **Custom agents** (https://opencode.ai/docs/agents):
   - Defined via JSON config or Markdown files.
   - Two modes: `primary` (Tab-switchable) and `subagent` (invoked via @mention or by primary agents).
   - Configurable: model, prompt, temperature, tools, permissions, max steps, task permissions (which subagents it can invoke).
   - Granular permission control: per-tool allow/ask/deny, per-bash-command patterns.

3. **SDK** (https://opencode.ai/docs/sdk):
   - `@opencode-ai/sdk` npm package. Type-safe JS/TS client.
   - `createOpencode()` starts server + client; `createOpencodeClient()` connects to existing server.
   - Full API: sessions (create/list/prompt/abort/fork), messages, files, events (SSE), config, TUI control.
   - Can programmatically send prompts, manage sessions, subscribe to events.

4. **Server API** (https://opencode.ai/docs/server):
   - `opencode serve` exposes HTTP endpoints: sessions, messages, files, tools, events, agents, config.
   - SSE event stream for real-time monitoring.
   - OpenAPI 3.1 spec at `/doc`.

**Key finding**: OpenCode HAS a rich extension model. Unlike the old Go project, this is highly extensible via plugins (custom tools, event hooks), custom agents (configurable roles), and a programmatic SDK/server API. kasmos could potentially embed orchestration logic as an OpenCode plugin + custom agent set.

**Key finding**: However, OpenCode does NOT manage Zellij panes. It has its own TUI (bubbletea-equivalent in TS). Orchestrating multiple concurrent agents in separate terminal panes is not part of OpenCode's model — it uses subagents within the same process. The Zellij pane management problem persists regardless of how deep the OpenCode integration goes.

**Key finding**: The SDK + server architecture opens a different approach for Option B: instead of forking, kasmos could act as an **external orchestrator that drives multiple OpenCode server instances via SDK**. Each worker pane would run `opencode serve`, and kasmos coordinates them programmatically. This preserves kasmos's role while leveraging OpenCode's agent infrastructure.

**Key finding**: MIT license. No forking restrictions.

## R-009: Current kasmos Codebase Metrics

**Source**: Direct codebase inspection

| Metric | Value |
|--------|-------|
| Total LOC (Rust) | 12,325 |
| Source files | 35 |
| MCP tools | 9 |
| Crates | 1 (kasmos) |
| Key dependencies | rmcp, tokio, kdl, serde, clap, toml |
| Test coverage | Unit tests present, integration tests for MCP tools |

**Note**: The spec estimated ~3,500 LOC across 17 files. Actual is **12,325 LOC across 35 files** — significantly more code to migrate than initially scoped.

## R-010: Zellij Plugin API — Web Requests

**Source**: https://zellij.dev/documentation/plugin-api-commands#web_request, https://zellij.dev/documentation/plugin-api-events#webrequestresult

- `web_request` command (requires `WebAccess` permission) makes HTTP requests from plugin.
- Result returned via `WebRequestResult` event (status code, body, context dict).
- This is an async operation (request fires, result arrives later as event).

**Key finding**: While plugins can't host servers, they CAN make outbound HTTP requests. This opens the possibility of a plugin communicating with an external kasmos process via HTTP polling or webhooks — relevant for Option C hybrid architecture.
