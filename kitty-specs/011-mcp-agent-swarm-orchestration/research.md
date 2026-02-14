# Research: MCP Agent Swarm Orchestration

**Feature**: 011-mcp-agent-swarm-orchestration  
**Date**: 2026-02-13  
**Status**: Complete

---

## Executive Summary

This research investigates the feasibility and design of pivoting kasmos from a TUI-based orchestrator (ratatui, file watchers, FIFO pipes) to an MCP-powered agent swarm where a manager agent coordinates workers through Zellij panes. Five research threads were conducted in parallel: zellij-pane-tracker capabilities, Zellij layout management, current codebase analysis, Rust MCP server design, and OpenCode agent configuration. All threads completed successfully and produced actionable findings.

---

## Decision 1: MCP SDK Selection — rmcp v0.15

**Decision**: Use `rmcp` v0.15 with features `["server", "transport-io"]` as the Rust MCP SDK.

**Rationale**: rmcp is the official MCP Rust SDK from the `modelcontextprotocol` organization (same org that defines the protocol). It provides ergonomic proc macros (`#[tool]`, `#[tool_router]`, `#[tool_handler]`) that auto-generate JSON Schema from Rust types via `schemars`. The kasmos codebase already depends on every one of rmcp's core dependencies (serde, serde_json, thiserror, tokio, async-trait, tracing, chrono), so adding rmcp introduces minimal new dependency weight — only the rmcp crate itself plus schemars.

**Alternatives Considered**:
- `rust-mcp-sdk` v0.8.3: Community-maintained, slightly better documented (56% vs 35%), but not official. Uses separate schema crate. Less ergonomic `tool_box!` macro.
- `mcp-server` v0.1.0: Dead crate, single release, effectively merged into rmcp.
- Custom implementation: Too much protocol surface to implement correctly.

**Evidence**: [EVD-01, EVD-02, EVD-04]

---

## Decision 2: Transport — Stdio (Per-Agent Subprocess)

**Decision**: Use stdio transport where each OpenCode agent instance spawns its own `kasmos serve` subprocess. Communicate via stdin/stdout JSON-RPC 2.0.

**Rationale**: This matches the existing pattern for the zellij MCP server (`"type": "local"` in OpenCode config). Each agent gets its own server instance, eliminating the need for multi-client session management. Shared state (WP lanes, worker registry) is coordinated through the filesystem using file locking.

**Configuration**: Add to OpenCode kas profile `opencode.jsonc`:
```jsonc
"kasmos": {
  "type": "local",
  "command": ["kasmos", "serve"],
  "enabled": true
}
```

**Trade-offs**:
- Pro: Simple lifecycle (OpenCode manages process), no HTTP/socket complexity, proven pattern.
- Con: Multiple `kasmos serve` instances need filesystem coordination for shared state. Each agent burns one process.
- Mitigation: Use advisory file locks (`flock`) for task file writes. Worker registry is in-memory per instance, reconciled via the pane-tracker JSON on read.

**Evidence**: [EVD-03]

---

## Decision 3: Layout Architecture — Three-Tier Approach

**Decision**: Use a three-tier layout strategy:

1. **Session Layout (static KDL)**: Generated once at `kasmos` launch. Contains two tabs:
   - "MCP" tab: runs `kasmos serve` for visibility/logging
   - "orchestration" tab: manager pane (75% width) + message-log pane (25% width)

2. **Worker Tab Layouts (dynamic per wave)**: Generated programmatically when workers are needed. Loaded via `zellij action new-tab --layout <path> --name "agents-wN"`. Contains a grid of agent panes with commands and working directories.

3. **Swap Layouts (optional, P2)**: For dynamic add/remove within a worker tab without full tab replacement. Uses `swap_tiled_layout` with `exact_panes`/`min_panes` constraints for clean reflow.

**Rationale**: The existing `LayoutGenerator` already generates per-wave tab layouts with `generate_wave_tab()`. The 3-tier approach reuses this pattern while adding the static session layout (new) and optional swap layouts (P2 enhancement). Zellij's `new-tab --layout` and `new-pane --name --cwd` CLI commands provide all the primitives needed.

**Key Zellij Commands** (verified on 0.43.1):
- Session: `zellij --session kasmos --layout /path/session.kdl`
- New tab inside session: `zellij action new-tab --layout /path/wave.kdl --name "agents-w0"`
- Named pane with cwd: `zellij action new-pane --name "WP01-coder" --cwd /path -- bash -c "ocx oc ..."`
- Close pane: `zellij action close-pane` (focused)
- Remote: `zellij --session kasmos action ...` (from outside session)

**Evidence**: [EVD-05, EVD-06, EVD-07]

---

## Decision 4: Extend zellij-pane-tracker MCP Server

**Decision**: Fork the existing zellij-pane-tracker MCP server to add missing tools required for swarm orchestration.

**Tools to Add**:
- `close_pane(pane_id)`: Uses `zellij action close-pane` after navigating to target
- `rename_pane(pane_id, name)`: Uses `zellij action rename-pane` after navigating to target
- `list_tabs()`: Uses `zellij action query-tab-names` to return tab list

**Why Fork (Not Replace)**:
The existing MCP server is well-structured TypeScript (~800 lines), already handles pane resolution, tab navigation, and session detection. Adding 3 tools is straightforward. The alternative — implementing a parallel pane management system in Rust — would duplicate the navigation/resolution logic and introduce coordination issues with the existing plugin.

**Limitations Accepted**:
- Focus-cycling navigation remains (visible flicker) — acceptable for infrequent management operations
- No completion detection for run_in_pane — mitigated by the message-log protocol (workers explicitly signal via structured messages)
- maxPanesPerTab=10 — sufficient for expected worker counts (max 4-8 per wave)

**Limitations to Fix**:
- Per-session JSON path: Change from hardcoded `/tmp/zj-pane-names.json` to `/tmp/zj-pane-names-<session>.json` in both plugin and MCP server
- Concurrency: Add a mutex/lock file to prevent concurrent focus-cycling operations

**Evidence**: [EVD-08, EVD-09, EVD-13, EVD-14]

---

## Decision 5: Hybrid Message-Log (Pane + File)

**Decision**: Use a dual-layered communication channel:

1. **Message-log pane** (real-time): A Zellij pane (~25% width, right of manager) where workers write structured messages using `zellij MCP run_in_pane`. Manager reads via `dump_pane` or kasmos MCP `read_messages`.

2. **Persistent file** (audit trail): `.kasmos/messages.jsonl` for append-only persistence. Written by `kasmos serve` when it processes messages.

**Message Wire Format**:
```
echo "[KASMOS:<sender>:<event>] <json_data>"
```

Workers send this via `zellij MCP run_in_pane` targeting the message-log pane. The structured prefix makes parsing reliable even with ANSI noise in the pane.

**Why Not File-Only**: The pane provides immediate visual feedback to the user. They can watch the message-log pane to see inter-agent communication in real time.

**Why Not Pane-Only**: Pane scrollback has finite limits and is not persistent across session restarts. The JSONL file ensures full traceability and crash recovery.

**Evidence**: [EVD-08, EVD-10]

---

## Decision 6: Module Categorization (Codebase Strategy)

**Decision**: Categorize the 51 existing source files into four groups:

| Category     | Count | ~LOC  | Action                                                |
|-------------|-------|-------|-------------------------------------------------------|
| **KEEP**    | 12    | 3,350 | Use as-is in new architecture                         |
| **ADAPT**   | 4     | 2,400 | Modify for MCP server and new layout generation       |
| **UNWIRE**  | 5     | 2,000+| Disconnect from entry points, preserve code behind feature flag |
| **REPLACE** | 11    | 6,600+| Functionality replaced by MCP server + manager agent  |

**KEEP**: types.rs, state_machine.rs, git.rs, graph.rs, error.rs, parser.rs, persistence.rs, logging.rs, signals.rs, cleanup.rs, feature_arg.rs, review.rs

**ADAPT**: config.rs (add MCP settings, remove TUI settings), layout.rs (new session/wave layouts), zellij.rs (new pane operations), prompt.rs (MCP-aware prompt generation)

**UNWIRE**: tui/ (entire module), hub/ (entire module), tui_cmd.rs, tui_preview.rs, report.rs — behind `#[cfg(feature = "tui")]`

**REPLACE**: engine.rs, session.rs, detector.rs, cmd.rs, commands.rs, command_handlers.rs, health.rs, shutdown.rs, review_coordinator.rs, start.rs, sendmsg.rs — functionality moves to MCP server tools and manager agent logic

**Rationale**: The type system, state machine, git integration, dependency graph, and parser are clean, portable modules with no TUI coupling. The engine's core algorithms (dependency satisfaction, capacity limiting, wave progression) are extractable into pure functions that MCP tool handlers can call. TUI code is preserved for potential future reintegration (FR-024).

**Evidence**: [EVD-11]

---

## Decision 7: Manager + Workers Agent Architecture

**Decision**: The orchestration uses two distinct agent tiers:

**Manager Agent** (new `manager` role in OpenCode profile):
- Model: claude-opus-4-6, temperature 0.3, reasoning high
- MCP access: kasmos (orchestration) + zellij (pane management) + exa + context7
- Context: Full spec, plan, task board, architecture memory, project structure (FR-028)
- Responsibilities: Workflow assessment, delegation, monitoring, transitions, status reporting

**Worker Agents** (existing coder/reviewer/release roles):
- MCP access: zellij only (for message-log communication)
- Context: Role-specific per FR-029/030/031 (coder gets WP task file + standards, reviewer gets WP + diff + criteria, release gets all WP statuses + branch structure)
- Responsibilities: Execute assigned task, report progress/completion via message-log

**Why Workers Don't Get kasmos MCP**: 
- Prevents N+1 `kasmos serve` processes (only manager runs kasmos MCP)
- Eliminates state conflicts from workers modifying WP lanes directly
- Reduces token cost (workers don't need orchestration tool descriptions)
- Workers communicate via the message-log pane (simple echo commands via zellij MCP)

**Evidence**: [EVD-10, EVD-12]

---

## Decision 8: CLI Restructuring

**Decision**: Replace the current CLI structure with three modes:

1. **`kasmos [spec-prefix]`** — Bootstrapper/Launcher
   - Validates environment, generates session layout, launches Zellij session
   - If inside Zellij: opens as new tab
   - If spec-prefix provided: primes manager with binding
   - Exits after launch (does not run as daemon)

2. **`kasmos serve`** — MCP Server
   - Runs as stdio MCP server (spawned by OpenCode)
   - Exposes 8 orchestration tools
   - Manages worker registry in-memory, reads/writes task files

3. **`kasmos setup`** — Environment Validation
   - Checks required dependencies (Zellij, OpenCode, pane-tracker plugin)
   - Validates configuration files
   - Generates defaults where missing

**Preserved** (unchanged):
- `kasmos list` — list unfinished specs
- `kasmos status [feature]` — show WP progress

**Removed** (replaced by MCP tools):
- `kasmos start <feature>` — replaced by `kasmos [spec-prefix]`
- `kasmos cmd <subcommand>` — replaced by kasmos MCP tools
- `kasmos attach/stop` — replaced by Zellij session management

**Evidence**: [EVD-11, spec FR-001 through FR-006]

---

## Decision 9: Filesystem as Single Source of Truth

**Decision**: Spec-kitty task files remain the authoritative source for work package state. No separate state database.

**How It Works**:
- `transition_wp` MCP tool reads and modifies task file YAML frontmatter (lane field)
- `workflow_status` MCP tool scans task files and computes current state
- Worker registry (pane_id → role/wp_id) is in-memory, ephemeral per `kasmos serve` instance
- On crash recovery, manager calls `workflow_status` to reconstruct state from task files + `get_panes` to discover surviving workers

**Why Not a Database**:
- Task files are already the SSOT in spec-kitty (all existing tooling reads them)
- Adding a database creates sync/consistency issues with task files
- Filesystem is observable by humans (git diff, file viewers)
- Manager agent can reconstruct full state from task files + pane discovery at any time

**Concurrent Access**: Use `flock` advisory locks when writing task files from multiple `kasmos serve` instances.

**Evidence**: [EVD-10, EVD-11]

---

## Decision 10: Direct Zellij CLI for Pane Lifecycle

**Decision**: kasmos MCP server uses `zellij` CLI commands directly for pane creation and closure, rather than routing everything through zellij-pane-tracker.

**For Spawning Workers**: `zellij --session kasmos action new-pane --name "WP01-coder" --cwd /worktree -- bash -c "ocx oc -p kas -- --agent coder --prompt ..."`

**For Closing Workers**: `zellij --session kasmos action close-pane` (after navigating to target pane via focus-cycling from zellij-pane-tracker)

**Why Direct CLI**: 
- `new-pane` via CLI supports `--name` and `--cwd` which the zellij-pane-tracker `new_pane` tool does NOT
- Avoids an extra hop through the MCP server for operations kasmos can do directly
- The zellij-pane-tracker is still used for: pane discovery (`get_panes`), scrollback reading (`dump_pane`), command injection (`run_in_pane`), session naming (`rename_session`)

**Evidence**: [EVD-07, EVD-08]

---

## Open Questions & Risks

### Risk 1: Focus-Cycling Reliability (HIGH)

**Issue**: The zellij-pane-tracker's core navigation mechanism (focus-cycling through all panes to find a target) is inherently fragile. Concurrent operations will race. Visual flicker disrupts the user experience.

**Mitigation**: 
- Add a mutex file lock around focus-cycling operations in the forked MCP server
- Minimize the number of focus-cycling operations (use direct Zellij CLI where possible)
- The manager agent serializes its MCP calls (no parallel dump_pane/run_in_pane)
- Accept some visual flicker as a trade-off for the simplicity of the approach

**Residual Risk**: If a user manually interacts with panes during a focus-cycling operation, the navigation may fail. This is unlikely but possible.

### Risk 2: Multi-Instance State Coordination (MEDIUM)

**Issue**: Each OpenCode agent spawns its own `kasmos serve` subprocess. If both the manager and a worker have kasmos MCP (currently only manager does), they could have conflicting views of state.

**Mitigation**: Workers do NOT get kasmos MCP. Only the manager runs kasmos MCP tools. File locking on task file writes prevents corruption. Worker registry is per-instance (manager's view is authoritative).

### Risk 3: Zellij Version Compatibility (LOW)

**Issue**: The implementation relies on specific Zellij CLI commands and flags. Future Zellij versions may change behavior.

**Mitigation**: Pin to Zellij 0.43.x minimum. Test CLI commands in setup validation. Document required Zellij version.

### Risk 4: Message-Log Parsing Reliability (MEDIUM)

**Issue**: Messages in the pane mix with shell prompts, ANSI codes, and potential garbage from previous sessions.

**Mitigation**: Use a distinctive structured prefix `[KASMOS:<sender>:<event>]` that is unlikely to appear in normal shell output. Parser should be tolerant of noise lines. The persistent JSONL file provides a clean fallback.

### Risk 5: Token Cost of Manager Agent (MEDIUM)

**Issue**: The manager agent has broad context (spec, plan, task board, architecture) which could lead to high token consumption during long orchestration sessions.

**Mitigation**: kasmos MCP tools return structured JSON (compact), not raw file contents. The manager requests only what it needs via specific tool calls. Conversation compression can trim historical context. OpenCode already has context management capabilities.
