---
work_package_id: WP14
title: New Spec/Plan Dialog (n key)
lane: planned
dependencies:
- WP03
- WP08
subtasks:
- Add n key binding to keyMap
- Type picker overlay (Feature Spec / GSD / Planning)
- Feature Spec form (slug, mission) -> spec-kitty agent feature create-feature
- GSD form (filename, initial tasks) -> write checkbox .md file
- Planning form (title, description) -> create freeform planning doc
- Auto-load new source into dashboard after creation
- specCreateCmd for subprocess execution of spec-kitty
phase: Wave 4 - Dashboard Enhancements
assignee: ''
agent: ''
shell_pid: ''
review_status: ''
reviewed_by: ''
history:
- timestamp: '2026-02-18T00:00:00Z'
  lane: planned
  agent: planner
  action: Specified by user request
---

# Work Package Prompt: WP14 - New Spec/Plan Dialog (`n` key)

## Mission

Implement a two-stage dialog triggered by the `n` key that lets users create new
specs and plans without leaving the dashboard. Stage 1 is a type picker (Feature
Spec, GSD Task List, Planning). Stage 2 is a type-specific form that creates the
artifact and optionally loads it as the active task source.

## Scope

### Files to Create

```
internal/tui/newdialog.go    # Type picker model + type-specific form models
```

### Files to Modify

```
internal/tui/keys.go         # Add New key binding (n)
internal/tui/model.go        # New dialog state fields
internal/tui/update.go       # New dialog message handlers, key routing
internal/tui/messages.go     # New dialog messages
internal/tui/commands.go     # specCreateCmd, gsdCreateCmd
internal/tui/styles.go       # Picker/form styles (reuse existing dialog palette)
```

### External Dependencies

- `spec-kitty agent feature create-feature <slug> --mission <mission> --json`
  Creates a feature directory under `kitty-specs/` and returns JSON with the path.
- Available missions (from `spec-kitty mission list`):
  - `software-dev` — Software Dev Kitty
  - `documentation` — Documentation Kitty
  - `research` — Deep Research Kitty

## Implementation

### Key Binding

Add `New` to keyMap:

```go
New: key.NewBinding(
    key.WithKeys("n"),
    key.WithHelp("n", "new spec/plan"),
),
```

Enable when no overlay is active. Add to ShortHelp and FullHelp.

### Stage 1: Type Picker Overlay

A simple vertical list of 3 options rendered as a centered overlay (same pattern
as quit confirm dialog):

```
 New Spec/Plan
 ─────────────
 > Feature Spec    create a spec-kitty feature with research + planning
   GSD Task List   create a checkbox task markdown file
   Planning Doc    create a freeform planning document

 ↑/↓ select · enter confirm · esc cancel
```

Model fields:

```go
showNewDialog     bool
newDialogStage    int          // 0 = picker, 1 = form
newDialogType     string       // "feature-spec", "gsd", "planning"
newDialogPicker   int          // selected index in picker (0-2)
newForm           *newFormModel // stage 2 form (type depends on newDialogType)
```

Key handling in picker:
- `up`/`down` or `j`/`k` — move selection
- `enter` — advance to stage 2 form
- `esc` — cancel and close

### Stage 2a: Feature Spec Form

Uses raw textinput components (same pattern as spawn dialog):

```
 New Feature Spec
 ────────────────
 Slug:    [my-new-feature_____________]
 Mission: [software-dev_______________]

 Slug is the feature identifier (e.g. "user-auth", "api-refactor").
 Mission: software-dev | documentation | research

 enter submit · tab next field · esc cancel
```

Fields:
- **Slug** (required): textinput, validated non-empty, kebab-case
- **Mission** (required): textinput with default "software-dev", validated against
  known missions (`software-dev`, `documentation`, `research`)

On submit:
1. Run `spec-kitty agent feature create-feature <slug> --mission <mission> --json`
   via a tea.Cmd (subprocess, like analyzeCmd pattern)
2. Parse JSON response for the created feature path
3. Emit `specCreatedMsg{Path, Slug, Err}`
4. On success: load the new feature dir as a spec-kitty task source, replacing
   the current source. Update `m.taskSource`, `m.taskSourceType`, `m.taskSourcePath`,
   `m.loadedTasks`. Recalculate layout (task panel may appear/disappear).
5. On error: show error in viewport

### Stage 2b: GSD Task List Form

```
 New GSD Task List
 ─────────────────
 Filename: [tasks.md______________________]
 Tasks:    (one per line, blank to finish)
 ┌──────────────────────────────────────────┐
 │ Set up database schema                   │
 │ Implement user authentication            │
 │ Write API endpoint tests                 │
 │                                          │
 └──────────────────────────────────────────┘

 enter submit · tab next field · esc cancel
```

Fields:
- **Filename** (required): textinput, default "tasks.md"
- **Tasks** (required): textarea, one task per line

On submit:
1. Write the file to disk as checkbox markdown:
   ```markdown
   - [ ] Set up database schema
   - [ ] Implement user authentication
   - [ ] Write API endpoint tests
   ```
2. Emit `gsdCreatedMsg{Path, TaskCount, Err}`
3. On success: load the new file as a GSD task source

### Stage 2c: Planning Doc Form

```
 New Planning Doc
 ────────────────
 Filename: [plan.md_______________________]
 Title:    [API Refactor Plan_____________]
 Content:  (freeform planning notes)
 ┌──────────────────────────────────────────┐
 │ Goals:                                   │
 │ - Migrate to v3 API endpoints            │
 │ - Deprecate legacy handlers              │
 │                                          │
 └──────────────────────────────────────────┘

 enter submit · tab next field · esc cancel
```

Fields:
- **Filename** (required): textinput, default "plan.md"
- **Title** (required): textinput
- **Content** (optional): textarea

On submit:
1. Write a markdown file with `# {Title}` header and content body
2. Emit `planCreatedMsg{Path, Err}`
3. Planning docs do NOT auto-load as task source (they're freeform reference docs)
4. Show confirmation in viewport: "Created plan.md"

### Messages

```go
type newDialogPickedMsg struct{ Type string }
type newDialogCancelledMsg struct{}

type specCreatedMsg struct {
    Slug string
    Path string
    Err  error
}

type gsdCreatedMsg struct {
    Path      string
    TaskCount int
    Err       error
}

type planCreatedMsg struct {
    Path string
    Err  error
}
```

### Commands

```go
func specCreateCmd(slug, mission string) tea.Cmd {
    // exec: spec-kitty agent feature create-feature <slug> --mission <mission> --json
    // parse JSON output for feature path
}

func gsdCreateCmd(path string, tasks []string) tea.Cmd {
    // write checkbox markdown to path
}

func planCreateCmd(path, title, content string) tea.Cmd {
    // write markdown to path
}
```

### Source Hot-Swap

When a new source is created and loaded, the Model needs to:
1. Replace `m.taskSource` with the new Source
2. Reset `m.loadedTasks`, `m.selectedTaskIdx`, `m.taskSourceType`, `m.taskSourcePath`
3. Call `m.recalculateLayout()` (task panel may now appear in wide mode)
4. Call `m.updateKeyStates()`
5. Trigger persist

Add a helper method:

```go
func (m *Model) swapTaskSource(source task.Source) {
    m.taskSource = source
    m.taskSourceType = source.Type()
    m.taskSourcePath = source.Path()
    m.selectedTaskIdx = 0
    if source.Type() != "ad-hoc" {
        if tasks, err := source.Load(); err == nil {
            m.loadedTasks = tasks
        }
    } else {
        m.loadedTasks = nil
    }
    m.recalculateLayout()
    m.updateKeyStates()
    m.triggerPersist()
}
```

## What NOT to Do

- Do NOT call `spec-kitty specify` interactively — use `agent feature create-feature`
  which is the programmatic/non-interactive API
- Do NOT implement mission switching after creation — just set it at creation time
- Do NOT validate slug format beyond non-empty (spec-kitty handles validation)
- Do NOT block the TUI during subprocess execution — use tea.Cmd async pattern
- Do NOT auto-run `spec-kitty research` after feature creation — that's a separate
  workflow the user triggers manually or via a worker

## Acceptance Criteria

1. Press `n` — type picker overlay appears with 3 options
2. Select "Feature Spec", fill slug + mission, submit — spec-kitty creates the feature
   dir and it loads as task source
3. Select "GSD Task List", fill filename + tasks, submit — markdown file created and
   loaded as GSD source
4. Select "Planning Doc", fill fields, submit — markdown file created, confirmation shown
5. `esc` at any stage cancels and closes the dialog
6. Error from spec-kitty CLI shown in viewport (not a crash)
7. `n` disabled when any other overlay is active
8. `go test ./...` passes
9. `go build ./cmd/kasmos` passes
