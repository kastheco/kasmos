---
work_package_id: WP08
title: Task Source Framework + Adapters
lane: doing
dependencies:
- WP02
subtasks:
- internal/task/source.go - Source interface, Task struct, TaskState enum
- internal/task/speckitty.go - SpecKittySource (YAML frontmatter parser)
- internal/task/gsd.go - GsdSource (checkbox markdown parser)
- internal/task/adhoc.go - AdHocSource (empty/noop)
- 'CLI argument parsing: detect source type from path'
- Unit tests for all adapters
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
- timestamp: '2026-02-18T14:12:30.527335431+00:00'
  lane: doing
  actor: manager
  shell_pid: '472734'
  action: transition active (Launching WP08 coder - task source framework + adapters)
---

# Work Package Prompt: WP08 - Task Source Framework + Adapters

## Mission

Implement the complete `internal/task/` package: the Source interface, Task domain
type, and all three task source adapters (spec-kitty, GSD, ad-hoc). Also add CLI
argument parsing to detect and load the appropriate source. This package has no
TUI dependency -- the UI integration happens in WP09.

## Scope

### Files to Create

```
internal/task/source.go       # Source interface, Task struct, TaskState enum
internal/task/speckitty.go    # SpecKittySource (reads plan.md + WP frontmatter)
internal/task/gsd.go          # GsdSource (reads checkbox markdown)
internal/task/adhoc.go        # AdHocSource (empty source)
internal/task/source_test.go
internal/task/speckitty_test.go
internal/task/gsd_test.go
```

### Files to Modify

```
cmd/kasmos/main.go            # Accept positional arg for task source path
internal/tui/model.go         # Add taskSource field (Source interface)
```

### Technical References

- `kitty-specs/016-kasmos-agent-orchestrator/research/tui-technical.md`:
  - **Section 4**: Source interface, Task struct, TaskState enum (lines 560-614)
  - **Section 4**: SpecKittySource implementation notes (lines 617-661)
  - **Section 4**: GsdSource implementation notes (lines 663-686)
  - **Section 4**: AdHocSource implementation notes (lines 688-699)
  - **Section 2**: tasksLoadedMsg, taskStateChangedMsg (lines 368-384)
- `kitty-specs/016-kasmos-agent-orchestrator/data-model.md`:
  - Task entity fields and state machine (lines 54-78)
  - Source interface (lines 131-140)
- `kitty-specs/016-kasmos-agent-orchestrator/tasks/README.md`:
  - WP file format with YAML frontmatter (lines 17-45)
  - Valid lane values (lines 48-53)

## Implementation

### source.go

Define the Source interface and Task types exactly from tui-technical.md Section 4:

```go
type Source interface {
    Type() string              // "spec-kitty", "gsd", "ad-hoc"
    Path() string              // file/directory path (empty for ad-hoc)
    Load() ([]Task, error)     // parse and return tasks
    Tasks() []Task             // cached from last Load()
}
```

Task struct: ID, Title, Description, SuggestedRole, Dependencies, State, WorkerID, Metadata.

TaskState enum: TaskUnassigned, TaskBlocked, TaskInProgress, TaskDone, TaskFailed.

Add helper: `DetectSourceType(path string) (Source, error)` that:
- If path is empty: return AdHocSource
- If path is a directory containing plan.md or tasks/*.md: return SpecKittySource
- If path is a .md file with checkboxes: return GsdSource
- Otherwise: return error with helpful message

### speckitty.go

Parse a spec-kitty feature directory:

1. Walk `{dir}/tasks/WP*.md` files
2. For each file, split on `---` frontmatter delimiters
3. Parse YAML frontmatter using `gopkg.in/yaml.v3`
4. Extract fields from frontmatter (matching tasks/README.md format):
   - `work_package_id` -> Task.ID
   - `title` -> Task.Title
   - `dependencies` -> Task.Dependencies
   - `lane` -> Task.State (planned->Unassigned, doing->InProgress, for_review->InProgress, done->Done)
   - `phase` -> Task.Metadata["phase"]
   - `subtasks` -> Task.Metadata["subtasks"] (comma-joined)
5. Body after frontmatter -> Task.Description
6. Infer Task.SuggestedRole from phase metadata:
   - Phase contains "spec" or "clarifying" -> "planner"
   - Phase contains "implementation" -> "coder"
   - Phase contains "review" -> "reviewer"
   - Phase contains "release" -> "release"
   - Default: "" (user selects)

YAML frontmatter struct:
```go
type wpFrontmatter struct {
    WorkPackageID string   `yaml:"work_package_id"`
    Title         string   `yaml:"title"`
    Lane          string   `yaml:"lane"`
    Dependencies  []string `yaml:"dependencies"`
    Subtasks      []string `yaml:"subtasks"`
    Phase         string   `yaml:"phase"`
}
```

**Dependency resolution**: After loading all tasks, check Dependencies. If any
dependency's Task.State is not TaskDone, set this task to TaskBlocked.

### gsd.go

Parse a simple markdown file with checkboxes:

1. Read the file line by line
2. Match lines against `^- \[( |x)\] (.+)$` regex
3. For each match:
   - `[ ]` -> TaskUnassigned, `[x]` -> TaskDone
   - Task.ID = `T-NNN` (sequential)
   - Task.Title = checkbox text
   - Task.Description = same as title
   - Task.SuggestedRole = "" (user selects)
   - Task.Dependencies = [] (GSD doesn't track deps)
4. Ignore non-checkbox lines

### adhoc.go

Zero-value source:
```go
type AdHocSource struct{}
func (s *AdHocSource) Type() string           { return "ad-hoc" }
func (s *AdHocSource) Path() string           { return "" }
func (s *AdHocSource) Load() ([]Task, error)  { return nil, nil }
func (s *AdHocSource) Tasks() []Task          { return nil }
```

### CLI Argument Parsing (main.go)

Add to cobra root command:
```go
cmd.Args = cobra.MaximumNArgs(1) // optional: path to task source
```

In the run function:
1. If arg provided: `source, err := task.DetectSourceType(arg)`
2. If no arg: `source = &task.AdHocSource{}`
3. Call `source.Load()` to parse tasks
4. Pass source to `tui.NewModel(backend, source)`

### Testing

**speckitty_test.go**: Create testdata directory with sample WP files (matching
the frontmatter format from tasks/README.md). Test:
- Single WP file parsing
- Multiple WP files with dependencies
- Dependency resolution (blocked state)
- Role inference from phase
- Missing/malformed frontmatter handling
- Invalid YAML graceful error

**gsd_test.go**: Test with sample markdown:
```markdown
- [ ] Implement auth
- [x] Review PR
- [ ] Deploy
```
Test: correct count, state mapping, ID generation, non-checkbox line skipping.

**source_test.go**: Test DetectSourceType with various paths.

## What NOT to Do

- Do NOT implement the task panel UI (WP09)
- Do NOT implement batch spawning (WP09)
- Do NOT implement task state updates from worker events (WP09)
- Do NOT implement tasksLoadedMsg handling in TUI (WP09)
- This WP is pure data: parse files, return Task structs

## Acceptance Criteria

1. `go test ./internal/task/...` passes with all tests green
2. SpecKittySource correctly parses WP files matching the frontmatter format
3. GsdSource correctly parses checkbox markdown
4. DetectSourceType correctly identifies source type from path
5. Dependency resolution marks blocked tasks correctly
6. CLI accepts optional positional arg: `kasmos path/to/source`
7. Running `kasmos kitty-specs/016-kasmos-agent-orchestrator/` loads WPs (even
   though the task panel UI isn't built yet -- source loads without error)
8. `go vet ./...` clean, `go test -race ./internal/task/...` clean
