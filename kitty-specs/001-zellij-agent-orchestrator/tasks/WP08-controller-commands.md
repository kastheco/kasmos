---
work_package_id: WP08
title: Controller Commands
lane: done
dependencies:
- WP01
subtasks: [T045, T046, T047, T048, T049, T050, T051, T052, T053]
phase: Phase 4 - Control
assignee: controller-wp08
agent: controller-wp08
shell_pid: ''
review_status: approved
reviewed_by: reviewer
history:
- timestamp: '2026-02-09T00:00:00Z'
  lane: planned
  agent: system
  shell_pid: ''
  action: Prompt generated via /spec-kitty.tasks
- timestamp: '2026-02-09T00:00:00Z'
  lane: for_review
  agent: controller-wp08
  shell_pid: ''
  action: Implementation completed - added commands.rs with FIFO reader and command parser, command_handlers.rs with EngineAction dispatch and status formatting, wired modules in lib.rs, updated Cargo.toml with tokio and nix dependencies, all 75 tests passing
- timestamp: '2026-02-09T00:00:00Z'
  lane: done
  agent: reviewer
  shell_pid: ''
  action: Final review approved - moved to done
---

# Work Package Prompt: WP08 – Controller Commands

## IMPORTANT: Review Feedback Status

Before starting implementation, check the **Review Feedback** section below.
- If empty → This is fresh work. Proceed with implementation.
- If populated → This WP was previously reviewed and needs changes. Address ALL feedback items before marking as done.

## Review Feedback

*(Empty — no review feedback yet)*

## Dependency Rebase Guidance

This WP depends on **WP01** (core types) and **WP05** (session manager for pane operations).

**Implementation command**:
```bash
spec-kitty implement WP08 --base WP05
```

## Objectives & Success Criteria

**Objective**: Implement a FIFO-based command input system that allows the operator to control the orchestration from the controller pane. Commands are written to `.kasmos/cmd.pipe` and parsed by a reader task that dispatches actions to the wave engine and session manager.

**Success Criteria**:
1. FIFO is created at `.kasmos/cmd.pipe` and a reader task processes commands
2. Command grammar supports: restart, pause, status, focus, zoom, abort, force-advance, retry
3. Each command validates its arguments and produces clear error messages for invalid input
4. Commands interact correctly with session manager and wave engine state
5. Invalid commands produce helpful usage messages
6. FIFO reader doesn't block the event loop (non-blocking I/O)

## Context & Constraints

- **Crate location**: `crates/kasmos/`
- **Dependencies**: `nix` crate for mkfifo, `tokio` for async I/O
- **FIFO location**: `.kasmos/cmd.pipe` (created during init, cleaned up on exit)
- **Reference**: [plan.md](../plan.md) WP08 section; [spec.md](../spec.md) FR-007, FR-012, FR-013
- **Constraint**: FIFO must be opened with O_NONBLOCK to avoid blocking when no writer is connected
- **Constraint**: Commands are single-line strings, one per line
- **Constraint**: Command output goes to the controller pane via session manager's write_to_pane

**Command Grammar**:
```
restart <WP_ID>       — Restart a failed/crashed WP pane
pause <WP_ID>         — Pause a running WP
resume <WP_ID>        — Resume a paused WP
status                — Show current orchestration status
focus <WP_ID>         — Focus a specific pane
zoom <WP_ID>          — Focus and zoom a pane
abort                 — Graceful shutdown of entire orchestration
advance               — Confirm wave advancement (wave-gated mode)
force-advance <WP_ID> — Skip failed WP, unblock dependents
retry <WP_ID>         — Re-run failed WP from scratch
help                  — Show available commands
```

## Subtasks & Detailed Guidance

### Subtask T045 – FIFO Command Input

**Purpose**: Create the named pipe and spawn an async reader task that reads command lines and dispatches them.

**Steps**:

1. Create `crates/kasmos/src/commands.rs`:
   ```rust
   use nix::sys::stat::Mode;
   use nix::unistd::mkfifo;
   use tokio::io::{AsyncBufReadExt, BufReader};
   use tokio::sync::mpsc;
   use std::path::{Path, PathBuf};

   pub struct CommandReader {
       pipe_path: PathBuf,
       command_tx: mpsc::Sender<ControllerCommand>,
   }

   #[derive(Debug, Clone)]
   pub enum ControllerCommand {
       Restart { wp_id: String },
       Pause { wp_id: String },
       Resume { wp_id: String },
       Status,
       Focus { wp_id: String },
       Zoom { wp_id: String },
       Abort,
       Advance,
       ForceAdvance { wp_id: String },
       Retry { wp_id: String },
       Help,
       Unknown { input: String },
   }

   impl CommandReader {
       pub fn new(kasmos_dir: &Path, command_tx: mpsc::Sender<ControllerCommand>) -> Result<Self> {
           let pipe_path = kasmos_dir.join("cmd.pipe");

           // Create FIFO if it doesn't exist
           if !pipe_path.exists() {
               mkfifo(&pipe_path, Mode::S_IRUSR | Mode::S_IWUSR)
                   .map_err(|e| anyhow::anyhow!("Failed to create FIFO: {}", e))?;
               tracing::info!(path = %pipe_path.display(), "Command FIFO created");
           }

           Ok(Self { pipe_path, command_tx })
       }

       /// Spawn the FIFO reader as a tokio task.
       pub async fn start(self) -> Result<tokio::task::JoinHandle<()>> {
           let handle = tokio::spawn(async move {
               loop {
                   // Open FIFO for reading (reopens after each writer disconnects)
                   match tokio::fs::File::open(&self.pipe_path).await {
                       Ok(file) => {
                           let reader = BufReader::new(file);
                           let mut lines = reader.lines();

                           while let Ok(Some(line)) = lines.next_line().await {
                               let line = line.trim().to_string();
                               if line.is_empty() { continue; }

                               match Self::parse_command(&line) {
                                   Ok(cmd) => {
                                       tracing::info!(command = ?cmd, "Received command");
                                       if self.command_tx.send(cmd).await.is_err() {
                                           tracing::error!("Command channel closed");
                                           return;
                                       }
                                   }
                                   Err(e) => {
                                       tracing::warn!(input = %line, error = %e, "Invalid command");
                                   }
                               }
                           }
                       }
                       Err(e) => {
                           tracing::error!(error = %e, "Failed to open FIFO");
                           tokio::time::sleep(std::time::Duration::from_secs(1)).await;
                       }
                   }
               }
           });

           Ok(handle)
       }

       /// Cleanup: remove the FIFO.
       pub fn cleanup(&self) -> Result<()> {
           if self.pipe_path.exists() {
               std::fs::remove_file(&self.pipe_path)?;
           }
           Ok(())
       }
   }
   ```

**Files**:
- `crates/kasmos/src/commands.rs` (new, ~80 lines)

### Subtask T046 – Command Grammar Parsing

**Purpose**: Parse command strings into structured ControllerCommand variants with validation.

**Steps**:

1. Add parser:
   ```rust
   impl CommandReader {
       fn parse_command(input: &str) -> Result<ControllerCommand> {
           let parts: Vec<&str> = input.split_whitespace().collect();
           if parts.is_empty() {
               return Err(anyhow::anyhow!("Empty command"));
           }

           match parts[0].to_lowercase().as_str() {
               "restart" => {
                   let wp_id = parts.get(1)
                       .ok_or_else(|| anyhow::anyhow!("Usage: restart <WP_ID>"))?;
                   Ok(ControllerCommand::Restart { wp_id: wp_id.to_string() })
               }
               "pause" => {
                   let wp_id = parts.get(1)
                       .ok_or_else(|| anyhow::anyhow!("Usage: pause <WP_ID>"))?;
                   Ok(ControllerCommand::Pause { wp_id: wp_id.to_string() })
               }
               "resume" => {
                   let wp_id = parts.get(1)
                       .ok_or_else(|| anyhow::anyhow!("Usage: resume <WP_ID>"))?;
                   Ok(ControllerCommand::Resume { wp_id: wp_id.to_string() })
               }
               "status" => Ok(ControllerCommand::Status),
               "focus" => {
                   let wp_id = parts.get(1)
                       .ok_or_else(|| anyhow::anyhow!("Usage: focus <WP_ID>"))?;
                   Ok(ControllerCommand::Focus { wp_id: wp_id.to_string() })
               }
               "zoom" => {
                   let wp_id = parts.get(1)
                       .ok_or_else(|| anyhow::anyhow!("Usage: zoom <WP_ID>"))?;
                   Ok(ControllerCommand::Zoom { wp_id: wp_id.to_string() })
               }
               "abort" => Ok(ControllerCommand::Abort),
               "advance" => Ok(ControllerCommand::Advance),
               "force-advance" => {
                   let wp_id = parts.get(1)
                       .ok_or_else(|| anyhow::anyhow!("Usage: force-advance <WP_ID>"))?;
                   Ok(ControllerCommand::ForceAdvance { wp_id: wp_id.to_string() })
               }
               "retry" => {
                   let wp_id = parts.get(1)
                       .ok_or_else(|| anyhow::anyhow!("Usage: retry <WP_ID>"))?;
                   Ok(ControllerCommand::Retry { wp_id: wp_id.to_string() })
               }
               "help" => Ok(ControllerCommand::Help),
               _ => Ok(ControllerCommand::Unknown { input: input.to_string() }),
           }
       }
   }
   ```

**Files**:
- `crates/kasmos/src/commands.rs` (continued, ~50 lines)

### Subtask T047 – Restart Command [P]

**Purpose**: Restart a failed or crashed WP pane with the same prompt configuration.

**Steps**:

1. Create `crates/kasmos/src/command_handlers.rs`:
   ```rust
   pub struct CommandHandler {
       session: Arc<Mutex<SessionManager>>,
       engine_tx: mpsc::Sender<EngineAction>,
   }

   pub enum EngineAction {
       Restart(String),
       Pause(String),
       Resume(String),
       ForceAdvance(String),
       Retry(String),
       Advance,
       Abort,
   }

   impl CommandHandler {
       pub async fn handle(&self, cmd: ControllerCommand) -> Result<String> {
           match cmd {
               ControllerCommand::Restart { wp_id } => {
                   self.engine_tx.send(EngineAction::Restart(wp_id.clone())).await?;
                   Ok(format!("Restarting {}...", wp_id))
               }
               // ... other handlers
           }
       }
   }
   ```

**Files**:
- `crates/kasmos/src/command_handlers.rs` (new, ~30 lines for this subtask)

**Parallel**: Yes — independent once T045-T046 exist.

### Subtask T048 – Pause Command [P]

**Purpose**: Pause a running WP's pane (stop accepting input).

**Steps**: Implement handler that transitions WP state to Paused and notifies the engine.

**Files**: `crates/kasmos/src/command_handlers.rs` (continued, ~15 lines)

**Parallel**: Yes.

### Subtask T049 – Status Command [P]

**Purpose**: Display current orchestration status as a formatted table.

**Steps**:

1. Add status formatter:
   ```rust
   impl CommandHandler {
       fn format_status(run: &OrchestrationRun) -> String {
           let mut out = String::new();
           out.push_str(&format!("\n[kasmos] Orchestration Status: {}\n", run.feature));
           out.push_str(&format!("Mode: {:?} | State: {:?}\n", run.mode, run.state));
           out.push_str("─".repeat(60).as_str());
           out.push('\n');
           out.push_str(&format!("{:<8} {:<30} {:<12} {:<10}\n", "WP", "Title", "State", "Duration"));
           out.push_str("─".repeat(60).as_str());
           out.push('\n');

           for wp in &run.work_packages {
               let duration = match (&wp.started_at, &wp.completed_at) {
                   (Some(start), Some(end)) => format_duration(end.duration_since(*start).unwrap_or_default()),
                   (Some(start), None) => format!("{}...", format_duration(start.elapsed().unwrap_or_default())),
                   _ => "-".to_string(),
               };
               out.push_str(&format!(
                   "{:<8} {:<30} {:<12} {:<10}\n",
                   wp.id, wp.title, format!("{:?}", wp.state), duration
               ));
           }

           out.push_str("─".repeat(60).as_str());
           out.push('\n');
           out
       }
   }
   ```

**Files**: `crates/kasmos/src/command_handlers.rs` (continued, ~30 lines)

**Parallel**: Yes.

### Subtask T050 – Focus/Zoom Commands [P]

**Purpose**: Focus or zoom a specific WP pane by ID.

**Steps**: Delegate to session manager's focus_pane and focus_and_zoom methods.

**Files**: `crates/kasmos/src/command_handlers.rs` (continued, ~15 lines)

**Parallel**: Yes.

### Subtask T051 – Abort Command [P]

**Purpose**: Graceful shutdown of the entire orchestration.

**Steps**: Send EngineAction::Abort, which triggers the graceful shutdown sequence (WP10).

**Files**: `crates/kasmos/src/command_handlers.rs` (continued, ~10 lines)

**Parallel**: Yes.

### Subtask T052 – Force-Advance Command [P]

**Purpose**: Skip a failed WP and unblock its dependents, treating it as manually completed.

**Steps**: Send EngineAction::ForceAdvance to wave engine, which calls force_advance() (WP07).

**Files**: `crates/kasmos/src/command_handlers.rs` (continued, ~10 lines)

**Parallel**: Yes.

### Subtask T053 – Retry Command [P]

**Purpose**: Re-run a failed WP from scratch (reset state to Pending, relaunch).

**Steps**: Send EngineAction::Retry, which resets WP state and triggers relaunch.

**Files**: `crates/kasmos/src/command_handlers.rs` (continued, ~10 lines)

**Parallel**: Yes.

## Test Strategy

- Unit test: parse each command variant (restart WP01, status, abort, etc.)
- Unit test: parse invalid commands → Unknown variant
- Unit test: parse commands missing required args → error message
- Unit test: format_status produces readable table
- Integration test: write to FIFO, verify command received on mpsc channel

## Risks & Mitigations

| Risk | Impact | Mitigation |
|------|--------|------------|
| FIFO blocks when no reader | Medium | Open with O_NONBLOCK, async I/O via tokio |
| Command during state transition | Medium | Commands go through mpsc → engine processes sequentially |
| FIFO permission issues | Low | Create with user-only permissions (0o600) |
| Operator doesn't know available commands | Low | help command + initial message on controller pane |

## Review Guidance

**Acceptance Checkpoints**:
- [ ] FIFO is created and cleaned up properly
- [ ] All command variants parse correctly
- [ ] Invalid commands produce helpful error messages
- [ ] Commands dispatch to correct engine actions
- [ ] Status formatting is readable and accurate
- [ ] FIFO reader doesn't block the event loop

## Activity Log

2026-02-09T00:00:00Z – system – lane=planned – Prompt created.
2026-02-09T00:00:00Z – reviewer – lane=done – Final review approved and moved to done.

### Updating Lane Status

To update this work package's lane, either:
1. Edit the `lane` field in the frontmatter directly, or
2. Run: `spec-kitty agent tasks move-task WP08 --to <lane>`

Valid lanes: `planned`, `doing`, `for_review`, `done`

### File Structure

This file lives in `tasks/` (flat directory). Lane status is tracked ONLY in the `lane:` frontmatter field, NOT by directory location.
