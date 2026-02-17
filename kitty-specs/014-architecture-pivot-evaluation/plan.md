# 014 - Architecture Pivot Evaluation: Plan

> Architecture evaluation with recommendation, PoC scope, and migration roadmap.
> Based on verified research in research.md.
> Produced: 2026-02-17

## Executive Summary

**Recommendation: Option C — Hybrid Bridge Architecture**

kasmos should keep its MCP server binary and add a purpose-built Zellij plugin as a bridge for pane lifecycle observability, process management, and inter-agent communication. This solves the three highest-severity pain points (blind orchestrator, fragile message parsing, CLI opacity) while preserving all existing MCP infrastructure and requiring the least migration effort.

Confidence: **High**. The Zellij plugin API has verified coverage for every capability kasmos needs on the Zellij side (R-001 through R-003), and the hybrid approach avoids the showstopper constraint that blocks Option A (WASM plugins cannot host MCP servers — R-006).

---

## Per-Option Evaluation

### Option A: Zellij WASM Plugin (Full Migration)

Move all orchestration logic into a Zellij WASM plugin compiled to `wasm32-wasip1`.

#### Strengths
- **Pane lifecycle**: Native event subscription (PaneUpdate, CommandPaneOpened/Exited, PaneClosed). No polling, no stale registry. [R-001]
- **Process management**: open_command_pane variants with context dicts, close_terminal_pane by ID, rerun_command_pane. [R-002]
- **Inter-agent comms**: Pipes for structured plugin↔plugin and CLI↔plugin messaging with backpressure. [R-003]
- **Deployment**: Single .wasm file in `~/.config/zellij/plugins/`. No cargo install, no PATH setup.
- **Layout control**: new_tabs_with_layout accepts stringified KDL. dump_session_layout serializes current state. [R-002]

#### Showstoppers
- **Cannot host MCP server** (R-006): WASM plugins have no TCP/stdio server capability. kasmos's 9 MCP tools served via rmcp cannot run in WASM. AI agents connect to MCP servers via stdio — a plugin cannot expose this interface. This is a **hard blocker**.
- **No tokio/async**: All async must go through Zellij workers (message passing). The current MCP server uses tokio extensively. [R-004]
- **Migration cost**: 12,325 LOC of Rust must be rewritten for wasm32-wasip1 target. Key crates (rmcp, tokio) do not compile to WASM. [R-009]
- **Filesystem limitations**: Mapped paths (/host, /data, /tmp), slow scanning, no native path access without FullHdAccess. [R-005]

#### Verdict
**Eliminated.** The MCP server incompatibility is a hard blocker. Without MCP tools, AI agents cannot interact with kasmos. The migration cost (complete rewrite) compounds this into a non-viable option.

---

### Option B: OpenCode Extension / SDK Integration

Embed orchestration logic into the AI agent runtime via OpenCode's plugin system, custom agents, and SDK.

OpenCode (anomalyco/opencode) is a 106k-star TypeScript project with a rich extension model — plugins, custom agents, SDK, and headless server mode. This is NOT the archived Go project (opencode-ai/opencode, now Crush). [R-008]

#### Sub-Options (per EC-002)

- **B1: OpenCode Plugin** — Write a kasmos-orchestrator plugin that hooks into session events, registers custom tools, and coordinates agents within a single OpenCode instance.
- **B2: SDK-Driven Multi-Instance** — kasmos remains a separate Rust binary but drives multiple `opencode serve` instances via `@opencode-ai/sdk`, one per worker pane.
- **B3: Fork** — Fork the OpenCode TypeScript codebase and embed orchestration directly. MIT license, no restrictions, but 9,380 commits / 757 contributors.

#### Strengths
- **Rich plugin system** (R-008): JS/TS plugins with event hooks (session.idle, tool.execute.before/after, file.edited, etc.) and custom tool registration.
- **Custom agents** (R-008): Can define planner, coder, reviewer, release as OpenCode agents with per-role prompts, tools, permissions, and model overrides.
- **SDK for programmatic control** (R-008): Type-safe client can create sessions, send prompts, subscribe to events (SSE), manage files.
- **Server mode** (R-008): `opencode serve` runs headless with HTTP API. Multiple instances can run in parallel in separate panes.
- **MIT license**: No restrictions on forking or distribution.
- **Native filesystem access**: No WASM constraints.

#### Weaknesses
- **Does NOT solve Zellij pane management**: OpenCode manages its own TUI. It does NOT create, monitor, or manage Zellij terminal panes. The blind orchestrator problem persists — kasmos still needs to create/observe/close Zellij panes. This is the root cause and Option B alone doesn't fix it.
- **Language mismatch**: kasmos is Rust; OpenCode plugins are TypeScript. B1/B2 require maintaining two languages. B3 means abandoning Rust.
- **Subagent model ≠ pane model**: OpenCode subagents run within a single process. kasmos needs agents in separate Zellij panes for visual isolation, independent crash domains, and manual intervention.
- **Resource overhead** (B2): Each worker pane running a full `opencode serve` is heavier than a simple `opencode run` command.
- **Fork maintenance** (B3): 9,380 commits, 757 contributors, very active development. Unsustainable fork.

#### Verdict
**Partially viable but doesn't solve the core problem.** OpenCode's extension model could significantly improve agent coordination (custom agents, SDK-driven sessions, plugin hooks), but it does NOT address Zellij pane lifecycle opacity. The best sub-option is **B2 combined with Option C** — use the Zellij bridge plugin for pane management and OpenCode's SDK for richer agent control. This is noted as a future enhancement in the roadmap.

---

### Option C: Hybrid Bridge (kasmos binary + Zellij plugin)

Keep kasmos as the MCP server binary. Add a Zellij plugin that acts as a bridge, providing pane introspection and lifecycle events that kasmos currently lacks. Communication between kasmos and the plugin via pipes and/or `zellij pipe` CLI.

#### Strengths
- **Pane lifecycle**: Plugin subscribes to PaneUpdate, CommandPaneOpened/Exited/ReRun, PaneClosed events and forwards them to kasmos. [R-001]
- **MCP server preserved**: kasmos keeps its rmcp MCP server with all 9 tools. No migration needed for agent-facing interface. [R-006 avoided]
- **Code reuse**: 90%+ of existing kasmos code remains. Plugin is a new ~500-1500 LOC WASM component. [R-009]
- **Process management**: Plugin can open_command_pane, close_terminal_pane, rename_terminal_pane, focus_terminal_pane by ID. kasmos tells the plugin what to do via pipes. [R-002, R-003]
- **Inter-agent comms**: Two-channel approach — MCP for agent↔kasmos (structured tools), pipes for kasmos↔plugin (pane operations). Replaces fragile message-log pane parsing.
- **Incremental migration**: Can be adopted module-by-module. Start with pane lifecycle, then add process management, then replace message-log parsing. Current architecture continues to work during migration.
- **Reversible**: If the plugin approach fails, remove the plugin and fall back to current CLI-based approach. No bridges burned.
- **Dependency reduction**: Plugin replaces both zellij-pane-tracker plugin AND bun MCP server. Net reduction in external dependencies.

#### Risks
- **Bridge protocol design**: Need to define the communication protocol between kasmos binary and the Zellij plugin (message format, pipe naming, error handling).
- **Two artifacts to distribute**: A Rust binary (kasmos) and a WASM binary (plugin). Build pipeline needs to produce both.
- **Plugin API stability**: Zellij plugin API is pre-1.0. Breaking changes are possible. Mitigation: pin Zellij version, abstract plugin API calls behind an interface.
- **Latency**: Pipe-based communication adds latency vs direct function calls. For orchestration operations (spawn worker, check status), this is negligible.

#### Architecture

```
Manager Agent (OpenCode/Claude Code)
    │
    ├── MCP stdio ──→ kasmos serve (Rust binary)
    │                    │
    │                    ├── Workflow logic (task transitions, feature detection)
    │                    ├── Worker registry (now backed by plugin events)
    │                    ├── Config, audit, lock management
    │                    └── zellij pipe ──→ kasmos-bridge plugin (WASM)
    │                                          │
    │                                          ├── PaneUpdate subscription
    │                                          ├── CommandPaneOpened/Exited events
    │                                          ├── open_command_pane / close_terminal_pane
    │                                          ├── rename_terminal_pane / focus_terminal_pane
    │                                          └── write_to_pane_id (agent STDIN)
    │
Worker Agents (in Zellij panes)
```

#### Verdict
**Recommended.** Solves the core problems, preserves existing investment, incremental and reversible.

---

### Option D: Status Quo with Targeted Fixes

Accept the current architecture and address pain points incrementally.

#### Per-Pain-Point Assessment

| Pain Point | Potential Fix | Effort | Effectiveness |
|-----------|--------------|--------|--------------|
| 1. Zellij CLI opacity / stale registry | Periodic polling via `zellij action dump-layout`, parse KDL output | Medium | Low — polling is unreliable, dump-layout may not include process state |
| 2. Process boundary friction (3 processes) | No fix possible without architectural change | N/A | None — this is structural |
| 3. KDL layout generation brittleness | Template-based KDL instead of programmatic generation | Low | Medium — reduces code but doesn't eliminate KDL v1/v2 issues |
| 4. Worktree path confusion | Centralize path resolution into a `PathResolver` type | Low | Medium — reduces bugs but doesn't eliminate the problem |
| 5. External dependency chain | Bundle zellij-pane-tracker and zjstatus configs, automate setup | Low | Low — still 7+ dependencies |
| 6. Fragile message-log pane parsing | Structured JSON messages with schema validation | Medium | Medium — more robust but still indirect channel |

#### Strengths
- **Zero migration cost**: Continue with existing code.
- **MCP server works**: No changes to agent interface.
- **Full filesystem access**: No WASM constraints.

#### Weaknesses
- **Blind orchestrator remains**: No fix can provide real-time pane lifecycle events without a plugin.
- **Process boundary remains**: Three-process architecture cannot be simplified without a bridge.
- **Diminishing returns**: Each targeted fix is isolated. They don't compound into a fundamentally better architecture.
- **Technical debt accumulates**: Workarounds on top of workarounds increase maintenance burden over time.

#### Verdict
**Viable but insufficient.** Appropriate if the hybrid approach proves too complex in the PoC phase, but does not solve the root cause (external process trying to orchestrate an uncooperative terminal multiplexer).

---

## Comparison Matrix

### Scoring: 1 (poor) to 5 (excellent)

| Dimension | Weight | A: WASM Plugin | B: OC Extension | C: Hybrid Bridge | D: Status Quo |
|-----------|--------|----------------|-----------------|-------------------|---------------|
| ED-001: Pane Lifecycle Observability | 5 | 5 | 2 | 5 | 1 |
| ED-002: Inter-Agent Communication | 4 | 4 | 4 | 4 | 2 |
| ED-003: Process Spawning & Management | 4 | 5 | 3 | 4 | 2 |
| ED-004: Filesystem & State Access | 3 | 2 | 5 | 5 | 5 |
| ED-005: MCP Server Compatibility | 5 | 1 | 4 | 5 | 5 |
| ED-006: Development Velocity | 4 | 1 | 2 | 4 | 5 |
| ED-007: User Experience | 3 | 4 | 3 | 4 | 2 |
| ED-008: Extensibility | 2 | 3 | 5 | 4 | 3 |
| **Weighted Total** | | **90** | **95** | **131** | **91** |

### Weight Rationale

- **5 (critical)**: ED-001 and ED-005 — pane lifecycle is the root cause problem; MCP compatibility is non-negotiable for agent integration.
- **4 (high)**: ED-002, ED-003, ED-006 — inter-agent comms and process management are daily friction; dev velocity determines whether the migration is practical.
- **3 (moderate)**: ED-004, ED-007 — filesystem access is a concern but solvable; UX matters but is secondary to functionality.
- **2 (low)**: ED-008 — extensibility is forward-looking, not a current pain point.

### Score Justification

**Option A scores:**
- ED-001 (5): Native event subscription, complete lifecycle coverage.
- ED-005 (1): Cannot host MCP server. Hard blocker.
- ED-006 (1): 12,325 LOC rewrite to WASM target. Key deps (rmcp, tokio) incompatible.

**Option B scores:**
- ED-001 (2): OpenCode doesn't manage Zellij panes. Same opacity problem persists.
- ED-002 (4): SDK + events (SSE) + plugin hooks enable rich structured communication between agents. Scores well here.
- ED-005 (4): OpenCode natively consumes MCP servers. kasmos MCP could be exposed to OpenCode agents. Not a perfect 5 because the current rmcp stdio transport would need adaptation for SDK-driven flow.
- ED-006 (2): Plugin/SDK work is TypeScript alongside Rust kasmos. Two-language maintenance. Better than forking but still costly.
- ED-008 (5): OpenCode's plugin system, custom agents, custom tools, and SDK make it the most extensible option by far.

**Option C scores:**
- ED-001 (5): Plugin provides same events as Option A.
- ED-005 (5): MCP server preserved as-is.
- ED-006 (4): ~90% code reuse, plugin is new but small (~500-1500 LOC).
- ED-007 (4): Replaces 2 external deps (pane-tracker + bun) with 1 WASM file.

**Option D scores:**
- ED-001 (1): No real fix possible without plugin. Polling is unreliable.
- ED-006 (5): Zero migration cost — highest velocity for existing code.
- ED-007 (2): All current UX issues remain.

---

## Recommendation

### Primary: Option C — Hybrid Bridge Architecture

**Rationale:**
1. Scores highest in weighted comparison (131 vs next-best 91).
2. Solves the root cause (pane lifecycle opacity) with verified API support.
3. Preserves the most valuable existing investment (MCP server, workflow logic, config system).
4. Incremental migration — each module can be migrated independently.
5. Fully reversible — remove plugin and fall back to current approach.
6. Net dependency reduction — replaces pane-tracker plugin + bun runtime with single WASM file.

**Dissenting considerations:**
- Option D (status quo) is tempting for its zero migration cost. However, the "blind orchestrator" problem is fundamental and worsens as complexity grows. We experienced it firsthand in this session — `spawn_worker` registered a worker but the Zellij pane was never created, with no error feedback. This class of bug is **unsolvable** without pane lifecycle events.
- Option B (OpenCode extension) scores well on extensibility and communication, and the SDK-driven approach (B2) is genuinely compelling. However, it doesn't solve the root cause (Zellij pane opacity). The ideal long-term architecture is **C + B2**: Zellij bridge plugin for pane management, OpenCode SDK for agent control. Phase 5 in the roadmap could explore this.
- Option A (full plugin) would be ideal architecturally but is blocked by MCP server incompatibility. If Zellij ever supports WASI networking, this option should be revisited.

**Conditions to revisit:**
1. If Zellij plugin API adds WASI socket support → reconsider Option A.
2. If the PoC bridge protocol proves too complex or latency-sensitive → fall back to Option D with targeted fixes.
3. After Option C is stable, consider **Phase 5: OpenCode SDK integration** (B2) to replace CLI-based agent spawning with programmatic SDK-driven sessions. This would give kasmos richer control over agent behavior without changing the Zellij bridge layer.

---

## PoC Scope Definition

### Goal
Validate that a Zellij WASM plugin can reliably bridge pane lifecycle events to the kasmos binary via pipes, and that the kasmos binary can request pane operations from the plugin.

### Highest-Risk Integration Point
**Pipe-based bidirectional communication between kasmos binary and Zellij plugin.** This is the novel component — everything else (plugin events, pane commands, MCP server) uses verified APIs.

### PoC Work Packages

#### WP01: kasmos-bridge plugin (WASM)

**Scope**: Minimal Zellij plugin that:
1. Subscribes to `PaneUpdate`, `CommandPaneOpened`, `CommandPaneExited`, `PaneClosed` events.
2. On each event, serializes pane state to JSON and sends via `cli_pipe_output` or writes to `/data/kasmos-events.jsonl`.
3. Implements `pipe` lifecycle method to receive commands from kasmos (e.g., "open pane with command X", "close pane Y", "rename pane Z").
4. Executes received commands via plugin API (open_command_pane, close_terminal_pane, etc.).
5. Requests permissions: `ReadApplicationState`, `ChangeApplicationState`, `RunCommands`, `OpenTerminalsOrPlugins`, `WriteToStdin`, `ReadCliPipes`.

**Deliverables**: `kasmos-bridge.wasm` compiled from Rust using `zellij-tile` crate.

**Success criteria**:
- Plugin loads in Zellij without errors.
- Pane events are captured and serialized.
- `zellij pipe --plugin kasmos-bridge -- '{"cmd":"open_pane","command":"echo hello"}'` creates a pane.
- Pane close event is captured when user closes the pane.

**Estimated effort**: 2-3 days.

#### WP02: kasmos integration layer

**Scope**: Add to the kasmos MCP server:
1. A `PluginBridge` module that communicates with kasmos-bridge plugin via `zellij pipe`.
2. Modify `spawn_worker` to route pane creation through the plugin bridge instead of `zellij action new-pane`.
3. Modify `WorkerRegistry` to reconcile state from plugin-reported pane events (instead of trusting in-memory state only).
4. Event polling: periodically read `/data/kasmos-events.jsonl` or receive events via pipe callback.

**Deliverables**: Updated kasmos serve with plugin bridge integration.

**Success criteria**:
- `spawn_worker` MCP tool creates a pane via the plugin (not via `zellij action`).
- If a pane is closed externally, the WorkerRegistry reflects this within 2 seconds.
- `list_workers` reports accurate pane status based on plugin events.
- All existing MCP tools continue to work.

**Estimated effort**: 2-3 days.

### Failure Criteria
- Pipe communication is unreliable (messages lost, out of order, or >500ms latency).
- Plugin event subscription doesn't capture all pane lifecycle events.
- Plugin permissions model prevents necessary operations.
- Build/distribution complexity (two artifacts) is unmanageable.

### Reversibility
The PoC is fully reversible:
- The plugin is an additional artifact, not a replacement.
- kasmos spawn_worker can have a config flag to use plugin bridge vs direct CLI.
- If PoC fails, remove the plugin and config flag. Zero impact on existing functionality.

---

## Migration Roadmap

### Phase 0: PoC (WP01 + WP02)
**Duration**: 1 week
**Goal**: Validate bridge architecture.
**Modules affected**: New plugin crate, modified spawn_worker and registry in kasmos.
**Reversible**: Yes — config flag toggles between plugin bridge and CLI mode.

### Phase 1: Pane Lifecycle Migration
**Duration**: 1 week
**Goal**: Replace in-memory-only WorkerRegistry with plugin-event-backed registry.
**Modules affected**:
| Current module | Change |
|---------------|--------|
| `serve/registry.rs` | Add event reconciliation from plugin pane events |
| `serve/mod.rs` (spawn_worker) | Route through plugin bridge |
| `serve/mod.rs` (despawn_worker) | Use plugin's close_terminal_pane |
| `serve/mod.rs` (list_workers) | Reconcile with plugin state |

**Reusable code**: All workflow logic, config, audit, lock management, prompt building. (~80% of codebase)
**Rewrite required**: WorkerRegistry reconciliation (~200 LOC), spawn/despawn routing (~150 LOC).

### Phase 2: Message Channel Migration
**Duration**: 1 week
**Goal**: Replace message-log pane parsing with pipe-based structured communication.
**Modules affected**:
| Current module | Change |
|---------------|--------|
| `serve/mod.rs` (read_messages) | Read from plugin event stream instead of pane scrollback |
| `serve/mod.rs` (wait_for_event) | Wait on plugin pipe events instead of pane polling |
| Message-log pane | Becomes optional debug/audit view, not a communication channel |

**Dependency eliminated**: zellij-pane-tracker plugin, bun runtime.

### Phase 3: Layout Simplification
**Duration**: 3 days
**Goal**: Simplify KDL layout generation using plugin's `new_tabs_with_layout`.
**Modules affected**:
| Current module | Change |
|---------------|--------|
| `launch/layout.rs` | Simplify or replace with template-based approach. Plugin can apply layouts via command. |
| KDL v1/v2 workarounds | Eliminated — plugin's new_tabs_with_layout handles serialization. |

### Phase 4: Cleanup
**Duration**: 2 days
**Goal**: Remove deprecated code paths, update documentation, update AGENTS.md.
**Modules affected**:
- Remove CLI-based pane management fallback (or keep as --no-plugin mode).
- Remove bun/pane-tracker from `kasmos setup` validation.
- Update architecture.md with new system overview.
- Update constitution.md if needed.

### Total Effort Estimate

| Phase | Effort | Cumulative |
|-------|--------|------------|
| Phase 0 (PoC) | 1 week | 1 week |
| Phase 1 (Pane lifecycle) | 1 week | 2 weeks |
| Phase 2 (Message channels) | 1 week | 3 weeks |
| Phase 3 (Layout simplification) | 3 days | ~3.5 weeks |
| Phase 4 (Cleanup) | 2 days | ~4 weeks |

### Module Reuse Map

| Current module | Fate in hybrid architecture |
|---------------|----------------------------|
| `config.rs` | **Reuse** — add `[plugin]` config section |
| `serve/mod.rs` (MCP server) | **Reuse** — core MCP server unchanged |
| `serve/registry.rs` | **Modify** — add plugin event reconciliation |
| `serve/lock.rs` | **Reuse** — feature lock unchanged |
| `serve/audit.rs` | **Reuse** — audit writer unchanged |
| `launch/detect.rs` | **Reuse** — feature detection unchanged |
| `launch/session.rs` | **Modify** — integrate plugin loading into session setup |
| `launch/layout.rs` | **Simplify** — reduce KDL generation, delegate to plugin |
| `prompt.rs` | **Reuse** — prompt building unchanged |
| `types.rs` | **Reuse** — core types unchanged |
| `main.rs` / `cli.rs` | **Reuse** — CLI unchanged |
| `setup.rs` | **Modify** — validate plugin .wasm instead of pane-tracker/bun |

**Summary**: ~75% reuse, ~20% modify, ~5% new (plugin crate).

---

## Risk Register

| Risk | Severity | Likelihood | Mitigation |
|------|----------|-----------|------------|
| Zellij plugin API breaking changes (pre-1.0) | High | Medium | Pin Zellij version in kasmos setup validation. Abstract plugin calls behind trait. |
| Pipe communication unreliability | High | Low | PoC validates this first. Fallback to file-based event passing (/data/). |
| WASM build complexity (two artifacts) | Medium | Medium | Single `cargo build` command builds both. Distribute plugin .wasm alongside binary. |
| Plugin permission prompts annoy users | Low | High | Plugins in layout config auto-approve. Document required permissions. |
| Filesystem access limitations in plugin | Medium | Low | Plugin uses run_command for file ops that need full paths. Most file ops stay in kasmos binary. |
| Plugin state loss on reload | Medium | Low | Plugin state is transient (pane tracking). kasmos binary is the source of truth for persistent state. |
