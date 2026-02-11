# Data Model: Ratatui TUI Controller Panel

## New Types

### App (TUI root state)

```rust
pub struct App {
    /// Latest orchestration state from engine (via watch channel)
    pub run: OrchestrationRun,
    /// Currently active tab
    pub active_tab: Tab,
    /// Active notifications requiring operator attention
    pub notifications: Vec<Notification>,
    /// Dashboard tab state
    pub dashboard: DashboardState,
    /// Review tab state
    pub review: ReviewState,
    /// Logs tab state
    pub logs: LogsState,
    /// Channel to send commands to engine
    pub action_tx: mpsc::Sender<EngineAction>,
    /// Session controller for focus/zoom
    pub session: Arc<SessionManager>,
    /// Exit flag
    pub should_quit: bool,
}
```

### Tab

```rust
pub enum Tab {
    Dashboard,  // Index 0, key '1'
    Review,     // Index 1, key '2'
    Logs,       // Index 2, key '3'
}
```

### Notification

```rust
pub struct Notification {
    pub id: u64,
    pub kind: NotificationKind,
    pub wp_id: String,
    pub message: Option<String>,
    pub created_at: Instant,
}

pub enum NotificationKind {
    /// WP reached for_review lane
    ReviewPending,
    /// WP entered Failed state
    Failure,
    /// Agent signaled it needs operator input
    InputNeeded,
    /// Automated tiered review failed for a WP
    ReviewAutomationError,
}
```

### ReviewAutomationConfig

```rust
pub struct ReviewAutomationConfig {
    pub enabled: bool,
    pub mode: ReviewTriggerMode,             // Slash | Prompt
    pub slash_command: String,               // default: "/kas:verify"
    pub fallback_to_prompt: bool,
    pub model: String,                       // default: "openai/gpt-5.3-codex"
    pub reasoning: ReasoningLevel,           // default: High
    pub timeout_seconds: u64,
    pub policy: ReviewAutomationPolicy,      // ManualOnly | AutoThenManualApprove | AutoAndMarkDone
}

pub enum ReviewTriggerMode {
    Slash,
    Prompt,
}

pub enum ReasoningLevel {
    Low,
    Medium,
    High,
}

pub enum ReviewAutomationPolicy {
    ManualOnly,
    AutoThenManualApprove,
    AutoAndMarkDone,
}
```

### ReviewResult

```rust
pub struct ReviewResult {
    pub wp_id: String,
    pub mode: ReviewTriggerMode,
    pub command: Option<String>,
    pub model: Option<String>,
    pub reasoning: Option<ReasoningLevel>,
    pub status: ReviewRunStatus,             // Pass | Fail | Error
    pub summary: String,
    pub findings: Vec<String>,
    pub started_at: SystemTime,
    pub completed_at: Option<SystemTime>,
}

pub enum ReviewRunStatus {
    Pass,
    Fail,
    Error,
}
```

### DashboardState

```rust
pub struct DashboardState {
    /// Which lane column is focused (0=planned, 1=doing, 2=for_review, 3=done)
    pub focused_lane: usize,
    /// Selected WP index within the focused lane
    pub selected_index: usize,
    /// Vertical scroll offset per lane
    pub scroll_offsets: [usize; 4],
}
```

### ReviewState

```rust
pub struct ReviewState {
    /// Index of selected review item in the for_review list
    pub selected_index: usize,
    /// Scroll offset for review detail pane
    pub detail_scroll: usize,
}
```

### LogsState

```rust
pub struct LogsState {
    /// All log entries
    pub entries: Vec<LogEntry>,
    /// Active filter text (empty = show all)
    pub filter: String,
    /// Whether filter input is active
    pub filter_active: bool,
    /// Scroll offset
    pub scroll_offset: usize,
    /// Auto-scroll to bottom on new entries
    pub auto_scroll: bool,
}

pub struct LogEntry {
    pub timestamp: SystemTime,
    pub level: LogLevel,
    pub wp_id: Option<String>,
    pub message: String,
}

pub enum LogLevel {
    Info,
    Warn,
    Error,
}
```

## Modified Types

### OrchestrationRun (existing, no changes needed)

Already has all fields the TUI needs:
- `work_packages: Vec<WorkPackage>` — kanban data
- `waves: Vec<Wave>` — wave grouping
- `state: RunState` — overall status
- `mode: ProgressionMode` — wave-gated vs continuous
- `config: Config` — display settings

### EngineAction (modified in WP02)

Review actions extend existing controls:
- Restart, Pause, Resume, ForceAdvance, Retry, Advance, Abort
- Approve(String), Reject { wp_id, relaunch }

### WPState (modified in WP02)

`ForReview` variant added as a first-class state (WP02 T007). Dashboard renders WPs grouped by state:
- `Pending` → planned lane
- `Active` → doing lane
- `Paused` → doing lane (with paused badge)
- `Failed` → doing lane (with failed badge)
- `ForReview` → for_review lane
- `Completed` → done lane

The task file `lane:` frontmatter field still exists for spec-kitty tracking, but the TUI uses `WPState::ForReview` as its source of truth.

## Relationships

```
App ──has──▶ OrchestrationRun (from engine, read-only)
App ──has──▶ Vec<Notification> (derived from state diffs)
App ──has──▶ DashboardState, ReviewState, LogsState (UI state)
App ──sends──▶ EngineAction (via mpsc)
App ──calls──▶ SessionManager (focus/zoom panes)
App ──reads──▶ ReviewResult (for Review tab context)

Notification ──references──▶ WorkPackage.id
DashboardState ──indexes into──▶ OrchestrationRun.work_packages (grouped by lane)
ReviewState ──indexes into──▶ OrchestrationRun.work_packages (filtered to for_review)
LogsState ──populated by──▶ state transition diffs + engine events
ReviewResult ──references──▶ WorkPackage.id
```
