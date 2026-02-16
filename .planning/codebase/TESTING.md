# Testing Patterns

**Analysis Date:** 2026-02-16

## Test Framework

**Runner:**
- Rust's built-in `#[test]` and `#[tokio::test]` — no external test runner framework
- Async runtime: `tokio` with `#[tokio::test]`
- Config: No separate test config file — uses `Cargo.toml` default test profile

**Assertion Library:**
- Standard `assert!()`, `assert_eq!()`, `assert!(matches!())` — no external assertion library
- Pattern matching assertions for error variant checking:
  ```rust
  assert!(matches!(
      result.unwrap_err(),
      crate::error::KasmosError::Zellij(ZellijError::SessionExists { .. })
  ));
  ```

**Run Commands:**
```bash
cargo test                           # Run all tests (default features)
cargo test --features tui            # Run all tests including TUI-gated tests
cargo clippy -p kasmos -- -D warnings  # Lint (treat warnings as errors)
cargo clippy --all-targets --all-features -- -D warnings  # Full lint
```

**Justfile shortcuts:**
```bash
just test    # cargo test
just lint    # cargo clippy --all-targets --all-features -- -D warnings
```

## Test File Organization

**Location:**
- Co-located: all tests live in `#[cfg(test)] mod tests { ... }` at the bottom of each source file
- No separate `tests/` directory for integration tests
- No separate `*_test.rs` files

**Naming:**
- Test functions use `test_` prefix: `test_parse_frontmatter_valid()`, `test_wp_pending_to_active()`
- Descriptive names encode the scenario: `test_capacity_limiting()`, `test_force_advance_unblocks_dependents()`
- Invalid/error cases use explicit names: `test_wp_invalid_completed_to_active()`, `test_session_name_validation_invalid_chars()`

**Coverage:**
- 45 test modules across the codebase (every major module has tests)
- ~419 individual test functions
- Tests exist for: `error.rs`, `types.rs`, `config.rs`, `state_machine.rs`, `graph.rs`, `parser.rs`, `session.rs`, `engine.rs`, `detector.rs`, `zellij.rs`, `review.rs`, `cmd.rs`, `logging.rs`, `serve/mod.rs`, `serve/audit.rs`, `serve/lock.rs`, `serve/messages.rs`, `serve/tools/workflow_status.rs`, `serve/tools/transition_wp.rs`, `serve/tools/wait_for_event.rs`, `serve/tools/read_messages.rs`, `launch/mod.rs`, `launch/detect.rs`, `launch/session.rs`, `launch/layout.rs`, `prompt.rs`, `persistence.rs`, `shutdown.rs`, `health.rs`, `cleanup.rs`, `signals.rs`, `commands.rs`, `command_handlers.rs`, `report.rs`, `layout.rs`, `git.rs`, `review_coordinator.rs`, `hub/scanner.rs`, `hub/app.rs`, `hub/actions.rs`, `hub/keybindings.rs`, `tui/keybindings.rs`, `tui/app.rs`, `tui/widgets/dependency_graph.rs`

## Test Structure

**Suite Organization:**
```rust
#[cfg(test)]
mod tests {
    use super::*;

    // Optional: helper functions at top
    fn create_test_wp(id: &str, deps: Vec<&str>) -> WorkPackage { ... }

    // Optional: shared test fixtures
    fn test_lock_config(timeout_minutes: u64) -> LockConfig { ... }

    #[test]
    fn test_descriptive_scenario_name() {
        // Arrange
        let config = Config::default();
        
        // Act
        let result = config.validate();
        
        // Assert
        assert!(result.is_ok());
    }

    #[tokio::test]
    async fn test_async_operation() {
        // ...
    }
}
```

**Patterns:**

**Setup with `tempfile::TempDir`:**
Tests that need filesystem isolation use `tempfile::TempDir` (dependency in `Cargo.toml`):
```rust
let tmp = tempfile::tempdir().expect("create tempdir");
let path = tmp.path().join("kasmos.toml");
std::fs::write(&path, r#"[agent]\nmax_parallel_workers = 6\n"#).expect("write toml");
```

**Environment Variable Test Safety:**
```rust
static ENV_TEST_LOCK: Mutex<()> = Mutex::new(());

#[test]
fn env_overrides_take_precedence() {
    let _guard = ENV_TEST_LOCK.lock().expect("env lock");
    unsafe { std::env::set_var("KASMOS_AGENT_MAX_PARALLEL_WORKERS", "9"); }
    // ... test ...
    unsafe { std::env::remove_var("KASMOS_AGENT_MAX_PARALLEL_WORKERS"); }
}
```

**Table-Driven Tests:**
```rust
#[test]
fn test_fifo_line_formatting() {
    let cases = vec![
        (FifoCommand::Status, "status"),
        (FifoCommand::Restart { wp_id: "WP01".into() }, "restart WP01"),
        (FifoCommand::Abort, "abort"),
    ];
    for (command, expected) in cases {
        assert_eq!(command.as_fifo_line(), expected);
    }
}
```

**Data-Driven State Machine Tests:**
Each valid/invalid transition gets its own test function for clear failure reporting:
```rust
#[test]
fn test_wp_pending_to_active() {
    assert!(WPState::Pending.can_transition_to(&WPState::Active));
    assert!(WPState::Pending.transition(WPState::Active, "WP01").is_ok());
}

#[test]
fn test_wp_invalid_completed_to_active() {
    assert!(!WPState::Completed.can_transition_to(&WPState::Active));
    assert!(WPState::Completed.transition(WPState::Active, "WP01").is_err());
}
```

## Mocking

**Framework:** No mocking framework — hand-written mocks using traits

**Pattern: Trait-based mock for external dependencies (`crates/kasmos/src/session.rs`):**
```rust
/// Mock Zellij CLI for testing.
struct MockZellijCli {
    sessions: Mutex<Vec<SessionInfo>>,
    calls: Mutex<Vec<String>>,  // Track all method calls
}

impl MockZellijCli {
    fn new() -> Self {
        Self {
            sessions: Mutex::new(Vec::new()),
            calls: Mutex::new(Vec::new()),
        }
    }

    fn add_session(&self, name: &str, state: SessionState) { ... }
    fn get_calls(&self) -> Vec<String> { ... }
}

#[async_trait]
impl ZellijCli for MockZellijCli {
    async fn create_session(&self, name: &str, _layout: Option<&std::path::Path>) -> Result<()> {
        self.calls.lock().unwrap().push(format!("create_session:{}", name));
        // ... mock behavior ...
    }
    // ... all trait methods implemented ...
}
```

**Pattern: Using the mock:**
```rust
#[tokio::test]
async fn test_start_session_creates_new() {
    let cli = Arc::new(MockZellijCli::new());
    let config = Arc::new(Config::default());
    let mut manager = SessionManager::new("test-session".to_string(), cli.clone(), config).unwrap();

    let result = manager.start_session().await;
    assert!(result.is_ok());
    assert!(cli.get_calls().iter().any(|c| c.contains("create_session:test-session")));
}
```

**Pattern: Callback/closure mocks for prompt functions (`crates/kasmos/src/launch/mod.rs`):**
```rust
#[tokio::test]
async fn test_selector_runs_before_preflight_failures() {
    let called = Arc::new(AtomicBool::new(false));
    let called_in_prompt = Arc::clone(&called);
    let mut prompt = move |_max: usize| {
        called_in_prompt.store(true, Ordering::SeqCst);
        Ok(1)
    };

    let err = run_with_detection_and_prompt(&config, &specs_root, detection, &mut prompt)
        .await
        .expect_err("preflight should fail");
    assert!(called.load(Ordering::SeqCst));
}
```

**What to Mock:**
- External CLI tools (Zellij) — via `ZellijCli` trait
- User input prompts — via closure parameters
- File system — via `tempfile::TempDir`

**What NOT to Mock:**
- Internal state machines (`WPState`, `RunState`) — tested directly
- Configuration parsing — tested with real TOML strings
- Serialization/deserialization — tested with `serde_json::to_string()` / `from_str()`
- Dependency graph algorithms — tested with in-memory data structures

## Fixtures and Factories

**Test Data — Helper Functions:**
```rust
fn create_test_wp(id: &str, deps: Vec<&str>) -> WorkPackage {
    WorkPackage {
        id: id.to_string(),
        title: format!("WP {}", id),
        state: WPState::Pending,
        dependencies: deps.iter().map(|s| s.to_string()).collect(),
        wave: 0,
        pane_id: None,
        pane_name: format!("wp_{}", id.to_lowercase()),
        worktree_path: None,
        prompt_path: None,
        started_at: None,
        completed_at: None,
        completion_method: None,
        failure_count: 0,
    }
}

fn create_test_run(
    wps: Vec<(String, Vec<String>, usize)>,
    mode: ProgressionMode,
) -> OrchestrationRun { ... }
```

**Test Data — Audit Writer Factory (`crates/kasmos/src/serve/audit.rs`):**
```rust
fn new_writer(config: &AuditConfig) -> (tempfile::TempDir, AuditWriter) {
    let tmp = tempdir().expect("tempdir");
    let feature = tmp.path().join("011-feature");
    std::fs::create_dir_all(&feature).expect("feature dir");
    let writer = AuditWriter::new(&feature, "011-feature", config).expect("writer");
    (tmp, writer)
}
```

**Test Data — Inline YAML/TOML:**
```rust
let yaml_content = r#"---
work_package_id: WP01
title: Core Types & Config
dependencies: []
lane: planned
---

# Work Package Content
"#;
std::fs::write(&file_path, yaml_content).unwrap();
```

**Location:**
- Helper functions are defined within each `#[cfg(test)] mod tests { }` block
- No shared test utilities directory — each module has its own helpers
- `tempfile` crate (`tempfile = "3.25.0"`) is a regular dependency (not dev-only), used in both tests and production

## Coverage

**Requirements:** No enforced coverage target
**Coverage Tool:** Not configured — no `cargo-tarpaulin` or `cargo-llvm-cov` in dependencies or Justfile

**View Coverage:**
```bash
# Not currently configured; to add:
cargo install cargo-tarpaulin
cargo tarpaulin --out Html
```

## Test Types

**Unit Tests:**
- All tests are co-located unit tests
- Test individual functions, state transitions, data transformations
- Use `#[test]` for synchronous, `#[tokio::test]` for async
- Heavily used for: state machines (31 tests in `state_machine.rs`), dependency graph (14 tests), serialization round-trips, error display messages

**Integration Tests:**
- No dedicated `tests/` directory — integration-style testing happens within module tests
- MCP server tests in `crates/kasmos/src/serve/mod.rs` test tool registration, input validation, and multi-tool workflows
- Launch flow tests in `crates/kasmos/src/launch/mod.rs` test the full detection → selection → preflight pipeline
- Lock concurrency test in `crates/kasmos/src/serve/lock.rs` uses `std::thread` and `Barrier` for real concurrency testing

**E2E Tests:**
- Not present — the system relies on external tools (Zellij, spec-kitty, git) making E2E difficult
- Launch/setup commands require real binaries in PATH

## Common Patterns

**Async Testing:**
```rust
#[tokio::test]
async fn test_continuous_auto_launch() {
    let run = Arc::new(RwLock::new(create_test_run(...)));
    let (completion_tx, completion_rx) = mpsc::channel(10);
    let (_action_tx, action_rx) = mpsc::channel(10);

    let (mut engine, _launch_rx) = create_test_engine(run.clone(), completion_rx, action_rx);
    engine.init_graph().await.unwrap();

    engine.launch_eligible_wps().await.unwrap();

    {
        let r = run.read().await;
        assert_eq!(r.work_packages[0].state, WPState::Active);
    }
}
```

**Error Testing:**
```rust
#[test]
fn test_invalid_values_produce_clear_errors() {
    let mut config = Config::default();
    config.agent.max_parallel_workers = 0;
    let err = config.validate().expect_err("validation should fail");
    assert!(err.to_string().contains("agent.max_parallel_workers"));
}
```

**Serialization Round-Trip Testing:**
```rust
#[test]
fn test_wp_state_serialization() {
    let state = WPState::Active;
    let json = serde_json::to_string(&state).expect("serialize");
    assert_eq!(json, "\"active\"");

    let deserialized: WPState = serde_json::from_str(&json).expect("deserialize");
    assert_eq!(deserialized, WPState::Active);
}
```

**Filesystem Testing:**
```rust
#[test]
fn test_feature_dir_scan() {
    let temp_dir = TempDir::new().unwrap();
    let tasks_dir = temp_dir.path().join("tasks");
    fs::create_dir(&tasks_dir).unwrap();

    fs::write(tasks_dir.join("WP01-core.md"), "---\nwork_package_id: WP01\n---\n").unwrap();
    
    let feature_dir = FeatureDir::scan(temp_dir.path()).unwrap();
    assert_eq!(feature_dir.wp_files.len(), 1);
}
```

**Concurrency Testing (`crates/kasmos/src/serve/lock.rs`):**
```rust
#[test]
fn concurrent_acquire_allows_single_winner() {
    let barrier = Arc::new(Barrier::new(2));
    let outcomes = Arc::new(Mutex::new(Vec::new()));

    let mut handles = Vec::new();
    for i in 0..2 {
        let barrier = Arc::clone(&barrier);
        handles.push(thread::spawn(move || {
            barrier.wait();
            let outcome = manager.acquire(false);
            outcomes.lock().unwrap().push(outcome);
        }));
    }
    for handle in handles { handle.join().unwrap(); }

    let success_count = outcomes.lock().unwrap().iter().filter(|r| r.is_ok()).count();
    assert_eq!(success_count, 1);
}
```

**MCP Contract Testing (`crates/kasmos/src/serve/mod.rs`):**
```rust
#[test]
fn server_registers_all_contract_tools() {
    let server = KasmosServer::new(Config::default()).expect("server init");
    let mut names = server.tool_router.list_all()
        .into_iter()
        .map(|tool| tool.name.to_string())
        .collect::<Vec<_>>();
    names.sort();

    assert_eq!(names, vec![
        "despawn_worker", "infer_feature", "list_features",
        "list_workers", "read_messages", "spawn_worker",
        "transition_wp", "wait_for_event", "workflow_status"
    ]);
}

#[test]
fn spawn_worker_input_rejects_invalid_payloads() {
    let invalid = serde_json::json!({ "unexpected": true, ... });
    let err = parse_json_object::<SpawnWorkerInput>(invalid).expect_err("must fail");
    assert_eq!(err.code, rmcp::model::ErrorCode::INVALID_PARAMS);
}
```

## Test Expectations

**When adding new code:**
- Every new module should include a `#[cfg(test)] mod tests` block
- State transitions need both valid and invalid transition tests
- MCP tools need: input validation tests, happy-path handler tests, error-path tests
- Configuration fields need: default validation, TOML parsing, env override tests
- File operations need: `tempfile::TempDir` isolation, missing file handling, permission edge cases

**When modifying existing code:**
- Run `cargo test` before committing — all 419+ tests must pass
- Run `cargo clippy -p kasmos -- -D warnings` — zero warnings allowed
- For feature-gated code: `cargo test --features tui`

---

*Testing analysis: 2026-02-16*
