# External Integrations

**Analysis Date:** 2026-02-16

## APIs & External Services

**MCP Protocol (Model Context Protocol):**
- kasmos implements an MCP stdio server via `kasmos serve`
- SDK: `rmcp` 0.15 (features: server, transport-io)
- Implementation: `crates/kasmos/src/serve/mod.rs`
- Transport: stdio (stdin/stdout JSON-RPC)
- Spawned as a subprocess by the manager agent's OpenCode profile
- Config: `config/profiles/kasmos/opencode.jsonc` registers `kasmos serve` as MCP server

**MCP Tools Exposed (9 tools):**
- `spawn_worker` - Spawn planner/coder/reviewer/release worker pane (`crates/kasmos/src/serve/tools/spawn_worker.rs`)
- `despawn_worker` - Close worker pane and deregister (`crates/kasmos/src/serve/tools/despawn_worker.rs`)
- `list_workers` - List tracked workers with status (`crates/kasmos/src/serve/tools/list_workers.rs`)
- `read_messages` - Parse message-log pane scrollback (`crates/kasmos/src/serve/tools/read_messages.rs`)
- `wait_for_event` - Block until matching event or timeout (`crates/kasmos/src/serve/tools/wait_for_event.rs`)
- `workflow_status` - Feature phase, wave info, lock metadata (`crates/kasmos/src/serve/tools/workflow_status.rs`)
- `transition_wp` - Validate and apply WP lane transitions in task files (`crates/kasmos/src/serve/tools/transition_wp.rs`)
- `list_features` - List known feature specs (`crates/kasmos/src/serve/tools/list_features.rs`)
- `infer_feature` - Infer feature slug from arg/branch/cwd (`crates/kasmos/src/serve/tools/infer_feature.rs`)

## External CLI Tools

**Zellij (Terminal Multiplexer):**
- Purpose: Hosts all orchestration sessions, tabs, and panes
- Binary: Configurable via `kasmos.toml` `[paths].zellij_binary` (default: `"zellij"`)
- Version support: 0.41+ (adaptations for missing `list-panes`, `focus-pane-by-name`)
- ANSI output parsing: Handles 0.44+ format with ANSI codes in `list-sessions` output
- Integration layer: `crates/kasmos/src/launch/session.rs` (session/tab bootstrap + pane actions)
- Operations used:
  - `list-sessions` - Session discovery
  - `attach --create-background` / `attach --create` - Session creation
  - `kill-sessions` - Session cleanup
  - `action new-tab --layout` - Tab creation with KDL layout
  - `action new-pane`, `action close-pane` - Pane lifecycle
  - `action focus-next-pane`, `action focus-previous-pane` - Pane navigation
  - `action ToggleFocusFullscreen` - Pane zoom
  - `action rename-tab`, `action go-to-tab-name`, `action query-tab-names` - Tab management
  - `action dump-screen` - Fallback scrollback reading (degraded mode)
  - `run -n <name> -- <command>` - Launch named pane with command
- Layout: Generated as KDL in `crates/kasmos/src/launch/layout.rs`, written to temp files
- Session detection: `ZELLIJ_SESSION_NAME` env var in `crates/kasmos/src/launch/session.rs`

**OpenCode / ocx (AI Agent CLI):**
- Purpose: Launches AI agent instances (manager, coder, planner, reviewer, release)
- Binary: Configurable via `kasmos.toml` `[agent].opencode_binary` (default: `"ocx"`)
- Profile: Configurable via `[agent].opencode_profile` (default: `"kas"`)
- Used in: KDL layout generation for manager pane (`crates/kasmos/src/launch/layout.rs`)
- Manager command: `ocx oc -p kas -- --agent manager --prompt "<prompt>"`
- MCP integration: Manager agent uses `kasmos serve` as an MCP server via `config/profiles/kasmos/opencode.jsonc`
- Worker spawning: Workers launched in Zellij panes running OpenCode instances

**spec-kitty (Feature Specification Tool):**
- Purpose: Feature/task lifecycle management, work package planning
- Binary: Configurable via `kasmos.toml` `[paths].spec_kitty_binary` (default: `"spec-kitty"`)
- Artifact paths: `kitty-specs/` directory containing feature specs, plans, and task files
- Task file format: YAML frontmatter with `work_package_id`, `title`, `lane`, `dependencies`, `subtasks`, `phase`
- Lane values: `planned`, `doing`, `for_review`, `done`, `rework` (managed by `transition_wp` MCP tool)
- Integration: kasmos reads/writes spec-kitty task files directly (no CLI invocation at runtime)
- Parsing: `crates/kasmos/src/parser.rs` - YAML frontmatter extraction from `tasks/WP*.md` files
- Worktree convention: `.worktrees/{feature_name}-{wp_id}/` created by spec-kitty, discovered by kasmos

**pane-tracker / zellij-pane-tracker:**
- Purpose: Structured pane content reading and command execution within named Zellij panes
- Binary: Either `pane-tracker` or `zellij-pane-tracker` in PATH (checked via `which`)
- Discovery: `crates/kasmos/src/serve/messages.rs` (`pane_tracker_binary()`)
- Operations:
  - `dump-pane --pane-name <name>` - Read pane scrollback (multiple arg format fallbacks)
  - `run-in-pane --pane-name <name> --command <cmd>` - Write content to pane
- MCP config: Also registered as MCP server in `config/profiles/kasmos/opencode.jsonc`
- Fallback: If unavailable, falls back to `zellij action dump-screen` (degraded mode)

**zjstatus (Zellij Status Bar Plugin):**
- Purpose: Renders the status bar in all kasmos-generated Zellij layouts
- Plugin file: `~/.config/zellij/plugins/zjstatus.wasm`
- Source: https://github.com/dj95/zjstatus
- Integration: Used by generated layouts from `crates/kasmos/src/launch/layout.rs`
- Setup validation: `check_zjstatus()` in `crates/kasmos/src/setup/mod.rs`
- Configuration: Rose Pine Moon theme with zjstatus-hints pipe integration
- Features used: mode indicators, tab styles, datetime, pipe format (zjstatus_hints)

**Git:**
- Purpose: Repository discovery and branch detection used by launch/lock flows
- Binary: `git` in PATH (validated by `kasmos setup`)
- Operations:
  - `rev-parse --show-toplevel` - Repo root discovery (`crates/kasmos/src/serve/lock.rs`)
  - `branch --show-current` - Current branch detection (`crates/kasmos/src/launch/detect.rs`)

## Data Storage

**Databases:**
- None - All state is file-based

**File-Based State:**
- Audit logs: JSONL files at `kitty-specs/{feature}/.kasmos/messages.jsonl` (`crates/kasmos/src/serve/audit.rs`)
  - Retention: Configurable max_bytes (512MB default), max_age_days (14 default)
  - Rotation: Auto-prune by size and age every 64 writes
- Feature locks: JSON files at `.kasmos/locks/{feature_slug}.lock` + `.lock.guard` (`crates/kasmos/src/serve/lock.rs`)
  - Advisory locking via `flock()` for concurrent access safety
  - Heartbeat-based stale detection with configurable timeout
- Worker registry: In-memory `HashMap` in `crates/kasmos/src/serve/registry.rs` (not persisted)
- Task files: `kitty-specs/{feature}/tasks/WP*.md` with YAML frontmatter (read/written by `transition_wp`)

**File Storage:**
- Local filesystem only
- Temp files: `std::env::temp_dir()` for KDL layouts and pane content transfer

**Caching:**
- None (all reads are direct filesystem access)

## Authentication & Identity

**Auth Provider:**
- None (local CLI tool, no network auth)

**Process Identity:**
- Lock owner ID: `{pid}@{hostname}` format (`crates/kasmos/src/serve/lock.rs`)
- Hostname resolution: `HOSTNAME` env var -> `/etc/hostname` -> `hostname` command -> `"unknown-host"`

## Monitoring & Observability

**Error Tracking:**
- None (no external service)

**Structured Logging:**
- Framework: `tracing` + `tracing-subscriber` (env-filter, fmt layers)
- Config: `RUST_LOG` env var (default: `kasmos=info`)
- Output: stderr
- Implementation: `crates/kasmos/src/logging.rs`

**Audit Trail:**
- JSONL audit files per feature with structured entries
- Fields: timestamp, actor, action, feature_slug, wp_id, status, summary, details
- Debug payload: Optional full request payload (controlled by `audit.debug_full_payload`)
- Implementation: `crates/kasmos/src/serve/audit.rs`

## CI/CD & Deployment

**Hosting:**
- Local installation only (`cargo install --path crates/kasmos`)

**CI Pipeline:**
- None detected (no `.github/workflows/`, `.gitlab-ci.yml`, etc.)

**Build Commands:**
- `cargo build` - Default build
- `cargo test` - Run all tests
- `cargo clippy -p kasmos -- -D warnings` - Lint
- `just install` - Install to `~/.cargo/bin/`

**Contract Verification:**
- `scripts/check-cli-contract.sh` - Validates CLI surface against `contracts/cli-contract.md`

## Message Protocol

**Inter-Agent Communication:**
- Protocol: Custom structured message format in Zellij pane scrollback
- Format: `[KASMOS:{sender}:{event}] {json_payload}`
- Parsing: Regex-based in `crates/kasmos/src/serve/messages.rs`
- Known events: `STARTED`, `PROGRESS`, `DONE`, `ERROR`, `REVIEW_PASS`, `REVIEW_REJECT`, `NEEDS_INPUT`, `SPAWN`, `DESPAWN`
- Message pane: Named `msg-log` in Zellij layout
- Reading: Via `pane-tracker dump-pane` (primary) or `zellij action dump-screen` (fallback)
- Writing: Via `pane-tracker run-in-pane` with tempfile-based content transfer

## Environment Configuration

**Required env vars:**
- None strictly required (all have defaults in `kasmos.toml` or code)

**Critical runtime binaries (validated by `kasmos setup`):**
- `zellij` - Terminal multiplexer
- `ocx` (OpenCode) - AI agent launcher
- `spec-kitty` - Feature specification tool
- `pane-tracker` or `zellij-pane-tracker` - Pane content manager
- `git` - Version control

**Optional env vars:**
- `RUST_LOG` - Log level
- `ZELLIJ_SESSION_NAME` - Auto-detected for session vs tab creation
- `KASMOS_*` - Config overrides (see STACK.md)
- `NO_COLOR` - Disable colored output in setup checks
- `HOSTNAME` - Used for lock owner identity

**Secrets location:**
- `.env` file listed in `.gitignore` (existence noted, contents not read)
- No API keys, tokens, or credentials required by kasmos itself
- AI provider credentials are managed by OpenCode/ocx, not kasmos

## Webhooks & Callbacks

**Incoming:**
- None (kasmos is a CLI/MCP-stdio server, no HTTP endpoints)

**Outgoing:**
- None (all communication is local process/file-based)

## External Tool Ecosystem

**spec-kitty Artifacts Structure:**
```
kitty-specs/
  {NNN}-{feature-slug}/
    spec.md                    # Feature specification
    plan.md                    # Implementation plan
    tasks/
      WP01-{description}.md   # Work package task files (YAML frontmatter)
      WP02-{description}.md
    audit/
      kasmos-audit-*.jsonl     # Audit trail
```

**kasmos Runtime Artifacts:**
```
.kasmos/
  locks/
    {feature_slug}.lock        # Feature lock (JSON)
    {feature_slug}.lock.guard  # Advisory lock guard file
```

**Git Worktree Layout:**
```
.worktrees/
  {feature_name}-WP01/        # Isolated worktree per work package
  {feature_name}-WP02/
```

---

*Integration audit: 2026-02-16*
