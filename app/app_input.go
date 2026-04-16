package app

import (
	"fmt"
	"github.com/charmbracelet/x/ansi"
	"github.com/kastheco/kasmos/config"
	"github.com/kastheco/kasmos/config/auditlog"
	"github.com/kastheco/kasmos/config/taskstate"
	"github.com/kastheco/kasmos/internal/clickup"
	"github.com/kastheco/kasmos/keys"
	"github.com/kastheco/kasmos/log"
	"github.com/kastheco/kasmos/session"
	"github.com/kastheco/kasmos/session/tmux"
	"github.com/kastheco/kasmos/ui"
	"github.com/kastheco/kasmos/ui/overlay"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
	zone "github.com/lrstanley/bubblezone/v2"
	"github.com/mattn/go-runewidth"
)

// firstRuneIsPrintable reports whether the first rune of s is a printable
// character per unicode.IsPrint. It decodes the rune via utf8 so multi-byte
// UTF-8 input (e.g. non-ASCII keystrokes) is classified correctly; indexing
// a string with [0] would inspect only the leading byte and misclassify
// valid printable runes as non-printable.
func firstRuneIsPrintable(s string) bool {
	if s == "" {
		return false
	}
	r, _ := utf8.DecodeRuneInString(s)
	if r == utf8.RuneError {
		return false
	}
	return unicode.IsPrint(r)
}

func (m *home) applyPendingSetStatus(picked string) (tea.Model, tea.Cmd) {
	status, state, err := taskstate.ResolveManualOverride(picked)
	if err != nil {
		m.state = stateDefault
		m.pendingSetStatusTask = ""
		return m, m.handleError(err)
	}
	if err := m.taskState.ForceSetLifecycle(m.pendingSetStatusTask, status, state); err != nil {
		m.state = stateDefault
		m.pendingSetStatusTask = ""
		return m, m.handleError(err)
	}
	m.audit(auditlog.EventPlanTransition, "manual override → "+picked,
		auditlog.WithPlan(m.pendingSetStatusTask),
		auditlog.WithDetail("manual override"))
	m.loadTaskState()
	m.updateSidebarTasks()
	m.toastManager.Success(fmt.Sprintf("status → %s", picked))
	m.state = stateDefault
	m.pendingSetStatusTask = ""
	return m, tea.Batch(tea.RequestWindowSize, m.toastTickCmd())
}

func (m *home) handleMenuHighlighting(msg tea.KeyPressMsg) (cmd tea.Cmd, returnEarly bool) {
	// Handle menu highlighting when you press a button. We intercept it here and immediately return to
	// update the ui while re-sending the keypress. Then, on the next call to this, we actually handle the keypress.
	if m.keySent {
		m.keySent = false
		return nil, false
	}
	if m.state == statePrompt || m.state == stateHelp || m.state == stateConfirm || m.state == stateNewPlan || m.state == stateNewPlanDeriving || m.state == stateNewPlanTopic || m.state == stateSpawnHarnessPicker || m.state == stateSpawnAgent || m.state == stateSearch || m.state == stateContextMenu || m.state == statePRTitle || m.state == statePRBody || m.state == stateRenameInstance || m.state == stateRenameTask || m.state == stateSendPrompt || m.state == stateFocusAgent || m.state == stateChangeTopic || m.state == stateSetStatus || m.state == stateClickUpSearch || m.state == stateClickUpPicker || m.state == stateClickUpFetching || m.state == stateClickUpWorkspacePicker || m.state == statePermission || m.state == stateTmuxBrowser || m.state == stateChatAboutTask || m.state == stateAuditCursor || m.state == stateLauncher || m.state == stateKeybindBrowser || m.state == stateWaveDecision {
		return nil, false
	}
	// If it's in the global keymap, we should try to highlight it.
	name, ok := keys.GlobalKeyStringsMap[msg.String()]
	if !ok {
		return nil, false
	}

	if m.nav.GetSelectedInstance() != nil && m.nav.GetSelectedInstance().Paused() && name == keys.KeyEnter {
		return nil, false
	}
	// (no special-cased keys to skip here)

	// Skip the menu highlighting if the key is not in the map or we are using the shift up and down keys.
	// TODO: cleanup: when you press enter on stateNew, we use keys.KeySubmitName. We should unify the keymap.
	if name == keys.KeyEnter && m.state == stateNew {
		name = keys.KeySubmitName
	}
	m.keySent = true
	return tea.Batch(
		func() tea.Msg { return msg },
		m.keydownCallback(name)), true
}

// handleMouseWheel processes mouse wheel events for scrolling.
func (m *home) handleMouseWheel(msg tea.MouseWheelMsg) (tea.Model, tea.Cmd) {
	if handled, err := m.forwardMouseWheelToPreviewTerminal(msg); handled {
		if err != nil {
			return m, m.handleError(err)
		}
		return m, nil
	}

	agentPane := zone.Get(ui.ZoneAgentPane)
	if !agentPane.InBounds(msg) {
		return m, nil
	}

	if m.tabbedWindow.IsDocumentMode() {
		switch msg.Button {
		case tea.MouseWheelUp:
			m.tabbedWindow.ContentScrollUp()
		case tea.MouseWheelDown:
			m.tabbedWindow.ContentScrollDown()
		}
		return m, nil
	}

	selected := m.nav.GetSelectedInstance()
	if selected != nil && selected.Status != session.Paused && m.previewTerminal == nil {
		switch msg.Button {
		case tea.MouseWheelUp:
			m.tabbedWindow.ContentScrollUp()
		case tea.MouseWheelDown:
			m.tabbedWindow.ContentScrollDown()
		}
	}
	return m, nil
}

func (m *home) forwardMouseWheelToPreviewTerminal(msg tea.MouseWheelMsg) (bool, error) {
	if m.previewTerminal == nil || m.tabbedWindow.IsDocumentMode() {
		return false, nil
	}

	agentPane := zone.Get(ui.ZoneAgentPane)
	if !agentPane.InBounds(msg) {
		return false, nil
	}

	previewWidth, previewHeight := m.tabbedWindow.GetPreviewSize()
	if previewWidth <= 0 || previewHeight <= 0 {
		return true, nil
	}

	x, y := agentPane.Pos(msg)
	if x < 1 || y < 0 || y >= previewHeight || x > previewWidth {
		// Ignore wheel events that land on the preview border chrome, but still
		// consume them so focus mode doesn't fall back to kasmos scroll mode.
		return true, nil
	}
	x-- // ZoneAgentPane includes the left window border; the terminal does not.

	button := ansi.EncodeMouseButton(
		ansi.MouseButton(msg.Button),
		false,
		msg.Mod.Contains(tea.ModShift),
		msg.Mod.Contains(tea.ModAlt),
		msg.Mod.Contains(tea.ModCtrl),
	)
	return true, m.previewTerminal.SendKey([]byte(ansi.MouseSgr(button, x, y, false)))
}

// handleMouseClick processes mouse click events for left/right click interactions.
func (m *home) handleMouseClick(msg tea.MouseClickMsg) (tea.Model, tea.Cmd) {
	if m.overlays.IsActive() {
		return m.handleActiveOverlayMouse(msg)
	}

	// Focus/interactive mode: clicks outside the agent pane exit focus and
	// fall through to normal click handling so the intended target is activated.
	if m.state == stateFocusAgent {
		if !zone.Get(ui.ZoneAgentPane).InBounds(msg) {
			m.exitFocusMode()
			// Fall through — stateDefault click handlers below will process the click.
		} else {
			// Click inside the agent pane — stay in focus mode, no-op.
			return m, nil
		}
	}

	if m.state != stateDefault {
		return m, nil
	}

	// Right-click: show context menu
	if msg.Button == tea.MouseRight {
		return m.handleRightClick(msg)
	}

	// Only handle left clicks from here
	if msg.Button != tea.MouseLeft {
		return m, nil
	}

	// Zone-based click: search box
	if zone.Get(ui.ZoneNavSearch).InBounds(msg) {
		m.setFocusSlot(slotNav)
		m.nav.ActivateSearch()
		m.state = stateSearch
		return m, nil
	}

	// Zone-based click: dynamic instance tab headers — switch without stealing sidebar focus.
	for i := 0; i < m.tabbedWindow.TabCount(); i++ {
		if zone.Get(ui.InstanceTabZoneID(i)).InBounds(msg) {
			m.tabbedWindow.SetActiveTab(i)
			return m, m.tabSwitched()
		}
	}

	// Zone-based click: "view plan doc" button in info tab
	if zone.Get(ui.ZoneViewPlan).InBounds(msg) {
		return m.viewSelectedPlan()
	}

	// Zone-based click: nav panel rows
	if zone.Get(ui.ZoneNavPanel).InBounds(msg) {
		m.setFocusSlot(slotNav)
		for i := range m.nav.RowCount() {
			if zone.Get(ui.NavRowZoneID(i)).InBounds(msg) {
				m.tabbedWindow.ClearDocumentMode()
				m.nav.ClickItem(i)
				return m, m.instanceChanged()
			}
		}
		return m, nil
	}

	// Click in tabbed window area — sidebar retains focus.
	return m, nil
}

func (m *home) handleActiveOverlayMouse(msg tea.MouseClickMsg) (tea.Model, tea.Cmd) {
	if msg.Button != tea.MouseLeft {
		return m, nil
	}

	current := m.overlays.Current()
	result := m.overlays.HandleMouse(msg)
	if result == (overlay.Result{}) && m.overlays.IsActive() {
		return m, nil
	}

	switch m.state {
	case stateContextMenu:
		m.state = stateDefault
		if result.Action != "" {
			m.clearContextMenuTracking()
			return m.executeContextAction(result.Action)
		}
		// If dismissed without an action and there was a pending log event, clear it.
		m.clearContextMenuTracking()
		m.handleAuditCursorContextMenuDismissed()
		return m, nil

	case stateHelp:
		if to, ok := current.(*overlay.TextOverlay); ok && to.OnDismiss != nil {
			to.OnDismiss()
		}
		m.state = stateDefault
		pending := m.pendingAttachInstance
		m.pendingAttachInstance = nil
		if pending != nil && pending.Started() && !pending.Paused() && pending.TmuxAlive() {
			return m, tea.Exec(tmux.NewAttachExecCommand(pending), func(err error) tea.Msg {
				if err != nil {
					return err
				}
				return tmuxAttachReturnMsg{}
			})
		}
		return m, tea.Sequence(tea.RequestWindowSize, func() tea.Msg {
			m.menu.SetState(ui.StateDefault)
			return nil
		})

	case statePermission:
		return m.finishPermissionOverlay(result)

	case stateConfirm:
		if result.Submitted {
			if m.pendingAllCompleteTaskFile != "" {
				if m.allCompleteAdvancing == nil {
					m.allCompleteAdvancing = make(map[string]bool)
				}
				m.allCompleteAdvancing[m.pendingAllCompleteTaskFile] = true
			}
			action := m.pendingConfirmAction
			m.state = stateDefault
			m.pendingConfirmAction = nil
			return m, action
		}
		if m.pendingAllCompleteTaskFile != "" {
			planFile := m.pendingAllCompleteTaskFile
			m.allCompleteDismissed[planFile] = true
			m.resolveAllCompleteToast(planFile, overlay.ToastInfo,
				fmt.Sprintf("review deferred for '%s'", taskstate.DisplayName(planFile)))
			m.pendingAllCompleteTaskFile = ""
		}
		if m.pendingPlannerTaskFile != "" {
			m.plannerPrompted[m.pendingPlannerTaskFile] = true
			m.killExistingPlanAgent(m.pendingPlannerTaskFile, session.AgentTypePlanner)
			_ = m.saveAllInstances()
			m.updateNavPanelStatus()
			m.pendingPlannerInstanceTitle = ""
			m.pendingPlannerTaskFile = ""
		}
		m.state = stateDefault
		m.pendingConfirmAction = nil
		return m, nil

	case stateWaveDecision:
		// Outside click behaves like esc: dismiss, reset latch, start cooldown.
		wd, ok := current.(*overlay.WaveDecisionOverlay)
		if result.Submitted && ok {
			planFile := wd.Input().PlanFile
			m.state = stateDefault
			switch result.Action {
			case "advance":
				entry, entryOK := m.taskState.Entry(planFile)
				if !entryOK {
					return m, m.handleError(fmt.Errorf("task not found: %s", planFile))
				}
				capturedEntry := entry
				capturedPlanFile := planFile
				return m, func() tea.Msg {
					return waveAdvanceMsg{planFile: capturedPlanFile, entry: capturedEntry}
				}
			case "retry":
				entry, entryOK := m.taskState.Entry(planFile)
				if !entryOK {
					return m, m.handleError(fmt.Errorf("task not found: %s", planFile))
				}
				capturedEntry := entry
				capturedPlanFile := planFile
				return m, func() tea.Msg {
					return waveRetryMsg{planFile: capturedPlanFile, entry: capturedEntry}
				}
			case "abort":
				capturedPlanFile := planFile
				return m, func() tea.Msg {
					return waveAbortMsg{planFile: capturedPlanFile}
				}
			}
			return m, nil
		}
		// Dismissed without submit (outside click or cancel button).
		if ok {
			planFile := wd.Input().PlanFile
			if orch, orchOK := m.waveOrchestrators[planFile]; orchOK {
				orch.ResetConfirm()
			}
			m.waveConfirmDismissedAt = time.Now()
		}
		m.state = stateDefault
		return m, nil

	case statePrompt:
		selected := m.nav.GetSelectedInstance()
		if selected == nil {
			m.state = stateDefault
			m.menu.SetState(ui.StateDefault)
			return m, tea.RequestWindowSize
		}
		m.state = stateDefault
		return m, tea.Sequence(
			tea.RequestWindowSize,
			func() tea.Msg {
				m.menu.SetState(ui.StateDefault)
				m.showHelpScreen(helpStart(selected), nil)
				return nil
			},
		)

	case statePRTitle:
		m.pendingPRWorktree = nil
		m.state = stateDefault
		m.menu.SetState(ui.StateDefault)
		return m, tea.RequestWindowSize

	case statePRBody:
		m.pendingPRTitle = ""
		m.pendingPRWorktree = nil
		m.state = stateDefault
		m.menu.SetState(ui.StateDefault)
		return m, tea.RequestWindowSize

	case stateRenameInstance:
		m.state = stateDefault
		m.menu.SetState(ui.StateDefault)
		return m, tea.RequestWindowSize

	case stateRenameTask:
		m.state = stateDefault
		m.menu.SetState(ui.StateDefault)
		return m, tea.RequestWindowSize

	case stateChatAboutTask:
		m.pendingChatAboutTask = ""
		m.state = stateDefault
		m.menu.SetState(ui.StateDefault)
		return m, tea.RequestWindowSize

	case stateSendPrompt:
		m.state = stateDefault
		m.menu.SetState(ui.StateDefault)
		return m, tea.RequestWindowSize

	case stateNewPlan:
		m.state = stateDefault
		m.menu.SetState(ui.StateDefault)
		return m, tea.RequestWindowSize

	case stateNewPlanTopic:
		topic := ""
		if result.Submitted {
			picked := result.Value
			if picked != "(No topic)" {
				topic = picked
			}
		}
		if err := m.createTaskEntry(m.pendingPlanName, m.pendingPlanDesc, topic); err != nil {
			m.state = stateDefault
			m.menu.SetState(ui.StateDefault)
			m.pendingPlanName = ""
			m.pendingPlanDesc = ""
			return m, m.handleError(err)
		}
		m.loadTaskState()
		m.updateSidebarTasks()
		m.state = stateDefault
		m.menu.SetState(ui.StateDefault)
		m.pendingPlanName = ""
		m.pendingPlanDesc = ""
		return m, tea.RequestWindowSize

	case stateSpawnHarnessPicker:
		if result.Submitted && result.Value != "" {
			m.showSpawnAgentForm(result.Value)
			return m, nil
		}
		m.pendingSpawnProgram = ""
		m.state = stateDefault
		m.menu.SetState(ui.StateDefault)
		return m, tea.RequestWindowSize

	case stateSpawnAgent:
		m.pendingSpawnProgram = ""
		m.state = stateDefault
		m.menu.SetState(ui.StateDefault)
		return m, tea.RequestWindowSize

	case stateChangeTopic:
		if result.Submitted && m.taskState != nil && m.pendingChangeTopicTask != "" {
			picked := result.Value
			newTopic := ""
			if picked != "(No topic)" {
				newTopic = picked
			}
			if err := m.taskState.SetTopic(m.pendingChangeTopicTask, newTopic); err != nil {
				m.state = stateDefault
				m.pendingChangeTopicTask = ""
				return m, m.handleError(err)
			}
			m.updateSidebarTasks()
		}
		m.state = stateDefault
		m.pendingChangeTopicTask = ""
		return m, tea.RequestWindowSize

	case stateSetStatus:
		if result.Submitted && m.taskState != nil && m.pendingSetStatusTask != "" {
			picked := result.Value
			if picked != "" {
				return m.applyPendingSetStatus(picked)
			}
		}
		m.state = stateDefault
		m.pendingSetStatusTask = ""
		return m, tea.RequestWindowSize

	case stateClickUpSearch:
		m.state = stateDefault
		return m, nil

	case stateClickUpPicker:
		if result.Submitted {
			selected := result.Value
			if selected != "" {
				for _, r := range m.clickUpResults {
					label := r.ID + " · " + r.Name
					if strings.HasPrefix(selected, label) {
						m.state = stateClickUpFetching
						m.toastManager.Info("fetching task details...")
						return m, tea.Batch(m.fetchClickUpTaskWithTimeout(r.ID), m.toastTickCmd())
					}
				}
			}
		}
		m.state = stateDefault
		return m, nil

	case stateClickUpWorkspacePicker:
		if result.Submitted {
			selected := result.Value
			if selected != "" && m.clickUpImporter != nil {
				wsID := selected
				if id, ok := m.clickUpWorkspaceMap[selected]; ok {
					wsID = id
				}
				m.clickUpImporter.SetWorkspaceID(wsID)
				if err := clickup.SaveProjectConfig(m.activeRepoPath, &clickup.ProjectConfig{WorkspaceID: wsID}); err != nil {
					log.WarningLog.Printf("failed to save clickup workspace config: %v", err)
				}
				query := m.clickUpPendingQuery
				m.clickUpPendingQuery = ""
				m.clickUpWorkspaceMap = nil
				m.state = stateClickUpFetching
				m.toastManager.Info("searching clickup...")
				return m, tea.Batch(m.searchClickUp(query), m.toastTickCmd())
			}
		}
		m.state = stateDefault
		m.clickUpPendingQuery = ""
		m.clickUpWorkspaceMap = nil
		return m, nil

	case stateTmuxBrowser:
		browser, _ := current.(*overlay.TmuxBrowserOverlay)
		m.state = stateDefault
		return m.handleTmuxBrowserAction(browser, result.Action)
	}

	return m, nil
}

func (m *home) finishPermissionOverlay(result overlay.Result) (tea.Model, tea.Cmd) {
	if result.Submitted {
		cacheKey := config.CacheKey(m.pendingPermissionPattern, m.pendingPermissionDesc)
		inst := m.pendingPermissionInstance

		var choice overlay.PermissionChoice
		switch result.Action {
		case "allow_always":
			choice = overlay.PermissionAllowAlways
		case "reject":
			choice = overlay.PermissionReject
		default:
			choice = overlay.PermissionAllowOnce
		}

		if choice == overlay.PermissionAllowAlways && cacheKey != "" && m.permissionStore != nil {
			m.permissionStore.Remember(m.activeProject(), cacheKey)
		}

		m.state = stateDefault

		if inst != nil {
			guardKey := cacheKey
			if guardKey == "" {
				guardKey = "__handled__"
			}
			m.permissionHandled[inst] = guardKey
		}

		if inst != nil {
			capturedInst := inst
			capturedChoice := tmux.PermissionChoice(choice)
			choiceStr := "allow once"
			switch choice {
			case overlay.PermissionAllowAlways:
				choiceStr = "allow always"
			case overlay.PermissionReject:
				choiceStr = "reject"
			}
			m.audit(auditlog.EventPermissionAnswered, choiceStr,
				auditlog.WithInstance(inst.Title),
			)
			m.pendingPermissionInstance = nil
			m.pendingPermissionPattern = ""
			m.pendingPermissionDesc = ""
			var cmds []tea.Cmd
			cmds = append(cmds, func() tea.Msg {
				capturedInst.SendPermissionResponse(capturedChoice)
				return nil
			})
			// Restore original nav row once the last queued prompt is answered.
			// Using SelectByID preserves non-instance selections (plan headers,
			// history rows) — not just instance rows.
			if m.preOverlayCaptured && len(m.deferredPermissionPrompts) == 0 {
				savedID := m.preOverlayNavID
				m.preOverlayNavID = ""
				m.preOverlayCaptured = false
				if savedID != "" && m.nav.SelectByID(savedID) {
					cmds = append(cmds, m.instanceChanged())
				}
			}
			return m, tea.Batch(cmds...)
		}
	}

	if m.pendingPermissionInstance != nil {
		guardKey := m.pendingPermissionPattern
		if guardKey == "" {
			guardKey = "__handled__"
		}
		m.permissionHandled[m.pendingPermissionInstance] = guardKey
	}
	m.pendingPermissionInstance = nil
	m.pendingPermissionPattern = ""
	m.pendingPermissionDesc = ""
	m.state = stateDefault
	// Restore original nav row once the last queued prompt is dismissed.
	// Using SelectByID preserves non-instance selections (plan headers,
	// history rows) — not just instance rows.
	if m.preOverlayCaptured && len(m.deferredPermissionPrompts) == 0 {
		savedID := m.preOverlayNavID
		m.preOverlayNavID = ""
		m.preOverlayCaptured = false
		if savedID != "" && m.nav.SelectByID(savedID) {
			return m, m.instanceChanged()
		}
	}
	return m, nil
}

// handleRightClick builds and shows a context menu based on what was right-clicked.
func (m *home) handleRightClick(msg tea.MouseClickMsg) (tea.Model, tea.Cmd) {
	// Right-click in nav panel: select the clicked row then show context menu
	if zone.Get(ui.ZoneNavPanel).InBounds(msg) {
		clickedRow := false
		for i := range m.nav.RowCount() {
			if zone.Get(ui.NavRowZoneID(i)).InBounds(msg) {
				m.nav.ClickItem(i)
				m.setFocusSlot(slotNav)
				clickedRow = true
				break
			}
		}
		if clickedRow {
			return m.openContextMenu()
		}
		return m, nil
	}
	return m, nil
}

func isCommandLauncherKey(msg tea.KeyPressMsg) bool {
	return msg.Code == tea.KeySpace && msg.Mod.Contains(tea.ModShift) || msg.String() == "shift+space"
}

func (m *home) handleKeyPress(msg tea.KeyPressMsg) (mod tea.Model, cmd tea.Cmd) {
	cmd, returnEarly := m.handleMenuHighlighting(msg)
	if returnEarly {
		return m, cmd
	}

	if m.state == stateContextMenu {
		if !m.overlays.IsActive() {
			m.state = stateDefault
			m.clearContextMenuTracking()
			m.handleAuditCursorContextMenuDismissed()
			return m, nil
		}
		result := m.overlays.HandleKey(msg)
		if result.Dismissed {
			m.state = stateDefault
			if result.Action != "" {
				m.clearContextMenuTracking()
				return m.executeContextAction(result.Action)
			}
			// Dismissed without action — clear any pending log event.
			m.clearContextMenuTracking()
			m.handleAuditCursorContextMenuDismissed()
			return m, nil
		}
		return m, nil
	}

	// stateAuditCursor: navigate log lines with ↑/↓, open context menu on enter/space, exit on esc.
	if m.state == stateAuditCursor {
		return m.handleAuditCursorKey(msg)
	}

	if m.state == stateHelp {
		return m.handleHelpState(msg)
	}

	if m.state == stateNew {
		// Handle quit commands first. Don't handle q because the user might want to type that.
		if msg.String() == "ctrl+c" {
			m.state = stateDefault
			m.newInstance = nil
			m.promptAfterName = false
			m.nav.Kill()
			return m, tea.Sequence(
				tea.RequestWindowSize,
				func() tea.Msg {
					m.menu.SetState(ui.StateDefault)
					return nil
				},
			)
		}

		instance := m.newInstance
		if instance == nil {
			// stateNew without a pending instance — shouldn't happen, return to default
			m.state = stateDefault
			m.menu.SetState(ui.StateDefault)
			return m, nil
		}
		switch msg.Code {
		// Start the instance (enable previews etc) and go back to the main menu state.
		case tea.KeyEnter:
			if len(instance.Title) == 0 {
				return m, m.handleError(fmt.Errorf("title cannot be empty"))
			}

			// Set loading status and transition to default state immediately
			instance.SetStatus(session.Loading)
			m.state = stateDefault
			m.newInstance = nil
			m.menu.SetState(ui.StateDefault)

			// Handle prompt-after-name flow
			if m.promptAfterName {
				m.state = statePrompt
				m.menu.SetState(ui.StatePrompt)
				tio := overlay.NewTextInputOverlay("enter prompt", "")
				tio.SetSize(50, 5)
				m.overlays.Show(tio)
				m.promptAfterName = false
			}

			// Start instance asynchronously
			startCmd := func() tea.Msg {
				return instanceStartedMsg{instance: instance, err: instance.Start(true)}
			}

			return m, tea.Batch(tea.RequestWindowSize, startCmd)
		case tea.KeyBackspace:
			runes := []rune(instance.Title)
			if len(runes) == 0 {
				return m, nil
			}
			if err := instance.SetTitle(string(runes[:len(runes)-1])); err != nil {
				return m, m.handleError(err)
			}
		case tea.KeySpace:
			if err := instance.SetTitle(instance.Title + " "); err != nil {
				return m, m.handleError(err)
			}
		case tea.KeyEscape:
			m.nav.Kill()
			m.state = stateDefault
			m.newInstance = nil
			m.instanceChanged()

			return m, tea.Sequence(
				tea.RequestWindowSize,
				func() tea.Msg {
					m.menu.SetState(ui.StateDefault)
					return nil
				},
			)
		default:
			if len(msg.Text) > 0 {
				if runewidth.StringWidth(instance.Title) >= 32 {
					return m, m.handleError(fmt.Errorf("title cannot be longer than 32 characters"))
				}
				if err := instance.SetTitle(instance.Title + msg.Text); err != nil {
					return m, m.handleError(err)
				}
			}
		}
		return m, nil
	} else if m.state == statePrompt {
		// Use the new TextInputOverlay component to handle all key events
		result := m.overlays.HandleKey(msg)

		// Check if the form was submitted or canceled
		if result.Dismissed {
			selected := m.nav.GetSelectedInstance()
			// TODO: this should never happen since we set the instance in the previous state.
			if selected == nil {
				return m, nil
			}
			if result.Submitted {
				promptText := result.Value
				if err := selected.SendPrompt(promptText); err != nil {
					// TODO: we probably end up in a bad state here.
					return m, m.handleError(err)
				}
				// Emit audit event for prompt sent (truncate to 200 chars).
				msg := promptText
				if len(msg) > 200 {
					msg = msg[:200]
				}
				m.audit(auditlog.EventPromptSent, msg,
					auditlog.WithInstance(selected.Title),
				)
			}

			// Close the overlay and reset state
			m.state = stateDefault
			return m, tea.Sequence(
				tea.RequestWindowSize,
				func() tea.Msg {
					m.menu.SetState(ui.StateDefault)
					m.showHelpScreen(helpStart(selected), nil)
					return nil
				},
			)
		}

		return m, nil
	}

	// Handle PR title input state
	if m.state == statePRTitle {
		if !m.overlays.IsActive() {
			m.state = stateDefault
			m.pendingPRWorktree = nil
			return m, nil
		}
		result := m.overlays.HandleKey(msg)
		if result.Dismissed {
			if result.Submitted {
				prTitle := result.Value
				if prTitle != "" {
					m.pendingPRTitle = prTitle

					// Generate a PR body from git data. Use pendingPRWorktree when
					// available (plan-level PR without a running instance), otherwise
					// fall back to the selected instance's worktree.
					var prWorktree interface {
						GeneratePRBody() (string, error)
					}
					if m.pendingPRWorktree != nil {
						prWorktree = m.pendingPRWorktree
					} else if selected := m.nav.GetSelectedInstance(); selected != nil {
						if wt, err := selected.GetGitWorktree(); err == nil {
							prWorktree = wt
						}
					}
					generatedBody := ""
					if prWorktree != nil {
						if body, genErr := prWorktree.GeneratePRBody(); genErr == nil {
							generatedBody = body
						}
					}

					// Transition to PR body editing state
					m.state = statePRBody
					tio := overlay.NewTextInputOverlay("pr description (edit or submit)", generatedBody)
					tio.SetSize(80, 20)
					m.overlays.Show(tio)
					return m, nil
				}
			}
			m.pendingPRWorktree = nil
			m.state = stateDefault
			m.menu.SetState(ui.StateDefault)
			return m, tea.RequestWindowSize
		}
		return m, nil
	}

	// Handle PR body input state
	if m.state == statePRBody {
		if !m.overlays.IsActive() {
			m.state = stateDefault
			m.pendingPRWorktree = nil
			return m, nil
		}
		result := m.overlays.HandleKey(msg)
		if result.Dismissed {
			if result.Submitted {
				prBody := result.Value
				prTitle := m.pendingPRTitle
				if prTitle != "" {
					m.pendingPRTitle = ""
					m.state = stateDefault
					m.menu.SetState(ui.StateDefault)
					m.pendingPRToastID = m.toastManager.Loading("creating PR...")
					prToastID := m.pendingPRToastID
					capturedPRTitle := prTitle

					// Use pendingPRWorktree (plan-level PR without a running instance)
					// when available; otherwise fall back to the selected instance's worktree.
					if pendingWT := m.pendingPRWorktree; pendingWT != nil {
						m.pendingPRWorktree = nil
						capturedWT := pendingWT
						return m, tea.Batch(tea.RequestWindowSize, func() tea.Msg {
							commitMsg := fmt.Sprintf("[kas] update on %s", time.Now().Format(time.RFC822))
							if err := capturedWT.CreatePR(capturedPRTitle, prBody, commitMsg); err != nil {
								return prErrorMsg{id: prToastID, err: err}
							}
							return prCreatedMsg{instanceTitle: capturedPRTitle, prTitle: capturedPRTitle}
						}, m.toastTickCmd())
					}

					selected := m.nav.GetSelectedInstance()
					if selected != nil {
						capturedTitle := selected.Title
						return m, tea.Batch(tea.RequestWindowSize, func() tea.Msg {
							commitMsg := fmt.Sprintf("[kas] update from '%s' on %s", capturedTitle, time.Now().Format(time.RFC822))
							worktree, err := selected.GetGitWorktree()
							if err != nil {
								return prErrorMsg{id: prToastID, err: err}
							}
							if err := worktree.CreatePR(capturedPRTitle, prBody, commitMsg); err != nil {
								return prErrorMsg{id: prToastID, err: err}
							}
							return prCreatedMsg{instanceTitle: capturedTitle, prTitle: capturedPRTitle}
						}, m.toastTickCmd())
					}

					// Neither worktree nor instance — surface an error.
					m.toastManager.Resolve(prToastID, overlay.ToastError, "no instance or branch to create PR from")
					return m, m.toastTickCmd()
				}
			}
			m.pendingPRTitle = ""
			m.pendingPRWorktree = nil
			m.state = stateDefault
			m.menu.SetState(ui.StateDefault)
			return m, tea.RequestWindowSize
		}
		return m, nil
	}

	// Handle instance rename state
	if m.state == stateRenameInstance {
		if !m.overlays.IsActive() {
			m.state = stateDefault
			return m, nil
		}
		result := m.overlays.HandleKey(msg)
		if result.Dismissed {
			if result.Submitted {
				newName := result.Value
				selected := m.nav.GetSelectedInstance()
				if selected != nil && newName != "" {
					if err := m.syncInstanceDisplayTitle(selected, newName); err != nil {
						m.state = stateDefault
						m.menu.SetState(ui.StateDefault)
						return m, m.handleError(err)
					}
				}
			}
			m.state = stateDefault
			m.menu.SetState(ui.StateDefault)
			return m, tea.RequestWindowSize
		}
		return m, nil
	}

	// Handle plan rename state
	if m.state == stateRenameTask {
		if !m.overlays.IsActive() {
			m.state = stateDefault
			return m, nil
		}
		result := m.overlays.HandleKey(msg)
		if result.Dismissed {
			if result.Submitted {
				newName := strings.TrimSpace(result.Value)
				planFile := m.nav.GetSelectedPlanFile()
				if planFile != "" && newName != "" && m.taskState != nil {
					oldFile := planFile
					newFile, err := m.taskState.Rename(oldFile, newName)
					if err != nil {
						m.state = stateDefault
						m.menu.SetState(ui.StateDefault)
						return m, m.handleError(err)
					}
					// Update any instances that referenced the old plan file.
					for _, inst := range m.nav.GetInstances() {
						if inst.TaskFile == oldFile {
							inst.TaskFile = newFile
						}
					}
					for _, inst := range m.allInstances {
						if inst.TaskFile == oldFile {
							inst.TaskFile = newFile
						}
					}
					_ = m.saveAllInstances()
					m.updateSidebarTasks()
					m.nav.SelectByID(ui.SidebarPlanPrefix + newFile)
				}
			}
			m.state = stateDefault
			m.menu.SetState(ui.StateDefault)
			return m, tea.RequestWindowSize
		}
		return m, nil
	}

	// Handle chat-about-plan question input
	if m.state == stateChatAboutTask {
		if !m.overlays.IsActive() {
			m.state = stateDefault
			return m, nil
		}
		result := m.overlays.HandleKey(msg)
		if result.Dismissed {
			if result.Submitted {
				question := result.Value
				planFile := m.pendingChatAboutTask
				m.pendingChatAboutTask = ""
				m.state = stateDefault
				m.menu.SetState(ui.StateDefault)
				if planFile != "" && question != "" {
					return m.spawnChatAboutTask(planFile, question)
				}
				return m, tea.RequestWindowSize
			}
			m.pendingChatAboutTask = ""
			m.state = stateDefault
			m.menu.SetState(ui.StateDefault)
			return m, tea.RequestWindowSize
		}
		return m, nil
	}

	// Handle focus mode — forward keys directly to the agent's PTY (tmux) or
	// compose prompts via overlay (sdk).
	if m.state == stateFocusAgent {
		// Ctrl+Space exits focus mode for both backends.
		if msg.Code == tea.KeySpace && msg.Mod.Contains(tea.ModCtrl) {
			m.exitFocusMode()
			return m, tea.RequestWindowSize
		}

		// Ctrl+Up/Down: cycle through active instances (wrapping) while staying in focus mode
		if msg.Code == tea.KeyUp && msg.Mod.Contains(tea.ModCtrl) || msg.Code == tea.KeyDown && msg.Mod.Contains(tea.ModCtrl) {
			if msg.Code == tea.KeyUp && msg.Mod.Contains(tea.ModCtrl) {
				m.nav.CyclePrevActive()
			} else {
				m.nav.CycleNextActive()
			}
			cmd := m.instanceChanged()
			// Re-enter focus mode for the newly selected instance
			focusCmd := m.enterFocusMode()
			return m, tea.Batch(cmd, focusCmd)
		}

		selected := m.nav.GetSelectedInstance()

		// SDK sessions have no PTY — handle key events without forwarding bytes.
		if selected != nil && session.NormalizeExecutionMode(selected.ExecutionMode) == session.ExecutionModeSDK {
			switch {
			case msg.Code == tea.KeyEscape:
				// Interrupt if actively running; exit focus mode if already idle.
				if selected.Status == session.Running {
					if err := selected.Interrupt(); err != nil {
						return m, m.handleError(err)
					}
					return m, nil
				}
				m.exitFocusMode()
				return m, tea.RequestWindowSize

			case msg.Code == tea.KeyEnter || firstRuneIsPrintable(msg.Text):
				// Open the send-prompt overlay seeded with the typed character so
				// letter keys type into the active input, matching overlay/search behaviour.
				seed := ""
				if msg.Code != tea.KeyEnter && len(msg.Text) > 0 {
					seed = msg.Text
				}
				m.exitFocusMode()
				m.state = stateSendPrompt
				tio := overlay.NewTextInputOverlay("send prompt", seed)
				tio.SetSize(60, 3)
				m.overlays.Show(tio)
				return m, nil
			}
			// All other keys are no-ops for SDK sessions in focus mode.
			return m, nil
		}

		// tmux path: Ctrl+Shift+Enter sends CR then exits focus mode.
		if msg.Code == tea.KeyEnter && msg.Mod.Contains(tea.ModCtrl) && msg.Mod.Contains(tea.ModShift) {
			if m.previewTerminal == nil {
				m.exitFocusMode()
				return m, tea.RequestWindowSize
			}
			if err := m.previewTerminal.SendKey([]byte{0x0D}); err != nil {
				return m, m.handleError(err)
			}
			m.exitFocusMode()
			return m, tea.RequestWindowSize
		}

		// Preview tab focus: forward to embedded terminal
		if m.previewTerminal == nil {
			if selected != nil && selected.Started() && selected.Status != session.Paused && selected.Status != session.Loading && !selected.Exited {
				m.previewRequested = true
				return m, tea.Batch(tea.RequestWindowSize, m.syncPreviewTerminal())
			}
			m.exitFocusMode()
			return m, nil
		}
		data := keyToBytes(msg)
		if data == nil {
			return m, nil
		}
		if err := m.previewTerminal.SendKey(data); err != nil {
			return m, m.handleError(err)
		}
		return m, nil
	}

	// Handle send prompt state
	if m.state == stateSendPrompt {
		if !m.overlays.IsActive() {
			m.state = stateDefault
			return m, nil
		}
		result := m.overlays.HandleKey(msg)
		if result.Dismissed {
			if result.Submitted {
				value := result.Value
				selected := m.nav.GetSelectedInstance()
				if selected != nil && value != "" {
					if err := selected.SendPrompt(value); err != nil {
						m.state = stateDefault
						m.menu.SetState(ui.StateDefault)
						return m, m.handleError(err)
					}
					selected.SetStatus(session.Running)
					// Emit audit event for prompt sent (truncate to 200 chars).
					auditMsg := value
					if len(auditMsg) > 200 {
						auditMsg = auditMsg[:200]
					}
					m.audit(auditlog.EventPromptSent, auditMsg,
						auditlog.WithInstance(selected.Title),
					)
				}
			}
			m.state = stateDefault
			m.menu.SetState(ui.StateDefault)
			return m, tea.RequestWindowSize
		}
		return m, nil
	}

	// Handle confirmation state (generic: all-complete, planner, coder-push).
	// Wave decisions are handled separately by stateWaveDecision below.
	if m.state == stateConfirm {
		if !m.overlays.IsActive() {
			m.state = stateDefault
			return m, nil
		}
		// Pre-intercept 'enter' as an alias for the confirm key.
		// ConfirmationOverlay.HandleKey only handles ConfirmKey ("y"/"r"), not "enter".
		effectiveMsg := msg
		if msg.Code == tea.KeyEnter {
			if co, ok := m.overlays.Current().(*overlay.ConfirmationOverlay); ok {
				effectiveMsg = tea.KeyPressMsg{Text: co.ConfirmKey, Code: rune(co.ConfirmKey[0])}
			}
		}
		result := m.overlays.HandleKey(effectiveMsg)
		if result.Dismissed {
			if result.Submitted {
				// Confirmed (ConfirmKey pressed).
				if m.pendingAllCompleteTaskFile != "" {
					if m.allCompleteAdvancing == nil {
						m.allCompleteAdvancing = make(map[string]bool)
					}
					m.allCompleteAdvancing[m.pendingAllCompleteTaskFile] = true
				}
				action := m.pendingConfirmAction
				m.state = stateDefault
				m.pendingConfirmAction = nil
				// Return the action as a tea.Cmd so bubbletea runs it asynchronously.
				// This prevents blocking the UI during I/O (git push, etc.).
				return m, action
			}
			// Cancelled (CancelKey or Esc).
			cancelKey := result.Action // Action holds the key that triggered cancel
			if cancelKey == "" {
				// Esc dismiss.
				if m.pendingAllCompleteTaskFile != "" {
					planFile := m.pendingAllCompleteTaskFile
					m.allCompleteDismissed[planFile] = true
					m.resolveAllCompleteToast(planFile, overlay.ToastInfo,
						fmt.Sprintf("review deferred for '%s'", taskstate.DisplayName(planFile)))
					m.pendingAllCompleteTaskFile = ""
				}
				// Planner signal esc: same as cancel — signal is consumed, can't re-trigger.
				if m.pendingPlannerTaskFile != "" {
					m.plannerPrompted[m.pendingPlannerTaskFile] = true
					m.killExistingPlanAgent(m.pendingPlannerTaskFile, session.AgentTypePlanner)
					_ = m.saveAllInstances()
					m.updateNavPanelStatus()
					m.pendingPlannerInstanceTitle = ""
					m.pendingPlannerTaskFile = ""
				}
				m.state = stateDefault
				m.pendingConfirmAction = nil
				return m, nil
			}
			// "No" — user explicitly declined.
			if m.pendingAllCompleteTaskFile != "" {
				planFile := m.pendingAllCompleteTaskFile
				m.allCompleteDismissed[planFile] = true
				m.resolveAllCompleteToast(planFile, overlay.ToastInfo,
					fmt.Sprintf("review deferred for '%s'", taskstate.DisplayName(planFile)))
				m.pendingAllCompleteTaskFile = ""
			}
			// Planner signal "no": kill planner instance, mark prompted, leave plan ready.
			if m.pendingPlannerTaskFile != "" {
				m.plannerPrompted[m.pendingPlannerTaskFile] = true
				m.killExistingPlanAgent(m.pendingPlannerTaskFile, session.AgentTypePlanner)
				_ = m.saveAllInstances()
				m.updateNavPanelStatus()
				m.pendingPlannerInstanceTitle = ""
				m.pendingPlannerTaskFile = ""
			}
			m.state = stateDefault
			m.pendingConfirmAction = nil
			return m, nil
		}
		return m, nil
	}

	// Handle wave decision state (intermediate wave: advance / retry / abort).
	if m.state == stateWaveDecision {
		if !m.overlays.IsActive() {
			m.state = stateDefault
			return m, nil
		}
		current := m.overlays.Current()
		result := m.overlays.HandleKey(msg)
		if result.Dismissed {
			wd, ok := current.(*overlay.WaveDecisionOverlay)
			if !ok {
				m.state = stateDefault
				return m, nil
			}
			planFile := wd.Input().PlanFile
			if !result.Submitted {
				// esc or cancel button — reset latch and start cooldown.
				if orch, orchOK := m.waveOrchestrators[planFile]; orchOK {
					orch.ResetConfirm()
				}
				m.waveConfirmDismissedAt = time.Now()
				m.state = stateDefault
				return m, nil
			}
			m.state = stateDefault
			switch result.Action {
			case "advance":
				entry, entryOK := m.taskState.Entry(planFile)
				if !entryOK {
					return m, m.handleError(fmt.Errorf("task not found: %s", planFile))
				}
				capturedEntry := entry
				capturedPlanFile := planFile
				return m, func() tea.Msg {
					return waveAdvanceMsg{planFile: capturedPlanFile, entry: capturedEntry}
				}
			case "retry":
				entry, entryOK := m.taskState.Entry(planFile)
				if !entryOK {
					return m, m.handleError(fmt.Errorf("task not found: %s", planFile))
				}
				capturedEntry := entry
				capturedPlanFile := planFile
				return m, func() tea.Msg {
					return waveRetryMsg{planFile: capturedPlanFile, entry: capturedEntry}
				}
			case "abort":
				capturedPlanFile := planFile
				return m, func() tea.Msg {
					return waveAbortMsg{planFile: capturedPlanFile}
				}
			}
		}
		return m, nil
	}

	// Handle permission prompt state
	if m.state == statePermission {
		if !m.overlays.IsActive() {
			m.state = stateDefault
			return m, nil
		}
		result := m.overlays.HandleKey(msg)
		if result.Dismissed {
			return m.finishPermissionOverlay(result)
		}
		return m, nil
	}

	// Handle new plan description state
	if m.state == stateNewPlan {
		if !m.overlays.IsActive() {
			m.state = stateDefault
			return m, nil
		}
		result := m.overlays.HandleKey(msg)
		if result.Dismissed {
			if result.Submitted {
				description := strings.TrimSpace(result.Value)
				if description == "" {
					m.state = stateDefault
					m.menu.SetState(ui.StateDefault)
					return m, m.handleError(fmt.Errorf("description cannot be empty"))
				}
				// Set heuristic title as fallback; AI title will replace it when it arrives
				m.pendingPlanName = heuristicPlanTitle(description)
				m.pendingPlanDesc = description

				// If the first line is already a viable slug, skip AI derivation
				if firstLineIsViableSlug(description) {
					topicNames := m.getTopicNames()
					topicNames = append([]string{"(No topic)"}, topicNames...)
					pickerTitle := fmt.Sprintf("assign to topic for '%s'", m.pendingPlanName)
					po := overlay.NewPickerOverlay(pickerTitle, topicNames)
					po.SetAllowCustom(true)
					m.overlays.Show(po)
					m.state = stateNewPlanTopic
					return m, nil
				}

				m.state = stateNewPlanDeriving
				if m.toastManager != nil {
					m.toastManager.Info("deriving title...")
					return m, tea.Batch(aiDerivePlanTitleCmd(description), m.toastTickCmd())
				}
				return m, aiDerivePlanTitleCmd(description)
			}
			m.state = stateDefault
			m.menu.SetState(ui.StateDefault)
			return m, tea.RequestWindowSize
		}
		return m, nil
	}

	// Handle deriving state — waiting for AI title before showing topic picker
	if m.state == stateNewPlanDeriving {
		if msg.Code == tea.KeyEscape {
			m.state = stateDefault
			m.pendingPlanName = ""
			m.pendingPlanDesc = ""
			return m, nil
		}
		return m, nil
	}

	// Handle new plan topic picker state
	if m.state == stateNewPlanTopic {
		if !m.overlays.IsActive() {
			m.state = stateDefault
			m.pendingPlanName = ""
			m.pendingPlanDesc = ""
			return m, nil
		}
		result := m.overlays.HandleKey(msg)
		if result.Dismissed {
			topic := ""
			if result.Submitted {
				picked := result.Value
				if picked != "(No topic)" {
					topic = picked
				}
			}
			if err := m.createTaskEntry(m.pendingPlanName, m.pendingPlanDesc, topic); err != nil {
				m.state = stateDefault
				m.menu.SetState(ui.StateDefault)
				m.pendingPlanName = ""
				m.pendingPlanDesc = ""
				return m, m.handleError(err)
			}
			m.loadTaskState()
			m.updateSidebarTasks()
			m.state = stateDefault
			m.menu.SetState(ui.StateDefault)
			m.pendingPlanName = ""
			m.pendingPlanDesc = ""
			return m, tea.RequestWindowSize
		}
		return m, nil
	}

	// Handle harness picker state (first step of spawn flow when multiple programs available)
	if m.state == stateSpawnHarnessPicker {
		if !m.overlays.IsActive() {
			m.pendingSpawnProgram = ""
			m.state = stateDefault
			m.menu.SetState(ui.StateDefault)
			return m, nil
		}
		result := m.overlays.HandleKey(msg)
		if result.Dismissed {
			if result.Submitted && result.Value != "" {
				m.showSpawnAgentForm(result.Value)
				return m, nil
			}
			m.pendingSpawnProgram = ""
			m.state = stateDefault
			m.menu.SetState(ui.StateDefault)
			return m, tea.RequestWindowSize
		}
		return m, nil
	}

	// Handle spawn agent form state
	if m.state == stateSpawnAgent {
		if !m.overlays.IsActive() {
			m.pendingSpawnProgram = ""
			m.state = stateDefault
			return m, nil
		}
		// Type-assert before HandleKey to access Name/Branch/WorkPath after dismiss.
		fo, _ := m.overlays.Current().(*overlay.FormOverlay)
		result := m.overlays.HandleKey(msg)
		if result.Dismissed {
			if result.Submitted && fo != nil {
				name := fo.Name()
				branch := fo.Branch()
				workPath := fo.WorkPath()

				if name == "" {
					m.pendingSpawnProgram = ""
					m.state = stateDefault
					m.menu.SetState(ui.StateDefault)
					return m, m.handleError(fmt.Errorf("name cannot be empty"))
				}

				selectedProgram := m.pendingSpawnProgram
				m.pendingSpawnProgram = ""
				return m.spawnAdHocAgent(name, branch, workPath, selectedProgram)
			}
			m.pendingSpawnProgram = ""
			m.state = stateDefault
			m.menu.SetState(ui.StateDefault)
			return m, tea.RequestWindowSize
		}
		return m, nil
	}

	// Handle change-topic picker for existing plans
	if m.state == stateChangeTopic {
		if !m.overlays.IsActive() {
			m.state = stateDefault
			m.pendingChangeTopicTask = ""
			return m, nil
		}
		result := m.overlays.HandleKey(msg)
		if result.Dismissed {
			if result.Submitted && m.taskState != nil && m.pendingChangeTopicTask != "" {
				picked := result.Value
				newTopic := ""
				if picked != "(No topic)" {
					newTopic = picked
				}
				if err := m.taskState.SetTopic(m.pendingChangeTopicTask, newTopic); err != nil {
					m.state = stateDefault
					m.pendingChangeTopicTask = ""
					return m, m.handleError(err)
				}
				m.updateSidebarTasks()
			}
			m.state = stateDefault
			m.pendingChangeTopicTask = ""
			return m, tea.RequestWindowSize
		}
		return m, nil
	}

	// Handle set-status picker for force-overriding a plan's status
	if m.state == stateSetStatus {
		if !m.overlays.IsActive() {
			m.state = stateDefault
			m.pendingSetStatusTask = ""
			return m, nil
		}
		result := m.overlays.HandleKey(msg)
		if result.Dismissed {
			if result.Submitted && m.taskState != nil && m.pendingSetStatusTask != "" {
				picked := result.Value
				if picked != "" {
					return m.applyPendingSetStatus(picked)
				}
			}
			m.state = stateDefault
			m.pendingSetStatusTask = ""
			return m, tea.RequestWindowSize
		}
		return m, nil
	}

	// Handle ClickUp search input state
	if m.state == stateClickUpSearch {
		if !m.overlays.IsActive() {
			m.state = stateDefault
			return m, nil
		}
		result := m.overlays.HandleKey(msg)
		if result.Dismissed {
			if result.Submitted {
				query := strings.TrimSpace(result.Value)
				if query != "" {
					m.state = stateClickUpFetching
					m.toastManager.Info("searching clickup...")
					return m, tea.Batch(m.searchClickUp(query), m.toastTickCmd())
				}
			}
			m.state = stateDefault
		}
		return m, nil
	}

	// Handle ClickUp task picker state
	if m.state == stateClickUpPicker {
		if !m.overlays.IsActive() {
			m.state = stateDefault
			return m, nil
		}
		result := m.overlays.HandleKey(msg)
		if result.Dismissed {
			if result.Submitted {
				selected := result.Value
				if selected != "" {
					for _, r := range m.clickUpResults {
						label := r.ID + " · " + r.Name
						if strings.HasPrefix(selected, label) {
							m.state = stateClickUpFetching
							m.toastManager.Info("fetching task details...")
							return m, tea.Batch(m.fetchClickUpTaskWithTimeout(r.ID), m.toastTickCmd())
						}
					}
				}
			}
			m.state = stateDefault
		}
		return m, nil
	}

	if m.state == stateClickUpFetching {
		return m, nil
	}

	// Handle ClickUp workspace picker state
	if m.state == stateClickUpWorkspacePicker {
		if !m.overlays.IsActive() {
			m.state = stateDefault
			return m, nil
		}
		result := m.overlays.HandleKey(msg)
		if result.Dismissed {
			if result.Submitted {
				selected := result.Value
				if selected != "" && m.clickUpImporter != nil {
					// Resolve label ("name (id)") back to bare workspace ID.
					wsID := selected
					if id, ok := m.clickUpWorkspaceMap[selected]; ok {
						wsID = id
					}
					m.clickUpImporter.SetWorkspaceID(wsID)
					// Persist choice so user isn't prompted again for this project.
					if err := clickup.SaveProjectConfig(m.activeRepoPath, &clickup.ProjectConfig{
						WorkspaceID: wsID,
					}); err != nil {
						log.WarningLog.Printf("failed to save clickup workspace config: %v", err)
					}
					query := m.clickUpPendingQuery
					m.clickUpPendingQuery = ""
					m.clickUpWorkspaceMap = nil
					m.state = stateClickUpFetching
					m.toastManager.Info("searching clickup...")
					return m, tea.Batch(m.searchClickUp(query), m.toastTickCmd())
				}
			}
			m.state = stateDefault
			m.clickUpPendingQuery = ""
			m.clickUpWorkspaceMap = nil
		}
		return m, nil
	}

	if m.state == stateTmuxBrowser {
		if !m.overlays.IsActive() {
			m.state = stateDefault
			return m, nil
		}
		browser, _ := m.overlays.Current().(*overlay.TmuxBrowserOverlay)
		result := m.overlays.HandleKey(msg)
		if result.Dismissed {
			m.state = stateDefault
			return m.handleTmuxBrowserAction(browser, result.Action)
		}
		// Handle non-dismissed actions (e.g. "kill" keeps the browser open for
		// multi-kill workflow — the action handler decides whether to dismiss).
		if result.Action != "" {
			return m.handleTmuxBrowserAction(browser, result.Action)
		}
		return m, nil
	}

	// Handle command launcher state
	if m.state == stateLauncher {
		if !m.overlays.IsActive() {
			m.state = stateDefault
			return m, nil
		}
		result := m.overlays.HandleKey(msg)
		if result.Dismissed {
			m.state = stateDefault
			if result.Action != "" {
				return m.executeLauncherAction(result.Action)
			}
			return m, tea.RequestWindowSize
		}
		return m, nil
	}

	// Handle keybind browser state
	if m.state == stateKeybindBrowser {
		if !m.overlays.IsActive() {
			m.state = stateDefault
			return m, nil
		}
		result := m.overlays.HandleKey(msg)
		if result.Dismissed {
			m.state = stateDefault
			return m, tea.RequestWindowSize
		}
		return m, nil
	}

	// Handle search state — allows typing to filter AND arrow keys to navigate
	if m.state == stateSearch {
		switch {
		case msg.String() == "esc":
			m.nav.DeactivateSearch()
			m.state = stateDefault
			return m, nil
		case msg.String() == "enter":
			m.nav.DeactivateSearch()
			m.state = stateDefault
			return m, nil
		case msg.String() == "up":
			m.nav.Up()
			return m, m.instanceChanged()
		case msg.String() == "down":
			m.nav.Down()
			return m, m.instanceChanged()
		case msg.Code == tea.KeyPgUp:
			m.nav.PageUp()
			return m, m.instanceChanged()
		case msg.Code == tea.KeyPgDown:
			m.nav.PageDown()
			return m, m.instanceChanged()
		case msg.Code == tea.KeyBackspace:
			q := m.nav.GetSearchQuery()
			if len(q) > 0 {
				runes := []rune(q)
				m.nav.SetSearchQuery(string(runes[:len(runes)-1]))
			}
			return m, nil
		case msg.Code == tea.KeySpace:
			m.nav.SetSearchQuery(m.nav.GetSearchQuery() + " ")
			return m, nil
		case len(msg.Text) > 0:
			m.nav.SetSearchQuery(m.nav.GetSearchQuery() + msg.Text)
			return m, nil
		}
		return m, nil
	}

	// Exit scrolling mode when ESC is pressed and preview pane is in scrolling mode.
	// Always check for escape key first to ensure it doesn't get intercepted elsewhere.
	if msg.Code == tea.KeyEscape {
		// Exit document mode (plan viewer) on Esc
		if m.tabbedWindow.IsDocumentMode() {
			m.tabbedWindow.ClearDocumentMode()
			return m, m.instanceChanged()
		}
		// If in scroll mode, exit scroll mode
		if m.tabbedWindow.IsPreviewInScrollMode() {
			// Use the selected instance from the list
			selected := m.nav.GetSelectedInstance()
			err := m.tabbedWindow.ResetPreviewToNormalMode(selected)
			if err != nil {
				return m, m.handleError(err)
			}
			return m, m.instanceChanged()
		}
	}

	// Forward key events to the viewport when in document or scroll mode.
	// This enables viewport native keys like PgUp/PgDn and arrow keys.
	if m.tabbedWindow.IsDocumentMode() && msg.Code == tea.KeyLeft {
		m.tabbedWindow.ClearDocumentMode()
		return m, m.instanceChanged()
	}

	if m.tabbedWindow.IsDocumentMode() || m.tabbedWindow.IsPreviewInScrollMode() {
		cmd := m.tabbedWindow.ViewportUpdate(msg)

		// Keep existing shift+up/down behavior as fallback handlers.
		if msg.String() != "shift+up" && msg.String() != "shift+down" {
			if m.tabbedWindow.ViewportHandlesKey(msg) {
				return m, cmd
			}
		}

		if cmd != nil {
			return m, cmd
		}
	}

	// Ctrl+Up/Down: cycle through active instances (wrapping)
	if msg.Code == tea.KeyUp && msg.Mod.Contains(tea.ModCtrl) || msg.Code == tea.KeyDown && msg.Mod.Contains(tea.ModCtrl) {
		if msg.Code == tea.KeyUp && msg.Mod.Contains(tea.ModCtrl) {
			m.nav.CyclePrevActive()
		} else {
			m.nav.CycleNextActive()
		}
		return m, m.instanceChanged()
	}

	if msg.Code == tea.KeyPgUp || msg.Code == tea.KeyPgDown {
		if msg.Code == tea.KeyPgUp {
			m.nav.PageUp()
		} else {
			m.nav.PageDown()
		}
		if m.focusSlot != slotNav {
			m.setFocusSlot(slotNav)
		}
		m.tabbedWindow.ClearDocumentMode()
		return m, m.instanceChanged()
	}

	// Handle quit commands first
	if msg.String() == "ctrl+c" || msg.String() == "q" {
		return m.handleQuit()
	}

	// Delete key: dismiss a finished (non-running) instance from the list.
	if msg.Code == tea.KeyDelete || msg.Code == tea.KeyBackspace {
		selected := m.nav.GetSelectedInstance()
		if selected != nil && (selected.Exited || (selected.Status != session.Running && selected.Status != session.Loading)) {
			return m, m.dismissInstanceFromList(selected)
		}
		return m, nil
	}

	if isCommandLauncherKey(msg) {
		return m.openCommandLauncher()
	}

	// Number shortcuts: pass 1/2/3 through to the agent's PTY when the
	// embedded VT display is active. One-shot send, no mode change.
	if m.previewTerminal != nil && (msg.String() == "1" || msg.String() == "2" || msg.String() == "3") {
		selected := m.nav.GetSelectedInstance()
		if selected != nil && selected.Started() && !selected.Paused() {
			if err := m.previewTerminal.SendKey([]byte(msg.Text)); err != nil {
				return m, m.handleError(err)
			}
			return m, nil
		}
	}

	// Ctrl+O: pass through to the agent's PTY (e.g. claude code "open file").
	if m.previewTerminal != nil && msg.Code == 'o' && msg.Mod.Contains(tea.ModCtrl) {
		selected := m.nav.GetSelectedInstance()
		if selected != nil && selected.Started() && !selected.Paused() {
			if err := m.previewTerminal.SendKey([]byte{0x0F}); err != nil {
				return m, m.handleError(err)
			}
			return m, nil
		}
	}

	// Double-tap detection (default-state path only; all non-default states have
	// already returned early above via their own handlers at lines 617–1556).
	// canonicalDoubleTapKey returns "" for ctrl/alt chords — those flow through
	// the GlobalKeyStringsMap lookup below unchanged.
	dtKey := canonicalDoubleTapKey(msg)

	// Any key press that is NOT the pending debounced key must flush the pending
	// single-press action immediately. This covers non-printable keys (arrows,
	// tab, enter — where dtKey is ""), conflict-free double-tap keys, and
	// unrelated printable keys.
	if m.pendingDoubleTapKey != "" && dtKey != m.pendingDoubleTapKey {
		prevAction := m.pendingDoubleTapAction
		m.pendingDoubleTapKey = ""
		m.pendingDoubleTapAction = 0
		m.pendingDoubleTapSeq++ // invalidate in-flight timeout
		_, flushCmd := m.handleResolvedKey(prevAction)
		// For non-printable keys, just flush and let the key fall through.
		if dtKey == "" {
			name, ok := keys.GlobalKeyStringsMap[msg.String()]
			if !ok {
				return m, flushCmd
			}
			_, newCmd := m.handleResolvedKey(name)
			return m, tea.Batch(flushCmd, newCmd)
		}
		// For printable keys, flush and continue into the double-tap logic below
		// so the new key is processed in the same Update call. Any cmd the
		// remainder of the function returns must be batched with flushCmd —
		// otherwise async cmds like quickLaunchAgent's startCmd get dropped
		// and the loading instance they enqueue never starts.
		if flushCmd != nil {
			defer func() {
				if cmd == nil {
					cmd = flushCmd
					return
				}
				cmd = tea.Batch(flushCmd, cmd)
			}()
		}
	}

	if dtKey == "" {
		// Non-printable keys (arrows, enter, tab, function keys, and any
		// ctrl/alt chord) must clear any in-flight conflict-free triple-tap
		// window so sequences like k,k,up,k cannot escalate to
		// KeyKillAndRemove. canonicalDoubleTapKey returns "" for anything
		// that is not a printable single-rune key or literal space, so this
		// covers every non-participant that falls through to
		// GlobalKeyStringsMap below. Mirrors the resets inside the
		// dtKey != "" block for unrelated printable keys and debounced
		// interruptors.
		m.ensureDoubleTap().Reset()
	}

	if dtKey != "" {
		// Conflict-free keys (k, K, u, d): no single-press binding — swallow 1st tap,
		// dispatch mapped action on the 2nd tap within the threshold, and escalate to
		// the triple-tap action on the 3rd tap when a TripleTapMap entry exists.
		tracker := m.ensureDoubleTap()
		if action, ok := keys.TripleTapMap[dtKey]; ok && tracker.DetectTriple(dtKey) {
			return m.handleResolvedKey(action)
		}
		if action, ok := keys.DoubleTapMap[dtKey]; ok {
			if tracker.Detect(dtKey) {
				return m.handleResolvedKey(action)
			}
			// First tap: swallow (no single-press action for these keys).
			return m, nil
		}

		// Debounced keys (s, space): first tap defers via timeout; second tap within
		// the window cancels the timeout and fires the double-tap action.
		if action, ok := keys.DebouncedDoubleTapMap[dtKey]; ok {
			// A debounced interruptor must clear any in-flight conflict-free
			// triple-tap window so k+k+s+k and k+k+space+k cannot escalate to
			// KeyKillAndRemove. This mirrors the reset below for unrelated
			// printable keys.
			tracker.Reset()
			if m.pendingDoubleTapKey == dtKey {
				// Second tap — cancel pending and dispatch double-tap action immediately.
				m.pendingDoubleTapKey = ""
				m.pendingDoubleTapAction = 0
				m.pendingDoubleTapSeq++
				return m.handleResolvedKey(action)
			}
			// First tap: defer single-press until the debounce timeout fires.
			singleAction := keys.GlobalKeyStringsMap[dtKey]
			m.pendingDoubleTapKey = dtKey
			m.pendingDoubleTapAction = singleAction
			m.pendingDoubleTapSeq++
			seq := m.pendingDoubleTapSeq
			return m, scheduleDoubleTapTimeout(m.doubleTapThreshold(), dtKey, seq)
		}

		// Unrelated printable key: reset conflict-free tracker so stale state does
		// not accumulate (e.g. k → a → k should not count as k+k).
		m.ensureDoubleTap().Reset()
	}

	name, ok := keys.GlobalKeyStringsMap[msg.String()]
	if !ok {
		return m, nil
	}
	return m.handleResolvedKey(name)
}

// handleResolvedKey dispatches a resolved KeyName action. It is called both
// from the direct key path (handleKeyPress) and from the debounce-timeout path
// (handleDoubleTapTimeout) so that ctrl-chord actions and double-tap actions
// remain behaviourally identical.
func (m *home) handleResolvedKey(name keys.KeyName) (tea.Model, tea.Cmd) {
	switch name {
	case keys.KeyHalfPageUp:
		m.tabbedWindow.HalfPageUp()
		return m, nil
	case keys.KeyHalfPageDown:
		m.tabbedWindow.HalfPageDown()
		return m, nil
	case keys.KeyHelp:
		return m.openKeybindBrowser()
	case keys.KeyPrompt:
		if m.tmuxSessionCount >= GlobalInstanceLimit {
			return m, m.handleError(
				fmt.Errorf("you can't create more than %d instances (%d tmux sessions active)", GlobalInstanceLimit, m.tmuxSessionCount))
		}
		instance, err := session.NewInstance(session.InstanceOptions{
			Title:   "",
			Path:    m.activeRepoPath,
			Program: m.programForAgent(""),
		})
		if err != nil {
			return m, m.handleError(err)
		}

		m.addInstanceFinalizer(instance, m.nav.AddInstance(instance))
		m.newInstance = instance
		m.nav.SetSelectedInstance(m.nav.NumInstances() - 1)
		m.state = stateNew
		m.menu.SetState(ui.StateNewInstance)
		m.promptAfterName = true

		return m, nil
	case keys.KeyUp:
		m.tabbedWindow.ClearDocumentMode()
		if m.focusSlot != slotNav {
			m.setFocusSlot(slotNav)
		}
		m.nav.Up()
		return m, m.instanceChanged()
	case keys.KeyDown:
		m.tabbedWindow.ClearDocumentMode()
		if m.focusSlot != slotNav {
			m.setFocusSlot(slotNav)
		}
		m.nav.Down()
		return m, m.instanceChanged()
	case keys.KeyTab:
		return m, m.nextFocusSlot()
	case keys.KeySpace:
		if m.focusSlot == slotNav && m.nav.GetSelectedID() == ui.SidebarImportClickUp {
			m.state = stateClickUpSearch
			tio := overlay.NewTextInputOverlay("enter clickup id or url", "")
			tio.SetSize(50, 1)
			m.overlays.Show(tio)
			return m, nil
		}
		if m.focusSlot == slotNav {
			if m.nav.GetSelectedInstance() != nil {
				return m.openContextMenu()
			}
			if m.nav.ToggleSelectedExpand() {
				return m, nil
			}
		}
		return m, nil
	case keys.KeyInfoTab:
		// Toggle the compact info header without stealing sidebar focus or changing the instance tab.
		m.tabbedWindow.SetShowInfo(!m.tabbedWindow.IsShowingInfo())
		return m, nil
	case keys.KeyTabInfo:
		return m.switchToTab(name)
	case keys.KeyTabAgent:
		return m.exclamationAutoFocus()
	case keys.KeySendPrompt, keys.KeyExitFocus:
		// Ensure the preview terminal is ready when entering focus mode.
		m.previewRequested = true
		selected := m.nav.GetSelectedInstance()
		// When a plan header is selected (no instance), find the best instance for that plan.
		if selected == nil {
			if pf := m.nav.GetSelectedPlanFile(); pf != "" {
				if best := m.nav.FindPlanInstance(pf); best != nil {
					m.nav.SelectInstance(best)
					selected = best
				}
			}
		}
		if selected == nil || !selected.Started() || selected.Paused() {
			return m, nil
		}
		return m, m.enterFocusMode()
	case keys.KeySendYes:
		selected := m.nav.GetSelectedInstance()
		if selected == nil || !selected.Started() || selected.Paused() || !selected.PromptDetected {
			return m, nil
		}
		selected.QueuedPrompt = "yes"
		selected.AwaitingWork = true
		return m, nil
	case keys.KeyKill:
		// Soft kill: terminate tmux session only, keep instance in list.
		selected := m.nav.GetSelectedInstance()
		if selected == nil || !selected.Started() || selected.Paused() || selected.Exited {
			return m, nil
		}
		m.audit(auditlog.EventAgentKilled, "killed instance",
			auditlog.WithInstance(selected.Title),
			auditlog.WithAgent(selected.AgentType),
			auditlog.WithPlan(selected.TaskFile),
		)
		return m, softKillInstanceCmd(selected)
	case keys.KeyKillAndRemove:
		// Soft kill and remove: terminate tmux session and dismiss instance from list.
		selected := m.nav.GetSelectedInstance()
		if selected == nil || !selected.Started() || selected.Paused() || selected.Exited {
			return m, nil
		}
		m.audit(auditlog.EventAgentKilled, "killed and removed instance",
			auditlog.WithInstance(selected.Title),
			auditlog.WithAgent(selected.AgentType),
			auditlog.WithPlan(selected.TaskFile),
		)
		return m, tea.Batch(softKillInstanceCmd(selected), m.dismissInstanceFromList(selected))
	case keys.KeyAbort:
		// Full abort: kill tmux, remove worktree, remove from list + persistence.
		selected := m.nav.GetSelectedInstance()
		if selected == nil {
			return m, nil
		}

		// Pre-kill checks run async; model mutations happen in Update via killInstanceMsg.
		title := selected.Title
		killAction := func() tea.Msg {
			worktree, err := selected.GetGitWorktree()
			if err != nil {
				return err
			}
			checkedOut, err := worktree.IsBranchCheckedOut()
			if err != nil {
				return err
			}
			if checkedOut {
				return fmt.Errorf("instance %s is currently checked out", selected.Title)
			}
			return killInstanceMsg{title: title}
		}

		// Show confirmation modal
		message := fmt.Sprintf("stop session '%s'? branch will be preserved.", selected.Title)
		return m, m.confirmAction(message, killAction)
	case keys.KeySubmit:
		selected := m.nav.GetSelectedInstance()
		if selected == nil {
			return m, nil
		}

		// Create the push action as a tea.Cmd
		pushAction := func() tea.Msg {
			// Default commit message with timestamp
			commitMsg := fmt.Sprintf("[kas] update from '%s' on %s", selected.Title, time.Now().Format(time.RFC822))
			worktree, err := selected.GetGitWorktree()
			if err != nil {
				return err
			}
			if err = worktree.PushChanges(commitMsg, true); err != nil {
				return err
			}
			return nil
		}

		// Show confirmation modal
		message := fmt.Sprintf("[!] push changes from session '%s'?", selected.Title)
		return m, m.confirmAction(message, pushAction)
	case keys.KeyCreatePR:
		selected := m.nav.GetSelectedInstance()
		if selected == nil {
			return m, nil
		}
		m.state = statePRTitle
		tio := overlay.NewTextInputOverlay("pr title", selected.Title)
		tio.SetSize(60, 3)
		m.overlays.Show(tio)
		return m, nil
	case keys.KeyCheckout:
		selected := m.nav.GetSelectedInstance()
		if selected == nil {
			return m, nil
		}

		// Show help screen before pausing
		m.showHelpScreen(helpTypeInstanceCheckout{}, func() {
			if err := selected.Pause(); err != nil {
				m.handleError(err)
			}
			m.instanceChanged()
		})
		return m, nil
	case keys.KeyResume:
		selected := m.nav.GetSelectedInstance()
		if selected == nil || !selected.Paused() {
			return m, nil
		}
		if err := selected.Resume(); err != nil {
			return m, m.handleError(err)
		}
		return m, tea.RequestWindowSize
	case keys.KeyEnter:
		// Sidebar always has focus: handle plan/instance interactions first.
		if m.nav.GetSelectedID() == ui.SidebarImportClickUp {
			m.state = stateClickUpSearch
			tio := overlay.NewTextInputOverlay("enter clickup id or url", "")
			tio.SetSize(50, 1)
			m.overlays.Show(tio)
			return m, nil
		}
		selected := m.nav.GetSelectedInstance()
		if selected != nil {
			if !selected.Started() || selected.Paused() {
				return m, nil
			}
			if !selected.TmuxAlive() {
				m.toastManager.Error(fmt.Sprintf("session for '%s' is not running", selected.Title))
				return m, m.toastTickCmd()
			}
			if session.NormalizeExecutionMode(selected.ExecutionMode) == session.ExecutionModeSDK {
				m.toastManager.Info(fmt.Sprintf("%s is running in sdk mode; attach is disabled", selected.Title))
				return m, nil
			}
			// Queue the selected instance, then show the attach help overlay.
			// Actual attach (via tea.Exec) happens in handleHelpState once the user
			// dismisses the help screen - this keeps bubbletea's event loop free.
			m.pendingAttachInstance = selected
			m.showHelpScreen(helpTypeInstanceAttach{}, nil)
			// If the overlay was skipped (already seen), showHelpScreen returns without
			// setting m.state = stateHelp. In that case consume pendingAttachInstance
			// immediately so the attach is not silently abandoned.
			if m.state != stateHelp && m.pendingAttachInstance != nil {
				pending := m.pendingAttachInstance
				m.pendingAttachInstance = nil
				if session.NormalizeExecutionMode(pending.ExecutionMode) == session.ExecutionModeSDK {
					m.toastManager.Info(fmt.Sprintf("%s is running in sdk mode; attach is disabled", pending.Title))
					return m, nil
				}
				return m, tea.Exec(tmux.NewAttachExecCommand(pending), func(err error) tea.Msg {
					if err != nil {
						return err
					}
					return tmuxAttachReturnMsg{}
				})
			}
			return m, nil
		}
		// Plan header or plan file: open plan context menu
		if m.nav.IsSelectedPlanHeader() {
			return m.openTaskContextMenu()
		}
		if planFile := m.nav.GetSelectedPlanFile(); planFile != "" {
			return m.openTaskContextMenu()
		}
		if m.nav.NumInstances() == 0 {
			return m, nil
		}
		return m, nil
	case keys.KeyViewPlan:
		return m.viewSelectedPlan()
	case keys.KeyBrowser:
		return m.openPlanBrowserForSelection()
	case keys.KeyToggleSidebar:
		if m.sidebarHidden {
			// Show sidebar, keep current focus
			m.sidebarHidden = false
		} else {
			// Hide sidebar
			m.sidebarHidden = true
			// If sidebar was focused, move focus to agent tab
			if m.focusSlot == slotNav {
				m.setFocusSlot(slotAgent)
			}
		}
		return m, tea.RequestWindowSize
	case keys.KeyAuditToggle:
		if m.auditPane != nil {
			m.auditPane.ToggleVisible()
		}
		return m, tea.RequestWindowSize
	case keys.KeyAuditCursor:
		return m.enterAuditCursorMode()
	case keys.KeyArrowLeft:
		// With multiple instance tabs, navigate to the previous tab.
		if m.tabbedWindow.TabCount() > 1 {
			m.tabbedWindow.PrevTab()
			return m, m.tabSwitched()
		}
		// Otherwise no-op (sidebar already focused).
		return m, nil
	case keys.KeyArrowRight:
		// With multiple instance tabs, navigate to the next tab.
		if m.tabbedWindow.TabCount() > 1 {
			m.tabbedWindow.NextTab()
			return m, m.tabSwitched()
		}
		// Otherwise: preserve existing expand/menu/ClickUp behavior.
		if m.nav.GetSelectedID() == ui.SidebarImportClickUp {
			m.state = stateClickUpSearch
			tio := overlay.NewTextInputOverlay("enter clickup id or url", "")
			tio.SetSize(50, 1)
			m.overlays.Show(tio)
			return m, nil
		}
		// Right on an instance: open the instance context menu (same as space).
		if m.nav.GetSelectedInstance() != nil {
			return m.openContextMenu()
		}
		// Right on a plan: view the plan (same as [p]).
		if m.nav.GetSelectedPlanFile() != "" {
			return m.viewSelectedPlan()
		}
		// Right on topic/other headers: expand/collapse.
		m.nav.ToggleSelectedExpand()
		return m, nil
	case keys.KeyNewPlan:
		m.state = stateNewPlan
		tio := overlay.NewTextInputOverlay("new plan", "")
		tio.SetMultiline(true)
		tio.SetPlaceholder("describe what you want to work on...")
		tio.SetSize(70, 8)
		m.overlays.Show(tio)
		return m, nil
	case keys.KeySpawnAgent:
		return m.beginSpawnAgentFlow()
	case keys.KeyQuickLaunch:
		return m.quickLaunchAgent()
	case keys.KeyTmuxBrowser:
		return m, m.discoverTmuxSessions()
	case keys.KeySearch:
		m.nav.ActivateSearch()
		m.nav.SelectFirst()
		m.state = stateSearch
		m.setFocusSlot(slotNav)
		return m, nil
	default:
		return m, nil
	}
}

// canonicalDoubleTapKey returns the canonical double-tap token for msg, or ""
// if the key should not participate in double-tap detection.
//
// Rules:
//   - ctrl or alt modifier → "" (leave on existing paths)
//   - empty Text → "" (special keys: arrows, function keys, etc.)
//   - Text == " " → "space"  (normalise to match DebouncedDoubleTapMap["space"])
//   - any other single printable rune → that character  (includes shift-only like "K")
//   - anything else → ""
func canonicalDoubleTapKey(msg tea.KeyPressMsg) string {
	if msg.Mod.Contains(tea.ModCtrl) || msg.Mod.Contains(tea.ModAlt) {
		return ""
	}
	if len(msg.Text) == 0 {
		return ""
	}
	if msg.Text == " " {
		return "space"
	}
	runes := []rune(msg.Text)
	if len(runes) != 1 || !unicode.IsPrint(runes[0]) {
		return ""
	}
	return msg.Text
}

// doubleTapThreshold returns the configured double-tap timing window.
// Returns 300 ms when appConfig is nil (lightweight test environments).
func (m *home) doubleTapThreshold() time.Duration {
	if m.appConfig != nil {
		return m.appConfig.DoubleTapThreshold()
	}
	return 300 * time.Millisecond
}

// ensureDoubleTap lazily initialises m.doubleTap from the current config threshold.
// Lazy init lets tests use bare &home{} or newTestHome() without a
// DoubleTapTracker constructor call.
func (m *home) ensureDoubleTap() *keys.DoubleTapTracker {
	if m.doubleTap == nil {
		m.doubleTap = keys.NewDoubleTapTracker(m.doubleTapThreshold())
	}
	return m.doubleTap
}

// handleDoubleTapTimeout is called from Update when a doubleTapTimeoutMsg arrives.
// If the timeout is still fresh (matching key and seq, state is stateDefault, pending
// is set), the queued single-press action is dispatched via handleResolvedKey.
func (m *home) handleDoubleTapTimeout(msg doubleTapTimeoutMsg) (tea.Model, tea.Cmd) {
	if m.pendingDoubleTapKey == "" ||
		msg.key != m.pendingDoubleTapKey ||
		msg.seq != m.pendingDoubleTapSeq {
		return m, nil
	}
	// Non-default state (overlay, focus, etc.): discard the pending action
	// rather than letting it linger and fire on the next matching key press.
	if m.state != stateDefault {
		m.pendingDoubleTapKey = ""
		m.pendingDoubleTapAction = 0
		return m, nil
	}
	action := m.pendingDoubleTapAction
	m.pendingDoubleTapKey = ""
	m.pendingDoubleTapAction = 0
	return m.handleResolvedKey(action)
}

// keyToBytes translates a Bubble Tea key message to raw bytes for PTY forwarding.
func keyToBytes(msg tea.KeyPressMsg) []byte {
	if msg.Code == tea.KeyEnter && msg.Mod != 0 {
		return kittyCSIu(13, msg.Mod)
	}

	// Handle modifier combinations first.
	if msg.Mod.Contains(tea.ModCtrl) {
		ctrlCode := msg.Code
		if key := msg.Key(); key.BaseCode != 0 {
			ctrlCode = key.BaseCode
		}
		ctrlCode = unicode.ToLower(ctrlCode)
		// Ctrl+letter → raw control character byte (0x01..0x1A).
		if ctrlCode >= 'a' && ctrlCode <= 'z' {
			return []byte{byte(ctrlCode - 'a' + 1)}
		}
	}
	if msg.Mod.Contains(tea.ModShift) {
		switch msg.Code {
		case tea.KeyTab:
			return []byte("\x1b[Z")
		case tea.KeyUp:
			return []byte("\x1b[1;2A")
		case tea.KeyDown:
			return []byte("\x1b[1;2B")
		case tea.KeyRight:
			return []byte("\x1b[1;2C")
		case tea.KeyLeft:
			return []byte("\x1b[1;2D")
		case tea.KeyHome:
			return []byte("\x1b[1;2H")
		case tea.KeyEnd:
			return []byte("\x1b[1;2F")
		}
	}
	if msg.Mod.Contains(tea.ModAlt) {
		if len(msg.Text) > 0 {
			return append([]byte{0x1b}, []byte(msg.Text)...)
		}
		if msg.Code != 0 && unicode.IsPrint(msg.Code) {
			return append([]byte{0x1b}, []byte(string(msg.Code))...)
		}
	}

	// Printable text with no modifiers.
	if len(msg.Text) > 0 {
		return []byte(msg.Text)
	}

	// Special keys (no modifiers).
	switch msg.Code {
	case tea.KeyEnter:
		return []byte{0x0D}
	case tea.KeyBackspace:
		return []byte{0x7F}
	case tea.KeyTab:
		return []byte{0x09}
	case tea.KeySpace:
		return []byte{0x20}
	case tea.KeyUp:
		return []byte("\x1b[A")
	case tea.KeyDown:
		return []byte("\x1b[B")
	case tea.KeyRight:
		return []byte("\x1b[C")
	case tea.KeyLeft:
		return []byte("\x1b[D")
	case tea.KeyDelete:
		return []byte("\x1b[3~")
	case tea.KeyEscape:
		return []byte{0x1b}
	default:
		return nil
	}
}

func kittyCSIu(code rune, mod tea.KeyMod) []byte {
	modifier := 1
	if mod.Contains(tea.ModShift) {
		modifier++
	}
	if mod.Contains(tea.ModAlt) {
		modifier += 2
	}
	if mod.Contains(tea.ModCtrl) {
		modifier += 4
	}
	return []byte(fmt.Sprintf("\x1b[%d;%du", code, modifier))
}

func (m *home) handleError(err error) tea.Cmd {
	log.ErrorLog.Printf("%v", err)
	m.toastManager.Error(err.Error())
	m.audit(auditlog.EventError, err.Error(), auditlog.WithLevel("error"))
	return m.toastTickCmd()
}

// confirmAction shows a confirmation modal and stores the action to execute on confirm.
// The action is a tea.Cmd that will be returned from Update() to run asynchronously —
// never called synchronously, which would block the UI during I/O operations.
func (m *home) confirmAction(message string, action tea.Cmd) tea.Cmd {
	m.state = stateConfirm
	m.pendingConfirmAction = action

	co := overlay.NewConfirmationOverlay(message)
	m.overlays.Show(co)

	return nil
}

// keydownCallback clears the menu option highlighting after 500ms.
func (m *home) keydownCallback(name keys.KeyName) tea.Cmd {
	m.menu.Keydown(name)
	return func() tea.Msg {
		select {
		case <-m.ctx.Done():
		case <-time.After(500 * time.Millisecond):
		}

		return keyupMsg{}
	}
}

// softKillInstanceCmd returns a tea.Cmd that terminates inst's tmux session and
// resets its status to Ready. Used by both KeyKill and KeyKillAndRemove so the
// async kill behaviour stays single-sourced.
func softKillInstanceCmd(inst *session.Instance) tea.Cmd {
	return func() tea.Msg {
		inst.StopTmux()
		inst.SetStatus(session.Ready)
		return instanceChangedMsg{}
	}
}
