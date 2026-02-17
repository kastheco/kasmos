# CLI Contract: kasmos

**Canonical location**: `contracts/cli-contract.md`
**Last verified**: 2026-02-17
**Freshness check**: `scripts/check-cli-contract.sh`

The kasmos CLI is a public interface contract. Changes to subcommand names, flag names, defaults, or argument semantics are breaking changes and must be versioned accordingly.

## Top-Level Commands

| Subcommand | Description | Since |
|------------|-------------|-------|
| `kasmos [PREFIX]` | Launch orchestration session for a feature spec prefix | v0.3 |
| `kasmos serve` | Run MCP server (stdio transport, spawned by manager agent) | v0.3 |
| `kasmos setup` | Validate environment and generate default configs | v0.3 |
| `kasmos list` | List available feature specs from `kitty-specs/` | v0.3 |
| `kasmos status [feature]` | Show orchestration status for a feature | v0.3 |

## `kasmos [PREFIX]`

| Argument | Type | Required | Default | Validation | Error Behavior |
|----------|------|----------|---------|------------|----------------|
| `[PREFIX]` | String (positional, optional) | No | Auto-detect from arg/branch/cwd; prompts selection if unresolved | If provided, must resolve to exactly one feature directory under `kitty-specs/` | Errors on zero matches ("no prefix match found") or multiple matches ("Ambiguous feature") |

Launch behavior:
- Resolves feature selection (explicit prefix, inferred context, or interactive chooser).
- Runs preflight dependency checks (same validation surface used by `kasmos setup`).
- Acquires feature lock, generates session layout, and bootstraps Zellij orchestration session/tab.

## `kasmos serve`

| Argument | Type | Required | Default | Validation | Error Behavior |
|----------|------|----------|---------|------------|----------------|
| *(none)* | -- | -- | -- | -- | Exits with an error if config load or MCP server startup fails |

### MCP tool contract (stdio server)

| Tool | Purpose |
|------|---------|
| `spawn_worker` | Spawn a planner/coder/reviewer/release worker pane |
| `despawn_worker` | Close a worker pane and remove it from registry |
| `list_workers` | List tracked workers with optional status filter |
| `read_messages` | Read and parse message-log pane events |
| `wait_for_event` | Block until matching event appears or timeout |
| `workflow_status` | Return feature phase, wave status, lock metadata |
| `transition_wp` | Validate and apply WP lane transitions in task files |
| `list_features` | List known feature specs and artifact availability |
| `infer_feature` | Infer feature slug from arg, branch, and cwd context |

## `kasmos setup`

| Argument | Type | Required | Default | Validation | Error Behavior |
|----------|------|----------|---------|------------|----------------|
| *(none)* | -- | -- | -- | -- | Exits with an error if environment validation or config generation fails |

## `kasmos list`

| Argument | Type | Required | Default | Validation | Error Behavior |
|----------|------|----------|---------|------------|----------------|
| *(none)* | -- | -- | -- | -- | Prints "No kitty-specs/ directory found." when specs root is missing |

## `kasmos status [feature]`

| Argument | Type | Required | Default | Validation | Error Behavior |
|----------|------|----------|---------|------------|----------------|
| `[feature]` | String (positional, optional) | No | Current directory when omitted | If provided, must resolve to an existing feature directory (full slug, prefix, or path) | Errors if feature cannot be resolved, `.kasmos/` is missing, or `state.json` is missing/invalid |

## Versioning

| Version | Changes |
|---------|---------|
| v0.3 | MCP-first CLI surface: `[PREFIX]`, `serve`, `setup`, `list`, `status` |

## Freshness Rules

This contract is the canonical reference for the kasmos CLI surface. It MUST be updated whenever:
- A new subcommand is added to `Commands` enum in `main.rs`
- Launch argument behavior for `spec_prefix` in `main.rs` changes
- MCP tool names or semantics in `crates/kasmos/src/serve/mod.rs` change
- Argument names, types, defaults, or validation rules change
- A command is deprecated or removed

Run `scripts/check-cli-contract.sh` to verify this contract matches the implemented CLI.
