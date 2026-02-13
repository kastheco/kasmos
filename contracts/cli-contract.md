# CLI Contract: kasmos

**Canonical location**: `contracts/cli-contract.md`
**Last verified**: 2026-02-12
**Freshness check**: `scripts/check-cli-contract.sh`

The kasmos CLI is a public interface contract. Changes to subcommand names, flag names, defaults, or argument semantics are breaking changes and must be versioned accordingly.

## Top-Level Commands

| Subcommand | Description | Since |
|------------|-------------|-------|
| `kasmos list` | List available feature specs from `kitty-specs/` | v0.1 |
| `kasmos start <feature>` | Start orchestration for a feature | v0.1 |
| `kasmos status [feature]` | Show orchestration status | v0.1 |
| `kasmos cmd [--feature <f>] <command>` | Send controller command via FIFO | v0.1 |
| `kasmos attach <feature>` | Attach to existing Zellij session | v0.1 |
| `kasmos stop [feature]` | Gracefully stop orchestration | v0.1 |
| `kasmos tui [--count <N>]` | Launch TUI with animated mock data | v0.2 |

## `kasmos list`

| Argument | Type | Required | Default | Validation | Error Behavior |
|----------|------|----------|---------|------------|----------------|
| *(none)* | -- | -- | -- | -- | Prints "No kitty-specs/ directory found." if `kitty-specs/` missing |

## `kasmos start`

| Argument | Type | Required | Default | Validation | Error Behavior |
|----------|------|----------|---------|------------|----------------|
| `<feature>` | String (positional) | Yes | -- | Must match exactly one `kitty-specs/###-*` directory by prefix | Errors on zero matches ("no prefix match found") or multiple matches ("Ambiguous feature") |
| `--mode` | String | No | `"wave-gated"` | Accepted: `"continuous"`, `"wave-gated"` | Clap rejects unrecognized values |
| `--tui` | Flag (bool) | No | `false` | -- | -- |

## `kasmos status`

| Argument | Type | Required | Default | Validation | Error Behavior |
|----------|------|----------|---------|------------|----------------|
| `[feature]` | String (positional, optional) | No | Auto-detect from `.kasmos/` in cwd | Same prefix matching as `start` when provided | Same resolution errors as `start` |

## `kasmos cmd`

| Argument | Type | Required | Default | Validation | Error Behavior |
|----------|------|----------|---------|------------|----------------|
| `--feature` | String (optional) | No | Auto-detect from cwd | Same prefix matching as `start` when provided | Same resolution errors |
| `<command>` | Subcommand | Yes | -- | Must be valid FifoCommand variant | Clap rejects unknown subcommands |

### FIFO Subcommands

| Subcommand | Argument | Type | Description |
|------------|----------|------|-------------|
| `status` | -- | -- | Display orchestration state table |
| `restart <wp_id>` | `wp_id` | String | Restart a failed/crashed work package |
| `pause <wp_id>` | `wp_id` | String | Pause a running work package |
| `resume <wp_id>` | `wp_id` | String | Resume a paused work package |
| `focus <wp_id>` | `wp_id` | String | Navigate to work package pane |
| `zoom <wp_id>` | `wp_id` | String | Focus and zoom pane to full view |
| `abort` | -- | -- | Gracefully shutdown orchestration |
| `advance` | -- | -- | Confirm wave advancement (wave-gated mode) |
| `finalize` | -- | -- | Mark orchestration as completed and finalize state |
| `force-advance <wp_id>` | `wp_id` | String | Skip failed WP, unblock dependents |
| `retry <wp_id>` | `wp_id` | String | Re-run a failed work package |
| `approve <wp_id>` | `wp_id` | String | Approve a work package in review (ForReview -> Completed) |
| `reject <wp_id>` | `wp_id` | String | Reject a work package in review (relaunch for rework) |
| `help` | -- | -- | Show command help |

All `wp_id` arguments follow the format `WP##` (e.g., `WP01`, `WP12`). FIFO commands require an active orchestration session with a valid `.kasmos/cmd.pipe`; errors with "No command pipe found" if the session is not running.

## `kasmos attach`

| Argument | Type | Required | Default | Validation | Error Behavior |
|----------|------|----------|---------|------------|----------------|
| `<feature>` | String (positional) | Yes | -- | Same prefix matching as `start` | Same resolution errors |

## `kasmos stop`

| Argument | Type | Required | Default | Validation | Error Behavior |
|----------|------|----------|---------|------------|----------------|
| `[feature]` | String (positional, optional) | No | Auto-detect from `.kasmos/` in cwd | Same prefix matching as `start` when provided | Same resolution errors |

## `kasmos tui`

| Argument | Type | Required | Default | Validation | Error Behavior |
|----------|------|----------|---------|------------|----------------|
| `--count` | `usize` | No | `12` | `RangedU64ValueParser` -- must be >= 1 | Clap error: "0 is not in 1..18446744073709551615" |

**No external dependencies**: Does not require Zellij, git, `kitty-specs/`, or a running orchestration session.

### Channel Contracts (TUI Preview)

| Channel | Direction | Type | Behavior |
|---------|-----------|------|----------|
| `watch::Receiver<OrchestrationRun>` | Engine -> TUI | State updates | Fed by `animation_loop` background task |
| `mpsc::Sender<EngineAction>` | TUI -> Engine | User commands | Receiver dropped; sends silently fail |

## Versioning

| Version | Changes |
|---------|---------|
| v0.1 | Initial CLI: `list`, `start`, `status`, `cmd`, `attach`, `stop` |
| v0.1+ | FIFO commands: `finalize`, `approve`, `reject` added |
| v0.2 | `tui` subcommand added (feature 009) |

## Maintenance

This contract is the canonical reference for the kasmos CLI surface. It MUST be updated whenever:
- A new subcommand is added to `Commands` enum in `main.rs`
- A new FIFO command is added to `FifoCommand` enum in `cmd.rs`
- Argument names, types, defaults, or validation rules change
- A command is deprecated or removed

Run `scripts/check-cli-contract.sh` to verify this contract matches the implemented CLI.
