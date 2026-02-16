# kasmos Architecture Intelligence

> Codebase discoveries and architectural knowledge accumulated during development.
> This file is the authority on how kasmos internals work and interact.
> Updated: 2026-02-13

## Worktree Structure

kasmos uses git worktrees for WP isolation during orchestration.

- Location: `<repo_root>/.worktrees/<feature_slug>-<wp_id>/`
- Each worktree is a full repo checkout on its own branch.
- The worktree contains its own copy of `kitty-specs/` files.
- The `.kittify/memory/` directory inside worktrees is a **symlink** back to the main repo's `.kittify/memory/`, so constitution and memory are shared.

### Worktree vs main repo file paths

This is a critical distinction that affects multiple subsystems:

- **Main repo** `kitty-specs/<slug>/tasks/WPxx.md` -- the canonical task files, versioned in git.
- **Worktree** `.worktrees/<slug>-<wp_id>/kitty-specs/<slug>/tasks/WPxx.md` -- the agent's working copy.

When an agent modifies a task file (e.g., moving its lane from `doing` to `for_review`), it modifies the **worktree copy**, not the main repo copy. Any subsystem that watches for file changes (e.g., `CompletionDetector` in `crates/kasmos/src/detector.rs`) must watch the worktree path, not the main repo path, when worktrees are in use.

**Known issue (2026-02-13)**: The `CompletionDetector` path construction in `crates/kasmos/src/start.rs` (lines ~399-427) builds `detector_paths` from `feature_scan.wp_files`, which scans the main repo's `kitty-specs/<slug>/tasks/`. This means the file watcher may not see lane transitions made by agents working in worktrees. Needs investigation -- it may work if agents push changes that get merged back, or it may be a real bug depending on the spec-kitty workflow.

## Zellij Integration

### Session architecture

- The hub TUI (`kasmos` with no args) runs in the `kasmos-hub` Zellij session.
- The orchestrator TUI (`kasmos start <feature>`) runs in a tab within that session.
- Agent panes are created in sibling tabs named `agents-w{wave_index}` (from `crates/kasmos/src/start.rs:529`).
- Agent panes are named `{wp_id}-pane` (from `crates/kasmos/src/start.rs:163`).

### Zellij CLI limitations (v0.41+)

- There is **no** `list-panes` or `focus-pane-by-name` CLI command.
- The `SessionManager` (`crates/kasmos/src/session.rs`) tracks panes internally via HashMap.
- Focus navigation between panes uses `focus-next-pane`/`focus-previous-pane` with shortest-path calculation.
- Inside a Zellij session, use `zellij action <cmd>` directly (no `--session` flag needed). The `--session` approach is for remote control from outside the session (see `kitty-specs/010-hub-tui-navigator/research.md` R-001).

### Pane ID tracking

`WorkPackage.pane_id` (`crates/kasmos/src/types.rs`) is `Option<u32>` and is set by the `SessionManager` after pane creation. In tests using `StubSessionController`, `pane_id` is always `None`. The dashboard/kanban view in `crates/kasmos/src/tui/tabs/dashboard.rs` displays pane IDs -- if they show as incorrect or missing, check whether real `SessionManager` assignment is happening or a stub is in use.

## TUI Architecture

### Two TUI contexts

1. **Hub TUI** (`crates/kasmos/src/hub/`) -- the feature browser launched by bare `kasmos`. Runs its own event loop in `hub/mod.rs`. Actions dispatch via `hub/actions.rs`.
2. **Orchestrator TUI** (`crates/kasmos/src/tui/`) -- the per-feature dashboard launched by `kasmos start`. Runs its event loop in `tui/mod.rs`. Has tabs: Dashboard, Review, Logs.

### Orchestrator TUI key files

| File | Purpose |
|------|---------|
| `crates/kasmos/src/tui/mod.rs` | Event loop, terminal setup, async task spawning |
| `crates/kasmos/src/tui/app.rs` | App state struct, tab management |
| `crates/kasmos/src/tui/keybindings.rs` | Key event dispatch per tab (including `handle_review_key`) |
| `crates/kasmos/src/tui/tabs/review.rs` | Review tab rendering |
| `crates/kasmos/src/tui/tabs/dashboard.rs` | Dashboard/kanban view rendering |

### Adding new async actions to the orchestrator TUI

Follow the pattern used by `open_hub_requested` in `tui/app.rs`:
1. Add an `Option<T>` field to the `App` struct for the request data.
2. Set it from a keybinding handler in `tui/keybindings.rs`.
3. Check and consume it in the event loop in `tui/mod.rs`, spawning an async task.
4. The async task interacts with Zellij via CLI commands.

## Key Type Definitions

| Type | Location | Notes |
|------|----------|-------|
| `WorkPackage` | `crates/kasmos/src/types.rs` | Has `pane_id: Option<u32>`, `worktree_path`, `pane_name` |
| `WPSummary` | `crates/kasmos/src/hub/scanner.rs` | Hub's view of a WP; has `worktree_path: Option<PathBuf>` |
| `SessionManager` | `crates/kasmos/src/session.rs` | Tracks panes via HashMap |
| `CompletionDetector` | `crates/kasmos/src/detector.rs` | Watches task files for lane transitions |
| `WorktreeManager` | `crates/kasmos/src/git.rs` | Creates worktrees at `.worktrees/{feature_name}-{wp_id}` |
| `HubAction` | `crates/kasmos/src/hub/actions.rs` | Contextual actions including `OpenWP` |

## In-Progress Work (2026-02-13)

### Enter-to-open-pane in orchestrator Review tab

Goal: Pressing Enter on a WP in the orchestrator TUI's Review tab should navigate to the pane running that WP's agent session (if the pane exists), or open a new session in a new pane pointed at that WP's worktree.

**This is ONLY for the Review tab** of the orchestrator TUI, NOT the hub detail view.

Completed:
- Added `worktree_path: Option<PathBuf>` to `WPSummary` in `hub/scanner.rs`
- Added `HubAction::OpenWP` variant to `hub/actions.rs` with Zellij dispatch logic
- Added `open_wp_pane_request` field to orchestrator `tui/app.rs` `App` struct (constructor not yet updated -- missing field in `App::new()`)

Remaining:
- Fix `App::new()` in `tui/app.rs` to include `open_wp_pane_request: None`
- Wire Enter key in `tui/keybindings.rs:handle_review_key` to set the request
- Handle the request in `tui/mod.rs:run()` event loop (spawn Zellij navigation)
- Update Review tab help text in `tui/tabs/review.rs`

### Completion detector worktree paths

The detector in `start.rs` may be watching main-repo task files instead of worktree copies. Needs verification and fix if confirmed.

### Pane ID investigation

`WorkPackage.pane_id` may be `None` in contexts where `StubSessionController` is used. Need to verify real `SessionManager` assigns IDs correctly during actual orchestration.

## OpenCode Agent Permissions and External Directories

### Problem discovered (2026-02-14)

Agents running in worktrees (e.g., `.worktrees/011-...-WP02/`) need read access to paths outside their CWD — specifically the main repo's `kitty-specs/` directory (which is gitignored and doesn't exist in worktrees) and `/tmp/` (where spec-kitty writes review prompts).

OpenCode's `external_directory` permission config does **not** expand `~` to the home directory. Paths like `"~/dev/kasmos/**": "allow"` silently fail to match absolute paths like `/home/kas/dev/kasmos/kitty-specs/...`, causing `auto-rejecting` when the agent runs non-interactively (e.g., `ocx oc -- run`).

**Fix**: Always use fully-qualified absolute paths in `external_directory` rules.

### Paths agents commonly need

| Path | Who needs it | Why |
|------|-------------|-----|
| `/home/kas/dev/kasmos/**` (main repo) | All agents | `kitty-specs/`, `.kittify/memory/`, docs |
| `/tmp/*`, `/tmp/**` | All agents | spec-kitty review prompts, temp files |
| `~/.config/opencode/**` | All agents | Self-reference for config |
| `~/.config/zellij/**` | Controller, release | Layout management |

### TODO: `kasmos setup` should bootstrap this (WP10)

The `kasmos setup` command (WP10 — Setup Command and Launch Hardening) should handle opencode permission configuration as part of environment bootstrap. Specifically:

1. **Config values needed** in `kasmos.toml`:
   - `paths.opencode_config` — path to the active opencode profile config (e.g., `~/.config/opencode/profiles/kas/opencode.jsonc`)
   - `paths.kitty_specs` — already exists, but `setup` should use it to pre-approve the directory
   - `paths.main_repo_root` — the canonical repo root (auto-detected from `git rev-parse --git-common-dir`)

2. **What `kasmos setup` should do**:
   - Read the opencode config file
   - Ensure `external_directory` rules exist for the main repo root (absolute path), `/tmp/**`, and config dirs
   - Warn if `~` is used instead of absolute paths (common mistake)
   - Optionally write/patch the rules if the user consents

This avoids the current fragile manual setup where a tilde-vs-absolute mismatch silently breaks non-interactive reviewer sessions.
