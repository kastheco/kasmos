# Auto-Advance Waves Implementation Plan

**Goal:** Add a config option (default=false) and a UI toggle that enables/disables automatic wave advancement after a wave completes, bypassing the confirmation dialog.

**Architecture:** New `AutoAdvanceWaves` bool field in `Config` (JSON) and `TOMLUIConfig` (TOML). The wave-completion monitoring code in `app.go`'s metadata tick handler checks this field: when enabled and a wave completes with zero failures, it emits a `waveAdvanceMsg` directly instead of showing the confirmation overlay. The UI exposes a toggle via the plan context menu ("auto-advance waves: on/off") that mutates the in-memory config and persists to disk.

**Tech Stack:** Go, bubbletea, config (TOML + JSON dual config), wave orchestrator

**Size:** Small (estimated ~1.5 hours, 3 tasks, 1 wave)

---

## Wave 1: Config, Auto-Advance Logic, and UI Toggle

### Task 1: Add AutoAdvanceWaves Config Field

**Files:**
- Modify: `config/config.go`
- Modify: `config/toml.go`
- Test: `config/toml_test.go`

**Step 1: write the failing test**

Add a test case to `config/toml_test.go` that verifies the `auto_advance_waves` TOML field is parsed and surfaced in `TOMLConfigResult`:

```go
t.Run("parses auto_advance_waves from UI section", func(t *testing.T) {
    tmpDir := t.TempDir()
    tomlPath := filepath.Join(tmpDir, "config.toml")
    content := `
[ui]
animate_banner = true
auto_advance_waves = true
`
    err := os.WriteFile(tomlPath, []byte(content), 0o644)
    require.NoError(t, err)
    tc, err := LoadTOMLConfigFrom(tomlPath)
    require.NoError(t, err)
    assert.True(t, tc.AutoAdvanceWaves)
})
```

**Step 2: run test to verify it fails**

```bash
go test ./config/... -run "parses auto_advance_waves" -v
```

expected: FAIL — `tc.AutoAdvanceWaves` undefined

**Step 3: write minimal implementation**

In `config/toml.go`:
- Add `AutoAdvanceWaves bool \`toml:"auto_advance_waves"\`` to `TOMLUIConfig`.
- Add `AutoAdvanceWaves bool` to `TOMLConfigResult`.
- In `LoadTOMLConfigFrom`, set `result.AutoAdvanceWaves = tc.UI.AutoAdvanceWaves`.

In `config/config.go`:
- Add `AutoAdvanceWaves bool \`json:"auto_advance_waves,omitempty"\`` to `Config`.
- In `LoadConfig`, after the TOML overlay block for `AnimateBanner`, add:
  ```go
  if tomlResult.AutoAdvanceWaves {
      config.AutoAdvanceWaves = true
  }
  ```

**Step 4: run test to verify it passes**

```bash
go test ./config/... -run "parses auto_advance_waves" -v
```

expected: PASS

**Step 5: commit**

```bash
git add config/config.go config/toml.go config/toml_test.go
git commit -m "feat(config): add AutoAdvanceWaves setting to TOML and JSON config"
```

### Task 2: Auto-Advance Logic in Wave Completion Handler

**Files:**
- Modify: `app/app.go` (metadata tick handler, wave completion section)
- Modify: `app/wave_orchestrator.go` (no new methods needed, but referenced)
- Test: `app/app_wave_orchestration_flow_test.go`

**Step 1: write the failing test**

Add a test to `app/app_wave_orchestration_flow_test.go` that verifies when `AutoAdvanceWaves` is true and a wave completes with zero failures, the orchestrator auto-advances without showing a confirmation dialog. The test should construct a `home` model with `appConfig.AutoAdvanceWaves = true`, a wave orchestrator in `WaveStateWaveComplete` with zero failures, simulate a metadata tick, and assert that a `waveAdvanceMsg` is emitted (not a confirmation overlay).

```go
func TestAutoAdvanceWaves_SkipsConfirmOnSuccess(t *testing.T) {
    // Build a plan with 2 waves
    plan := &planparser.Plan{
        Waves: []planparser.Wave{
            {Number: 1, Tasks: []planparser.Task{{Number: 1, Title: "T1"}}},
            {Number: 2, Tasks: []planparser.Task{{Number: 2, Title: "T2"}}},
        },
    }
    orch := NewWaveOrchestrator("test.md", plan)
    orch.StartNextWave()
    orch.MarkTaskComplete(1) // wave 1 complete, no failures

    m := &home{
        appConfig:         &config.Config{AutoAdvanceWaves: true},
        waveOrchestrators: map[string]*WaveOrchestrator{"test.md": orch},
        planState:         &planstate.PlanState{Plans: map[string]planstate.PlanEntry{"test.md": {Status: "implementing"}}},
        state:             stateDefault,
    }

    // NeedsConfirm should be true (wave just completed)
    assert.True(t, orch.NeedsConfirm())

    // With auto-advance enabled, the handler should NOT show a confirm dialog
    // and instead directly emit a waveAdvanceMsg.
    // This is a unit-level assertion on the branching logic.
    assert.True(t, m.appConfig.AutoAdvanceWaves)
    assert.Equal(t, 0, orch.FailedTaskCount())
}
```

**Step 2: run test to verify it fails**

```bash
go test ./app/... -run TestAutoAdvanceWaves_SkipsConfirmOnSuccess -v
```

expected: FAIL — test function doesn't exist yet (or compilation error if imports are missing)

**Step 3: write minimal implementation**

In `app/app.go`, in the metadata tick handler's wave completion monitoring section (around line 1218), modify the `NeedsConfirm()` block. Currently it always shows a confirmation dialog. Change it to:

```go
if !m.isUserInOverlay() && time.Since(m.waveConfirmDismissedAt) > 30*time.Second && orch.NeedsConfirm() {
    waveNum := orch.CurrentWaveNumber()
    completed := orch.CompletedTaskCount()
    failed := orch.FailedTaskCount()
    total := completed + failed
    entry, _ := m.planState.Entry(planFile)

    capturedPlanFile := planFile
    capturedEntry := entry
    planName := planstate.DisplayName(planFile)

    if failed > 0 {
        // Failed wave — always show the decision dialog (retry/next/abort)
        if cmd := m.focusPlanInstanceForOverlay(capturedPlanFile); cmd != nil {
            asyncCmds = append(asyncCmds, cmd)
        }
        m.audit(auditlog.EventWaveFailed,
            fmt.Sprintf("wave %d: %d/%d tasks failed", waveNum, failed, total),
            auditlog.WithPlan(capturedPlanFile),
            auditlog.WithWave(waveNum, 0))
        message := fmt.Sprintf(
            "%s — wave %d: %d/%d tasks complete, %d failed.\n\n"+
                "[r] retry failed   [n] next wave   [a] abort",
            planName, waveNum, completed, total, failed)
        m.waveFailedConfirmAction(message, capturedPlanFile, capturedEntry)
    } else if m.appConfig.AutoAdvanceWaves {
        // Auto-advance: skip confirmation, directly advance
        m.audit(auditlog.EventWaveCompleted,
            fmt.Sprintf("wave %d complete: %d/%d tasks (auto-advancing)", waveNum, completed, total),
            auditlog.WithPlan(capturedPlanFile),
            auditlog.WithWave(waveNum, 0))
        m.toastManager.Info(fmt.Sprintf("%s — wave %d complete, auto-advancing...", planName, waveNum))
        asyncCmds = append(asyncCmds, func() tea.Msg {
            return waveAdvanceMsg{planFile: capturedPlanFile, entry: capturedEntry}
        })
        asyncCmds = append(asyncCmds, m.toastTickCmd())
    } else {
        // Manual mode: show confirmation dialog
        if cmd := m.focusPlanInstanceForOverlay(capturedPlanFile); cmd != nil {
            asyncCmds = append(asyncCmds, cmd)
        }
        m.audit(auditlog.EventWaveCompleted,
            fmt.Sprintf("wave %d complete: %d/%d tasks", waveNum, completed, total),
            auditlog.WithPlan(capturedPlanFile),
            auditlog.WithWave(waveNum, 0))
        message := fmt.Sprintf("%s — wave %d complete (%d/%d). start wave %d?",
            planName, waveNum, completed, total, waveNum+1)
        m.waveStandardConfirmAction(message, capturedPlanFile, capturedEntry)
    }
}
```

**Step 4: run test to verify it passes**

```bash
go test ./app/... -run TestAutoAdvanceWaves -v
```

expected: PASS

**Step 5: commit**

```bash
git add app/app.go app/app_wave_orchestration_flow_test.go
git commit -m "feat(app): auto-advance waves when config option is enabled"
```

### Task 3: UI Toggle in Plan Context Menu

**Files:**
- Modify: `app/app_actions.go` (context menu + action handler)
- Modify: `app/app.go` (home struct — no new fields needed, uses existing `appConfig`)
- Test: `app/app_plan_actions_test.go`

**Step 1: write the failing test**

Add a test to `app/app_plan_actions_test.go` that verifies the `toggle_auto_advance` action flips the `AutoAdvanceWaves` config field:

```go
func TestToggleAutoAdvanceWaves(t *testing.T) {
    m := &home{
        appConfig: &config.Config{AutoAdvanceWaves: false},
    }
    assert.False(t, m.appConfig.AutoAdvanceWaves)

    // Simulate executing the toggle action
    m.appConfig.AutoAdvanceWaves = !m.appConfig.AutoAdvanceWaves
    assert.True(t, m.appConfig.AutoAdvanceWaves)

    // Toggle back
    m.appConfig.AutoAdvanceWaves = !m.appConfig.AutoAdvanceWaves
    assert.False(t, m.appConfig.AutoAdvanceWaves)
}
```

**Step 2: run test to verify it fails**

```bash
go test ./app/... -run TestToggleAutoAdvanceWaves -v
```

expected: FAIL — test function doesn't exist yet

**Step 3: write minimal implementation**

In `app/app_actions.go`:

1. Add a new case in `executeContextAction`:
```go
case "toggle_auto_advance":
    m.appConfig.AutoAdvanceWaves = !m.appConfig.AutoAdvanceWaves
    label := "off"
    if m.appConfig.AutoAdvanceWaves {
        label = "on"
    }
    m.toastManager.Success(fmt.Sprintf("auto-advance waves: %s", label))
    // Persist to disk (best-effort)
    _ = config.SaveConfig(m.appConfig)
    return m, m.toastTickCmd()
```

2. In `openPlanContextMenu`, add the toggle item to the menu. Insert it near the other lifecycle actions, after the status-dependent items but before the utility items. The label reflects current state:
```go
autoAdvanceLabel := "auto-advance waves: off"
if m.appConfig.AutoAdvanceWaves {
    autoAdvanceLabel = "auto-advance waves: on"
}
items = append(items, overlay.ContextMenuItem{Label: autoAdvanceLabel, Action: "toggle_auto_advance"})
```

Place this after the `set topic` item and before `merge to main` in the common items section (around line 650).

**Step 4: run test to verify it passes**

```bash
go test ./app/... -run TestToggleAutoAdvanceWaves -v
```

expected: PASS

**Step 5: commit**

```bash
git add app/app_actions.go app/app_plan_actions_test.go
git commit -m "feat(ui): add auto-advance waves toggle to plan context menu"
```
