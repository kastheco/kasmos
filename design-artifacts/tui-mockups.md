# kasmos TUI — View Mockups

> ASCII-art reference for every TUI view state. Each mockup shows exact border
> characters (lipgloss.RoundedBorder ╭╮╰╯, ThickBorder ┏┓┗┛), content placement,
> and component boundaries. Implementation should match these layouts character-for-character
> at the specified terminal widths.

## Table of Contents

- [V1: Main Dashboard (split, ≥120 cols)](#v1-main-dashboard)
- [V2: Spawn Worker Dialog (overlay)](#v2-spawn-worker-dialog)
- [V3: Full-Screen Output Viewport](#v3-full-screen-output)
- [V4: Task Source Panel (3-col, ≥160 cols)](#v4-task-source-panel)
- [V5: Continue Session Dialog (overlay)](#v5-continue-session-dialog)
- [V6: Help Overlay](#v6-help-overlay)
- [V7: Worker Continuation Chains](#v7-worker-chains)
- [V8: Narrow/Stacked Layout (<100 cols)](#v8-narrow-layout)
- [V9: AI Failure Analysis](#v9-failure-analysis)
- [V10: Daemon Mode Output](#v10-daemon-mode)
- [V11: Empty Dashboard (fresh launch)](#v11-empty-dashboard)
- [V12: Quit Confirmation Dialog](#v12-quit-confirmation)

---

## V1: Main Dashboard

**Trigger:** Default view after launch with workers present.
**Layout:** 2-column split. Left ~40%, Right ~60%.
**Components:** header (custom), table (bubbles/table, focused), viewport (bubbles/viewport, unfocused), status bar (lipgloss bg), help bar (bubbles/help short mode).
**Terminal:** 130×30 shown. Scales to ≥120 cols.

```
 kasmos  agent orchestrator                                                                    v0.1.0

╭─ Workers ───────────────────────────────────────╮ ╭─ Output: w-002 reviewer ──────────────────────────────────────╮
│                                                 │ │                                                              │
│   ID      Status       Role      Duration Task  │ │ ──────────────────────────────────────────────────────────── │
│  ──────────────────────────────────────────────  │ │ [reviewer] session: ses_a8f3k2                              │
│  w-001   ✓ done       coder     4m 12s   Auth…  │ │ [14:32:01] Reviewing changes in auth/...                    │
│ >w-002   ⣾ running    reviewer  1m 48s   Revi…  │ │ [14:32:03] Found 3 files modified:                          │
│  w-003   ⟳ running    planner   0m 34s   Plan…  │ │   • internal/auth/middleware.go                              │
│  w-004   ✗ failed(1)  coder     2m 01s   Fix …  │ │   • internal/auth/handler.go                                │
│  w-005   ○ pending    release     —      Tag …  │ │   • internal/auth/token.go                                  │
│  w-006   ☠ killed     coder     6m 44s   Refa…  │ │ [14:32:05] middleware.go: LGTM, clean implementation        │
│                                                 │ │ [14:32:07] handler.go: Suggestion: Extract token            │
│                                                 │ │   validation into a separate function for reuse.            │
│                                                 │ │ [14:32:09] token.go: Suggestion: Add expiry check           │
│                                                 │ │   before signature verification to fail fast.               │
│                                                 │ │ [14:32:11] Running test suite...                            │
│                                                 │ │ [14:32:14] ✓ All 42 tests passed                            │
│                                                 │ │ [14:32:15] Verdict: verified with suggestions               │
│                                                 │ │                                                              │
╰─────────────────────────────────────────────────╯ ╰──────────────────────────────────────────────────────────────╯
 ⣾ 2 running  ✓ 1 done  ✗ 1 failed  ☠ 1 killed  ○ 1 pending                          mode: ad-hoc  scroll: 100%
 s spawn · x kill · c continue · r restart · g gen prompt · a analyze · f fullscreen · tab panel · ? help · q quit
```

**Key details:**
- `>` prefix or highlighted row = `table.Selected` row (purple bg + cream fg)
- `⣾` in status column = `spinner.View()` inline for running workers
- Viewport title includes worker ID + role of selected worker
- Status bar: full-width purple background, cream text
- Help bar: purple bold keys, gray descriptions, `·` separators
- Table header: hot pink bold text, purple bottom border via `s.Header.BorderBottom(true)`

---

## V2: Spawn Worker Dialog

**Trigger:** Press `s` from dashboard.
**Layout:** Centered overlay via `lipgloss.Place()`. Fills background with `░` in dark gray.
**Components:** huh.Form with ThemeCharm(). Fields: Select (role), Text (prompt), Input (files), Confirm.
**Dialog:** 70 chars wide. Hot pink RoundedBorder.

```
░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░
░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░
░░░░░░░░░░░╭─ Spawn Worker ──────────────────────────────────────────────────╮░░░░░░░░░░░░░░░░
░░░░░░░░░░░│                                                                │░░░░░░░░░░░░░░░░
░░░░░░░░░░░│  Agent Role                                                    │░░░░░░░░░░░░░░░░
░░░░░░░░░░░│                                                                │░░░░░░░░░░░░░░░░
░░░░░░░░░░░│    ○ planner    Research and planning, read-only filesystem    │░░░░░░░░░░░░░░░░
░░░░░░░░░░░│    ● coder      Implementation, full tool access               │░░░░░░░░░░░░░░░░
░░░░░░░░░░░│    ○ reviewer   Code review, read-only + test execution        │░░░░░░░░░░░░░░░░
░░░░░░░░░░░│    ○ release    Merge, finalization, cleanup operations        │░░░░░░░░░░░░░░░░
░░░░░░░░░░░│                                                                │░░░░░░░░░░░░░░░░
░░░░░░░░░░░│  Prompt                                                        │░░░░░░░░░░░░░░░░
░░░░░░░░░░░│                                                                │░░░░░░░░░░░░░░░░
░░░░░░░░░░░│   ╭──────────────────────────────────────────────────────────╮  │░░░░░░░░░░░░░░░░
░░░░░░░░░░░│   │ Implement the auth middleware as described in           │  │░░░░░░░░░░░░░░░░
░░░░░░░░░░░│   │ WP-003. Use JWT with RS256 signing. Ensure all         │  │░░░░░░░░░░░░░░░░
░░░░░░░░░░░│   │ endpoints in /api/v1/ are protected.▎                  │  │░░░░░░░░░░░░░░░░
░░░░░░░░░░░│   │                                                        │  │░░░░░░░░░░░░░░░░
░░░░░░░░░░░│   ╰──────────────────────────────────────────────────────────╯  │░░░░░░░░░░░░░░░░
░░░░░░░░░░░│                                                                │░░░░░░░░░░░░░░░░
░░░░░░░░░░░│  Attach Files (optional)                                       │░░░░░░░░░░░░░░░░
░░░░░░░░░░░│                                                                │░░░░░░░░░░░░░░░░
░░░░░░░░░░░│   ╭──────────────────────────────────────────────────────────╮  │░░░░░░░░░░░░░░░░
░░░░░░░░░░░│   │ path/to/file.go, another/file.go                       │  │░░░░░░░░░░░░░░░░
░░░░░░░░░░░│   ╰──────────────────────────────────────────────────────────╯  │░░░░░░░░░░░░░░░░
░░░░░░░░░░░│                                                                │░░░░░░░░░░░░░░░░
░░░░░░░░░░░│   [ Spawn! ]     [ Cancel ]                                    │░░░░░░░░░░░░░░░░
░░░░░░░░░░░│                                                                │░░░░░░░░░░░░░░░░
░░░░░░░░░░░╰────────────────────────────────────────────────────────────────╯░░░░░░░░░░░░░░░░
░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░
░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░
```

**Key details:**
- `●` = selected radio (purple), `○` = unselected (midGray)
- Prompt textarea: purple RoundedBorder when focused, darkGray when blurred
- `▎` = cursor in textarea (hot pink)
- `[ Spawn! ]` = active button (purple bg), `[ Cancel ]` = inactive (darkGray bg)
- File input: single-line textinput with comma-separated paths
- Esc dismisses without spawning

---

## V3: Full-Screen Output

**Trigger:** Press `f` on dashboard, or `enter` on a worker.
**Layout:** Viewport fills full terminal minus header (2 lines), status bar (1 line), help bar (1 line).
**Components:** viewport (bubbles/viewport, focused), status bar, help bar.

```
 kasmos  agent orchestrator                                                                                      v0.1.0

╭─ Output: w-001 coder ─ Implement auth middleware ─────────────────────────────────────────────────────────────────╮
│                                                                                                                   │
│ [14:28:01] [coder] Starting task: Implement auth middleware                                                       │
│ [14:28:02] Reading project structure...                                                                           │
│ [14:28:04] Analyzing existing auth patterns in internal/...                                                       │
│ [14:28:06] Creating internal/auth/middleware.go                                                                    │
│ [14:28:08] Writing JWT validation middleware with RS256...                                                         │
│ [14:28:15] Creating internal/auth/handler.go                                                                      │
│ [14:28:22] Creating internal/auth/token.go                                                                        │
│ [14:28:30] Modifying cmd/server/main.go — adding auth middleware to router                                        │
│ [14:28:35] Running go build ./...                                                                                 │
│ [14:28:38] ✓ Build succeeded                                                                                      │
│ [14:28:39] Running go test ./internal/auth/...                                                                    │
│ [14:28:44]   PASS TestMiddleware_ValidToken                                                                       │
│ [14:28:44]   PASS TestMiddleware_ExpiredToken                                                                     │
│ [14:28:44]   PASS TestMiddleware_InvalidSignature                                                                 │
│ [14:28:44]   PASS TestMiddleware_MissingHeader                                                                    │
│ [14:28:45]   PASS TestHandler_Login                                                                               │
│ [14:28:45]   PASS TestHandler_Refresh                                                                             │
│ [14:28:45]   PASS TestToken_Generate                                                                              │
│ [14:28:45]   PASS TestToken_Verify                                                                                │
│ [14:28:46] ✓ All 8 tests passed (7.2s)                                                                            │
│ [14:28:47] Done. Auth middleware implemented and tested.                                                           │
│                                                                                                                   │
│                                                                                                                   │
╰───────────────────────────────────────────────────────────────────────────────────────────────────────────────────╯
 w-001 coder  ✓ done  exit(0)  duration: 4m 12s  session: ses_j4m9x1  parent: —                       scroll: 100%
 esc back · c continue · r restart · j/k scroll · G bottom · g top · / search
```

**Key details:**
- Viewport title includes: worker ID, role, task description (truncated to fit)
- Status bar includes: worker ID, role, status, exit code, duration, session ID, parent reference
- Auto-follow: if `viewport.AtBottom()` is true when new content arrives, call `GotoBottom()`
- If user scrolls up, auto-follow pauses. Status shows current scroll percentage instead of 100%
- `PASS` lines in green, `FAIL` in orange, timestamps in lightGray/faint, filenames in lightBlue

---

## V4: Task Source Panel

**Trigger:** `kasmos kitty-specs/015-auth-overhaul/` or `kasmos tasks.md`
**Layout:** 3-column at ≥160 cols. Tasks ~25%, Workers ~35%, Output ~40%.
**Components:** list (bubbles/list, filterable), table (bubbles/table), viewport (bubbles/viewport).

```
 kasmos  agent orchestrator                                                                                          v0.1.0
  spec-kitty: kitty-specs/015-auth-overhaul/plan.md

╭─ Tasks (6) ──────────────────╮ ╭─ Workers ─────────────────────────────────╮ ╭─ Output ──────────────────────────────────────╮
│                              │ │                                           │ │                                               │
│  WP-001  Auth middleware     │ │   ID     Status      Role     Duration    │ │  Select a worker to view output               │
│  JWT RS256 validation layer  │ │  ─────────────────────────────────────    │ │                                               │
│  deps: none                  │ │  w-001  ✓ done      coder    4m 12s      │ │                                               │
│  ✓ done                      │ │  w-002  ⣾ running   reviewer 1m 48s      │ │                                               │
│                              │ │  w-003  ⟳ running   planner  0m 34s      │ │                                               │
│ >WP-002  Login endpoint      │ │                                           │ │                                               │
│  POST /api/v1/login handler  │ │                                           │ │                                               │
│  deps: WP-001                │ │                                           │ │                                               │
│  ○ unassigned                │ │                                           │ │                                               │
│                              │ │                                           │ │                                               │
│  WP-003  Token refresh       │ │                                           │ │                                               │
│  Refresh token rotation      │ │                                           │ │                                               │
│  deps: WP-001, WP-002       │ │                                           │ │                                               │
│  ○ blocked (WP-002)         │ │                                           │ │                                               │
│                              │ │                                           │ │                                               │
│  WP-004  RBAC middleware     │ │                                           │ │                                               │
│  Role-based access control   │ │                                           │ │                                               │
│  deps: WP-001                │ │                                           │ │                                               │
│  ○ unassigned                │ │                                           │ │                                               │
│                              │ │                                           │ │                                               │
╰──────────────────────────────╯ ╰───────────────────────────────────────────╯ ╰───────────────────────────────────────────────╯
 tasks: 1 done · 1 in-progress · 4 pending     workers: 2 running · 1 done                        mode: spec-kitty  scroll: —
 s spawn from task · enter assign role · / filter tasks · tab switch panel · b batch spawn · ? help · q quit
```

**Key details:**
- Task list uses `bubbles/list` with custom delegate rendering multi-line items
- Task status badges: ✓ done (green bg), ⟳ in-progress (purple bg), ○ unassigned (midGray bg), ⊘ blocked (orange text)
- `>` = selected task in list (purple highlight)
- Pressing `s` on a selected task opens spawn dialog pre-filled with task's suggested role and description
- Pressing `b` opens batch spawn: select multiple tasks, assign roles, spawn all
- `/` activates list's built-in filter using `FilterValue()` on task titles
- At ≥120 but <160 cols: tasks panel collapses, switch to 2-col (workers + output) with task as a toggleable sidebar

---

## V5: Continue Session Dialog

**Trigger:** Press `c` on a completed/exited worker.
**Layout:** Centered overlay, same pattern as spawn dialog.
**Components:** huh.Form with read-only parent info section + textarea for follow-up message.

```
░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░
░░░░░░░░░░░░░╭─ Continue Session ─────────────────────────────────────────────╮░░░░░░
░░░░░░░░░░░░░│                                                               │░░░░░░
░░░░░░░░░░░░░│  Parent Worker                                                │░░░░░░
░░░░░░░░░░░░░│                                                               │░░░░░░
░░░░░░░░░░░░░│  ID:      w-002   Role: reviewer   Status: ✓ done             │░░░░░░
░░░░░░░░░░░░░│  Session: ses_a8f3k2                                          │░░░░░░
░░░░░░░░░░░░░│  Result:  Verified with suggestions                           │░░░░░░
░░░░░░░░░░░░░│                                                               │░░░░░░
░░░░░░░░░░░░░│  Follow-up Message                                            │░░░░░░
░░░░░░░░░░░░░│                                                               │░░░░░░
░░░░░░░░░░░░░│   ╭───────────────────────────────────────────────────────╮    │░░░░░░
░░░░░░░░░░░░░│   │ Apply suggestions 1 and 3. Skip suggestion 2.       │    │░░░░░░
░░░░░░░░░░░░░│   │ For suggestion 1, use a helper function rather      │    │░░░░░░
░░░░░░░░░░░░░│   │ than inline logic.▎                                 │    │░░░░░░
░░░░░░░░░░░░░│   │                                                     │    │░░░░░░
░░░░░░░░░░░░░│   ╰───────────────────────────────────────────────────────╯    │░░░░░░
░░░░░░░░░░░░░│                                                               │░░░░░░
░░░░░░░░░░░░░│   [ Continue ]     [ Cancel ]                                 │░░░░░░
░░░░░░░░░░░░░│                                                               │░░░░░░
░░░░░░░░░░░░░╰───────────────────────────────────────────────────────────────╯░░░░░░
░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░
```

**Key details:**
- Parent info section is read-only styled text (not form fields)
- Role badge uses same color coding as table (coder=purple, reviewer=lightBlue, planner=green, release=purple variant)
- On confirm, spawns: `opencode run --continue -s ses_a8f3k2 "Apply suggestions 1 and 3..."`
- New worker appears in table with parent reference: `├─w-007` under `w-002`

---

## V6: Help Overlay

**Trigger:** Press `?` from any view.
**Layout:** Centered overlay, same backdrop pattern.
**Components:** bubbles/help with `ShowAll = true`, wrapped in hot pink RoundedBorder.

```
░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░
░░░░░░░░░╭─ Keybindings ──────────────────────────────────────────────────────╮░░░░░░
░░░░░░░░░│                                                                   │░░░░░░
░░░░░░░░░│  Navigation              Workers                Output            │░░░░░░
░░░░░░░░░│                                                                   │░░░░░░
░░░░░░░░░│  ↑/k     move up         s       spawn worker   f     fullscreen  │░░░░░░
░░░░░░░░░│  ↓/j     move down       x       kill worker    j/k   scroll     │░░░░░░
░░░░░░░░░│  tab     next panel      c       continue       G     bottom     │░░░░░░
░░░░░░░░░│  S-tab   prev panel      r       restart        g     top        │░░░░░░
░░░░░░░░░│  enter   select          b       batch spawn    /     search     │░░░░░░
░░░░░░░░░│  esc     back/close      g       gen prompt     esc   back       │░░░░░░
░░░░░░░░░│                          a       analyze fail                    │░░░░░░
░░░░░░░░░│                                                                   │░░░░░░
░░░░░░░░░│  General                  Tasks                                   │░░░░░░
░░░░░░░░░│                                                                   │░░░░░░
░░░░░░░░░│  ?       toggle help      /       filter tasks                    │░░░░░░
░░░░░░░░░│  q       quit kasmos     enter   assign + spawn                  │░░░░░░
░░░░░░░░░│  ctrl+c  force quit                                              │░░░░░░
░░░░░░░░░│                                                                   │░░░░░░
░░░░░░░░░│                             press ? or esc to close               │░░░░░░
░░░░░░░░░╰───────────────────────────────────────────────────────────────────╯░░░░░░
░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░
```

**Key details:**
- FullHelp returns `[][]key.Binding` — each inner slice is one column
- Column headers (Navigation, Workers, etc.) are hot pink bold
- Key names are purple bold, descriptions are gray/faint
- This replaces the dashboard — full overlay. Esc returns.

---

## V7: Worker Chains

**Trigger:** When continuation workers exist, the table renders parent-child relationships.
**Layout:** Same as V1 (2-column split). The ID column width expands to accommodate tree glyphs.

```
╭─ Workers ───────────────────────────────────────╮ ╭─ Output: w-007 coder ← w-002 ────────────────────────────────╮
│                                                 │ │                                                              │
│   ID          Status       Role     Duration    │ │ ← continued from w-002 (reviewer)                            │
│  ───────────────────────────────────────────    │ │ ──────────────────────────────────────────────────────────── │
│  w-001       ✓ done       coder    4m 12s       │ │ [14:34:22] Applying suggestions from review...               │
│  w-002       ✓ done       reviewer 3m 20s       │ │ [14:34:24] Suggestion 1: Extracting token                    │
│  ├─w-005    ✓ done       coder    1m 45s       │ │   validation into validateToken() helper                     │
│  │ └─w-006  ✓ done       reviewer 2m 10s       │ │ [14:34:28] Suggestion 3: Adding expiry check                 │
│  └──w-007   ⣾ running    coder    0m 52s       │ │   before signature verification...                           │
│  w-003       ⟳ running    planner  5m 02s       │ │ [14:34:30] Modifying token.go                                │
│  w-004       ✗ failed(1)  coder    2m 01s       │ │ [14:34:33] Running tests...                                  │
│                                                 │ │                                                              │
╰─────────────────────────────────────────────────╯ ╰──────────────────────────────────────────────────────────────╯
```

**Key details:**
- Tree glyphs: `├─` (sibling continues), `└─` (last child), `│ ` (connector), spaces for depth
- Tree glyphs rendered in midGray/faint — not prominent, just structural
- Viewport title shows chain: `w-007 coder ← w-002`
- First line of viewport shows continuation badge: `← continued from w-002 (reviewer)` in lightBlue
- Status bar shows `chain depth: N` when a chained worker is selected
- Workers with children are collapsible (future: press `h` to collapse, `l` to expand)

---

## V8: Narrow Layout

**Trigger:** Terminal width <100 cols.
**Layout:** Stacked — table on top (~50% height), viewport on bottom (~50% height).
**Components:** Same as V1 but JoinVertical instead of JoinHorizontal.

```
 kasmos  agent orchestrator                v0.1.0

╭─ Workers ──────────────────────────────────────╮
│                                                │
│   ID      Status       Role      Duration      │
│  ────────────────────────────────────────────  │
│  w-001   ✓ done       coder     4m 12s         │
│ >w-002   ⣾ running    reviewer  1m 48s         │
│  w-003   ⟳ running    planner   0m 34s         │
│  w-004   ✗ failed(1)  coder     2m 01s         │
│                                                │
╰────────────────────────────────────────────────╯
╭─ Output: w-002 reviewer ──────────────────────╮
│ [14:32:05] Reviewing changes in auth/...       │
│ [14:32:07] Found 3 files modified              │
│ [14:32:09] middleware.go: LGTM                 │
│ [14:32:11] handler.go: Suggestion: Extract...  │
│ [14:32:14] Running test suite...               │
│                                                │
╰────────────────────────────────────────────────╯
 2 running · 1 done · 1 failed    ad-hoc  100%
 s spawn · x kill · c continue · ? help · q quit
```

**Key details:**
- "Task" column hidden (not enough width)
- "Prompt" column hidden
- Help bar shows fewer bindings (only essentials)
- Status bar is more compact

---

## V9: AI Failure Analysis

**Trigger:** Press `a` on a failed worker.
**Layout:** Same 2-column split as V1. Viewport shows analysis results instead of raw output.
**Components:** viewport content is formatted analysis from on-demand AI helper.

```
╭─ Workers ─────────────────────────────────╮ ╭─ Analysis: w-004 coder ────────────────────────────────────────╮
│                                           │ │                                                                │
│  (table content same as V1)               │ │ 🔍 Failure Analysis                                            │
│                                           │ │ ──────────────────────────────────────────────────────────────  │
│ >w-004   ✗ failed(1)  coder   2m 01s     │ │                                                                │
│                                           │ │ Root Cause: Compilation error in                               │
│                                           │ │ internal/auth/validator.go — undefined                         │
│                                           │ │ reference to ValidateCredentials function.                     │
│                                           │ │                                                                │
│                                           │ │ The function was renamed to CheckCredentials                   │
│                                           │ │ in commit a3f8e21 but the worker's codebase                   │
│                                           │ │ snapshot predates this change.                                 │
│                                           │ │                                                                │
│                                           │ │ Suggested Fix:                                                 │
│                                           │ │ Restart with updated prompt: "Fix login                        │
│                                           │ │ validation. Note: ValidateCredentials was                      │
│                                           │ │ renamed to CheckCredentials in the latest                      │
│                                           │ │ main branch. Use the new function name."                       │
│                                           │ │                                                                │
│                                           │ │ Press r to restart with suggested prompt                       │
╰───────────────────────────────────────────╯ ╰────────────────────────────────────────────────────────────────╯
                                                                                       analysis complete  100%
 r restart with suggestion · c continue · esc dismiss · ? help · q quit
```

**Key details:**
- While AI is analyzing, viewport shows spinner: `⣾ Analyzing failure...`
- "Root Cause:" label in orange bold, "Suggested Fix:" in green bold
- File/function references in lightBlue
- Commit refs in lightBlue
- Press `r` → opens restart dialog with suggested prompt pre-filled
- Press `esc` → returns to normal output view for that worker
- The analysis viewport title says "Analysis:" not "Output:"

---

## V10: Daemon Mode

**Trigger:** `kasmos -d` or non-interactive terminal detected.
**Layout:** No TUI. Pure stdout lines.
**Components:** None (WithoutRenderer). JSON events to stdout.

```
$ kasmos -d --tasks tasks.md --spawn-all --format json
{"ts":"2026-02-17T14:28:00Z","event":"session_start","mode":"gsd","source":"tasks.md","tasks":4}
{"ts":"2026-02-17T14:28:01Z","event":"worker_spawn","id":"w-001","role":"coder","task":"Implement auth"}
{"ts":"2026-02-17T14:28:01Z","event":"worker_spawn","id":"w-002","role":"coder","task":"Fix login flow"}
{"ts":"2026-02-17T14:28:01Z","event":"worker_spawn","id":"w-003","role":"reviewer","task":"Review PR #42"}
{"ts":"2026-02-17T14:28:01Z","event":"worker_spawn","id":"w-004","role":"planner","task":"Plan DB schema"}
{"ts":"2026-02-17T14:30:12Z","event":"worker_exit","id":"w-003","code":0,"duration":"2m11s","session":"ses_k2m9"}
{"ts":"2026-02-17T14:32:14Z","event":"worker_exit","id":"w-001","code":0,"duration":"4m13s","session":"ses_j4m9"}
{"ts":"2026-02-17T14:33:01Z","event":"worker_exit","id":"w-004","code":0,"duration":"5m00s","session":"ses_m7x2"}
{"ts":"2026-02-17T14:34:02Z","event":"worker_exit","id":"w-002","code":1,"duration":"6m01s","session":"ses_p1q3"}
{"ts":"2026-02-17T14:34:02Z","event":"session_end","total":4,"passed":3,"failed":1,"duration":"6m02s","exit_code":1}
$ echo $?
1
```

**Key details:**
- One JSON object per line (NDJSON)
- Timestamps are RFC3339
- `session_end` event includes aggregate stats
- Process exit code = 0 if all workers passed, 1 if any failed
- `--format default` outputs human-readable instead of JSON (simpler one-liners)

---

## V11: Empty Dashboard

**Trigger:** Fresh `kasmos` launch with no arguments and no prior session.
**Layout:** Same 2-column split as V1, but with empty states.

```
 kasmos  agent orchestrator                                                                    v0.1.0

╭─ Workers ───────────────────────────────────────╮ ╭─ Output ─────────────────────────────────────────────────────╮
│                                                 │ │                                                              │
│                                                 │ │                                                              │
│                                                 │ │                                                              │
│                                                 │ │               🫧 Welcome to kasmos!                           │
│          No workers yet                         │ │                                                              │
│                                                 │ │    Spawn your first worker to get started.                   │
│     Press s to spawn your first worker          │ │    Select a worker to view its output here.                  │
│                                                 │ │                                                              │
│                                                 │ │    Tip: Run kasmos setup to scaffold                         │
│                                                 │ │    agent configurations if you haven't yet.                  │
│                                                 │ │                                                              │
│                                                 │ │                                                              │
│                                                 │ │                                                              │
│                                                 │ │                                                              │
│                                                 │ │                                                              │
╰─────────────────────────────────────────────────╯ ╰──────────────────────────────────────────────────────────────╯
 0 workers                                                                                    mode: ad-hoc  scroll: —
 s spawn · ? help · q quit
```

**Key details:**
- Empty table shows centered dim text: "No workers yet" and "Press s to spawn your first worker"
- Viewport shows welcome message with 🫧 emoji (the Charm bubblegum signature)
- Help bar only shows available actions (spawn, help, quit) — context-dependent keys are disabled
- "kasmos setup" in lightBlue in the welcome text

---

## V12: Quit Confirmation

**Trigger:** Press `q` while workers are still running.
**Layout:** Small centered dialog. Uses ThickBorder (┏┓┗┛) for urgency.
**Components:** huh.Confirm or custom dialog.

```
░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░
░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░
░░░░░░░░░░░░┏━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┓░░░░░░░░░
░░░░░░░░░░░░┃                                 ┃░░░░░░░░░
░░░░░░░░░░░░┃   ⚠ Quit kasmos?                ┃░░░░░░░░░
░░░░░░░░░░░░┃                                 ┃░░░░░░░░░
░░░░░░░░░░░░┃   2 workers are still running.  ┃░░░░░░░░░
░░░░░░░░░░░░┃   They will be terminated.      ┃░░░░░░░░░
░░░░░░░░░░░░┃                                 ┃░░░░░░░░░
░░░░░░░░░░░░┃   [ Force Quit ]  [ Cancel ]    ┃░░░░░░░░░
░░░░░░░░░░░░┃                                 ┃░░░░░░░░░
░░░░░░░░░░░░┗━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┛░░░░░░░░░
░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░
░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░
```

**Key details:**
- ThickBorder (┏━┓┃┗━┛) in orange — the only view that uses ThickBorder
- `⚠` in orange bold
- "Force Quit" button in orange bg, "Cancel" in darkGray bg
- If no workers running, pressing `q` exits immediately without this dialog
- Force Quit: SIGTERM all workers → 3s grace → SIGKILL → persist state → exit
- Esc or Cancel returns to dashboard
