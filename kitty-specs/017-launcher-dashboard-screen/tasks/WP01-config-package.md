---
work_package_id: WP01
title: Config Package
lane: planned
dependencies: []
subtasks:
- 'Config struct with AgentConfig per role (planner, coder, reviewer, release)'
- 'Each AgentConfig has model (string) and reasoning (string) fields'
- 'Top-level default_task_source field (spec-kitty, gsd, or yolo)'
- 'Load() reads from .kasmos/config.toml, returns defaults if file missing'
- 'Save() writes config to .kasmos/config.toml, creates .kasmos/ dir if needed'
- 'DefaultConfig() returns sensible defaults'
- 'Tests: load existing, load missing (defaults), save + reload round-trip, corrupt file handling'
phase: Wave 1 - Foundation
assignee: ''
agent: ''
shell_pid: ''
review_status: ''
reviewed_by: ''
history:
- timestamp: '2026-02-18T00:00:00Z'
lane: done
  agent: planner
  action: Prompt generated via /spec-kitty.tasks
---

# Work Package Prompt: WP01 - Config Package

## Mission

Create `internal/config/` for TOML-based configuration. It must load and save
`.kasmos/config.toml` with per-agent-role model settings and default task source.

## Scope
### Files to Create / Modify

```text
internal/config/config.go
internal/config/config_test.go
go.mod
```

### Technical References

- `kitty-specs/017-launcher-dashboard-screen/plan.md`
- `internal/persist/session.go` (path and file I/O conventions)
- `internal/task/source.go` (task source values)

## Implementation

Define and implement:

```go
type Config struct {
    DefaultTaskSource string                 `toml:"default_task_source"`
    Agents            map[string]AgentConfig `toml:"agents"`
}

type AgentConfig struct {
    Model     string `toml:"model"`
    Reasoning string `toml:"reasoning"`
}
```

Use this TOML shape:

```toml
default_task_source = "yolo"

[agents.planner]
model = "claude-opus-4-6"
reasoning = "high"

[agents.coder]
model = "claude-sonnet-4"
reasoning = "default"

[agents.reviewer]
model = "claude-opus-4-6"
reasoning = "high"

[agents.release]
model = "claude-sonnet-4"
reasoning = "default"
```

Requirements:
- Add `github.com/pelletier/go-toml/v2` to `go.mod`
- `DefaultConfig()` returns sensible defaults for roles + default task source
- `Load()` reads `.kasmos/config.toml`; missing file returns defaults without error
- `Save()` writes `.kasmos/config.toml` and creates `.kasmos/` if missing
- Corrupt TOML returns a wrapped error
- Keep API focused and table-testable

## Verification

- `go test ./internal/config -run Test`
- `go test ./...`
- Confirm tests cover: existing load, missing load -> defaults, save/reload round-trip, corrupt file handling
