# Codebase Concerns

**Analysis Date:** 2026-02-16

## Tech Debt

**Config Duplication — Legacy Flat Fields vs Sectioned Config:**
- Issue: `Config` has duplicate fields for the same settings. Legacy flat fields (`max_agent_panes`, `zellij_binary`, `opencode_binary`, `spec_kitty_binary`, `kasmos_dir`, `poll_interval_secs`, `debounce_ms`, `controller_width_pct`, `opencode_profile`) exist alongside their sectioned counterparts in `AgentConfig`, `PathsConfig`, `SessionConfig`, `CommunicationConfig`.
- Files: `crates/kasmos/src/config.rs` (lines 27-48, 176-204, 287-319)
- Impact: Double storage of config values. The legacy flat fields are read by `start.rs` and other TUI-legacy modules. The sectioned fields are used by the MCP serve path. Divergence between the two surfaces is inevitable — one could be updated without the other.
- Fix approach: Remove the legacy flat fields from `Config`. Update all legacy TUI modules (`start.rs`, `hub/`, `tui/`) to read from the sectioned sub-structs. The legacy env var aliases in `load_from_env()` can stay for backward compatibility but should write to the sectioned fields (which they already do).

**Legacy TUI Modules Behind Feature Gate:**
- Issue: 9 modules are preserved behind `#[cfg(feature = "tui")]` with `#[allow(dead_code)]`: `attach`, `cmd`, `hub`, `report`, `sendmsg`, `start`, `stop`, `tui_cmd`, `tui_preview`. These modules import from `lib.rs` using the `kasmos::` path (extern crate style) and are compiled but never called from `main.rs`.
- Files: `crates/kasmos/src/main.rs` (lines 10-38), `crates/kasmos/src/start.rs`, `crates/kasmos/src/stop.rs`, `crates/kasmos/src/hub/`, `crates/kasmos/src/tui/`, `crates/kasmos/src/tui_cmd.rs`, `crates/kasmos/src/tui_preview.rs`, `crates/kasmos/src/attach.rs`, `crates/kasmos/src/sendmsg.rs`, `crates/kasmos/src/report.rs`
- Impact: ~5,500 lines of code that increase compile time and cognitive load but serve no production purpose. Several of these modules (e.g., `start.rs` at 657 lines) contain substantial logic that cannot run because it's gated. The `hub/` module alone is ~2,700 lines across 4 files.
- Fix approach: Either (a) wire TUI modules back into the CLI surface as an alternative command (`kasmos start --tui`), or (b) archive them to a separate branch and remove from the workspace. Given the README says "Legacy TUI modules are preserved behind feature flag `tui`", this is intentional preservation — but the code should have a deadline for reintegration or removal.

**Dummy Dependency Graph in WaveEngine Constructor:**
- Issue: `WaveEngine::new()` creates an empty dummy `DependencyGraph` with empty HashMaps, then rebuilds it in `init_graph()`. The comment says "We create a dummy graph here and rebuild it on first use if needed."
- Files: `crates/kasmos/src/engine.rs` (lines 86-91)
- Impact: Between construction and the first `init_graph()` call, the engine has a non-functional graph. If any method were called before `init_graph()`, dependency checks would silently pass (empty deps = all satisfied). This is a latent correctness bug.
- Fix approach: Take `DependencyGraph` as a constructor parameter, or make it `Option<DependencyGraph>` and error if used before initialization. Alternatively, make `init_graph` part of construction with an async builder pattern.

**StubSessionController in start.rs:**
- Issue: `start.rs` uses a `StubSessionController` that logs warnings but does nothing for focus/zoom commands. The comment says "This will be replaced with a real SessionManager-backed implementation."
- Files: `crates/kasmos/src/start.rs` (lines 639-657)
- Impact: Focus and zoom commands from the TUI dashboard are silently dropped. The TUI shows controls that don't work.
- Fix approach: Wire `SessionManager` into `CommandHandler` so focus/zoom operations invoke real Zellij pane navigation.

**Empty Dashboard Module:**
- Issue: `crates/kasmos/src/serve/dashboard.rs` is a 2-line file containing only a module-level doc comment. The actual dashboard rendering lives in `wait_for_event.rs` (`format_worker_table`).
- Files: `crates/kasmos/src/serve/dashboard.rs` (2 lines)
- Impact: Misleading module structure. Dashboard logic is scattered.
- Fix approach: Either move `format_worker_table` and dashboard update logic into `dashboard.rs`, or remove the empty file and the `pub mod dashboard` declaration in `serve/mod.rs`.

**RunState::transition Uses Placeholder WPState in Error:**
- Issue: `RunState::transition()` creates a `StateError::InvalidTransition` with `from: WPState::Pending, to: WPState::Pending` as placeholders because the error variant is designed for WP states, not run states.
- Files: `crates/kasmos/src/state_machine.rs` (lines 97-108)
- Impact: Error messages for invalid run state transitions will display misleading WP state information (e.g., "Invalid state transition: Pending -> Pending for WP run").
- Fix approach: Add a `RunStateTransitionError` variant to `StateError` (or a separate `InvalidRunTransition` variant) that carries `RunState` values instead of `WPState`.

## Known Bugs

**`check_git_activity` Always Detects Git:**
- Symptoms: Any worktree with `.git/refs/heads` is considered to have "git activity," even if no new commits exist. This is a false positive: `.git/refs/heads` exists in every git worktree from the moment it's created.
- Files: `crates/kasmos/src/detector.rs` (lines 256-266)
- Trigger: Any worktree-based completion check falls through to secondary detection.
- Workaround: Primary detection (YAML frontmatter lane) takes precedence, so this rarely fires. But if primary detection fails (parse error), the secondary signal will incorrectly report completion.

**Message Cursor Re-parse on Every Poll:**
- Symptoms: `read_messages_since()` reads the entire scrollback on every poll, re-parses all messages, then filters by `message_index >= since_index`. As the message log grows, this becomes O(n) on each poll.
- Files: `crates/kasmos/src/serve/messages.rs` (lines 123-134)
- Trigger: Long-running orchestrations with many messages; `wait_for_event` polls every `poll_interval_secs` (default 5s).
- Workaround: None. The message cursor (`since_index`) provides semantic filtering but doesn't skip parsing.

## Security Considerations

**Unsafe libc Calls (11 instances):**
- Risk: Direct `unsafe` calls to `libc::flock`, `libc::kill`, and `std::env::set_var`/`std::env::remove_var`. The `flock` and `kill` calls are well-understood POSIX patterns, but the env var mutations in tests are unsound under Rust 2024 edition rules.
- Files:
  - `crates/kasmos/src/serve/lock.rs` (lines 510, 522) — `libc::flock` for advisory file locking
  - `crates/kasmos/src/stop.rs` (line 47) — `libc::kill` for SIGTERM
  - `crates/kasmos/src/start.rs` (line 38) — `libc::kill` for PID existence check
  - `crates/kasmos/src/hub/scanner.rs` (line 412) — `libc::kill` for PID existence check
  - `crates/kasmos/src/launch/session.rs` (lines 144, 148) — `std::env::set_var`/`remove_var` in tests
  - `crates/kasmos/src/setup/mod.rs` (lines 406, 424) — `std::env::set_var`/`remove_var` in tests
  - `crates/kasmos/src/config.rs` (lines 717, 721) — `std::env::set_var`/`remove_var` in tests
- Current mitigation: The `libc::kill` and `libc::flock` calls check return values and handle errno. The env var mutations are test-only.
- Recommendations: Replace `libc::kill(pid, 0)` with `nix::sys::signal::kill(Pid::from_raw(pid), None)` — the `nix` crate is already a dependency. Replace `libc::flock` with `nix::fcntl::flock`. For env var tests, use the `temp_env` crate or a mutex-guarded test helper.

**Shell Command Injection Surface:**
- Risk: Multiple places shell out to external commands (`zellij`, `git`, `pane-tracker`). While `validate_identifier()` and `contains_shell_metacharacters()` guard pane/session names, file paths are passed through `shell_quote_path()` which uses single-quote escaping.
- Files:
  - `crates/kasmos/src/zellij.rs` — all `Command::new("zellij")` calls
  - `crates/kasmos/src/serve/messages.rs` (lines 147-151, 196-219) — `pane-tracker` invocations
  - `crates/kasmos/src/git.rs` — all `Command::new("git")` calls
  - `crates/kasmos/src/start.rs` (lines 543-553) — `zellij action new-tab`
- Current mitigation: `tokio::process::Command` with `args()` array (not shell interpolation), identifier validation, and POSIX shell quoting for paths.
- Recommendations: The current approach is sound. Continue using `Command` with explicit argument arrays rather than string interpolation.

**Lock File PID Parsing in stop.rs:**
- Risk: `stop.rs` reads a PID from a lock file and sends SIGTERM to it. If the lock file is corrupted or the PID has been recycled, the signal goes to the wrong process.
- Files: `crates/kasmos/src/stop.rs` (lines 40-47)
- Current mitigation: Checks `ESRCH` errno (process doesn't exist) and cleans up stale files. The advisory flock in `serve/lock.rs` is more robust.
- Recommendations: Add a process name/identity check (e.g., verify `/proc/{pid}/cmdline` contains "kasmos") before sending signals.

## Performance Bottlenecks

**Full Scrollback Re-parse on Every Message Read:**
- Problem: `read_messages_since()` reads the complete pane scrollback, strips ANSI codes from every line, and regex-matches every line on each poll cycle.
- Files: `crates/kasmos/src/serve/messages.rs` (lines 102-134)
- Cause: No incremental read mechanism. The `pane-tracker` binary dumps the entire scrollback each time.
- Improvement path: Either (a) have `pane-tracker` support `--since-line N` to return only new content, or (b) cache parsed messages in the server state and only parse new lines (diff against previous scrollback length).

**State Broadcast Clones Entire OrchestrationRun:**
- Problem: `broadcast_state()` clones the entire `OrchestrationRun` (which includes all `WorkPackage`s, `Wave`s, and the full `Config`) on every state change event.
- Files: `crates/kasmos/src/engine.rs` (lines 192-197)
- Cause: `watch::Sender::send()` requires an owned value. `OrchestrationRun` derives `Clone` but contains `PathBuf`, `Config`, `Vec<WorkPackage>`, etc.
- Improvement path: Use `Arc<OrchestrationRun>` in the watch channel, or broadcast a lightweight delta/diff struct instead of the full state.

**Linear WP Lookup in Engine:**
- Problem: Every engine operation (`launch_wp`, `handle_completion`, `restart_wp`, etc.) does `run.work_packages.iter().find(|w| w.id == wp_id)` — an O(n) linear scan.
- Files: `crates/kasmos/src/engine.rs` (lines 275-283, 609-617, 640-648, etc.)
- Cause: Work packages are stored in a `Vec<WorkPackage>` rather than indexed by ID.
- Improvement path: Add a `HashMap<String, usize>` index from WP ID to position in the vec, or use `IndexMap` for ordered + keyed access. For typical orchestrations (<20 WPs) this is not a real bottleneck, but the pattern is repeated ~10 times.

## Fragile Areas

**Completion Detector Event Pipeline:**
- Files: `crates/kasmos/src/detector.rs` (lines 125-174, 318-397)
- Why fragile: The detector bridges sync (`notify` callback) and async (tokio mpsc) worlds via `blocking_send()`. If the async processing task falls behind, the `blocking_send()` in the `notify` callback could block the OS file notification thread. The debounce window (500ms) and deduplication are implemented manually rather than using established debouncing libraries.
- Safe modification: When changing event processing logic, always test with rapid file modifications (e.g., `touch` in a tight loop). The retry-on-read-failure pattern (3 attempts, 200ms delay) handles atomic file writes well.
- Test coverage: Unit tests cover `check_completion()` parsing but not the async pipeline (`process_events`, debounce, deduplication). No integration tests for the watcher lifecycle.

**MCP Message Protocol Parsing:**
- Files: `crates/kasmos/src/serve/messages.rs` (lines 16-19, 75-99)
- Why fragile: The message format `[KASMOS:sender:event] payload` is defined by regex only, with no formal schema. Message parsing silently drops non-matching lines. If an agent outputs a line that partially matches the regex, it could be misinterpreted.
- Safe modification: Always update `KNOWN_EVENTS` constant and add test cases when adding new event types. The `message.known_event` flag allows forward-compatible parsing of unknown events.
- Test coverage: Good coverage for `parse_message()` and `parse_scrollback()`. No coverage for the degraded fallback path.

**Zellij CLI Abstraction:**
- Files: `crates/kasmos/src/zellij.rs`, `crates/kasmos/src/session.rs`
- Why fragile: The implementation works around Zellij 0.41+ limitations (no `list-panes`, no `focus-pane-by-name`). Focus navigation uses `focus-next-pane`/`focus-previous-pane` with shortest-path calculation based on internal pane tracking. If Zellij changes its CLI surface or behavior, the workarounds break silently.
- Safe modification: Any Zellij version upgrade should be tested with the full session lifecycle (create, pane management, tab creation, focus navigation, kill). The `ZellijCli` trait enables mock testing.
- Test coverage: `session.rs` has extensive tests via `MockZellijCli`. No integration tests against a real Zellij binary.

**Wave Launch Handler in start.rs:**
- Files: `crates/kasmos/src/start.rs` (lines 486-583)
- Why fragile: The wave launch handler creates Zellij tabs by shelling out to `zellij action new-tab`. This runs in a spawned tokio task with no cancellation token, so it continues even after shutdown is triggered. The `go-to-tab 1` call at line 577 assumes tab 1 is the TUI dashboard, which is fragile if tabs are reordered.
- Safe modification: Wire the wave launch handler into the shutdown coordinator. Use tab names instead of indices.
- Test coverage: No test coverage — the handler is only exercised in live orchestration.

## Scaling Limits

**Worker Registry (In-Memory, Non-Persistent):**
- Current capacity: Works for typical orchestrations (4-10 workers).
- Limit: The `WorkerRegistry` is an in-memory `HashMap` per server instance. If the MCP server restarts, all worker tracking is lost. No persistence layer.
- Files: `crates/kasmos/src/serve/registry.rs`
- Scaling path: Add optional persistence to the registry (write to `$feature_dir/.kasmos/workers.json`). For multi-server scenarios (unlikely), use the lock file as coordination point.

**Single-Feature MCP Server:**
- Current capacity: One feature per `kasmos serve` instance.
- Limit: The `feature_slug` in `KasmosServer` is set once during construction from `specs_root` inference. The server cannot switch features without restart.
- Files: `crates/kasmos/src/serve/mod.rs` (lines 37, 43-54)
- Scaling path: Not a concern for the intended architecture — one MCP server per feature session.

## Dependencies at Risk

**`notify` Pre-Release Version (9.0.0-rc.1):**
- Risk: Using a release candidate version of `notify` for filesystem watching. RC versions may have undiscovered bugs, and the API could change before stable release.
- Files: `crates/kasmos/Cargo.toml` (line 28)
- Impact: Filesystem completion detection depends entirely on `notify`. A bug in the watcher could cause missed completions or spurious events.
- Migration plan: Track `notify` 9.0 stable release and upgrade. The `notify` API surface used is minimal (`recommended_watcher`, `Watcher::watch`, event filtering), so migration should be low-effort.

**`serde_yml` (0.0.12) — Pre-1.0:**
- Risk: `serde_yml` is at version 0.0.12, indicating early development. This is the `serde_yaml` successor after `serde_yaml` was archived.
- Files: `crates/kasmos/Cargo.toml` (line 13)
- Impact: Used for frontmatter parsing in `detector.rs`, `parser.rs`, and `transition_wp.rs`. A breaking change would affect WP state detection.
- Migration plan: Pin the version and monitor for 0.1 or 1.0 release. The YAML surface used is simple (frontmatter deserialization), so migration risk is low.

**`rmcp` (0.15):**
- Risk: `rmcp` is the MCP SDK. At version 0.15, it's pre-1.0 and the MCP protocol itself is rapidly evolving.
- Files: `crates/kasmos/Cargo.toml` (line 37), `crates/kasmos/src/serve/mod.rs`
- Impact: The entire MCP server surface depends on `rmcp` macros (`#[tool_router]`, `#[tool_handler]`, `#[tool]`). A major version bump would require updating all tool definitions.
- Migration plan: Track `rmcp` releases. The macro-based approach means changes are concentrated in `serve/mod.rs`.

## Missing Critical Features

**No Pane Lifecycle Management in MCP Path:**
- Problem: The MCP `spawn_worker` tool registers a worker in the in-memory registry but does NOT actually create a Zellij pane. Pane creation is the manager agent's responsibility. If the manager agent doesn't create the pane, the worker exists in the registry but has no pane.
- Files: `crates/kasmos/src/serve/tools/spawn_worker.rs`
- Blocks: The MCP path relies entirely on the manager agent to do the right thing. There's no server-side validation that a pane was actually created.

**No Health Monitoring in MCP Path:**
- Problem: `HealthMonitor` exists but is only wired into the legacy TUI `start.rs` path. The MCP serve path has no health monitoring for worker panes.
- Files: `crates/kasmos/src/health.rs`, `crates/kasmos/src/serve/mod.rs`
- Blocks: Crashed worker panes are not automatically detected in MCP mode. The manager agent must poll for worker status manually.

**No Shutdown Coordination in MCP Path:**
- Problem: `ShutdownCoordinator` exists but is only wired into the legacy TUI `start.rs` path. The MCP server has no graceful shutdown beyond tokio signal handling.
- Files: `crates/kasmos/src/shutdown.rs`, `crates/kasmos/src/serve/mod.rs`
- Blocks: Abrupt MCP server termination leaves worker panes running, lock files stale, and state un-persisted.

## Test Coverage Gaps

**Untested Source Files (no `#[cfg(test)]` module):**
- What's not tested: `attach.rs`, `feature_arg.rs`, `lib.rs`, `list_specs.rs`, `main.rs`, `sendmsg.rs`, `start.rs`, `status.rs`, `stop.rs`, `tui_cmd.rs`, `tui_preview.rs`
- Files: All listed above in `crates/kasmos/src/`
- Risk: `feature_arg.rs` (123 lines) handles feature directory resolution — a critical path for all commands. `start.rs` (657 lines) is the entire legacy orchestration flow. `stop.rs` (90 lines) sends SIGTERM to processes.
- Priority: High for `feature_arg.rs` (used by both MCP and launch paths). Low for TUI-gated modules unless they're being reactivated.

**No Integration Tests:**
- What's not tested: End-to-end flows — launching a session, spawning agents, detecting completion, shutdown. All tests are unit tests with mocks.
- Files: No `tests/` directory exists.
- Risk: The interaction between `WaveEngine`, `CompletionDetector`, `SessionManager`, and `ReviewCoordinator` is only tested in isolation. Race conditions between async components are not exercised.
- Priority: Medium. The MCP tool handlers have reasonable integration tests within their modules, but no test exercises the full lifecycle.

**Completion Detector Async Pipeline Not Tested:**
- What's not tested: `CompletionDetector::process_events()`, debounce logic, deduplication, retry-on-read-failure in the async context.
- Files: `crates/kasmos/src/detector.rs` (lines 318-397)
- Risk: The debounce window, retry delays, and deduplication window are hardcoded constants that could interact poorly. A race between file write and read could cause missed events.
- Priority: High. Completion detection is the primary signal for wave progression.

**Degraded Message Mode Not Tested:**
- What's not tested: The fallback `direct_scrollback_read()` path when `pane-tracker` is unavailable.
- Files: `crates/kasmos/src/serve/messages.rs` (lines 175-194)
- Risk: Degraded mode uses a different scrollback source. If it returns differently formatted content, message parsing could break.
- Priority: Medium. Degraded mode is a resilience feature.

---

*Concerns audit: 2026-02-16*
