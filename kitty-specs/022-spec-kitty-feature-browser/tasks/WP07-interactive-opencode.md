---
work_package_id: WP07
title: Interactive opencode for tmux Workers
lane: planned
dependencies: []
subtasks: [T035, T036, T037, T038]
phase: implementation
history:
- timestamp: '2026-02-20T14:00:00Z'
  lane: planned
  actor: manager
  action: created work package - switch tmux backend from opencode run to interactive opencode
---

# WP07: Interactive opencode for tmux Workers

## Implementation Command

```bash
spec-kitty implement WP07
```

## Objective

Change the tmux backend to spawn `opencode` (interactive TUI) instead of `opencode run` (headless CLI). This gives users a full interactive opencode session in the tmux pane -- they can see the agent work in real-time, provide input, approve tool calls, and interact naturally. The subprocess backend remains headless (`opencode run`) for pipe-captured output.

**Independence note**: This WP has zero overlap with WP01-05 (browser feature) or WP06 (theming). It can run in parallel with any of them.

## Context

### The Problem

Both `SubprocessBackend` and `TmuxBackend` currently use `opencode run` to spawn workers. This is correct for the subprocess backend (headless, pipe-captured output) but wrong for the tmux backend. The tmux backend creates a real PTY pane specifically so users can interact with the worker -- but `opencode run` is non-interactive and exits after completing its task. The user cannot intervene, provide clarification, or steer the agent.

Additionally, passing large prompts (e.g., a 460-line WP task body) as a positional argument to `opencode run` can cause immediate exit with code 0, as observed with WP04 of this feature.

### opencode CLI Modes

**Headless** (`opencode run`):
```
opencode run --agent coder --model provider/model --variant high -f file.go "prompt text"
```
- Prompt is a positional argument
- Supports `--variant` (reasoning effort) and `-f` (file attachments)
- Exits when done, output goes to stdout
- Non-interactive

**Interactive** (`opencode`):
```
opencode --agent coder --model provider/model --prompt "prompt text"
```
- Prompt passed via `--prompt` flag (not positional)
- Has `--agent`, `--model`, `--continue`, `--session`
- Does NOT have `--variant` or `-f` flags
- Starts the full TUI, user can interact
- Keeps running until user exits

### Flag Mapping

| SpawnConfig field   | `opencode run` flag          | `opencode` flag       | Notes                                |
|---------------------|------------------------------|-----------------------|--------------------------------------|
| Role                | `--agent <role>`             | `--agent <role>`      | Same                                 |
| Prompt              | positional `"prompt"`        | `--prompt "prompt"`   | Different!                           |
| ContinueSession     | `--continue -s <id>`         | `--continue -s <id>`  | Same                                 |
| Model               | `--model <model>`            | `--model <model>`     | Same                                 |
| Reasoning           | `--variant <level>`          | N/A                   | Not available in interactive mode    |
| Files               | `-f <file>` (repeatable)     | N/A                   | Not available in interactive mode    |
| WorkDir             | `--dir <path>`               | positional `[project]`| Different!                           |

---

## Subtask T035: Separate buildArgs for Subprocess vs tmux

**Purpose**: Split the arg-building logic so each backend constructs the correct command for its mode.

**Steps**:

1. The `SubprocessBackend.buildArgs()` in `internal/worker/subprocess.go` stays unchanged -- it already correctly builds `opencode run` args.

2. Rewrite `TmuxBackend.buildArgs()` in `internal/worker/tmux.go` to build interactive opencode args:

   ```go
   func (b *TmuxBackend) buildArgs(cfg SpawnConfig) []string {
       args := []string{}  // No "run" subcommand

       if cfg.Role != "" {
           args = append(args, "--agent", cfg.Role)
       }
       if cfg.ContinueSession != "" {
           args = append(args, "--continue", "-s", cfg.ContinueSession)
       }
       if cfg.Model != "" {
           args = append(args, "--model", cfg.Model)
       }
       if cfg.Prompt != "" {
           args = append(args, "--prompt", cfg.Prompt)
       }

       return args
   }
   ```

3. Note: `Reasoning` and `Files` are intentionally omitted -- the interactive `opencode` CLI does not support `--variant` or `-f`. If these fields are set, they are silently ignored in tmux mode.

**Files**: `internal/worker/tmux.go`

**Validation**:
- [ ] `TmuxBackend.buildArgs` does NOT include "run" as first arg
- [ ] Prompt passed via `--prompt` flag, not positional
- [ ] `SubprocessBackend.buildArgs` unchanged (still uses "run")
- [ ] Reasoning and Files gracefully ignored in tmux mode

---

## Subtask T036: Add WorkDir Support to tmux SplitWindow

**Purpose**: The interactive `opencode` command takes a project path as a positional argument (`opencode [project]`). The tmux `split-window` command also supports `-c <dir>` to set the starting directory. Add WorkDir support so workers run in the correct directory (e.g., a worktree).

**Steps**:

1. Add `Dir` field to `SplitOpts` in `internal/worker/tmux_cli.go`:

   ```go
   type SplitOpts struct {
       Target     string
       Horizontal bool
       Size       string
       Dir        string   // -c: starting directory
       Command    []string
       Env        []string
   }
   ```

2. Update `tmuxExec.SplitWindow()` to use it:

   ```go
   if opts.Dir != "" {
       args = append(args, "-c", opts.Dir)
   }
   ```

3. Update `TmuxBackend.Spawn()` to pass `cfg.WorkDir`:

   ```go
   paneID, err := b.cli.SplitWindow(ctx, SplitOpts{
       Target:     b.kasmosPaneID,
       Horizontal: true,
       Size:       splitSize,
       Dir:        cfg.WorkDir,
       Command:    cmd,
       Env:        buildEnvArgs(cfg.Env),
   })
   ```

**Files**: `internal/worker/tmux_cli.go`, `internal/worker/tmux.go`

**Validation**:
- [ ] Worker pane starts in `cfg.WorkDir` when set
- [ ] Empty WorkDir uses the parent pane's directory (default tmux behavior)
- [ ] `SplitOpts.Dir` uses the `-c` flag in the tmux command

---

## Subtask T037: Update buildArgs Tests

**Purpose**: Add tests verifying the divergent arg construction between subprocess (headless) and tmux (interactive) backends.

**Steps**:

1. In `internal/worker/backend_test.go`, the existing `TestBuildArgs` tests cover `SubprocessBackend`. Add parallel test cases for `TmuxBackend.buildArgs`:

   ```go
   func TestTmuxBuildArgs(t *testing.T) {
       backend := &TmuxBackend{}
       tests := []struct {
           name string
           cfg  SpawnConfig
           want []string
       }{
           {
               name: "basic prompt",
               cfg:  SpawnConfig{Prompt: "implement feature"},
               want: []string{"--prompt", "implement feature"},
           },
           {
               name: "role and prompt",
               cfg:  SpawnConfig{Role: "coder", Prompt: "do work"},
               want: []string{"--agent", "coder", "--prompt", "do work"},
           },
           {
               name: "continue session",
               cfg:  SpawnConfig{ContinueSession: "ses_abc123", Role: "reviewer", Prompt: "continue"},
               want: []string{"--agent", "reviewer", "--continue", "-s", "ses_abc123", "--prompt", "continue"},
           },
           {
               name: "reasoning ignored in interactive mode",
               cfg:  SpawnConfig{Role: "coder", Reasoning: "high", Prompt: "code"},
               want: []string{"--agent", "coder", "--prompt", "code"},
           },
           {
               name: "files ignored in interactive mode",
               cfg:  SpawnConfig{Role: "coder", Files: []string{"main.go"}, Prompt: "review"},
               want: []string{"--agent", "coder", "--prompt", "review"},
           },
       }
       for _, tt := range tests {
           t.Run(tt.name, func(t *testing.T) {
               got := backend.buildArgs(tt.cfg)
               // assert equality
           })
       }
   }
   ```

**Files**: `internal/worker/backend_test.go` or `internal/worker/tmux_test.go`

**Validation**:
- [ ] Test confirms no "run" in tmux args
- [ ] Test confirms `--prompt` flag used (not positional)
- [ ] Test confirms `--variant` and `-f` are dropped
- [ ] Existing subprocess tests still pass

---

## Subtask T038: Update Architecture Documentation

**Purpose**: Record the dual-mode design decision in the architecture memory.

**Steps**:

1. Update `.kittify/memory/architecture.md` Worker Lifecycle section to document:

   - `SubprocessBackend` uses `opencode run` (headless, pipe-captured)
   - `TmuxBackend` uses `opencode` (interactive TUI, full PTY)
   - The `--prompt` flag pre-fills the initial prompt for interactive sessions
   - `--variant` and `-f` are only available in headless mode
   - WorkDir maps to `-c` in tmux split-window and positional `[project]` in opencode

2. Update the constitution if the current wording implies all workers use `opencode run`.

**Files**: `.kittify/memory/architecture.md`, `.kittify/memory/constitution.md`

**Validation**:
- [ ] Architecture doc reflects both backend modes
- [ ] Constitution language is accurate (not exclusively `opencode run`)

---

## Definition of Done

- [ ] `TmuxBackend` spawns `opencode` (interactive), not `opencode run`
- [ ] `SubprocessBackend` still spawns `opencode run` (headless) -- no regression
- [ ] Prompt passed via `--prompt` flag in tmux mode
- [ ] WorkDir supported via `-c` in tmux split-window
- [ ] `go build ./internal/worker/` succeeds
- [ ] `go test ./internal/worker/` passes
- [ ] Architecture documentation updated

## Risks

- **`--variant` and `-f` unavailable in interactive mode**: If a user configures reasoning effort (e.g., "high") or file attachments in their agent settings, these are silently ignored for tmux workers. Document this tradeoff. Could be addressed in future by using `opencode run` with `--attach` if needed.
- **Prompt escaping**: Long prompts with special characters passed via `--prompt` should work since tmux `split-window` passes args directly (no shell interpretation). Test with prompts containing backticks, quotes, and newlines.
- **opencode TUI size**: The interactive opencode TUI needs sufficient terminal width/height in its pane. With a 50% horizontal split, narrow terminals may cause layout issues in opencode. The existing `narrowLayout` flag could help (uses default tmux sizing instead of 50%).

## Reviewer Guidance

- Verify the subprocess backend is completely untouched
- Verify `--prompt` is used instead of positional arg in tmux buildArgs
- Verify WorkDir support uses tmux `-c` flag (not `cd && ...`)
- Check that long prompts (460+ lines) work correctly via `--prompt`
- Confirm `buildArgs` test cases cover the flag differences between modes
