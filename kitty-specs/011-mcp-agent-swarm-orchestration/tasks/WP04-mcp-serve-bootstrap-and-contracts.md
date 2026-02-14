---
work_package_id: WP04
title: MCP Serve Bootstrap and Contract Wiring
lane: "for_review"
dependencies: [WP02]
base_branch: 011-mcp-agent-swarm-orchestration-WP02
base_commit: 839ff563e7dfa7894ce4b53b37f439478bf887a6
created_at: '2026-02-14T22:27:41.958224+00:00'
subtasks:
- T021
- T022
- T023
- T024
- T025
- T026
phase: Phase 1 - Launch Topology and MCP Runtime Skeleton
assignee: ''
agent: "opencode"
shell_pid: "3957847"
review_status: ''
reviewed_by: ''
history:
- timestamp: '2026-02-14T16:27:48Z'
  lane: planned
  agent: system
  shell_pid: ''
  action: Prompt generated via /spec-kitty.tasks
---

# Work Package Prompt: WP04 - MCP Serve Bootstrap and Contract Wiring

## Important: Review Feedback Status

- **Has review feedback?**: Check the `review_status` field above.

---

## Review Feedback

*[This section is empty initially.]*

---

## Implementation Command

```bash
spec-kitty implement WP04 --base WP02
```

---

## Objectives & Success Criteria

Stand up `kasmos serve` as an RMCP stdio server with all 9 contract-defined tools registered and schema-validated. After this WP:

1. `kasmos serve` starts, reads JSON-RPC from stdin, writes to stdout
2. `tools/list` returns 9 tools matching the contract schema in `contracts/kasmos-serve.json`
3. Tool input/output types are fully typed with `schemars` for JSON Schema generation
4. `list_features` and `infer_feature` are fully functional
5. Remaining 7 tools have validated stubs that accept correct input and return correct error codes
6. Shared server state (worker registry, message cursor, config) is initialized

## Context & Constraints

- **Depends on WP02**: Config system available for loading into serve state
- **Contract**: `kitty-specs/011-mcp-agent-swarm-orchestration/contracts/kasmos-serve.json` defines all 9 tools with exact input/output schemas and error codes
- **Data model**: `kitty-specs/011-mcp-agent-swarm-orchestration/data-model.md` defines entity types
- **rmcp crate**: Provides `#[tool]` proc macro, `ServerHandler` trait, and stdio transport. Uses `schemars` for schema generation.
- **Runtime model**: `kasmos serve` runs as MCP stdio subprocess spawned by manager agent. It does NOT run in its own pane.

## Subtasks & Detailed Guidance

### Subtask T021 - Implement RMCP serve bootstrap with stdio transport

**Purpose**: Create the `kasmos serve` entry point that initializes an RMCP server on stdio transport.

**Steps**:
1. Populate `crates/kasmos/src/serve/mod.rs`:
   ```rust
   use rmcp::transport::io::stdio;
   use rmcp::ServiceExt;

   pub mod registry;
   pub mod messages;
   pub mod audit;
   pub mod lock;
   pub mod tools;

   pub async fn run() -> anyhow::Result<()> {
       // Load config
       let config = crate::config::Config::load()?;
       // Initialize shared state
       let state = KasmosServer::new(config)?;
       // Start stdio transport
       let transport = stdio();
       let server = state.serve(transport).await?;
       // Wait for server to complete
       server.waiting().await?;
       Ok(())
   }
   ```
2. The `KasmosServer` struct implements `ServerHandler` (or uses rmcp's tool registration pattern).
3. Wire `Commands::Serve` in `main.rs` to call `serve::run()`.
4. The server must handle graceful shutdown when stdin closes (manager agent exits).

**Files**: `crates/kasmos/src/serve/mod.rs`, `crates/kasmos/src/main.rs`
**Validation**: `echo '{"jsonrpc":"2.0","method":"tools/list","id":1}' | kasmos serve` returns tool list.

### Subtask T022 - Define shared server state

**Purpose**: Create the shared state containers that all tool handlers access.

**Steps**:
1. Create `crates/kasmos/src/serve/registry.rs`:
   ```rust
   use std::collections::HashMap;
   use std::sync::Arc;
   use tokio::sync::RwLock;

   /// Active worker registry
   pub struct WorkerRegistry {
       workers: HashMap<String, WorkerEntry>,
   }

   /// Entry for a tracked worker
   pub struct WorkerEntry {
       pub wp_id: String,
       pub role: AgentRole,
       pub pane_name: String,
       pub pane_id: Option<String>,
       pub worktree_path: Option<String>,
       pub status: WorkerStatus,
       pub spawned_at: chrono::DateTime<chrono::Utc>,
       pub updated_at: chrono::DateTime<chrono::Utc>,
       pub last_event: Option<MessageEvent>,
   }

   pub enum AgentRole { Planner, Coder, Reviewer, Release }
   pub enum WorkerStatus { Active, Done, Errored, Aborted }
   ```
2. The `KasmosServer` struct holds:
   ```rust
   pub struct KasmosServer {
       pub config: Config,
       pub registry: Arc<RwLock<WorkerRegistry>>,
       pub message_cursor: Arc<RwLock<u64>>,
       pub feature_slug: Option<String>,
       // Lock handler added in WP05
       // Audit writer added in WP06
   }
   ```
3. Derive `schemars::JsonSchema` on all types that appear in tool inputs/outputs.

**Files**: `crates/kasmos/src/serve/registry.rs`, `crates/kasmos/src/serve/mod.rs`
**Validation**: Server state initializes correctly with default values.

### Subtask T023 - Implement typed tool request/response structs

**Purpose**: Create strongly-typed structs for all 9 tool handlers matching the contract schema.

**Steps**:
1. Create `crates/kasmos/src/serve/tools/mod.rs` with submodule declarations for each tool.
2. For each tool, create a file in `crates/kasmos/src/serve/tools/`:
   - `spawn_worker.rs`: `SpawnWorkerInput`, `SpawnWorkerOutput`
   - `despawn_worker.rs`: `DespawnWorkerInput`, `DespawnWorkerOutput`
   - `list_workers.rs`: `ListWorkersInput`, `ListWorkersOutput`
   - `read_messages.rs`: `ReadMessagesInput`, `ReadMessagesOutput`
   - `wait_for_event.rs`: `WaitForEventInput`, `WaitForEventOutput`
   - `workflow_status.rs`: `WorkflowStatusInput`, `WorkflowStatusOutput`
   - `transition_wp.rs`: `TransitionWpInput`, `TransitionWpOutput`
   - `list_features.rs`: `ListFeaturesInput`, `ListFeaturesOutput`
   - `infer_feature.rs`: `InferFeatureInput`, `InferFeatureOutput`
3. Each struct derives `Serialize, Deserialize, JsonSchema` and matches the contract exactly.
4. Use shared enums for `AgentRole`, `WorkerStatus`, `MessageEvent`, etc.
5. Register all tools with the RMCP server using `#[tool]` attributes or manual registration.
6. For tools not yet implemented, return a stub response with `ok: true` and placeholder data, OR return an error with code `INTERNAL_ERROR` and message "Not yet implemented".

**Files**: `crates/kasmos/src/serve/tools/*.rs`
**Validation**: `tools/list` returns all 9 tools. Schema matches contract.

### Subtask T024 - Fully implement list_features tool

**Purpose**: Scan `kitty-specs/` and return feature availability information.

**Steps**:
1. Implement in `crates/kasmos/src/serve/tools/list_features.rs`:
   ```rust
   pub async fn list_features(config: &Config) -> Result<ListFeaturesOutput> {
       let specs_root = PathBuf::from(&config.paths.specs_root);
       let mut features = Vec::new();
       for entry in std::fs::read_dir(&specs_root)? {
           let entry = entry?;
           if entry.path().is_dir() {
               let slug = entry.file_name().to_string_lossy().to_string();
               let has_spec = entry.path().join("spec.md").exists();
               let has_plan = entry.path().join("plan.md").exists();
               let has_tasks = entry.path().join("tasks").is_dir()
                   && has_wp_files(&entry.path().join("tasks"));
               features.push(FeatureInfo { slug, has_spec, has_plan, has_tasks });
           }
       }
       features.sort_by(|a, b| a.slug.cmp(&b.slug));
       Ok(ListFeaturesOutput { ok: true, features })
   }
   ```
2. Reference existing `list_specs.rs` (146 lines) for the scanning pattern already used in the CLI.
3. Match contract output: `{ ok: bool, features: [{ slug, has_spec, has_plan, has_tasks }] }`

**Parallel?**: Yes - independent of T025 once shared utilities exist.
**Files**: `crates/kasmos/src/serve/tools/list_features.rs`
**Validation**: Returns correct feature list matching `kitty-specs/` contents.

### Subtask T025 - Fully implement infer_feature tool

**Purpose**: Infer feature slug from spec_prefix argument, git branch, or working directory.

**Steps**:
1. Implement in `crates/kasmos/src/serve/tools/infer_feature.rs`:
   - Reuse the detection logic from `launch/detect.rs` (T009)
   - Accept optional `spec_prefix` input
   - Return `{ ok, source: "arg"|"branch"|"directory"|"none", feature_slug? }`
2. Map `FeatureSource` enum to the contract's `source` string enum.
3. When `source` is `"none"`, `feature_slug` is absent and `ok` is still `true` (not an error).

**Parallel?**: Yes - independent of T024 once shared detection logic exists.
**Files**: `crates/kasmos/src/serve/tools/infer_feature.rs`
**Validation**: Returns correct source and slug for each detection scenario.

### Subtask T026 - Add contract-level tests for tool registration and error codes

**Purpose**: Verify that all 9 tools are registered, schemas match the contract, and standard error codes are returned correctly.

**Steps**:
1. Test that `tools/list` returns exactly 9 tools with correct names
2. Test that each tool accepts valid input without panicking
3. Test that invalid input returns `INVALID_INPUT` error code
4. Test that `list_features` returns expected structure
5. Test that `infer_feature` returns expected structure
6. Reference the error codes from the contract: `INVALID_INPUT`, `FEATURE_LOCK_CONFLICT`, `STALE_LOCK_CONFIRMATION_REQUIRED`, `WORKER_NOT_FOUND`, `TRANSITION_NOT_ALLOWED`, `DEPENDENCY_MISSING`, `INTERNAL_ERROR`

**Files**: Test modules in serve submodules
**Validation**: `cargo test` passes with contract tests.

## Risks & Mitigations

| Risk | Mitigation |
|------|-----------|
| Contract drift between code and JSON spec | Enforce schema snapshots in tests. Compare generated schemas against contract file. |
| rmcp API changes between versions | Pin to specific version. Add integration test that exercises stdio transport. |
| Incomplete input validation | Centralize validation helpers. Use serde's strict deserialization. |

## Review Guidance

- Verify all 9 tools from the contract are registered
- Verify `list_features` and `infer_feature` are fully functional (not stubs)
- Verify typed structs match contract schemas exactly
- Verify server starts and responds on stdio transport
- Verify error codes match contract specification
- Verify graceful shutdown on stdin close

## Activity Log

- 2026-02-14T16:27:48Z - system - lane=planned - Prompt generated via /spec-kitty.tasks
- 2026-02-14T22:25:37Z – unknown – shell_pid=3674163 – lane=planned – Moved to planned
- 2026-02-14T22:27:10Z – unknown – shell_pid=3674163 – lane=planned – Moved to planned
- 2026-02-14T22:39:40Z – opencode – shell_pid=3957847 – lane=doing – Assigned agent via workflow command
- 2026-02-14T22:55:34Z – opencode – shell_pid=3957847 – lane=for_review – Ready for review
