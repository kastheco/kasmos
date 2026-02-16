# Quickstart: Kasmos New Command

## What Success Looks Like

### Basic usage (no description)

```bash
$ kasmos new
# OpenCode launches in the current terminal as a planning agent.
# The agent automatically runs /spec-kitty.specify.
# The agent asks the user to describe their feature idea.
# After interactive discovery, a new spec is created:
#   kitty-specs/013-my-feature/spec.md
# When the user exits opencode, the terminal returns to the shell.
$ echo $?
0
```

### With an initial description

```bash
$ kasmos new add webhook support for external integrations
# OpenCode launches with the description pre-loaded.
# The agent passes "add webhook support for external integrations"
# as the starting point for /spec-kitty.specify discovery.
# Discovery may still ask follow-up questions.
```

### Missing dependency

```bash
$ kasmos new
Error: opencode not found in PATH
  Needed for: launching the planning agent session
  Fix: Install OpenCode and ensure its launcher binary is on PATH
$ echo $?
1
```

### End-to-end workflow

```bash
# 1. Create a new spec
$ kasmos new

# 2. (After spec creation completes, launch orchestration)
$ kasmos 013
```

## Verification Checklist

1. `kasmos new` launches opencode in-place (no new terminal, no Zellij tab)
2. The planning agent's first action is to run `/spec-kitty.specify`
3. The agent has project context (check for constitution/architecture references in its output)
4. `kasmos new "some description"` passes the description to the agent
5. After exiting opencode, the shell prompt returns in the same terminal
6. `kasmos new` with opencode missing prints an error and exits with code 1
7. `kasmos new` with spec-kitty missing prints an error and exits with code 1
8. Running `kasmos new` inside a Zellij pane works identically to running outside
