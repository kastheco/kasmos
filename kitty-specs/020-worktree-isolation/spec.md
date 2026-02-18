# Feature Specification: Git Worktree Isolation for Workers

**Feature Branch**: `020-worktree-isolation`
**Created**: 2026-02-18
**Status**: Draft
**Input**: User description: "use git worktrees for tasks (have this as a toggleable option in the settings view)"

## User Scenarios & Testing

### User Story 1 - Enable Worktree Mode in Settings (Priority: P1)

A user opens kasmos settings and toggles worktree isolation on. From that point forward, every new worker spawn creates a dedicated git worktree so that agents work in isolated branches without conflicting with each other or the main working tree. The setting persists across sessions.

**Why this priority**: The toggle is the gateway to the entire feature. Without a way to enable/disable worktrees, nothing else functions. It also lets users opt out when worktree isolation is unnecessary (e.g., read-only analysis tasks, small repos).

**Independent Test**: Toggle the setting on, close settings, reopen settings -- the value should persist. Toggle off and confirm workers spawn without worktrees.

**Acceptance Scenarios**:

1. **Given** the settings view is open, **When** the user navigates to the "use worktrees" row and cycles the value, **Then** the setting toggles between "on" and "off"
2. **Given** worktrees are toggled on and settings are saved, **When** kasmos is restarted, **Then** the setting is still "on"
3. **Given** worktrees are toggled off, **When** a worker is spawned, **Then** no worktree is created and the worker runs in the repository root as before

---

### User Story 2 - Worker Spawn Creates Worktree (Priority: P1)

When worktree mode is enabled, spawning a new worker creates a git worktree under `.worktrees/` at the repository root. The worker process runs inside that worktree directory. The branch name is derived from the task ID when the worker is associated with a task (e.g., `kasmos/WP01`), falling back to the worker ID for ad-hoc spawns (e.g., `kasmos/W-003`).

**Why this priority**: This is the core behavior -- without it, the toggle does nothing. Workers must be able to operate in isolated git branches to prevent file conflicts during parallel work.

**Independent Test**: Enable worktrees, spawn a worker, verify `.worktrees/<branch-slug>/` exists on disk and the worker process is running inside it.

**Acceptance Scenarios**:

1. **Given** worktree mode is on and a task-based worker is spawned for task WP01, **When** the worker starts, **Then** a worktree exists at `.worktrees/kasmos-WP01/` on the branch `kasmos/WP01` and the worker's working directory is set to that path
2. **Given** worktree mode is on and an ad-hoc worker W-005 is spawned, **When** the worker starts, **Then** a worktree exists at `.worktrees/kasmos-W-005/` on the branch `kasmos/W-005`
3. **Given** worktree mode is on and a branch `kasmos/WP01` already exists from a previous run, **When** a new worker is spawned for WP01, **Then** the system creates a uniquely named worktree (e.g., appending a suffix) to avoid branch conflicts
4. **Given** worktree mode is on, **When** worktree creation fails (e.g., git error, disk full), **Then** the worker spawn fails gracefully with an error visible in the viewport, and no zombie worktree is left behind

---

### User Story 3 - Continued Sessions Inherit Worktree (Priority: P2)

When a user continues a worker session (via the `c` keybind), the continuation worker inherits the parent worker's worktree directory rather than creating a new one. This ensures the follow-up session has access to all the file changes the original worker made.

**Why this priority**: Continuation is a core workflow in kasmos. If each continuation created a fresh worktree, the agent would lose all context of prior changes, making multi-step work impossible.

**Independent Test**: Spawn a worker with worktree mode on, let it exit, continue it, and verify the continued worker uses the same worktree path.

**Acceptance Scenarios**:

1. **Given** worker W-001 ran in worktree `.worktrees/kasmos-W-001/`, **When** the user continues W-001, **Then** the new worker W-002 runs in `.worktrees/kasmos-W-001/` (the parent's worktree)
2. **Given** a chain of continuations (W-001 -> W-002 -> W-003), **When** W-003 is spawned, **Then** it runs in W-001's original worktree (the chain root)
3. **Given** a worker was spawned without worktree mode (mode was off), **When** that worker is continued after worktree mode is turned on, **Then** the continuation runs in the repository root (inheriting the parent's non-worktree context)

---

### User Story 4 - Prune Stale Worktrees (Priority: P2)

The user can invoke a keybind to prune worktrees that are no longer needed. This removes worktrees tied to exited/failed/killed workers while preserving those belonging to running workers or active continuation chains.

**Why this priority**: Without cleanup, `.worktrees/` grows unboundedly across sessions. Manual `git worktree remove` is tedious and error-prone. A dedicated prune action keeps disk usage manageable.

**Independent Test**: Spawn several workers with worktrees, let them exit, invoke the prune keybind, confirm the stale worktrees are removed from disk and from git's worktree tracking.

**Acceptance Scenarios**:

1. **Given** worktrees exist for workers W-001 (exited), W-002 (running), and W-003 (failed), **When** the user invokes the prune action, **Then** worktrees for W-001 and W-003 are removed but W-002's worktree is preserved
2. **Given** there are no stale worktrees, **When** the user invokes prune, **Then** a message indicates nothing to prune
3. **Given** a worktree directory was manually deleted but git still tracks it, **When** prune is invoked, **Then** the orphaned git worktree reference is cleaned up
4. **Given** a continued session chain where the root worker exited but a child worker is still running, **When** prune is invoked, **Then** the shared worktree is NOT removed

---

### User Story 5 - Worktree Visibility in Dashboard (Priority: P3)

When worktree mode is active, the worker's worktree branch name and path are visible in the viewport header (alongside the prompt). This gives the user confirmation of which worktree a worker is operating in.

**Why this priority**: Informational. The feature works without this, but visibility prevents confusion when managing multiple parallel workers.

**Independent Test**: Spawn a worker with worktrees enabled, select it in the table, and confirm the viewport header shows the worktree path and branch.

**Acceptance Scenarios**:

1. **Given** a worker is running in a worktree, **When** the user selects it in the worker table, **Then** the viewport header shows the worktree branch name and path
2. **Given** a worker is running without a worktree (mode off or inherited from pre-worktree parent), **When** selected, **Then** no worktree info is shown in the header

---

### Edge Cases

- What happens when the user enables worktree mode in a non-git repository? The system should detect this and show an error, keeping the setting off.
- What happens when the repository has uncommitted changes in the main working tree? Worktree creation should still succeed since git worktrees branch from HEAD independently.
- What happens when multiple workers are spawned for the same task ID simultaneously? Each gets a uniquely suffixed worktree to avoid branch name collisions.
- What happens when a worktree's branch is manually deleted via `git branch -D` outside kasmos? The prune action should handle this gracefully.
- What happens during session restore (`--attach`) with worktree metadata? Restored workers should retain their worktree path associations but not attempt to re-create worktrees.
- What happens when the `.worktrees/` directory is deleted externally while kasmos is running? Workers already running keep their process working directory (OS handles), but new spawns should re-create the directory.

## Requirements

### Functional Requirements

- **FR-001**: System MUST provide a toggleable "use worktrees" setting in the settings view, persisted to the configuration file
- **FR-002**: System MUST create a git worktree under `.worktrees/` at the repository root for each new worker spawn when worktree mode is enabled
- **FR-003**: System MUST derive worktree branch names from the task ID when available (format: `kasmos/<task-id>`), falling back to worker ID (format: `kasmos/<worker-id>`)
- **FR-004**: System MUST set the worker process's working directory to the created worktree path
- **FR-005**: System MUST pass the parent worker's worktree path to continuation workers instead of creating a new worktree
- **FR-006**: System MUST handle worktree creation failures gracefully, reporting errors to the user without leaving orphaned directories
- **FR-007**: System MUST provide a user-invocable action to prune worktrees belonging to non-running workers
- **FR-008**: System MUST handle branch name collisions by appending a unique suffix when a branch already exists
- **FR-009**: System MUST validate that the current directory is a git repository before allowing worktree mode to be enabled
- **FR-010**: System MUST preserve worktree path associations in session persistence so restored sessions know which worktree each worker used
- **FR-011**: System MUST display the active worktree branch and path in the viewport header for worktree-enabled workers
- **FR-012**: System MUST protect worktrees from pruning if any worker in a continuation chain is still running

### Key Entities

- **Worktree**: A git worktree associated with a worker, characterized by a directory path under `.worktrees/`, a branch name, and a reference to the owning worker (or chain root worker for continuations)
- **WorktreeConfig**: The user-facing toggle (on/off) stored in the configuration file, controlling whether new worker spawns create worktrees

## Success Criteria

### Measurable Outcomes

- **SC-001**: Users can toggle worktree mode on or off within 2 interactions in the settings view
- **SC-002**: Workers spawned with worktree mode enabled operate in isolated git working copies that do not affect each other or the main working tree
- **SC-003**: Continued sessions retain full access to prior file changes by inheriting the parent's worktree
- **SC-004**: Users can prune all stale worktrees in a single action
- **SC-005**: The worktree toggle setting persists across kasmos restarts with no data loss

## Assumptions

- The host repository uses git as its version control system. Worktree mode is not applicable to non-git repositories.
- The user has a git version that supports `git worktree` (2.5+, released 2015). This is a safe assumption for modern development environments.
- Worktrees branch from the current HEAD of the repository. The feature does not provide a way to specify a custom base branch at spawn time (the user can change branches in the worktree after the agent starts).
- The `.worktrees/` directory should be added to `.gitignore` by the user or by kasmos setup. The feature does not auto-modify `.gitignore`.
