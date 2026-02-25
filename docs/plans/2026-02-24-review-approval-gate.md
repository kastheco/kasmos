# Review Approval Gate Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** After automated review completes (reviewer writes `review-approved` signal or dies), block auto-transition to `done` and instead show a two-popup manual approval flow — first to read the review, then to merge or create a PR.

**Architecture:** Add `pendingApprovals map[string]bool` to `home` struct to track plans awaiting user approval. Intercept `ReviewApproved` at both entry points (signal handler and reviewer-death detector) to set the flag and show Popup 1 instead of calling `fsm.Transition`. Popup 1 selects the reviewer instance and enters focus mode. When focus mode exits on a reviewer with a pending approval, show Popup 2 with merge/PR/dismiss choices. Context menu also offers merge/PR for `reviewing` plans as an alternate path.

**Tech Stack:** Go 1.24+, bubbletea v1.3.x, lipgloss v1.1.x, testify

---

## Wave 1: State Infrastructure

### Task 1: Add `pendingApprovals` field to home struct

**Files:**
- Modify: `app/app.go:227-230` (add field after `pendingReviewFeedback`)
- Modify: `app/app.go:268` (initialize in `newHome()`)

**Step 1: Add the field to the home struct**

In `app/app.go`, after line 229 (`pendingReviewFeedback map[string]string`), add:

```go
	// pendingApprovals tracks plans whose automated review approved but the user
	// hasn't yet confirmed merge/PR. Keyed by plan filename. In-memory only —
	// on restart, reviewer-death fallback re-triggers the approval popup.
	pendingApprovals map[string]bool
```

**Step 2: Initialize in `newHome()`**

In `app/app.go` at line 268, after `pendingReviewFeedback: make(map[string]string),`, add:

```go
		pendingApprovals:      make(map[string]bool),
```

**Step 3: Verify it compiles**

Run: `go build ./app/...`
Expected: SUCCESS

**Step 4: Commit**

```bash
git add app/app.go
git commit -m "feat(approval): add pendingApprovals field to home struct"
```

### Task 2: Add `pendingApprovals` to test helpers

**Files:**
- Modify: `app/app_plan_completion_test.go:271-282` (first test helper)
- Modify: `app/app_plan_completion_test.go:339-355` (second test helper)
- Modify: `app/app_plan_completion_test.go:420-436` (third test helper)

**Step 1: Add field to all three test `home` struct literals**

Each test helper that constructs a `&home{...}` literal needs `pendingApprovals: make(map[string]bool)`. Add it next to the existing `pendingReviewFeedback` line in each.

The three locations (line numbers approximate — find `pendingReviewFeedback: make(map[string]string),` in each):

1. `TestMetadataResultMsg_SignalDoesNotClobberFreshPlanState` (~line 275)
2. `TestImplementFinishedSignal_SpawnsReviewer` (~line 351)
3. `TestReviewChangesSignal_RespawnsCoder` (~line 432)

**Step 2: Verify tests compile**

Run: `go test ./app/... -run TestMetadata -count=1`
Expected: PASS

**Step 3: Commit**

```bash
git add app/app_plan_completion_test.go
git commit -m "fix(test): add pendingApprovals to test home struct literals"
```

## Wave 2: Intercept ReviewApproved — Show Popup 1

### Task 3: Write tests for approval interception

**Files:**
- Modify: `app/app_plan_completion_test.go` (append new tests)

**Step 1: Write test for signal-path interception**

Append to `app/app_plan_completion_test.go`:

```go
// TestReviewApprovedSignal_SetsPendingApproval verifies that when a
// review-approved sentinel is processed, the plan does NOT transition to done.
// Instead, pendingApprovals is set and the confirmation overlay appears.
func TestReviewApprovedSignal_SetsPendingApproval(t *testing.T) {
	const planFile = "2026-02-23-feature.md"

	dir := t.TempDir()
	plansDir := filepath.Join(dir, "docs", "plans")
	require.NoError(t, os.MkdirAll(plansDir, 0o755))
	ps, err := planstate.Load(plansDir)
	require.NoError(t, err)
	require.NoError(t, ps.Register(planFile, "feature", "plan/feature", time.Now()))
	seedPlanStatus(t, ps, planFile, planstate.StatusReviewing)

	// Create a reviewer instance bound to this plan.
	reviewerInst, err := session.NewInstance(session.InstanceOptions{
		Title:     "feature-review",
		Path:      dir,
		Program:   "claude",
		PlanFile:  planFile,
		AgentType: session.AgentTypeReviewer,
	})
	require.NoError(t, err)
	reviewerInst.IsReviewer = true

	sp := spinner.New(spinner.WithSpinner(spinner.Dot))
	list := ui.NewList(&sp, false)
	_ = list.AddInstance(reviewerInst)

	h := &home{
		ctx:                   context.Background(),
		state:                 stateDefault,
		appConfig:             config.DefaultConfig(),
		list:                  list,
		menu:                  ui.NewMenu(),
		sidebar:               ui.NewSidebar(),
		tabbedWindow:          ui.NewTabbedWindow(ui.NewPreviewPane(), ui.NewDiffPane(), ui.NewGitPane()),
		toastManager:          overlay.NewToastManager(&sp),
		planState:             ps,
		planStateDir:          plansDir,
		fsm:                   planfsm.New(plansDir),
		pendingReviewFeedback: make(map[string]string),
		pendingApprovals:      make(map[string]bool),
		plannerPrompted:       make(map[string]bool),
		activeRepoPath:        dir,
		program:               "claude",
	}

	signal := planfsm.Signal{
		Event:    planfsm.ReviewApproved,
		PlanFile: planFile,
	}
	msg := metadataResultMsg{
		PlanState: ps,
		Signals:   []planfsm.Signal{signal},
	}

	_, _ = h.Update(msg)

	// pendingApprovals must be set.
	assert.True(t, h.pendingApprovals[planFile],
		"review-approved signal must set pendingApprovals instead of transitioning to done")

	// Plan status must still be "reviewing" — NOT "done".
	reloaded, _ := planstate.Load(plansDir)
	entry, ok := reloaded.Entry(planFile)
	require.True(t, ok)
	assert.Equal(t, planstate.StatusReviewing, entry.Status,
		"plan must stay in reviewing until user manually approves")

	// Confirmation overlay must be shown.
	assert.Equal(t, stateConfirm, h.state,
		"confirmation overlay must be shown for review approval")
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./app/... -run TestReviewApprovedSignal_SetsPendingApproval -count=1 -v`
Expected: FAIL — the signal currently calls `fsm.Transition` and transitions to `done`.

**Step 3: Commit failing test**

```bash
git add app/app_plan_completion_test.go
git commit -m "test(approval): add failing test for review-approved interception"
```

### Task 4: Intercept `ReviewApproved` signal in signal handler

**Files:**
- Modify: `app/app.go:522-558` (signal processing loop in `metadataResultMsg` handler)

**Step 1: Add `ReviewApproved` case to the signal handler**

In `app/app.go`, inside the `switch sig.Event` block (~line 532), the `ReviewApproved` event
currently falls through to the default path (no `case` for it), so `fsm.Transition` at line 524
handles it. We need to intercept it BEFORE that.

Replace the signal loop (lines 522-558) with:

```go
		var signalCmds []tea.Cmd
		for _, sig := range msg.Signals {
			// Intercept ReviewApproved: block FSM transition, show approval popup instead.
			if sig.Event == planfsm.ReviewApproved {
				planfsm.ConsumeSignal(sig)
				m.pendingApprovals[sig.PlanFile] = true
				planName := planstate.DisplayName(sig.PlanFile)
				capturedPlanFile := sig.PlanFile
				m.confirmAction(
					fmt.Sprintf("review approved — %s", planName),
					func() tea.Msg {
						return reviewApprovalFocusMsg{planFile: capturedPlanFile}
					},
				)
				continue
			}

			if err := m.fsm.Transition(sig.PlanFile, sig.Event); err != nil {
				log.WarningLog.Printf("signal %s for %s rejected: %v", sig.Event, sig.PlanFile, err)
				planfsm.ConsumeSignal(sig)
				continue
			}
			planfsm.ConsumeSignal(sig)

			// Side effects: spawn agents in response to successful transitions.
			switch sig.Event {
			case planfsm.ImplementFinished:
				// Pause the coder that wrote this signal.
				for _, inst := range m.list.GetInstances() {
					if inst.PlanFile == sig.PlanFile && inst.AgentType == session.AgentTypeCoder {
						inst.ImplementationComplete = true
						_ = inst.Pause()
						break
					}
				}
				if cmd := m.spawnReviewer(sig.PlanFile); cmd != nil {
					signalCmds = append(signalCmds, cmd)
				}
			case planfsm.ReviewChangesRequested:
				feedback := sig.Body
				m.pendingReviewFeedback[sig.PlanFile] = feedback
				// Pause the reviewer that wrote this signal.
				for _, inst := range m.list.GetInstances() {
					if inst.PlanFile == sig.PlanFile && inst.IsReviewer {
						_ = inst.Pause()
						break
					}
				}
				if cmd := m.spawnCoderWithFeedback(sig.PlanFile, feedback); cmd != nil {
					signalCmds = append(signalCmds, cmd)
				}
			}
		}
```

**Step 2: Define the `reviewApprovalFocusMsg` type**

Add near the other msg types in `app/app.go` (around the `coderCompleteMsg`, `plannerCompleteMsg` types):

```go
// reviewApprovalFocusMsg is sent when the user confirms Popup 1 (review approved).
// It selects the reviewer instance and enters focus mode so the user can read the review.
type reviewApprovalFocusMsg struct {
	planFile string
}
```

**Step 3: Handle `reviewApprovalFocusMsg` in Update**

Add a case in the `Update` function's type switch (near other msg handlers):

```go
	case reviewApprovalFocusMsg:
		// Select the reviewer instance for this plan and enter focus mode.
		for _, inst := range m.list.GetInstances() {
			if inst.PlanFile == msg.planFile && inst.IsReviewer {
				m.list.SelectInstance(inst)
				return m, m.enterFocusMode()
			}
		}
		// Reviewer instance not found — show toast and leave pending for context menu path.
		m.toastManager.Info(fmt.Sprintf("reviewer session not found for %s — use context menu to merge or create pr", planstate.DisplayName(msg.planFile)))
		return m, m.toastTickCmd()
```

**Step 4: Run the test**

Run: `go test ./app/... -run TestReviewApprovedSignal_SetsPendingApproval -count=1 -v`
Expected: PASS

**Step 5: Commit**

```bash
git add app/app.go
git commit -m "feat(approval): intercept review-approved signal with approval popup"
```

### Task 5: Intercept reviewer-death auto-approve

**Files:**
- Modify: `app/app.go:647-668` (reviewer death detection block in `metadataResultMsg`)

**Step 1: Write a test for reviewer-death interception**

Append to `app/app_plan_completion_test.go`:

```go
// TestReviewerDeath_SetsPendingApproval verifies that when a reviewer's tmux
// session dies while the plan is in reviewing state, pendingApprovals is set
// instead of auto-transitioning to done.
func TestReviewerDeath_SetsPendingApproval(t *testing.T) {
	const planFile = "2026-02-23-feature.md"

	dir := t.TempDir()
	plansDir := filepath.Join(dir, "docs", "plans")
	require.NoError(t, os.MkdirAll(plansDir, 0o755))
	ps, err := planstate.Load(plansDir)
	require.NoError(t, err)
	require.NoError(t, ps.Register(planFile, "feature", "plan/feature", time.Now()))
	seedPlanStatus(t, ps, planFile, planstate.StatusReviewing)

	// Create a started reviewer instance (tmux will report dead).
	reviewerInst, err := session.NewInstance(session.InstanceOptions{
		Title:     "feature-review",
		Path:      dir,
		Program:   "claude",
		PlanFile:  planFile,
		AgentType: session.AgentTypeReviewer,
	})
	require.NoError(t, err)
	reviewerInst.IsReviewer = true
	reviewerInst.SetStatus(session.Running) // mark as started

	sp := spinner.New(spinner.WithSpinner(spinner.Dot))
	list := ui.NewList(&sp, false)
	_ = list.AddInstance(reviewerInst)

	h := &home{
		ctx:                   context.Background(),
		state:                 stateDefault,
		appConfig:             config.DefaultConfig(),
		list:                  list,
		menu:                  ui.NewMenu(),
		sidebar:               ui.NewSidebar(),
		tabbedWindow:          ui.NewTabbedWindow(ui.NewPreviewPane(), ui.NewDiffPane(), ui.NewGitPane()),
		toastManager:          overlay.NewToastManager(&sp),
		planState:             ps,
		planStateDir:          plansDir,
		fsm:                   planfsm.New(plansDir),
		pendingReviewFeedback: make(map[string]string),
		pendingApprovals:      make(map[string]bool),
		plannerPrompted:       make(map[string]bool),
		activeRepoPath:        dir,
		program:               "claude",
	}

	// Simulate metadata tick where reviewer's tmux is dead.
	msg := metadataResultMsg{
		PlanState: ps,
		Results: []metadataResult{
			{Title: "feature-review", TmuxAlive: false, ContentCaptured: true},
		},
	}

	_, _ = h.Update(msg)

	// pendingApprovals must be set.
	assert.True(t, h.pendingApprovals[planFile],
		"reviewer death must set pendingApprovals instead of transitioning to done")

	// Plan status must still be "reviewing".
	reloaded, _ := planstate.Load(plansDir)
	entry, ok := reloaded.Entry(planFile)
	require.True(t, ok)
	assert.Equal(t, planstate.StatusReviewing, entry.Status,
		"plan must stay in reviewing after reviewer death until user approves")
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./app/... -run TestReviewerDeath_SetsPendingApproval -count=1 -v`
Expected: FAIL — reviewer death currently calls `fsm.Transition(ReviewApproved)`.

**Step 3: Replace reviewer-death auto-approve with pending approval**

In `app/app.go`, replace lines 664-668 (the reviewer death block):

```go
			// Reviewer death → ReviewApproved: one-shot FSM transition, rare event.
			if err := m.fsm.Transition(inst.PlanFile, planfsm.ReviewApproved); err != nil {
				log.WarningLog.Printf("could not mark plan %q completed: %v", inst.PlanFile, err)
			}
```

With:

```go
			// Reviewer death → pending approval (manual gate).
			// Don't auto-transition to done — set pending approval so the user
			// can review and choose merge/PR via popup or context menu.
			if !m.pendingApprovals[inst.PlanFile] {
				m.pendingApprovals[inst.PlanFile] = true
				planName := planstate.DisplayName(inst.PlanFile)
				if m.state != stateConfirm {
					capturedPlanFile := inst.PlanFile
					m.confirmAction(
						fmt.Sprintf("review approved — %s", planName),
						func() tea.Msg {
							return reviewApprovalFocusMsg{planFile: capturedPlanFile}
						},
					)
				}
			}
```

**Step 4: Run both tests**

Run: `go test ./app/... -run "TestReviewerDeath_SetsPendingApproval|TestReviewApprovedSignal_SetsPendingApproval" -count=1 -v`
Expected: PASS

**Step 5: Run full test suite**

Run: `go test ./app/... -count=1`
Expected: PASS (existing tests should still pass — they don't check for `done` transition from ReviewApproved signal path)

**Step 6: Commit**

```bash
git add app/app.go app/app_plan_completion_test.go
git commit -m "feat(approval): intercept reviewer death with pending approval popup"
```

## Wave 3: Popup 2 — Post-Focus Merge/PR Choice

### Task 6: Add `reviewApprovalConfirmAction` helper

**Files:**
- Modify: `app/app_input.go` (add helper near `waveStandardConfirmAction`)

**Step 1: Add helper function**

Add after `waveFailedConfirmAction` (~line 1406) in `app/app_input.go`:

```go
// reviewApprovalConfirmAction shows a three-choice dialog for a plan with
// a pending review approval. Keys: m=merge to main, p=create PR, esc=dismiss.
func (m *home) reviewApprovalConfirmAction(planFile string) {
	planName := planstate.DisplayName(planFile)

	m.state = stateConfirm
	m.confirmationOverlay = overlay.NewConfirmationOverlay(
		fmt.Sprintf("merge to main or create pr for '%s'?\n\n[m] merge  [p] create pr  [esc] dismiss", planName),
	)
	m.confirmationOverlay.ConfirmKey = "m"
	m.confirmationOverlay.CancelKey = "p"
	m.confirmationOverlay.SetWidth(60)

	capturedPlanFile := planFile
	m.pendingConfirmAction = func() tea.Msg {
		return reviewMergeMsg{planFile: capturedPlanFile}
	}
	m.pendingWaveNextAction = func() tea.Msg {
		return reviewCreatePRMsg{planFile: capturedPlanFile}
	}
}
```

**Step 2: Define msg types**

Add to `app/app.go` near other msg types:

```go
// reviewMergeMsg is sent when the user chooses "merge" in the post-review popup.
type reviewMergeMsg struct {
	planFile string
}

// reviewCreatePRMsg is sent when the user chooses "create PR" in the post-review popup.
type reviewCreatePRMsg struct {
	planFile string
}
```

**Step 3: Verify it compiles**

Run: `go build ./app/...`
Expected: SUCCESS

**Step 4: Commit**

```bash
git add app/app_input.go app/app.go
git commit -m "feat(approval): add review approval confirm helper and msg types"
```

### Task 7: Show Popup 2 when exiting focus mode from a reviewer with pending approval

**Files:**
- Modify: `app/app_input.go:478-500` (focus mode key handler, Ctrl+Space exit path)

**Step 1: Modify focus mode exit to check for pending approval**

In `app/app_input.go`, replace the Ctrl+Space exit block (lines 481-483):

```go
		if msg.Type == tea.KeyCtrlAt {
			m.exitFocusMode()
			return m, tea.WindowSize()
		}
```

With:

```go
		if msg.Type == tea.KeyCtrlAt {
			// Check if the focused instance is a reviewer with a pending approval.
			selected := m.list.GetSelectedInstance()
			m.exitFocusMode()
			if selected != nil && selected.IsReviewer && selected.PlanFile != "" && m.pendingApprovals[selected.PlanFile] {
				m.reviewApprovalConfirmAction(selected.PlanFile)
				return m, nil
			}
			return m, tea.WindowSize()
		}
```

Also update the `!`/`@`/`#` jump-slot exit paths (lines 489-499). After `m.exitFocusMode()` and before the tab switch logic, add the same check. Replace:

```go
		if doJump {
			wasGitTab := m.tabbedWindow.IsInGitTab()
			m.exitFocusMode()
```

With:

```go
		if doJump {
			selected := m.list.GetSelectedInstance()
			wasGitTab := m.tabbedWindow.IsInGitTab()
			m.exitFocusMode()
			if selected != nil && selected.IsReviewer && selected.PlanFile != "" && m.pendingApprovals[selected.PlanFile] {
				m.reviewApprovalConfirmAction(selected.PlanFile)
				return m, nil
			}
```

**Step 2: Verify it compiles**

Run: `go build ./app/...`
Expected: SUCCESS

**Step 3: Commit**

```bash
git add app/app_input.go
git commit -m "feat(approval): show merge/pr popup when exiting focus mode from approved reviewer"
```

### Task 8: Handle `reviewMergeMsg` and `reviewCreatePRMsg` in Update

**Files:**
- Modify: `app/app.go` (add cases to `Update` type switch)

**Step 1: Handle `reviewMergeMsg`**

Add to the type switch in `Update()`:

```go
	case reviewMergeMsg:
		planFile := msg.planFile
		delete(m.pendingApprovals, planFile)
		if m.planState == nil {
			return m, m.handleError(fmt.Errorf("no plan state loaded"))
		}
		entry, ok := m.planState.Entry(planFile)
		if !ok {
			return m, m.handleError(fmt.Errorf("plan not found: %s", planFile))
		}
		branch := entry.Branch
		if branch == "" {
			branch = gitpkg.PlanBranchFromFile(planFile)
		}
		planName := planstate.DisplayName(planFile)
		m.toastManager.Loading(fmt.Sprintf("merging '%s' to main...", planName))
		capturedPlanFile := planFile
		capturedBranch := branch
		return m, tea.Batch(m.toastTickCmd(), func() tea.Msg {
			// Kill all instances bound to this plan.
			for i := len(m.allInstances) - 1; i >= 0; i-- {
				if m.allInstances[i].PlanFile == capturedPlanFile {
					_ = m.allInstances[i].Kill()
					m.allInstances = append(m.allInstances[:i], m.allInstances[i+1:]...)
				}
			}
			if err := gitpkg.MergePlanBranch(m.activeRepoPath, capturedBranch); err != nil {
				return err
			}
			if err := m.fsm.Transition(capturedPlanFile, planfsm.ReviewApproved); err != nil {
				return err
			}
			_ = m.saveAllInstances()
			m.loadPlanState()
			m.updateSidebarPlans()
			m.updateSidebarItems()
			return planRefreshMsg{}
		})
```

**Step 2: Handle `reviewCreatePRMsg`**

```go
	case reviewCreatePRMsg:
		planFile := msg.planFile
		delete(m.pendingApprovals, planFile)
		// Find the reviewer instance for this plan to get its worktree.
		var planInst *session.Instance
		for _, inst := range m.list.GetInstances() {
			if inst.PlanFile == planFile {
				planInst = inst
				break
			}
		}
		if planInst == nil {
			return m, m.handleError(fmt.Errorf("no active session for plan %s", planFile))
		}
		// Select the instance and enter the PR title flow.
		m.list.SelectInstance(planInst)
		planName := planstate.DisplayName(planFile)
		m.state = statePRTitle
		m.textInputOverlay = overlay.NewTextInputOverlay("pr title", planName)
		m.textInputOverlay.SetSize(60, 3)
		// Mark the plan as done via FSM so it moves to history after PR creation.
		if err := m.fsm.Transition(planFile, planfsm.ReviewApproved); err != nil {
			log.WarningLog.Printf("could not mark plan %q done after PR: %v", planFile, err)
		}
		m.loadPlanState()
		m.updateSidebarPlans()
		m.updateSidebarItems()
		return m, nil
```

**Step 3: Verify it compiles**

Run: `go build ./app/...`
Expected: SUCCESS

**Step 4: Run full test suite**

Run: `go test ./app/... -count=1`
Expected: PASS

**Step 5: Commit**

```bash
git add app/app.go
git commit -m "feat(approval): handle merge and create-pr messages from approval popup"
```

## Wave 4: Context Menu Alternate Path

### Task 9: Add merge/PR options to reviewing plan context menu

**Files:**
- Modify: `app/app_actions.go:525-530` (plan context menu for `StatusReviewing`)

**Step 1: Update the reviewing context menu**

In `app/app_actions.go`, replace the `StatusReviewing` case (lines 525-530):

```go
		case planstate.StatusReviewing:
			items = append(items,
				overlay.ContextMenuItem{Label: "start review", Action: "start_review"},
				overlay.ContextMenuItem{Label: "mark finished", Action: "mark_plan_done"},
			)
```

With:

```go
		case planstate.StatusReviewing:
			items = append(items,
				overlay.ContextMenuItem{Label: "start review", Action: "start_review"},
				overlay.ContextMenuItem{Label: "review & merge", Action: "review_merge"},
				overlay.ContextMenuItem{Label: "create pr & push", Action: "review_create_pr"},
				overlay.ContextMenuItem{Label: "mark finished", Action: "mark_plan_done"},
			)
```

**Step 2: Add action handlers for `review_merge` and `review_create_pr`**

In `app/app_actions.go`, in the `executeContextAction` switch, add two new cases (near the existing `merge_plan` case):

```go
	case "review_merge":
		planFile := m.sidebar.GetSelectedPlanFile()
		if planFile == "" || m.planState == nil {
			return m, nil
		}
		entry, ok := m.planState.Entry(planFile)
		if !ok {
			return m, m.handleError(fmt.Errorf("plan not found: %s", planFile))
		}
		branch := entry.Branch
		if branch == "" {
			branch = gitpkg.PlanBranchFromFile(planFile)
		}
		delete(m.pendingApprovals, planFile)
		planName := planstate.DisplayName(planFile)
		capturedPlanFile := planFile
		capturedBranch := branch
		mergeAction := func() tea.Msg {
			for i := len(m.allInstances) - 1; i >= 0; i-- {
				if m.allInstances[i].PlanFile == capturedPlanFile {
					_ = m.allInstances[i].Kill()
					m.allInstances = append(m.allInstances[:i], m.allInstances[i+1:]...)
				}
			}
			if err := gitpkg.MergePlanBranch(m.activeRepoPath, capturedBranch); err != nil {
				return err
			}
			if err := m.fsm.Transition(capturedPlanFile, planfsm.ReviewApproved); err != nil {
				return err
			}
			_ = m.saveAllInstances()
			m.loadPlanState()
			m.updateSidebarPlans()
			m.updateSidebarItems()
			return planRefreshMsg{}
		}
		return m, m.confirmAction(fmt.Sprintf("merge '%s' branch into main?", planName), mergeAction)

	case "review_create_pr":
		planFile := m.sidebar.GetSelectedPlanFile()
		if planFile == "" || m.planState == nil {
			return m, nil
		}
		delete(m.pendingApprovals, planFile)
		// Find an instance for this plan so the PR flow can find it.
		var planInst *session.Instance
		for _, inst := range m.list.GetInstances() {
			if inst.PlanFile == planFile {
				planInst = inst
				break
			}
		}
		if planInst == nil {
			return m, m.handleError(fmt.Errorf("no active session for this plan"))
		}
		m.list.SelectInstance(planInst)
		planName := planstate.DisplayName(planFile)
		m.state = statePRTitle
		m.textInputOverlay = overlay.NewTextInputOverlay("pr title", planName)
		m.textInputOverlay.SetSize(60, 3)
		// Transition plan to done.
		if err := m.fsm.Transition(planFile, planfsm.ReviewApproved); err != nil {
			log.WarningLog.Printf("could not mark plan %q done: %v", planFile, err)
		}
		m.loadPlanState()
		m.updateSidebarPlans()
		m.updateSidebarItems()
		return m, nil
```

**Step 3: Verify it compiles**

Run: `go build ./app/...`
Expected: SUCCESS

**Step 4: Run full test suite**

Run: `go test ./... -count=1`
Expected: PASS

**Step 5: Commit**

```bash
git add app/app_actions.go
git commit -m "feat(approval): add review & merge / create pr context menu options"
```

## Wave 5: Edge Cases and Cleanup

### Task 10: Clear pending approval on plan cancel/start-over

**Files:**
- Modify: `app/app_actions.go` (cancel_plan and start_over_plan handlers)

**Step 1: Add cleanup to cancel_plan handler**

In `app/app_actions.go`, in the `cancel_plan` case (~line 286), add `delete(m.pendingApprovals, planFile)` before the `cancelAction` closure:

```go
	case "cancel_plan":
		planFile := m.sidebar.GetSelectedPlanFile()
		if planFile == "" || m.planState == nil {
			return m, nil
		}
		delete(m.pendingApprovals, planFile)
		planName := planstate.DisplayName(planFile)
		// ... rest unchanged
```

**Step 2: Add cleanup to start_over_plan handler**

Find the `start_over_plan` case and add `delete(m.pendingApprovals, planFile)` similarly.

**Step 3: Add cleanup to mark_plan_done handler**

In the `mark_plan_done` case (~line 267), add `delete(m.pendingApprovals, planFile)` before the FSM transition.

**Step 4: Verify it compiles**

Run: `go build ./app/...`
Expected: SUCCESS

**Step 5: Run full test suite**

Run: `go test ./... -count=1`
Expected: PASS

**Step 6: Commit**

```bash
git add app/app_actions.go
git commit -m "fix(approval): clear pending approvals on cancel/start-over/mark-done"
```

### Task 11: Prevent duplicate approval popups

**Files:**
- Modify: `app/app.go` (both interception sites)

**Step 1: Guard signal-path interception against duplicate popups**

In the signal handler's `ReviewApproved` interception (added in Task 4), change the confirm popup to only show if not already in `stateConfirm`:

The code already has `m.confirmAction(...)` which sets `stateConfirm`. But if a popup is already showing (e.g. wave advance), we should skip showing another. Wrap the confirm call:

```go
			if sig.Event == planfsm.ReviewApproved {
				planfsm.ConsumeSignal(sig)
				m.pendingApprovals[sig.PlanFile] = true
				if m.state != stateConfirm {
					planName := planstate.DisplayName(sig.PlanFile)
					capturedPlanFile := sig.PlanFile
					m.confirmAction(
						fmt.Sprintf("review approved — %s", planName),
						func() tea.Msg {
							return reviewApprovalFocusMsg{planFile: capturedPlanFile}
						},
					)
				}
				continue
			}
```

The reviewer-death path (Task 5) already has this guard. This makes them consistent.

**Step 2: Guard reviewer-death path against already-approved plans**

The reviewer-death block already checks `!m.pendingApprovals[inst.PlanFile]` (added in Task 5). Good.

**Step 3: Verify**

Run: `go test ./app/... -count=1`
Expected: PASS

**Step 4: Commit**

```bash
git add app/app.go
git commit -m "fix(approval): guard against duplicate approval popups"
```

### Task 12: Esc from Popup 2 clears approval state cleanly

**Files:**
- Modify: `app/app_input.go:604-653` (cancel key handler in stateConfirm)

**Step 1: Verify esc behavior**

The `esc` path in the `stateConfirm` handler (lines 645-653) already resets `m.state = stateDefault` and clears overlays. When the user presses Esc on Popup 2:
- `pendingWaveNextAction` is set (we reuse it for the PR action) — it gets cleared
- `pendingApprovals` stays set — this is correct, the plan remains in `reviewing` and the user can come back via context menu

No code change needed for the esc path. The CancelKey (`p`) path fires `pendingWaveNextAction` which is the `reviewCreatePRMsg`. If the user presses `n` on the default cancel key handler, it would fire `pendingWaveNextAction` — but we set CancelKey to `p`, so the mapping is:
- `m` (ConfirmKey) → merge
- `p` (CancelKey) → create PR (fires `pendingWaveNextAction`)
- `esc` → dismiss, keep pending

This is correct — no additional code needed for this task. Just verify.

**Step 2: Run full test suite one final time**

Run: `go test ./... -count=1`
Expected: PASS

**Step 3: Commit (if any changes were needed)**

No commit needed if no changes. If any edge case fixes were required, commit them.

### Task 13: Final verification

**Step 1: Build the full binary**

Run: `go build ./cmd/...`
Expected: SUCCESS

**Step 2: Run all tests**

Run: `go test ./... -count=1`
Expected: PASS

**Step 3: Run typos check**

Run: `typos app/`
Expected: No new typos

**Step 4: Manual smoke test (if running locally)**

1. Start kasmos
2. Pick a plan in `reviewing` state
3. Verify context menu shows "review & merge" and "create pr & push"
4. If a reviewer is running: wait for it to die or write `review-approved` signal
5. Verify Popup 1 appears ("review approved — ...")
6. Press enter → should enter focus mode on reviewer
7. Exit focus mode (Ctrl+Space) → Popup 2 should appear with m/p/esc choices
8. Press `m` → merge completes, plan moves to done
