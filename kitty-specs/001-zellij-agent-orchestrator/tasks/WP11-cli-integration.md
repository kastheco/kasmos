---
work_package_id: WP11
title: CLI Entry Point & Integration
lane: planned
dependencies: []
subtasks: [T063, T064, T065, T066, T067, T068, T069]
phase: Phase 6 - Integration
assignee: ''
agent: ''
shell_pid: ''
review_status: ''
reviewed_by: ''
history:
- timestamp: '2026-02-09T00:00:00Z'
  lane: planned
  agent: system
  shell_pid: ''
  action: Prompt generated via /spec-kitty.tasks
---

# Work Package Prompt: WP11 – CLI Entry Point & Integration

## IMPORTANT: Review Feedback Status

Before starting implementation, check the **Review Feedback** section below.
- If empty → This is fresh work. Proceed with implementation.
- If populated → This WP was previously reviewed and needs changes. Address ALL feedback items before marking as done.

## Review Feedback

*(Empty — no review feedback yet)*

## Dependency Rebase Guidance

This WP depends on **ALL previous work packages** (WP01-WP10). It is the integration layer that wires everything together.

**Implementation command**:
```bash
spec-kitty implement WP11 --base WP10
```

## Objectives & Success Criteria

**Objective**: Create the CLI entry point using clap with four subcommands (launch, status, attach, stop), wire all modules together with proper error propagation, implement an end-to-end integration test with mocked Zellij, and generate the post-run summary report.

**Success Criteria**:
1. `kasmos launch <feature>` creates session, starts wave engine, begins orchestration
2. `kasmos status [<feature>]` displays current orchestration state from state file
3. `kasmos attach <feature>` reattaches to existing session with state reconciliation
4. `kasmos stop [<feature>]` triggers graceful shutdown
5. All modules are wired with anyhow error propagation and tracing spans
6. Integration test verifies full lifecycle with mock Zellij
7. Post-run report is generated at `.kasmos/report.md` with per-WP durations and statistics

## Context & Constraints

- **Crate location**: `crates/kasmos/`
- **Dependencies**: `clap` (derive macro for CLI), all modules from WP01-WP10
- **Binary name**: `kasmos`
- **Reference**: [plan.md](../plan.md) WP11 section; [spec.md](../spec.md) all FRs
- **Constraint**: Error messages must be user-friendly (not raw stack traces)
- **Constraint**: Integration test must work without a real Zellij installation (mock binary)
- **Constraint**: Report must be human-readable markdown

**Module wiring order** (in launch command):
1. Init logging (WP01)
2. Load config (WP01)
3. Validate dependencies in PATH — zellij, opencode, spec-kitty (WP04)
4. Scan feature directory, parse WP specs (WP02)
5. Build dependency graph, compute waves (WP02)
6. Generate prompts (WP04)
7. Generate KDL layout (WP03)
8. Create Zellij session (WP05)
9. Setup signal handlers (WP10)
10. Start completion detector (WP06)
11. Start health monitor (WP10)
12. Start command reader (WP08)
13. Run wave engine event loop (WP07)
14. On exit: generate report (WP11), persist final state (WP09), cleanup (WP10)

## Subtasks & Detailed Guidance

### Subtask T063 – `kasmos launch <feature>` Command

**Purpose**: The primary entry point that orchestrates an entire agent session from start to finish.

**Steps**:

1. Update `crates/kasmos/src/main.rs`:
   ```rust
   use clap::{Parser, Subcommand};

   #[derive(Parser)]
   #[command(name = "kasmos", about = "Zellij-based agent orchestrator")]
   struct Cli {
       #[command(subcommand)]
       command: Commands,
   }

   #[derive(Subcommand)]
   enum Commands {
       /// Launch an orchestration run for a feature
       Launch {
           /// Feature name or number (e.g., "001" or "001-zellij-agent-orchestrator")
           feature: String,
           /// Progression mode: continuous or wave-gated
           #[arg(long, default_value = "continuous")]
           mode: String,
           /// Maximum concurrent agent panes
           #[arg(long, default_value = "8")]
           max_panes: usize,
       },
       /// Show orchestration status
       Status {
           /// Feature name (optional — shows all if omitted)
           feature: Option<String>,
       },
       /// Attach to an existing orchestration session
       Attach {
           /// Feature name
           feature: String,
       },
       /// Stop a running orchestration
       Stop {
           /// Feature name (optional — stops all if omitted)
           feature: Option<String>,
       },
   }

   #[tokio::main]
   async fn main() -> anyhow::Result<()> {
       let cli = Cli::parse();

       match cli.command {
           Commands::Launch { feature, mode, max_panes } => {
               launch::run(feature, mode, max_panes).await?;
           }
           Commands::Status { feature } => {
               status::run(feature).await?;
           }
           Commands::Attach { feature } => {
               attach::run(feature).await?;
           }
           Commands::Stop { feature } => {
               stop::run(feature).await?;
           }
       }

       Ok(())
   }
   ```

2. Create `crates/kasmos/src/launch.rs` with the full wiring sequence (14 steps from Context above)
3. Acquire run lock (`.kasmos/run.lock`) at start, release on exit
4. Handle lock contention: if lock exists and process is alive, reject with clear error

**Files**:
- `crates/kasmos/src/main.rs` (rewrite, ~50 lines)
- `crates/kasmos/src/launch.rs` (new, ~150 lines)

### Subtask T064 – `kasmos status [<feature>]` Command

**Purpose**: Display the current orchestration state by reading the persisted state file.

**Steps**:

1. Create `crates/kasmos/src/status.rs`:
   ```rust
   pub async fn run(feature: Option<String>) -> Result<()> {
       // 1. Find feature directory
       let feature_dir = resolve_feature_dir(feature)?;
       let kasmos_dir = feature_dir.join(".kasmos");

       // 2. Load state
       let persister = StatePersister::new(&kasmos_dir);
       match persister.load()? {
           Some(run) => {
               // 3. Format and display
               println!("{}", CommandHandler::format_status(&run));
           }
           None => {
               println!("No active orchestration found for this feature.");
               println!("Use 'kasmos launch <feature>' to start one.");
           }
       }

       Ok(())
   }
   ```

2. If no feature specified, look for any `.kasmos/state.json` in common locations
3. Exit code: 0 if status shown, 1 if no state found

**Files**:
- `crates/kasmos/src/status.rs` (new, ~40 lines)

**Parallel**: Yes — can be developed alongside T063 once the wiring pattern is established.

### Subtask T065 – `kasmos attach <feature>` Command

**Purpose**: Reattach to an existing Zellij session with state reconciliation.

**Steps**:

1. Create `crates/kasmos/src/attach.rs`:
   ```rust
   pub async fn run(feature: String) -> Result<()> {
       init_logging()?;
       let config = Config::load()?;

       // 1. Resolve feature directory
       let feature_dir = resolve_feature_dir(Some(feature.clone()))?;
       let kasmos_dir = feature_dir.join(".kasmos");

       // 2. Load persisted state
       let persister = StatePersister::new(&kasmos_dir);
       let mut run = persister.load()?
           .ok_or_else(|| anyhow::anyhow!("No state found — nothing to attach to"))?;

       // 3. Check Zellij session exists
       let cli = RealZellijCli::new(&config.zellij_binary);
       let session_name = format!("kasmos-{}", feature);
       if !cli.session_exists(&session_name).await? {
           anyhow::bail!("Zellij session '{}' not found. The orchestration may have ended.", session_name);
       }

       // 4. Reconcile state
       let mut session = SessionManager::new(Box::new(cli), &feature);
       let corrections = persister.reconcile(&mut run, &session).await?;
       if !corrections.is_empty() {
           println!("State reconciled: {} corrections applied", corrections.len());
           for c in &corrections {
               println!("  {} {:?} → {:?}: {}", c.wp_id, c.from, c.to, c.reason);
           }
       }

       // 5. Attach to Zellij session (interactive)
       // This replaces the current process with the Zellij attach
       let status = std::process::Command::new(&config.zellij_binary)
           .args(["attach", &session_name])
           .status()?;

       std::process::exit(status.code().unwrap_or(1));
   }
   ```

**Files**:
- `crates/kasmos/src/attach.rs` (new, ~50 lines)

**Parallel**: Yes.

### Subtask T066 – `kasmos stop [<feature>]` Command

**Purpose**: Stop a running orchestration gracefully.

**Steps**:

1. Create `crates/kasmos/src/stop.rs`:
   ```rust
   pub async fn run(feature: Option<String>) -> Result<()> {
       // 1. Resolve feature
       let feature_dir = resolve_feature_dir(feature)?;
       let kasmos_dir = feature_dir.join(".kasmos");

       // 2. Send abort command via FIFO
       let pipe_path = kasmos_dir.join("cmd.pipe");
       if pipe_path.exists() {
           std::fs::write(&pipe_path, "abort\n")?;
           println!("Abort command sent. Waiting for graceful shutdown...");
       } else {
           // No FIFO — try killing the session directly
           let config = Config::default();
           let cli = RealZellijCli::new(&config.zellij_binary);
           let session_name = format!("kasmos-{}", feature_dir.file_name()
               .unwrap_or_default().to_string_lossy());
           cli.kill_session(&session_name).await?;
           println!("Session killed.");
       }

       Ok(())
   }
   ```

2. Prefer FIFO abort (allows graceful shutdown with state persistence)
3. Fallback to session kill if FIFO not available

**Files**:
- `crates/kasmos/src/stop.rs` (new, ~30 lines)

**Parallel**: Yes.

### Subtask T067 – Wire All Modules with anyhow Error Propagation

**Purpose**: Ensure all modules are connected with proper error handling, tracing spans, and clean error messages for the user.

**Steps**:

1. Create `crates/kasmos/src/lib.rs` with all module declarations:
   ```rust
   pub mod types;
   pub mod config;
   pub mod error;
   pub mod state_machine;
   pub mod logging;
   pub mod parser;
   pub mod graph;
   pub mod layout;
   pub mod prompt;
   pub mod zellij;
   pub mod session;
   pub mod detector;
   pub mod engine;
   pub mod commands;
   pub mod command_handlers;
   pub mod persistence;
   pub mod health;
   pub mod shutdown;
   pub mod launch;
   pub mod status;
   pub mod attach;
   pub mod stop;
   pub mod report;
   ```

2. Add tracing spans to major operations:
   ```rust
   #[tracing::instrument(skip_all, fields(feature = %feature))]
   pub async fn run(feature: String, mode: String, max_panes: usize) -> Result<()> { ... }
   ```

3. Ensure all `?` operators produce user-friendly errors (use `.context("...")` from anyhow)

4. Add a `resolve_feature_dir()` helper used by all commands:
   ```rust
   fn resolve_feature_dir(feature: Option<String>) -> Result<PathBuf> {
       // Search for feature in kitty-specs/
       // Accept "001" or "001-zellij-agent-orchestrator"
       // Return absolute path
   }
   ```

**Files**:
- `crates/kasmos/src/lib.rs` (rewrite, ~30 lines)
- `crates/kasmos/src/launch.rs` (additions for tracing spans)

### Subtask T068 – End-to-End Integration Test

**Purpose**: Verify the full orchestration lifecycle with a mock Zellij binary.

**Steps**:

1. Create `crates/kasmos/tests/integration.rs`:
   ```rust
   use tempfile::TempDir;

   /// Create a mock zellij binary (shell script) that simulates session creation
   /// and pane listing without a real Zellij installation.
   fn create_mock_zellij(dir: &Path) -> PathBuf {
       let mock_path = dir.join("zellij");
       let script = r#"#!/bin/bash
   case "$1" in
       "--layout") echo "Session created" ;;
       "list-sessions") echo "kasmos-test-feature" ;;
       "action")
           case "$2" in
               "list-panes") echo -e "1\tcontroller\t50\t100\n2\tWP01\t25\t50\n3\tWP02\t25\t50" ;;
               *) echo "OK" ;;
           esac
           ;;
       "kill-session") echo "Killed" ;;
       *) echo "Unknown command" ;;
   esac
   "#;
       std::fs::write(&mock_path, script).unwrap();
       #[cfg(unix)]
       {
           use std::os::unix::fs::PermissionsExt;
           std::fs::set_permissions(&mock_path, std::fs::Permissions::from_mode(0o755)).unwrap();
       }
       mock_path
   }

   /// Create a minimal feature directory with test WP files.
   fn create_test_feature(dir: &Path) -> PathBuf {
       let feature_dir = dir.join("kitty-specs/test-feature");
       let tasks_dir = feature_dir.join("tasks");
       std::fs::create_dir_all(&tasks_dir).unwrap();

       // Create WP01 task file
       let wp01 = r#"---
   work_package_id: "WP01"
   title: "Test WP 1"
   lane: "planned"
   dependencies: []
   subtasks: ["T001"]
   ---
   # WP01
   "#;
       std::fs::write(tasks_dir.join("WP01-test.md"), wp01).unwrap();

       // Create WP02 task file (depends on WP01)
       let wp02 = r#"---
   work_package_id: "WP02"
   title: "Test WP 2"
   lane: "planned"
   dependencies: ["WP01"]
   subtasks: ["T002"]
   ---
   # WP02
   "#;
       std::fs::write(tasks_dir.join("WP02-test.md"), wp02).unwrap();

       feature_dir
   }

   #[tokio::test]
   async fn test_full_lifecycle() {
       let tmp = TempDir::new().unwrap();
       let mock_zellij = create_mock_zellij(tmp.path());
       let feature_dir = create_test_feature(tmp.path());

       // Test: parse specs, build graph, compute waves
       let feature = FeatureDir::scan(&feature_dir).unwrap();
       assert_eq!(feature.wp_files.len(), 2);

       // Test: build dependency graph
       let frontmatters: Vec<WPFrontmatter> = feature.wp_files.iter()
           .map(|f| parse_frontmatter(f).unwrap())
           .collect();
       let graph = DependencyGraph::build(&frontmatters).unwrap();
       let waves = graph.compute_waves().unwrap();
       assert_eq!(waves.len(), 2); // Wave 0: WP01, Wave 1: WP02

       // Further integration tests...
   }
   ```

2. Test the full lifecycle: scan → parse → graph → layout → session → detect → complete
3. Use the mock ZellijCli trait implementation for unit-testable integration

**Files**:
- `crates/kasmos/tests/integration.rs` (new, ~120 lines)

### Subtask T069 – Generate Post-Run Summary Report

**Purpose**: Generate a markdown report at `.kasmos/report.md` with per-WP durations, wave timings, completion methods, and failure statistics.

**Steps**:

1. Create `crates/kasmos/src/report.rs`:
   ```rust
   pub struct ReportGenerator;

   impl ReportGenerator {
       /// Generate a post-run summary report.
       pub fn generate(run: &OrchestrationRun, kasmos_dir: &Path) -> Result<PathBuf> {
           let mut report = String::new();

           // Header
           report.push_str(&format!("# Orchestration Report: {}\n\n", run.feature));
           report.push_str(&format!("**Run ID**: {}\n", run.id));
           report.push_str(&format!("**Mode**: {:?}\n", run.mode));
           report.push_str(&format!("**Status**: {:?}\n", run.state));
           if let (Some(start), Some(end)) = (&run.started_at, &run.completed_at) {
               let duration = end.duration_since(*start).unwrap_or_default();
               report.push_str(&format!("**Duration**: {}\n", format_duration(duration)));
           }
           report.push_str("\n");

           // Wave Summary
           report.push_str("## Wave Summary\n\n");
           report.push_str("| Wave | WPs | Status | Duration |\n");
           report.push_str("|------|-----|--------|----------|\n");
           for wave in &run.waves {
               let wp_ids = wave.wp_ids.join(", ");
               report.push_str(&format!(
                   "| {} | {} | {:?} | - |\n",
                   wave.index, wp_ids, wave.state
               ));
           }
           report.push_str("\n");

           // WP Details
           report.push_str("## Work Package Details\n\n");
           report.push_str("| WP | Title | Status | Duration | Method | Failures |\n");
           report.push_str("|----|-------|--------|----------|--------|----------|\n");
           for wp in &run.work_packages {
               let duration = match (&wp.started_at, &wp.completed_at) {
                   (Some(s), Some(e)) => format_duration(e.duration_since(*s).unwrap_or_default()),
                   _ => "-".to_string(),
               };
               let method = wp.completion_method.as_ref()
                   .map(|m| format!("{:?}", m))
                   .unwrap_or("-".to_string());
               report.push_str(&format!(
                   "| {} | {} | {:?} | {} | {} | {} |\n",
                   wp.id, wp.title, wp.state, duration, method, wp.failure_count
               ));
           }
           report.push_str("\n");

           // Statistics
           let total = run.work_packages.len();
           let completed = run.work_packages.iter().filter(|w| matches!(w.state, WPState::Completed)).count();
           let failed = run.work_packages.iter().filter(|w| matches!(w.state, WPState::Failed)).count();
           let auto_detected = run.work_packages.iter()
               .filter(|w| matches!(w.completion_method, Some(CompletionMethod::AutoDetected)))
               .count();

           report.push_str("## Statistics\n\n");
           report.push_str(&format!("- **Total WPs**: {}\n", total));
           report.push_str(&format!("- **Completed**: {}\n", completed));
           report.push_str(&format!("- **Failed**: {}\n", failed));
           report.push_str(&format!("- **Auto-detected completions**: {}\n", auto_detected));
           report.push_str(&format!("- **Manual completions**: {}\n", completed - auto_detected));
           report.push_str(&format!("- **Total failures**: {}\n",
               run.work_packages.iter().map(|w| w.failure_count).sum::<u32>()));

           // Write report
           let report_path = kasmos_dir.join("report.md");
           std::fs::write(&report_path, &report)?;
           tracing::info!(path = %report_path.display(), "Post-run report generated");

           Ok(report_path)
       }
   }

   fn format_duration(d: std::time::Duration) -> String {
       let secs = d.as_secs();
       if secs < 60 {
           format!("{}s", secs)
       } else if secs < 3600 {
           format!("{}m {}s", secs / 60, secs % 60)
       } else {
           format!("{}h {}m", secs / 3600, (secs % 3600) / 60)
       }
   }
   ```

2. Report is generated at the end of the orchestration (after wave engine exits, before cleanup)
3. Report is NOT deleted by artifact cleanup (T062) — it's a persistent output

**Files**:
- `crates/kasmos/src/report.rs` (new, ~100 lines)

## Test Strategy

- Unit test: CLI argument parsing for all 4 subcommands
- Unit test: resolve_feature_dir with various input formats ("001", "001-slug", full path)
- Unit test: report generation with mock OrchestrationRun → valid markdown
- Unit test: format_duration for seconds, minutes, hours
- Integration test: full lifecycle with mock Zellij (T068)
- Integration test: `kasmos status` with existing state file
- Integration test: `kasmos stop` sends abort command via FIFO

## Risks & Mitigations

| Risk | Impact | Mitigation |
|------|--------|------------|
| Integration reveals interface mismatches | High | Trait interfaces defined in WP01/WP05, implement against them |
| Mock Zellij insufficient for real scenarios | Medium | Cover core commands (list-sessions, list-panes, create-session, kill-session) |
| Module wiring order wrong | Medium | Follow the 14-step sequence exactly, with tracing at each step |
| Error messages not user-friendly | Medium | Add `.context(...)` to all `?` operators in launch/status/attach/stop |

## Review Guidance

**Acceptance Checkpoints**:
- [ ] `kasmos launch` wires all 14 modules in correct order
- [ ] `kasmos status` reads and displays state correctly
- [ ] `kasmos attach` reconciles state before reattaching
- [ ] `kasmos stop` sends abort via FIFO (or kills session as fallback)
- [ ] All modules connected in lib.rs
- [ ] Integration test passes with mock Zellij
- [ ] Post-run report contains per-WP stats, wave summary, and totals
- [ ] Error messages are user-friendly (no raw stack traces)
- [ ] `cargo build` and `cargo test` pass

## Activity Log

2026-02-09T00:00:00Z – system – lane=planned – Prompt created.

### Updating Lane Status

To update this work package's lane, either:
1. Edit the `lane` field in the frontmatter directly, or
2. Run: `spec-kitty agent tasks move-task WP11 --to <lane>`

Valid lanes: `planned`, `doing`, `for_review`, `done`

### File Structure

This file lives in `tasks/` (flat directory). Lane status is tracked ONLY in the `lane:` frontmatter field, NOT by directory location.
