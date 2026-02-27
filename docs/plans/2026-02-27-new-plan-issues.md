# New Plan Input Overlay Bugs

**Goal:** Fix two bugs in the new-plan text input overlay: (1) confirmation dialogs overwrite the input state, losing user content; (2) the overlay resizes when the agent pane loads or switches sessions.

**Architecture:** Both bugs stem from the interaction between `stateNewPlan` and other app state transitions. Bug 1: `confirmAction()` unconditionally sets `m.state = stateConfirm`, destroying the `stateNewPlan` context. Fix: guard `confirmAction()` (and all other state-overwriting paths that fire from async ticks) to defer when an input overlay is active. Bug 2: `updateHandleWindowSizeEvent()` re-sizes the `textInputOverlay` on every `tea.WindowSize` event, but many actions emit `tea.WindowSize()` as a side-effect (e.g. `instanceStartedMsg`, `killInstanceMsg`). Fix: the overlay should record its initial size at creation and ignore subsequent `SetSize` calls, or `updateHandleWindowSizeEvent` should only resize when the terminal dimensions actually change.

**Tech Stack:** Go, bubbletea, lipgloss, overlay package (`ui/overlay/`)

**Size:** Small (estimated ~1.5 hours, 2 tasks, 1 wave)

---

## Wave 1: Fix Input Overlay State and Sizing

### Task 1: Guard State Transitions While Input Overlay Is Active

**Files:**
- Modify: `app/app_input.go`
- Modify: `app/app.go`
- Test: `app/app_plan_creation_test.go`

**Step 1: write the failing test**

Add a test that simulates entering `stateNewPlan`, then receiving a `confirmAction` call (as would happen from a `PlannerFinished` signal or wave completion during the metadata tick). Assert that the state remains `stateNewPlan` and the `textInputOverlay` is preserved.

```go
func TestConfirmActionDeferredWhileNewPlanActive(t *testing.T) {
    h := &home{
        state:            stateNewPlan,
        textInputOverlay: overlay.NewTextInputOverlay("new plan", "my plan description"),
    }
    h.textInputOverlay.SetMultiline(true)

    // Simulate a confirmation action arriving while typing
    h.confirmAction("some confirmation?", func() tea.Msg { return nil })

    // State should NOT have changed to stateConfirm
    require.Equal(t, stateNewPlan, h.state)
    require.NotNil(t, h.textInputOverlay)
    require.Nil(t, h.confirmationOverlay)
}
```

Also add a test for the topic picker state:

```go
func TestConfirmActionDeferredWhileTopicPickerActive(t *testing.T) {
    h := &home{
        state:           stateNewPlanTopic,
        pendingPlanName: "test plan",
        pendingPlanDesc: "test description",
        pickerOverlay:   overlay.NewPickerOverlay("topic", []string{"(No topic)"}),
    }

    h.confirmAction("some confirmation?", func() tea.Msg { return nil })

    require.Equal(t, stateNewPlanTopic, h.state)
    require.NotNil(t, h.pickerOverlay)
    require.Nil(t, h.confirmationOverlay)
}
```

**Step 2: run test to verify it fails**

```bash
go test ./app/... -run TestConfirmActionDeferredWhileNewPlanActive -v
go test ./app/... -run TestConfirmActionDeferredWhileTopicPickerActive -v
```

expected: FAIL — `confirmAction` currently overwrites state unconditionally.

**Step 3: write minimal implementation**

In `app/app_input.go`, modify `confirmAction()` to check if an input overlay is active. If so, defer the confirmation by ignoring it (the metadata tick will re-trigger it on the next cycle when the overlay is dismissed):

```go
func (m *home) confirmAction(message string, action tea.Cmd) tea.Cmd {
    // Guard: don't overwrite active input overlays — the user is typing.
    // The metadata tick will re-trigger the confirmation after the overlay closes.
    if m.isInputOverlayActive() {
        return nil
    }
    m.state = stateConfirm
    m.pendingConfirmAction = action
    m.confirmationOverlay = overlay.NewConfirmationOverlay(message)
    m.confirmationOverlay.SetWidth(50)
    return nil
}
```

Add the helper method to `app/app_input.go` or `app/app.go`:

```go
// isInputOverlayActive returns true when the user is actively typing in an
// input overlay that should not be interrupted by confirmation dialogs.
func (m *home) isInputOverlayActive() bool {
    switch m.state {
    case stateNewPlan, stateNewPlanTopic, statePrompt, stateSendPrompt,
        statePRTitle, statePRBody, stateRenameInstance, stateRenamePlan,
        stateSpawnAgent, stateClickUpSearch, stateSearch:
        return true
    }
    return false
}
```

Also guard the direct `m.state = stateConfirm` assignments in `app.go`'s `Update()` method (the `PlannerFinished` signal handler and `waveFailedConfirmAction`/`waveStandardConfirmAction` callers already check `m.state != stateConfirm` — extend those checks to also skip when `isInputOverlayActive()` returns true).

Specifically in the `metadataResultMsg` handler in `app.go`:
- The `PlannerFinished` signal handler (around line 676) already checks `m.state == stateConfirm` — add `|| m.isInputOverlayActive()` to that guard.
- The coder-exit push prompt (around line 888) already checks `m.state == stateConfirm` — add `|| m.isInputOverlayActive()`.
- The wave completion monitoring (around lines 980, 996) already checks `m.state != stateConfirm` — add `&& !m.isInputOverlayActive()`.
- The permission prompt detection (around line 798) already checks `m.state == stateDefault` — this is already safe.

**Step 4: run test to verify it passes**

```bash
go test ./app/... -run TestConfirmActionDeferred -v
```

expected: PASS

**Step 5: commit**

```bash
git add app/app_input.go app/app.go app/app_plan_creation_test.go
git commit -m "fix: guard confirmation dialogs from overwriting active input overlays"
```

### Task 2: Fix Text Input Overlay Resizing on Agent Pane Changes

**Files:**
- Modify: `ui/overlay/textInput.go`
- Test: `ui/overlay/textInput_test.go`

**Step 1: write the failing test**

```go
func TestTextInputOverlaySizeLockedAfterFirstSet(t *testing.T) {
    o := NewTextInputOverlay("test", "initial value")
    o.SetSize(70, 8)

    // Simulate a window resize event re-calling SetSize with different dimensions
    o.SetSize(120, 40)

    // The overlay should retain its original size
    rendered := o.Render()
    // The rendered width should reflect the original 70, not 120
    lines := strings.Split(rendered, "\n")
    maxWidth := 0
    for _, line := range lines {
        w := lipgloss.Width(line)
        if w > maxWidth {
            maxWidth = w
        }
    }
    // With padding+border the rendered width should be around 70, not 120+
    require.Less(t, maxWidth, 90, "overlay should not have grown to window size")
}
```

**Step 2: run test to verify it fails**

```bash
go test ./ui/overlay/... -run TestTextInputOverlaySizeLockedAfterFirstSet -v
```

expected: FAIL — `SetSize` currently always updates dimensions.

**Step 3: write minimal implementation**

Add a `sizeSet` flag to `TextInputOverlay` that prevents subsequent `SetSize` calls from changing the dimensions after the initial call:

In `ui/overlay/textInput.go`:

```go
type TextInputOverlay struct {
    // ... existing fields ...
    sizeSet bool // true after the first SetSize call
}

func (t *TextInputOverlay) SetSize(width, height int) {
    if t.sizeSet {
        return // ignore resize events after initial sizing
    }
    t.sizeSet = true
    t.textarea.SetHeight(height)
    t.width = width
    t.height = height
}
```

This is the minimal fix. The `updateHandleWindowSizeEvent` in `app.go` (line 427-429) calls `m.textInputOverlay.SetSize(...)` on every window size event — with the `sizeSet` guard, only the first call (from the creation site) takes effect.

**Step 4: run test to verify it passes**

```bash
go test ./ui/overlay/... -run TestTextInputOverlaySizeLockedAfterFirstSet -v
```

expected: PASS

**Step 5: commit**

```bash
git add ui/overlay/textInput.go ui/overlay/textInput_test.go
git commit -m "fix: lock text input overlay size after initial set to prevent resize on agent pane changes"
```
