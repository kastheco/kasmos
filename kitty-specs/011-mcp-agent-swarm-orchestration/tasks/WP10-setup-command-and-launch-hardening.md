---
work_package_id: WP10
title: Setup Command and Launch Hardening
lane: "for_review"
dependencies: [WP03]
base_branch: 011-mcp-agent-swarm-orchestration-WP03
base_commit: 5ede493dbac49ea7462a399719ed32e777981362
created_at: '2026-02-15T01:01:51.958671+00:00'
subtasks:
- T057
- T058
- T059
- T060
- T061
- T062
phase: Phase 3 - Setup UX, Role Context, and End-to-End Hardening
assignee: ''
agent: ''
shell_pid: "212269"
review_status: ''
reviewed_by: ''
history:
- timestamp: '2026-02-14T16:27:48Z'
  lane: planned
  agent: system
  shell_pid: ''
  action: Prompt generated via /spec-kitty.tasks
---

# Work Package Prompt: WP10 - Setup Command and Launch Hardening

## Important: Review Feedback Status

- **Has review feedback?**: Check the `review_status` field above.

---

## Review Feedback

*[This section is empty initially.]*

---

## Implementation Command

```bash
spec-kitty implement WP10 --base WP03
```

---

## Objectives & Success Criteria

Deliver `kasmos setup` as a first-time environment validation and configuration generation tool, and ensure launch-time preflight remains strict. After this WP:

1. `kasmos setup` validates all required dependencies and reports pass/fail for each
2. `kasmos setup` generates missing baseline config/profile assets idempotently
3. Missing dependencies produce actionable installation guidance
4. Launch preflight shares the same check engine and exits before session/tab creation
5. All failure paths return non-zero exit codes

## Context & Constraints

- **Depends on WP03**: Launch flow available with preflight stubs
- **Spec FR-022**: Setup command for first-time environment validation
- **Spec FR-021**: Launch preflight must validate dependencies before creating sessions/tabs
- **SC-009**: Clean setup on properly configured machine completes within 5 seconds
- **Existing code**: `which` crate (already a dependency) for binary detection

## Subtasks & Detailed Guidance

### Subtask T057 - Implement setup command validation flow

**Purpose**: Create the `kasmos setup` command that validates each dependency and reports structured results.

**Steps**:
1. Populate `crates/kasmos/src/setup/mod.rs`:
   ```rust
   pub struct SetupResult {
       pub checks: Vec<CheckResult>,
       pub all_passed: bool,
   }

   pub struct CheckResult {
       pub name: String,
       pub description: String,
       pub status: CheckStatus,
       pub guidance: Option<String>,
   }

   pub enum CheckStatus { Pass, Fail, Warn }

   pub async fn run() -> anyhow::Result<()> {
       let config = crate::config::Config::load()?;
       let result = validate_environment(&config)?;
       print_results(&result);
       if !result.all_passed {
           std::process::exit(1);
       }
       Ok(())
   }

   pub fn validate_environment(config: &Config) -> Result<SetupResult> {
       let mut checks = Vec::new();
       checks.push(check_zellij(&config.paths.zellij_binary));
       checks.push(check_opencode(&config.agent.opencode_binary));
       checks.push(check_spec_kitty(&config.paths.spec_kitty_binary));
       checks.push(check_pane_tracker());
       checks.push(check_git());
       checks.push(check_config_files());
       let all_passed = checks.iter().all(|c| c.status != CheckStatus::Fail);
       Ok(SetupResult { checks, all_passed })
   }
   ```
2. Each check validates one dependency:
   - `zellij`: Binary exists and is executable
   - `opencode` (ocx): Binary exists
   - `spec-kitty`: Binary exists
   - Pane-tracker: Zellij plugin is available
   - `git`: Binary exists and we're in a git repo
   - Config files: `kasmos.toml` exists (warn if missing, not fail)
3. Output format - clear, colorized terminal output:
   ```
   kasmos setup
   [PASS] zellij ........... /usr/local/bin/zellij (v0.41.2)
   [PASS] opencode (ocx) .. /usr/local/bin/ocx
   [PASS] spec-kitty ...... /usr/local/bin/spec-kitty
   [PASS] pane-tracker .... plugin available
   [PASS] git ............. /usr/bin/git (in git repo)
   [WARN] config .......... kasmos.toml not found (using defaults)

   All required checks passed.
   ```

**Files**: `crates/kasmos/src/setup/mod.rs`
**Validation**: `kasmos setup` reports all checks. Missing dependency shows [FAIL].

### Subtask T058 - Implement idempotent config/profile asset generation

**Purpose**: Generate missing baseline configuration and agent profile files on first setup.

**Steps**:
1. If `kasmos.toml` doesn't exist at repo root, generate a default:
   ```rust
   fn generate_default_config() -> Result<()> {
       let path = PathBuf::from("kasmos.toml");
       if path.exists() { return Ok(()); }  // idempotent
       let config = Config::default();
       let toml_str = toml::to_string_pretty(&config)?;
       std::fs::write(&path, toml_str)?;
       println!("Generated: kasmos.toml");
       Ok(())
   }
   ```
2. If `config/profiles/kasmos/` doesn't exist, generate default profile directory with:
   - `opencode.jsonc` - OpenCode MCP configuration pointing to `kasmos serve`
   - Agent prompt templates: `manager.md`, `coder.md`, `reviewer.md`, `release.md`
3. Never overwrite existing files. Only create missing ones.
4. Report what was created.

**Parallel?**: Yes - can proceed alongside T060 after core setup skeleton exists.
**Files**: `crates/kasmos/src/setup/mod.rs`
**Validation**: First run creates files. Second run creates nothing (idempotent).

### Subtask T059 - Ensure launch shares preflight engine

**Purpose**: The launch path and setup command should use the same validation engine to prevent drift.

**Steps**:
1. Extract the validation logic into a shared function:
   ```rust
   // In setup/mod.rs
   pub fn validate_environment(config: &Config) -> Result<SetupResult> { ... }

   // In launch/mod.rs
   pub fn preflight_checks(config: &Config) -> Result<()> {
       let result = crate::setup::validate_environment(config)?;
       if !result.all_passed {
           let failures: Vec<_> = result.checks.iter()
               .filter(|c| c.status == CheckStatus::Fail)
               .collect();
           print_failures(&failures);
           std::process::exit(1);
       }
       Ok(())
   }
   ```
2. The launch preflight calls the SAME validation function, just formats the output differently (launch shows only failures, setup shows all).
3. This ensures both paths check the same things.

**Files**: `crates/kasmos/src/setup/mod.rs`, `crates/kasmos/src/launch/mod.rs`
**Validation**: Adding a new check to setup automatically adds it to launch preflight.

### Subtask T060 - Add per-dependency remediation guidance

**Purpose**: Each failed check should include specific installation/configuration guidance.

**Steps**:
1. Per-dependency guidance strings:
   ```rust
   fn zellij_guidance() -> &'static str {
       "Install zellij: cargo install zellij\n\
        Or see: https://zellij.dev/documentation/installation"
   }

   fn opencode_guidance() -> &'static str {
       "Install opencode: see project documentation\n\
        Ensure 'ocx' binary is in PATH"
   }

   fn spec_kitty_guidance() -> &'static str {
       "Install spec-kitty: pip install spec-kitty\n\
        Or see project documentation"
   }

   fn pane_tracker_guidance() -> &'static str {
       "Install zellij-pane-tracker plugin.\n\
        See: https://github.com/example/zellij-pane-tracker"
   }
   ```
2. Guidance is attached to each `CheckResult` as the `guidance` field.
3. Launch preflight shows guidance for each failure.

**Parallel?**: Yes - can proceed alongside T058 after core setup skeleton exists.
**Files**: `crates/kasmos/src/setup/mod.rs`
**Validation**: Missing dependency shows actionable guidance.

### Subtask T061 - Ensure non-zero exit code mapping

**Purpose**: All failure scenarios in setup and launch preflight must return non-zero exit codes.

**Steps**:
1. `kasmos setup` with failures: exit code 1
2. `kasmos` launch with missing deps: exit code 1
3. `kasmos` launch with no specs: exit code 0 (not a failure, just nothing to do)
4. Ensure `std::process::exit()` is called appropriately, OR use `anyhow::Result` and let main() propagate the error.
5. Verify with shell: `kasmos setup; echo $?` should show 0 or 1.

**Files**: `crates/kasmos/src/setup/mod.rs`, `crates/kasmos/src/launch/mod.rs`, `crates/kasmos/src/main.rs`
**Validation**: Failed checks produce non-zero exit. Successful checks produce zero.

### Subtask T062 - Add tests for setup and launch hard-fail

**Purpose**: Test setup pass/fail scenarios and launch preflight guarantees.

**Steps**:
1. Test setup passes with all dependencies present
2. Test setup fails with missing binary
3. Test setup generates config file when missing
4. Test setup is idempotent (second run changes nothing)
5. Test launch preflight shares same checks as setup
6. Test launch exits non-zero before creating session on failure
7. Use PATH manipulation and tempfile for test isolation

**Files**: Test modules in setup/mod.rs and launch/mod.rs
**Validation**: `cargo test` passes with setup/preflight tests.

## Risks & Mitigations

| Risk | Mitigation |
|------|-----------|
| Setup and launch checks drift over time | Shared validation function prevents drift by design |
| False positives for pane-tracker availability | Include functional probe (version check) in addition to binary lookup |
| Config generation conflicts with existing files | Never overwrite. Only create missing files. |

## Review Guidance

- Verify setup and launch share the same validation engine
- Verify all failures include actionable remediation guidance
- Verify idempotent config generation (no overwrites)
- Verify non-zero exit codes on failure
- Verify output is clear and readable
- Verify SC-009: clean setup completes within 5 seconds

## Activity Log

- 2026-02-14T16:27:48Z - system - lane=planned - Prompt generated via /spec-kitty.tasks
- 2026-02-15T01:18:04Z – unknown – shell_pid=212269 – lane=for_review – Ready for review
