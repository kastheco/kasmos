---
work_package_id: WP10
title: Setup Command + Agent Scaffolding
lane: done
dependencies:
- WP01
subtasks:
- internal/setup/setup.go - Setup orchestration
- internal/setup/agents.go - Agent definition templates
- internal/setup/deps.go - Dependency validation
- cmd/kasmos setup.go - Cobra subcommand
- Unit tests
phase: Wave 2 - Task Sources + Worker Management
assignee: ''
agent: ''
shell_pid: ''
review_status: ''
reviewed_by: ''
history:
- timestamp: '2026-02-17T00:00:00Z'
  lane: planned
  agent: planner
  action: Prompt generated via /spec-kitty.tasks
- timestamp: '2026-02-18T14:24:29.725046729+00:00'
  lane: doing
  actor: manager
  shell_pid: '472734'
  action: transition active (Launching WP10 coder - setup command + agent scaffolding)
- timestamp: '2026-02-18T14:31:01.365155161+00:00'
  lane: done
  actor: manager
  shell_pid: '472734'
  action: transition done (Implemented and verified - setup command + agent scaffolding)
---

# Work Package Prompt: WP10 - Setup Command + Agent Scaffolding

## Mission

Implement `kasmos setup`: a CLI subcommand that validates dependencies (opencode, git),
scaffolds OpenCode agent definition files (planner, coder, reviewer, release), and
reports status. This is a self-contained feature with no TUI dependency. Delivers
User Story 6 (Setup and Agent Configuration).

## Scope

### Files to Create

```
internal/setup/setup.go     # Setup orchestration (run all steps)
internal/setup/agents.go    # Agent definition templates + write logic
internal/setup/deps.go      # Dependency validation (opencode, git)
internal/setup/setup_test.go
internal/setup/deps_test.go
```

### Files to Modify

```
cmd/kasmos/main.go          # Add cobra `setup` subcommand
```

### Technical References

- `kitty-specs/016-kasmos-agent-orchestrator/research/tui-technical.md`:
  - **Section 9**: Dependency validation (lines 1191-1215)
- `kitty-specs/016-kasmos-agent-orchestrator/spec.md`:
  - User Story 6 acceptance scenarios (lines 91-103)
- `.kittify/memory/constitution.md`:
  - OpenCode as sole agent harness (line 22)
  - Agent definitions: `.opencode/agents/*.md` (line 22)

## Implementation

### deps.go

Implement dependency validation from tui-technical.md Section 9:

```go
type DependencyCheck struct {
    Name        string
    Check       func() error
    Required    bool
    InstallHint string
}

func ValidateDependencies() []DependencyCheck {
    return []DependencyCheck{
        {
            Name: "opencode",
            Check: func() error {
                _, err := exec.LookPath("opencode")
                return err
            },
            Required: true,
            InstallHint: "go install github.com/anomalyco/opencode@latest",
        },
        {
            Name: "git",
            Check: func() error {
                _, err := exec.LookPath("git")
                return err
            },
            Required: true,
            InstallHint: "install via system package manager",
        },
    }
}
```

Run each check, collect results. Format as a table:
```
Checking dependencies...
  opencode   check found (/usr/bin/opencode)
  git        check found (/usr/bin/git)
```
Or on failure:
```
  opencode   x NOT FOUND
             Install: go install github.com/anomalyco/opencode@latest
```

### agents.go

Define 4 agent definition templates. Each produces a `.opencode/agents/{role}.md`
file. The file format follows OpenCode's custom agent spec (markdown with YAML
frontmatter for configuration).

Agent templates:

**planner.md**:
- Role: Research and planning, read-only filesystem
- System prompt: emphasis on analysis, plan generation, no code modification
- Tools: read-only (file read, grep, glob, web fetch)
- Model: default (user configures in OpenCode)

**coder.md**:
- Role: Implementation, full tool access
- System prompt: emphasis on implementation, testing, code quality
- Tools: full access (file write, shell, all reads)

**reviewer.md**:
- Role: Code review, read-only + test execution
- System prompt: emphasis on correctness, security, quality
- Tools: read-only + shell (for running tests)

**release.md**:
- Role: Merge, finalization, cleanup
- System prompt: emphasis on merge operations, cleanup, documentation
- Tools: full access

Write function:
```go
func WriteAgentDefinitions(dir string) error {
    agentDir := filepath.Join(dir, ".opencode", "agents")
    os.MkdirAll(agentDir, 0o755)
    for _, agent := range agentDefinitions {
        path := filepath.Join(agentDir, agent.Filename)
        if _, err := os.Stat(path); err == nil {
            // File exists -- skip (don't overwrite user customizations)
            continue
        }
        os.WriteFile(path, []byte(agent.Content), 0o644)
    }
}
```

Important: Do NOT overwrite existing agent files. Only create if missing.
Print what was created vs what was skipped.

### setup.go

Orchestrate the full setup:
1. Print "kasmos setup" header
2. Run dependency validation, print results
3. If any required dep is missing, print error and exit 1
4. Determine project root (walk up from cwd looking for go.mod or .git)
5. Write agent definitions to project root
6. Print summary: "N agent definitions created, M skipped (already exist)"
7. Print "Setup complete!" or "Setup failed" with exit code

### Cobra Subcommand (main.go)

Add `kasmos setup` as a cobra subcommand:
```go
setupCmd := &cobra.Command{
    Use:   "setup",
    Short: "Validate dependencies and scaffold agent configurations",
    RunE: func(cmd *cobra.Command, args []string) error {
        return setup.Run()
    },
}
rootCmd.AddCommand(setupCmd)
```

### Testing

**deps_test.go**: Test with mocked exec.LookPath (use a helper that accepts a
lookup function). Test found/not-found/error cases.

**setup_test.go**: Test agent definition writing to a temp directory:
- Fresh directory: all 4 files created
- Existing files: not overwritten
- File contents match expected templates

## What NOT to Do

- Do NOT create actual OpenCode configuration files (.opencode/config.json etc.)
- Do NOT validate OpenCode version (just check it exists)
- Do NOT make agent templates configurable (fixed templates for MVP)
- Do NOT add this to the TUI startup flow (setup is a standalone command)

## Acceptance Criteria

1. `kasmos setup` runs and prints dependency check results
2. Agent definition files are created in `.opencode/agents/`
3. Existing agent files are NOT overwritten
4. Missing dependencies are reported with install hints
5. Exit code 1 if required deps missing, 0 on success
6. `go test ./internal/setup/...` passes
7. `kasmos setup` is idempotent (running twice produces same result)
