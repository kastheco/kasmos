---
work_package_id: WP15
title: History View (h key)
lane: planned
dependencies:
- WP03
- WP08
- WP13
subtasks:
- Add h key binding to keyMap
- Session archiving on exit (move session.json to sessions/{id}.json)
- History scanner (kitty-specs, GSD files, archived sessions)
- History overlay with unified list
- Detail view for selected history entry
- Load from history as active task source
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

# Work Package Prompt: WP15 - History View (`h` key)

## Mission

Implement a history overlay triggered by the `h` key that shows all past work
across all three source types: spec-kitty features, GSD task files, and archived
ad-hoc sessions. Users can browse history, view details, and reload a past source
into the active dashboard.

## Scope

### Files to Create

```
internal/tui/history.go       # History overlay model, scanning, rendering
internal/history/scanner.go   # History entry scanning across all source types
internal/history/scanner_test.go
```

### Files to Modify

```
internal/tui/keys.go          # Add History key binding (h)
internal/tui/model.go         # History overlay state fields
internal/tui/update.go        # History message handlers, key routing
internal/tui/messages.go      # History messages
internal/tui/styles.go        # History entry styles (type badges, status colors)
internal/persist/session.go   # Archive session on close, list archived sessions
internal/persist/schema.go    # FinishedAt field for archive metadata
```

## Implementation

### Session Archiving

Currently `.kasmos/session.json` is a single file overwritten each save. To
support ad-hoc history, archive sessions when they end.

Add to `SessionPersister`:

```go
func (p *SessionPersister) Archive(state SessionState) error {
    // Write to .kasmos/sessions/{session_id}.json
    // Add FinishedAt timestamp to the state
}

func (p *SessionPersister) ListArchived() ([]SessionState, error) {
    // Glob .kasmos/sessions/*.json, parse each, return sorted by StartedAt desc
}
```

Add `FinishedAt` to `SessionState`:

```go
type SessionState struct {
    // ... existing fields ...
    FinishedAt *time.Time `json:"finished_at,omitempty"`
}
```

Archive trigger: when the TUI exits cleanly (quit confirmed or daemon complete),
call `p.Archive(finalState)` before exit. This preserves the session for history.

Directory structure:

```
.kasmos/
  session.json              # current/active session (existing)
  sessions/
    ks-1708123456-a1b2.json # archived sessions
    ks-1708234567-c3d4.json
```

### History Scanner (`internal/history/`)

A standalone package that scans all three source types and returns a unified list:

```go
package history

type EntryType string

const (
    EntrySpecKitty EntryType = "spec-kitty"
    EntryGSD       EntryType = "gsd"
    EntryAdHoc     EntryType = "ad-hoc"
)

type Entry struct {
    Type        EntryType
    Name        string    // feature slug, filename, or session ID
    Path        string    // directory or file path
    Date        time.Time // creation or last activity date
    Status      string    // "complete", "in-progress", "planned", etc.
    TaskCount   int       // total tasks/WPs
    DoneCount   int       // completed tasks/WPs
    WorkerCount int       // workers spawned (ad-hoc sessions)
    Summary     string    // one-line description
}

func Scan(specsRoot string, kasmosDir string) ([]Entry, error)
```

**Spec-kitty scanning**:
- Glob `kitty-specs/*/tasks/WP*.md`
- Group by parent feature directory
- For each feature: read WP frontmatter to count total/done tasks
- Feature name: directory basename (e.g. `016-kasmos-agent-orchestrator`)
- Status: "complete" if all WPs are `lane: done`, "in-progress" if any are
  `lane: doing`, "planned" otherwise
- Date: use git commit date of the feature directory or file mtime as fallback

**GSD scanning**:
- Glob common locations: `*.md`, `tasks/*.md`, `todo/*.md`
- For each file: attempt to parse as GSD (look for `- [ ]` / `- [x]` lines)
- Skip files with 0 checkbox lines (not a GSD file)
- Count total/done from checkboxes
- Date: file mtime
- Skip the file currently loaded as active task source (it's "current", not history)

**Ad-hoc scanning**:
- List `.kasmos/sessions/*.json`
- Parse each for session metadata
- Name: session ID
- Worker count / status from worker snapshots
- Status: "complete" if all workers exited/done, "partial" otherwise
- Date: `started_at` from session state
- Skip active session (matching current PID)

### Key Binding

Add `History` to keyMap:

```go
History: key.NewBinding(
    key.WithKeys("h"),
    key.WithHelp("h", "history"),
),
```

Enable when no overlay is active. Disable in fullscreen mode.
Add to ShortHelp and FullHelp.

### History Overlay

Full-screen overlay (like help, but with interactive list):

```
 History
 ───────

   TYPE          NAME                              DATE         STATUS       PROGRESS
 > spec-kitty   016-kasmos-agent-orchestrator      Feb 18       complete     13/13 WPs
   gsd           api-tasks.md                      Feb 15       in-progress   4/7 tasks
   ad-hoc        ks-1708123456-a1b2                Feb 12       complete      3 workers
   spec-kitty   015-auth-refactor                  Feb 10       planned       0/8 WPs
   gsd           bugfixes.md                       Feb 08       complete      5/5 tasks
   ad-hoc        ks-1708012345-e5f6                Feb 05       partial       2 workers

 ↑/↓ select · enter load as source · d detail · esc close
```

Model fields:

```go
showHistory      bool
historyEntries   []history.Entry
historySelected  int
historyDetail    bool    // showing detail view for selected entry
historyLoading   bool    // scanning in progress
```

### Rendering

Each entry row:
- **Type badge**: colored label using existing `roleBadge` pattern
  - `spec-kitty` = magenta
  - `gsd` = cyan
  - `ad-hoc` = yellow
- **Name**: truncated to fit column width
- **Date**: relative or short format (e.g. "Feb 18", "2d ago")
- **Status badge**: colored like `taskStatusBadge`
  - `complete` = green
  - `in-progress` = blue
  - `planned` = gray
  - `partial` = yellow
- **Progress**: "N/M WPs", "N/M tasks", or "N workers"

Use `j`/`k` or `up`/`down` to navigate.

### Detail View

Press `d` or `enter` on a selected entry to show details:

**Spec-kitty detail**:
```
 016-kasmos-agent-orchestrator
 ─────────────────────────────
 Type:     spec-kitty
 Path:     kitty-specs/016-kasmos-agent-orchestrator
 Status:   complete (13/13 WPs)

 Work Packages:
   WP01  Project Bootstrap              done
   WP02  Worker Backend                 done
   WP03  TUI Foundation                 done
   ...

 enter load as source · esc back
```

**GSD detail**:
```
 api-tasks.md
 ────────────
 Type:     gsd
 Path:     tasks/api-tasks.md
 Status:   in-progress (4/7 tasks)

 Tasks:
   [x] Set up database schema
   [x] Implement user auth
   [ ] Write API tests
   ...

 enter load as source · esc back
```

**Ad-hoc detail**:
```
 ks-1708123456-a1b2
 ───────────────────
 Type:     ad-hoc session
 Started:  Feb 12, 2026 14:30
 Workers:  3 (2 exited, 1 failed)

 Workers:
   w-001  coder     exited   2m30s
   w-002  reviewer  exited   1m15s
   w-003  coder     failed   0m45s

 enter load as source · esc back
```

### Load from History

Press `enter` on a history entry to load it:

- **Spec-kitty**: call `m.swapTaskSource(&task.SpecKittySource{Dir: entry.Path})`
  (uses the `swapTaskSource` helper from WP14)
- **GSD**: call `m.swapTaskSource(&task.GsdSource{FilePath: entry.Path})`
- **Ad-hoc**: load the archived session via `--attach` flow — restore workers from
  the session file. This is more complex: call `persist.Load()` on the archived
  session, restore worker snapshots, reset counter.

Close the history overlay after loading.

If WP14's `swapTaskSource` is not yet implemented, add it here with the same logic.

### Messages

```go
type historyScanCompleteMsg struct {
    Entries []history.Entry
    Err     error
}

type historyLoadMsg struct {
    Entry history.Entry
}
```

### Commands

```go
func historyScanCmd(specsRoot, kasmosDir string) tea.Cmd {
    return func() tea.Msg {
        entries, err := history.Scan(specsRoot, kasmosDir)
        return historyScanCompleteMsg{Entries: entries, Err: err}
    }
}
```

Scanning runs async via tea.Cmd so it doesn't block the TUI.

### Integration with Update Loop

In the main `Update()`:
- If `m.showHistory`, route keys to `updateHistoryKeys(msg)`
- Handle `historyScanCompleteMsg` to populate entries
- Handle `historyLoadMsg` to swap source or restore session

Key routing for history overlay:
```go
if m.showHistory {
    return m.updateHistoryKeys(msg)
}
```

Place this check early in `Update()`, after quit confirm and before spawn dialog,
since history is a full-screen overlay that captures all input.

## What NOT to Do

- Do NOT deep-scan the entire filesystem for GSD files — only check common
  locations relative to the project root (`.`, `tasks/`, `todo/`, `docs/`)
- Do NOT parse git history for dates if mtime is available — keep it fast
- Do NOT delete archived sessions from the history view
- Do NOT show the currently active session/source in history (it's already loaded)
- Do NOT make history scanning blocking — always async via tea.Cmd
- Do NOT implement search/filter in the first version — just a scrollable list

## Acceptance Criteria

1. Press `h` — spinner shows briefly, then history overlay appears with entries
   from all three source types
2. Spec-kitty features show correct WP counts and completion status
3. GSD files show checkbox task counts
4. Archived ad-hoc sessions show worker counts and status
5. Press `enter` on a spec-kitty entry — loads as task source, history closes
6. Press `enter` on a GSD entry — loads as task source, history closes
7. Press `d` on any entry — shows detail view
8. `esc` closes detail or history overlay
9. `h` disabled when other overlays are active
10. Sessions are archived to `.kasmos/sessions/` on clean exit
11. `go test ./...` passes (including history scanner tests)
12. `go build ./cmd/kasmos` passes
