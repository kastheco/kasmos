---
work_package_id: WP04
title: Worker-TUI Integration (Spawn + Output + Lifecycle)
lane: done
dependencies:
- WP02
- WP03
subtasks:
- internal/tui/commands.go - spawnWorkerCmd, readOutputCmd, waitCmd, killWorkerCmd
- Update model.go - Add WorkerManager + WorkerBackend fields
- Update update.go - Worker lifecycle message handlers
- Update panels.go - Table rows from worker data, viewport from output buffer
- Spawn dialog (huh form) in overlays.go
- Timer tick for duration updates
- Spinner for running worker status cells
phase: Wave 1 - Core TUI + Worker Lifecycle
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
- timestamp: '2026-02-18T08:53:56.379168648+00:00'
  lane: doing
  actor: manager
  shell_pid: '472734'
  action: 'transition active (WP02+WP03 done. Launching core integration: spawn, output, lifecycle.)'
- timestamp: '2026-02-18T13:30:47.372407497+00:00'
  lane: done
  actor: manager
  shell_pid: '472734'
  action: 'transition done (Verified: PASS. Fixed M1 (View purity). Spawn->output->exit flow works end-to-end.)'
---

# Work Package Prompt: WP04 - Worker-TUI Integration (Spawn + Output + Lifecycle)

## Mission

Connect the worker backend (WP02) to the TUI (WP03). This is the core integration
WP that makes kasmos functional: press `s` to open a spawn dialog, confirm to
start a real opencode worker, see output stream into the viewport, watch the
worker exit and update its status. After this WP, the MVP spawn-monitor-exit loop
works end-to-end.

## Scope

### Files to Create

```
internal/tui/commands.go    # All tea.Cmd constructors
internal/tui/overlays.go    # Spawn dialog (other overlays added in WP05)
```

### Files to Modify

```
internal/tui/model.go       # Add worker manager, backend, overlay state fields
internal/tui/update.go      # Add worker lifecycle message handlers + key handlers
internal/tui/panels.go      # Table rows from live worker data, viewport from output
internal/tui/keys.go        # Enable/disable spawn key
cmd/kasmos/main.go          # Initialize SubprocessBackend, pass to Model
```

### Technical References

- `kitty-specs/016-kasmos-agent-orchestrator/research/tui-technical.md`:
  - **Section 1**: SpawnConfig fields (lines 50-78) -- used by spawn dialog
  - **Section 2**: Worker lifecycle messages + command signatures (lines 218-272)
  - **Section 2**: Message flow diagram (lines 402-447) -- the full spawn flow
  - **Section 2**: Overlay/dialog messages (lines 308-335)
  - **Section 3**: Worker struct runtime fields (Handle, Output) (lines 458-482)
- `design-artifacts/tui-mockups.md`:
  - **V1**: Main dashboard with workers (lines 25-56) -- target rendering
  - **V2**: Spawn worker dialog (lines 68-116) -- huh form layout
- `design-artifacts/tui-keybinds.md`:
  - Dashboard table keys: s (spawn), j/k (navigate) (lines 19-32)
  - Key routing in Update (lines 309-366)
- `design-artifacts/tui-layout-spec.md`:
  - Worker table column widths (lines 268-291)
  - Status bar content (lines 340-365)

## Implementation

### commands.go

Implement the tea.Cmd constructors from tui-technical.md Section 2:

**spawnWorkerCmd(backend, cfg)**: Returns a tea.Cmd that:
1. Calls `backend.Spawn(ctx, cfg)`
2. On success: sends `workerSpawnedMsg{WorkerID, PID}`
3. On failure: sends `workerExitedMsg{WorkerID, Err: err}`

**readOutputCmd(workerID, reader, program)**: Returns a tea.Cmd that:
1. Reads from the worker's stdout in a loop (bufio.Scanner or fixed-size reads)
2. For each chunk: calls `program.Send(workerOutputMsg{WorkerID, Data})`
3. Loops until EOF (reader closes when process exits)
4. This runs in a goroutine -- use `p.Send()` not return values

**Important**: readOutputCmd needs access to `tea.Program` to call `.Send()` for
streaming. Pass `*tea.Program` to the Model at init, or use a channel-based
approach where a goroutine writes to a channel and a tea.Cmd reads from it.

Recommended approach: Start the output reader goroutine in the spawnWorkerCmd
handler (when workerSpawnedMsg is received). The goroutine reads from
`handle.Stdout()` and calls `p.Send(workerOutputMsg{...})`. Store `*tea.Program`
on the Model.

**waitWorkerCmd(workerID, handle)**: Returns a tea.Cmd that:
1. Calls `handle.Wait()` (blocks until exit)
2. Returns `workerExitedMsg{WorkerID, ExitCode, Duration, SessionID}`

**killWorkerCmd(workerID, handle, grace)**: Returns a tea.Cmd that:
1. Calls `handle.Kill(grace)`
2. Returns `workerKilledMsg{WorkerID, Err}`

### overlays.go (Spawn Dialog Only)

Implement the spawn dialog using `huh` forms (tui-mockups.md V2):
- Role selector: `huh.NewSelect()` with options planner/coder/reviewer/release,
  each with a description string
- Prompt textarea: `huh.NewText()` for multi-line input
- Files input: `huh.NewInput()` for comma-separated file paths
- Use `huh.ThemeCharm()` for styling (tui-styles.md line 539)

The form runs as a sub-model within the bubbletea Update loop:
- Model gets a `spawnForm *huh.Form` field and `showSpawnDialog bool`
- When `s` is pressed: create the form, set showSpawnDialog = true
- In Update: if showSpawnDialog, forward messages to spawnForm.Update()
- Check `spawnForm.State == huh.StateCompleted` to extract values
- On completion: emit spawnDialogSubmittedMsg with Role, Prompt, Files
- On abort (Esc): emit spawnDialogCancelledMsg

Render the form inside a centered dialog with hot pink RoundedBorder
and the backdrop pattern from tui-styles.md.

### model.go Updates

Add to Model struct:
- `backend worker.WorkerBackend`
- `manager *worker.WorkerManager` (or embed workers directly as `[]*worker.Worker`)
- `program *tea.Program` (for p.Send() in output goroutines)
- `showSpawnDialog bool`
- `spawnForm *huh.Form`
- `selectedWorkerID string` (tracks which worker's output to show)

Add a `NewModel(backend)` constructor that initializes everything.

### update.go Updates

Add handlers for:
- `spawnDialogSubmittedMsg`: Create SpawnConfig, call spawnWorkerCmd, add worker to manager
- `spawnDialogCancelledMsg`: Close dialog, return to dashboard
- `workerSpawnedMsg`: Update worker state to Running, start output reader goroutine,
  start waitWorkerCmd
- `workerOutputMsg`: Append data to worker's OutputBuffer, update viewport if this
  is the selected worker. Auto-follow: if viewport was at bottom before append,
  call GotoBottom() after setting content.
- `workerExitedMsg`: Update worker state (Exited/Failed based on exit code), set
  ExitedAt, extract SessionID from output
- `tickMsg`: Return tickCmd() to restart the timer. The table re-renders with
  updated durations on each View() call.
- `spinner.TickMsg`: Forward to spinner.Update()

Key handling additions:
- `s` key (table focused): open spawn dialog
- `j/k` keys (table focused): navigate table rows, update selectedWorkerID,
  update viewport content to show selected worker's output

### panels.go Updates

**Table rows**: Build `[]table.Row` from worker list:
- Column values: ID, statusIndicator(state), roleBadge(role), FormatDuration(), TaskID
- For running workers: use `m.spinner.View() + " running"` in the status cell
- Selected row styling via bubbles/table built-in selection

**Viewport content**: When a worker is selected, set viewport content to
`worker.Output.Content()`. When no worker selected, show welcome message.

**Status bar**: Show actual worker counts from manager.
Format: ` [spinner] N running  [check] N done  [x] N failed  ...  mode: ad-hoc  scroll: N%`

### main.go Updates

1. Create SubprocessBackend: `backend, err := worker.NewSubprocessBackend()`
2. Handle error (opencode not found)
3. Create Model: `model := tui.NewModel(backend)`
4. Create Program: `p := tea.NewProgram(model, tea.WithAltScreen(), tea.WithContext(ctx))`
5. Set program on model: `model.SetProgram(p)` (needed for p.Send in goroutines)
6. Run: `if _, err := p.Run(); err != nil { ... }`

## What NOT to Do

- Do NOT implement the continue dialog (WP05)
- Do NOT implement the quit confirmation dialog (WP05)
- Do NOT implement the help overlay rendering (already in WP03 as toggle)
- Do NOT implement kill/restart key handlers (WP07)
- Do NOT implement fullscreen viewport mode (WP06)
- Do NOT implement task panel or task source integration (WP08/WP09)
- The spawn dialog does NOT need task pre-fill (that comes with WP09)
- Focus on the spawn -> monitor -> exit flow only

## Acceptance Criteria

1. Press `s`, fill out the spawn dialog, confirm -- a worker appears in the table
2. Worker status shows spinner + "running" while active
3. Select the running worker -- its output streams into the viewport in real time
4. When the worker exits, status updates to "done" or "failed(N)" with duration
5. Multiple workers can run concurrently (spawn 2-3, all show in table)
6. Table navigation (j/k) switches the viewport to the selected worker's output
7. Status bar shows accurate worker counts
8. Duration column updates every second for running workers
9. `go test ./...` passes, `go vet ./...` clean
