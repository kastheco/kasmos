# 013 - Implementation Plan

## Architecture Decisions

### AD-01: Auto-detection strategy for pane-tracker path

Search these locations in order, stop at first hit where `mcp-server/index.ts` exists:

1. Value from `kasmos.toml` `[paths].pane_tracker_dir` (if already configured)
2. `KASMOS_PATHS_PANE_TRACKER_DIR` env var
3. Sibling of the WASM plugin: if `zellij-pane-tracker.wasm` was found in a plugins dir, look for the repo checkout at common paths
4. `/opt/zellij-pane-tracker`
5. `$HOME/zellij-pane-tracker`
6. `$HOME/.local/share/zellij-pane-tracker`

Interactive mode presents the auto-detected path as the default; user can override. Non-interactive mode uses auto-detected path silently.

### AD-02: MCP command fixup mechanism

Extend the existing `apply_selections_and_fixup()` with a new `fixup_mcp_pane_tracker_path()` function that walks into `config.mcp.zellij.command` and replaces any path element containing `/opt/zellij-pane-tracker` with the resolved path. This mirrors the existing `fixup_external_directory()` pattern.

### AD-03: zjstatus check follows existing plugin check pattern

Model `check_zjstatus()` identically to `check_pane_tracker()` -- look in `zellij_plugin_dir()` for the WASM file, report pass/fail with install guidance.

## Work Packages

### Dependency Graph

```
WP01 (config + template)  ----+
                               |
WP02 (zjstatus check)    ----+--> WP04 (tests)
                               |
WP03 (docs)               ----+

WP01 --> WP01b (interactive prompt + fixup)
WP01b -----> WP04
```

### Wave Assignment

| Wave | Work Packages | Rationale |
|------|--------------|-----------|
| 1    | WP01, WP02, WP03 | Independent foundations -- zero cross-deps |
| 2    | WP01b | Depends on WP01 config schema being in place |
| 3    | WP04 | Integration tests spanning all changes |

### WP01 - Config schema + template placeholder (Wave 1)

**Scope**: Add `pane_tracker_dir` to `PathsConfig`, wire up TOML/env loading, update the embedded opencode.jsonc template to use a recognizable placeholder path, and add the `detect_pane_tracker_dir()` auto-detection function.

**Files**:
- `crates/kasmos/src/config.rs` -- add field + defaults + env/TOML loading
- `config/profiles/kasmos/opencode.jsonc` -- keep `/opt/zellij-pane-tracker` (fixup replaces it)

**Acceptance**: `Config::default().paths.pane_tracker_dir` returns `"/opt/zellij-pane-tracker"`. TOML and env overrides work. `detect_pane_tracker_dir()` returns the first valid path or the default.

### WP02 - zjstatus.wasm setup check (Wave 1)

**Scope**: Add `check_zjstatus()` to the validation engine alongside the existing pane-tracker check. Include in the checks vector so both `kasmos setup` and launch preflight see it.

**Files**:
- `crates/kasmos/src/setup/mod.rs` -- new `check_zjstatus()` fn, add to checks vector

**Acceptance**: `kasmos setup` shows `[PASS] zjstatus` when the WASM exists, `[FAIL] zjstatus` with install guidance when missing. Launch preflight also catches it.

### WP03 - Documentation updates (Wave 1)

**Scope**: Add a Dependencies section to README.md. Update INTEGRATIONS.md with zjstatus entry. Update quickstart.md prerequisites.

**Files**:
- `README.md`
- `.planning/codebase/INTEGRATIONS.md`
- `kitty-specs/011-mcp-agent-swarm-orchestration/quickstart.md`

**Acceptance**: README lists all runtime deps (zellij, opencode/ocx, spec-kitty, git, zellij-pane-tracker, zjstatus). INTEGRATIONS.md has zjstatus entry.

### WP01b - Interactive pane-tracker path prompt + MCP fixup (Wave 2)

**Scope**: In `interactive_opencode_config()`, after the model/reasoning customization, prompt for the pane-tracker directory (using auto-detected default). Add `fixup_mcp_pane_tracker_path()` to `apply_selections_and_fixup()` to replace `/opt/zellij-pane-tracker` in the MCP command array. Handle the non-interactive codepath too.

**Depends on**: WP01 (needs `detect_pane_tracker_dir()` and the config field)

**Files**:
- `crates/kasmos/src/setup/mod.rs` -- prompt + fixup

**Acceptance**: Generated `.opencode/opencode.jsonc` has the user-provided path in `mcp.zellij.command`. Non-interactive mode uses auto-detected path. Existing `check_pane_tracker()` also validates the MCP server script exists at the configured path.

### WP04 - Tests (Wave 3)

**Scope**: Add/update tests for all new functionality. Update existing tests that assume no zjstatus check.

**Depends on**: WP01, WP01b, WP02

**Files**:
- `crates/kasmos/src/setup/mod.rs` (test module)
- `crates/kasmos/src/config.rs` (test module)

**Acceptance**: `cargo test` passes. New tests cover: zjstatus pass/fail, pane-tracker path detection, MCP path fixup, config field serialization, env override.

## Parallel Execution Plan

```
Time -->

Coder A: [=== WP01: config + detect ===]-->[=== WP01b: prompt + fixup ===]
Coder B: [=== WP02: zjstatus check ===]
Coder C: [=== WP03: docs ============]
                                                                            --> [== WP04: tests ==]
```

Wave 1 dispatches three coders simultaneously. WP01b starts as soon as WP01 merges. WP04 starts after WP01b, WP02 complete (WP03 is not a code dep for tests but should be done by then too).
