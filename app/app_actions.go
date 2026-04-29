package app

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/kastheco/kasmos/config"
	"github.com/kastheco/kasmos/config/auditlog"
	"github.com/kastheco/kasmos/config/taskfsm"
	"github.com/kastheco/kasmos/config/taskparser"
	"github.com/kastheco/kasmos/config/taskstate"
	"github.com/kastheco/kasmos/config/taskstore"
	"github.com/kastheco/kasmos/internal/initcmd/scaffold"
	"github.com/kastheco/kasmos/keys"
	"github.com/kastheco/kasmos/orchestration"
	"github.com/kastheco/kasmos/session"
	gitpkg "github.com/kastheco/kasmos/session/git"
	"github.com/kastheco/kasmos/session/tmux"
	"github.com/kastheco/kasmos/ui"
	"github.com/kastheco/kasmos/ui/overlay"

	tea "charm.land/bubbletea/v2"
	"github.com/atotto/clipboard"
)

// executeContextAction performs the action selected from a context menu.
func (m *home) executeContextAction(action string) (tea.Model, tea.Cmd) {
	switch action {
	case "cleanup_instance":
		selected := m.nav.GetSelectedInstance()
		if selected != nil {
			m.audit(auditlog.EventAgentKilled, "killed and removed instance",
				auditlog.WithInstance(selected.Title),
				auditlog.WithAgent(selected.AgentType),
				auditlog.WithPlan(selected.TaskFile),
				auditlog.WithKillDetails("kill_and_remove_instance", true, false),
			)
			return m, tea.Batch(softKillInstanceCmd(m, selected), m.dismissInstanceFromList(selected))
		}
		return m, nil

	case "kill_instance":
		selected := m.nav.GetSelectedInstance()
		if selected != nil {
			// Emit audit before attempting pause so the event is always recorded
			// even when the instance has not been started (e.g. exited, loading).
			m.audit(auditlog.EventAgentKilled, "agent stopped (branch preserved)",
				auditlog.WithInstance(selected.Title),
				auditlog.WithAgent(selected.AgentType),
				auditlog.WithPlan(selected.TaskFile),
				auditlog.WithKillDetails("kill_instance", false, true),
			)
			if err := selected.Pause(); err != nil {
				return m, m.handleError(err)
			}
			m.saveAllInstances()
			m.updateNavPanelStatus()
		}
		return m, tea.Batch(tea.RequestWindowSize, m.instanceChanged())

	case "open_instance":
		selected := m.nav.GetSelectedInstance()
		if selected == nil || !selected.Started() || selected.Paused() || !selected.TmuxAlive() {
			return m, nil
		}
		if session.NormalizeExecutionMode(selected.ExecutionMode) == session.ExecutionModeSDK {
			m.toastManager.Info(fmt.Sprintf("%s is running in sdk mode; attach is disabled", selected.Title))
			return m, nil
		}
		return m, tea.Exec(tmux.NewAttachExecCommand(selected), func(err error) tea.Msg {
			if err != nil {
				return err
			}
			return tmuxAttachReturnMsg{}
		})

	case "pause_instance":
		selected := m.nav.GetSelectedInstance()
		if selected != nil && selected.Status != session.Paused {
			if err := selected.Pause(); err != nil {
				return m, m.handleError(err)
			}
			m.audit(auditlog.EventAgentPaused, "agent paused",
				auditlog.WithInstance(selected.Title),
				auditlog.WithAgent(selected.AgentType),
				auditlog.WithPlan(selected.TaskFile),
			)
			m.saveAllInstances()
		}
		return m, tea.Batch(tea.RequestWindowSize, m.instanceChanged())

	case "resume_instance":
		selected := m.nav.GetSelectedInstance()
		if selected != nil && selected.Status == session.Paused {
			if err := selected.Resume(); err != nil {
				return m, m.handleError(err)
			}
			m.audit(auditlog.EventAgentResumed, "agent resumed",
				auditlog.WithInstance(selected.Title),
				auditlog.WithAgent(selected.AgentType),
				auditlog.WithPlan(selected.TaskFile),
			)
			m.saveAllInstances()
		}
		return m, tea.Batch(tea.RequestWindowSize, m.instanceChanged())

	case "push_instance":
		selected := m.nav.GetSelectedInstance()
		if selected == nil {
			return m, nil
		}
		return m.pushSelectedInstance()

	case "merge_instance":
		selected := m.nav.GetSelectedInstance()
		if selected == nil {
			return m, nil
		}
		return m.mergeTaskToMain(selected.TaskFile)

	case "create_pr_instance":
		selected := m.nav.GetSelectedInstance()
		if selected == nil {
			return m, nil
		}
		m.state = statePRTitle
		tio := overlay.NewTextInputOverlay("pr title", selected.Title)
		tio.SetSize(60, 3)
		m.overlays.Show(tio)
		return m, nil

	case "send_prompt_instance":
		selected := m.nav.GetSelectedInstance()
		if selected == nil || !selected.Started() || selected.Paused() {
			return m, nil
		}
		return m, m.enterFocusMode()

	case "copy_worktree_path":
		selected := m.nav.GetSelectedInstance()
		if selected == nil {
			return m, nil
		}
		worktree, err := selected.GetGitWorktree()
		if err != nil {
			return m, m.handleError(err)
		}
		_ = clipboard.WriteAll(worktree.GetWorktreePath())
		return m, nil

	case "copy_branch_name":
		selected := m.nav.GetSelectedInstance()
		if selected == nil {
			return m, nil
		}
		_ = clipboard.WriteAll(selected.Branch)
		return m, nil

	case "rename_instance":
		selected := m.nav.GetSelectedInstance()
		if selected == nil {
			return m, nil
		}
		m.state = stateRenameInstance
		tio := overlay.NewTextInputOverlay("rename instance", selected.DisplayName())
		tio.SetSize(60, 3)
		m.overlays.Show(tio)
		return m, nil

	case "mark_task_complete":
		selected := m.nav.GetSelectedInstance()
		if selected == nil || selected.TaskNumber == 0 {
			return m, nil
		}
		if orch, ok := m.waveOrchestrators[selected.TaskFile]; ok {
			orch.MarkTaskComplete(selected.TaskNumber)
		} else if m.taskStore != nil && m.taskStoreProject != "" {
			// No orchestrator — persist directly to store so the daemon sees it.
			_ = m.taskStore.UpdateSubtaskStatus(m.taskStoreProject, selected.TaskFile, selected.TaskNumber, taskstore.SubtaskStatusComplete)
		}
		selected.ImplementationComplete = true
		selected.SetStatus(session.Ready)
		m.toastManager.Success(fmt.Sprintf("task %d marked complete", selected.TaskNumber))
		return m, tea.Batch(m.instanceChanged(), m.toastTickCmd())

	case "mark_planner_finished":
		return m, m.emitSelectedInstanceSignal(taskfsm.PlannerFinished, "planner finished signal queued")

	case "mark_architect_finished":
		return m, m.emitSelectedInstanceSignal(taskfsm.ArchitectFinished, "architect pass finished signal queued")

	case "mark_implement_finished":
		return m, m.emitSelectedInstanceSignal(taskfsm.ImplementFinished, "implement finished signal queued")

	case "mark_review_approved":
		return m, m.emitSelectedInstanceSignal(taskfsm.ReviewApproved, "review approved signal queued")

	case "mark_review_changes_requested":
		return m, m.emitSelectedInstanceSignal(taskfsm.ReviewChangesRequested, "review changes requested signal queued")

	case "mark_verify_approved":
		return m, m.emitSelectedInstanceSignal(taskfsm.VerifyApproved, "verify approved signal queued")

	case "mark_verify_failed":
		return m, m.emitSelectedInstanceSignal(taskfsm.VerifyFailed, "verify failed signal queued")

	case "advance_review_cycle":
		selected := m.nav.GetSelectedInstance()
		if selected == nil || selected.TaskFile == "" || m.taskState == nil {
			return m, nil
		}
		if err := m.captureSelectedReviewFeedback(selected); err != nil {
			return m, m.handleError(err)
		}
		cycle, err := m.incrementReviewCycleAndRefresh(selected.TaskFile, selected.Title, selected.AgentType)
		if err != nil {
			return m, m.handleError(err)
		}
		m.toastManager.Success(fmt.Sprintf("advanced review cycle to %d", cycle))
		return m, m.toastTickCmd()

	case "change_topic":
		planFile := m.nav.GetSelectedPlanFile()
		if planFile == "" {
			return m, nil
		}
		m.pendingChangeTopicTask = planFile
		topicNames := m.getTopicNames()
		topicNames = append([]string{"(No topic)"}, topicNames...)
		po := overlay.NewPickerOverlay("Move to topic", topicNames)
		po.SetAllowCustom(true)
		m.overlays.Show(po)
		m.state = stateChangeTopic
		return m, nil

	case "set_status":
		planFile := m.nav.GetSelectedPlanFile()
		if planFile == "" {
			return m, nil
		}
		m.pendingSetStatusTask = planFile
		m.overlays.Show(overlay.NewPickerOverlay("set status", taskstate.ManualOverrideOptions()))
		m.state = stateSetStatus
		return m, nil

	case "start_plan":
		planFile := m.nav.GetSelectedPlanFile()
		if planFile == "" {
			return m, nil
		}
		return m.triggerTaskStage(planFile, "plan")

	case "start_implement":
		planFile := m.nav.GetSelectedPlanFile()
		if planFile == "" {
			return m, nil
		}
		return m.triggerTaskStage(planFile, "implement")

	case "start_implement_direct":
		planFile := m.nav.GetSelectedPlanFile()
		if planFile == "" {
			return m, nil
		}
		return m.triggerTaskStage(planFile, "implement_direct")

	case "start_solo":
		planFile := m.nav.GetSelectedPlanFile()
		if planFile == "" {
			return m, nil
		}
		return m.triggerTaskStage(planFile, "solo")

	case "start_review":
		planFile := m.nav.GetSelectedPlanFile()
		if planFile == "" {
			return m, nil
		}
		return m.triggerTaskStage(planFile, "review")

	case "start_verify":
		planFile := m.nav.GetSelectedPlanFile()
		if planFile == "" {
			return m, nil
		}
		entry, ok := m.refreshTaskEntry(planFile)
		if !ok {
			return m, lifecycleActionRejected("task not found; start verify cancelled")
		}
		if !taskActionStillAllowed(entry, "start_verify") {
			return m, lifecycleActionRejected("task state changed; start verify no longer available")
		}
		return m.executeTaskStage(planFile, "verify")

	case "start_fixer":
		planFile := m.nav.GetSelectedPlanFile()
		if planFile == "" {
			return m, nil
		}
		entry, ok := m.refreshTaskEntry(planFile)
		if !ok {
			return m, m.handleError(fmt.Errorf("task not found: %s", planFile))
		}
		if !taskActionStillAllowed(entry, "start_fixer") {
			return m, lifecycleActionRejected("task state changed; start fixer no longer available")
		}

		feedback := m.latestReviewFeedback(planFile)
		var cmds []tea.Cmd
		if cmd := m.handleReviewChangesRequested(planFile, feedback); cmd != nil {
			cmds = append(cmds, cmd)
		}

		if entry.Status == taskstate.StatusReviewing {
			if m.appConfig != nil && m.appConfig.MaxReviewFixCycles > 0 {
				if cycle, err := m.taskState.ReviewCycle(planFile); err == nil && cycle+1 > m.appConfig.MaxReviewFixCycles {
					planName := taskstate.DisplayName(planFile)
					m.toastManager.Error(fmt.Sprintf(
						"review-fix loop stopped: cycle limit reached (%d/%d) for %s",
						cycle+1, m.appConfig.MaxReviewFixCycles, planName))
					return m, m.toastTickCmd()
				}
			}
			if err := m.fsm.Transition(planFile, taskfsm.ReviewChangesRequested); err != nil {
				return m, m.handleError(err)
			}
			if err := m.taskState.IncrementReviewCycle(planFile); err != nil {
				return m, m.handleError(err)
			}
			m.audit(auditlog.EventPlanTransition, "reviewing → implementing (manual fixer)",
				auditlog.WithPlan(planFile))
			m.loadTaskState()
			m.updateSidebarTasks()
		}

		if cmd := m.spawnFixerWithFeedback(planFile, feedback); cmd != nil {
			cmds = append(cmds, cmd)
		}
		return m, tea.Batch(cmds...)

	case "inspect_plan":
		planFile := m.nav.GetSelectedPlanFile()
		if planFile != "" {
			m.nav.InspectPlan(planFile)
		}
		return m, tea.RequestWindowSize

	case "view_plan":
		return m.viewSelectedPlan()

	case "open_plan_browser":
		return m.openPlanBrowserForSelection()

	case "rename_plan":
		planFile := m.nav.GetSelectedPlanFile()
		if planFile == "" {
			return m, nil
		}
		currentName := taskstate.DisplayName(planFile)
		m.state = stateRenameTask
		tio := overlay.NewTextInputOverlay("rename task", currentName)
		tio.SetSize(60, 3)
		m.overlays.Show(tio)
		return m, nil

	case "chat_about_plan":
		planFile := m.nav.GetSelectedPlanFile()
		if planFile == "" {
			return m, nil
		}
		m.pendingChatAboutTask = planFile
		m.state = stateChatAboutTask
		tio := overlay.NewTextInputOverlay("ask about this task", "")
		tio.SetSize(60, 5)
		tio.SetMultiline(true)
		tio.SetPlaceholder("what would you like to know?")
		m.overlays.Show(tio)
		return m, nil

	case "push_plan_branch":
		planInst := m.findTaskInstance()
		if planInst == nil {
			return m, m.handleError(fmt.Errorf("no active session for this plan"))
		}
		capturedPlanTitle := planInst.Title
		capturedPlanBranch := planInst.Branch
		pushAction := func() tea.Msg {
			worktree, err := planInst.GetGitWorktree()
			if err != nil {
				return err
			}
			if err := worktree.PushChanges("update from kas", true); err != nil {
				return err
			}
			m.audit(auditlog.EventGitPush, fmt.Sprintf("pushed plan branch %s", capturedPlanBranch),
				auditlog.WithInstance(capturedPlanTitle),
			)
			return nil
		}
		message := fmt.Sprintf("push changes from plan '%s'?", planInst.Title)
		return m, m.confirmAction(message, pushAction)

	case "create_plan_pr":
		planFile := m.nav.GetSelectedPlanFile()
		if planFile == "" || m.taskState == nil {
			return m, m.handleError(fmt.Errorf("no plan selected"))
		}
		entry, ok := m.taskState.Entry(planFile)
		if !ok || entry.Branch == "" {
			return m, m.handleError(fmt.Errorf("plan has no branch — implement it first"))
		}
		// Prefer to use the running instance so GetSelectedInstance() works
		// in the PR-body submission path. Fall back to a worktree-only approach
		// (pendingPRWorktree) when there is no instance or the instance has an
		// empty branch (e.g. started on main without a worktree).
		planInst := m.findTaskInstance()
		if planInst != nil && planInst.Branch != "" {
			m.nav.SelectInstance(planInst)
			m.pendingPRWorktree = nil
		} else {
			// No valid running instance — build a GitWorktree directly from the
			// task store's authoritative branch so PR creation still works.
			m.pendingPRWorktree = gitpkg.NewSharedTaskWorktree(m.activeRepoPath, entry.Branch)
		}
		defaultTitle := taskstate.DisplayName(planFile)
		m.state = statePRTitle
		tio := overlay.NewTextInputOverlay("pr title", defaultTitle)
		tio.SetSize(60, 3)
		m.overlays.Show(tio)
		return m, nil

	case "merge_plan":
		return m.mergeTaskToMain(m.nav.GetSelectedPlanFile())

	case "mark_plan_done":
		planFile := m.nav.GetSelectedPlanFile()
		if planFile == "" {
			return m, nil
		}
		entry, ok := m.refreshTaskEntry(planFile)
		if !ok {
			return m, lifecycleActionRejected("task not found; mark done cancelled")
		}
		if entry.Status == taskstate.StatusCancelled {
			return m, lifecycleActionRejected("task is cancelled; mark done not available")
		}
		if entry.Status != taskstate.StatusDone {
			// Walk through any missing lifecycle stages before approval so mark-done
			// works from ready/implementing/reviewing/verifying states.
			if entry.Status != taskstate.StatusReviewing && entry.Status != taskstate.StatusVerifying {
				if err := m.fsmSetReviewing(planFile); err != nil {
					return m, m.handleError(err)
				}
			}
			if entry.Status != taskstate.StatusVerifying {
				if err := m.fsm.Transition(planFile, taskfsm.ReviewApproved); err != nil {
					return m, m.handleError(err)
				}
			}
			if err := m.fsm.Transition(planFile, taskfsm.VerifyApproved); err != nil {
				return m, m.handleError(err)
			}
			m.audit(auditlog.EventPlanTransition, string(entry.Status)+" → done (manual)",
				auditlog.WithPlan(planFile))
		}
		if err := m.clearExecutionState(planFile); err != nil {
			return m, m.handleError(err)
		}
		m.loadTaskState()
		m.updateSidebarTasks()
		return m, tea.RequestWindowSize

	case "request_review":
		planFile := m.nav.GetSelectedPlanFile()
		if planFile == "" {
			return m, nil
		}
		freshEntry, freshOk := m.refreshTaskEntry(planFile)
		if !freshOk {
			return m, lifecycleActionRejected("task not found; request review cancelled")
		}
		if !taskActionStillAllowed(freshEntry, "request_review") {
			return m, lifecycleActionRejected("task state changed; request review no longer available")
		}
		if err := m.fsm.Transition(planFile, taskfsm.RequestReview); err != nil {
			return m, m.handleError(err)
		}
		m.loadTaskState()
		m.updateSidebarTasks()
		if cmd := m.spawnReviewer(planFile); cmd != nil {
			return m, cmd
		}
		return m, tea.RequestWindowSize

	case "resume_implement":
		planFile := m.nav.GetSelectedPlanFile()
		if planFile == "" {
			return m, nil
		}
		freshEntry, freshOk := m.refreshTaskEntry(planFile)
		if !freshOk {
			return m, lifecycleActionRejected("task not found; resume implement cancelled")
		}
		if !taskActionStillAllowed(freshEntry, "resume_implement") {
			return m, lifecycleActionRejected("task state changed; resume implement no longer available")
		}
		if err := m.fsm.Transition(planFile, taskfsm.Reimplement); err != nil {
			return m, m.handleError(err)
		}
		m.loadTaskState()
		m.updateSidebarTasks()
		return m, tea.RequestWindowSize

	case "cancel_plan":
		planFile := m.nav.GetSelectedPlanFile()
		if planFile == "" {
			return m, nil
		}
		planName := taskstate.DisplayName(planFile)
		cancelAction := func() tea.Msg {
			// Re-validate with a fresh snapshot: the task may have changed while the
			// confirmation overlay was open.
			freshEntry, freshOk := m.refreshTaskEntry(planFile)
			if !freshOk {
				return lifecycleActionRejectedMsg{message: "task not found; cancel aborted"}
			}
			if freshEntry.Status == taskstate.StatusCancelled || freshEntry.Status == taskstate.StatusDone {
				return lifecycleActionRejectedMsg{message: "task state changed; cancel no longer needed"}
			}
			m.nav.SelectNextOrPrevExcludingTask(planFile)
			if err := m.fsm.Transition(planFile, taskfsm.Cancel); err != nil {
				return err
			}
			if err := m.clearExecutionState(planFile); err != nil {
				return err
			}
			m.audit(auditlog.EventPlanCancelled, "task cancelled by user: "+planName,
				auditlog.WithPlan(planFile))
			return taskRefreshMsg{}
		}
		return m, m.confirmAction(fmt.Sprintf("cancel task '%s'?", planName), cancelAction)

	case "modify_plan":
		planFile := m.nav.GetSelectedPlanFile()
		if planFile == "" {
			return m, nil
		}
		if _, ok := m.taskState.Entry(planFile); !ok {
			return m, m.handleError(fmt.Errorf("task not found: %s", planFile))
		}
		if err := m.fsm.Transition(planFile, taskfsm.PlanStart); err != nil {
			return m, m.handleError(err)
		}
		m.loadTaskState()
		m.updateSidebarTasks()
		return m.spawnTaskAgent(planFile, "plan", buildModifyTaskPrompt(planFile, m.taskStoreProject))

	case "start_over_plan":
		planFile := m.nav.GetSelectedPlanFile()
		if planFile == "" {
			return m, nil
		}
		planName := taskstate.DisplayName(planFile)
		instancesToKill := make([]*session.Instance, 0)
		for _, inst := range m.allInstances {
			if inst.TaskFile == planFile {
				instancesToKill = append(instancesToKill, inst)
			}
		}
		startOverAction := func() tea.Msg {
			// Re-validate with a fresh snapshot: the task may have changed while the
			// confirmation overlay was open.
			var freshEntry taskstate.TaskEntry
			var freshOk bool
			if m.taskStore != nil {
				entry, err := m.taskStore.Get(m.taskStoreProject, planFile)
				if err != nil {
					return lifecycleActionRejectedMsg{message: "task not found; start over aborted"}
				}
				freshEntry = taskstate.TaskEntry{
					Status:      taskstate.Status(entry.Status),
					Description: entry.Description,
					Branch:      entry.Branch,
				}
				freshOk = true
			} else if m.taskState != nil {
				freshEntry, freshOk = m.taskState.Entry(planFile)
			}
			if !freshOk {
				return lifecycleActionRejectedMsg{message: "task not found; start over aborted"}
			}
			if freshEntry.Status == taskstate.StatusCancelled {
				return lifecycleActionRejectedMsg{message: "task is cancelled; start over not available"}
			}
			for _, inst := range instancesToKill {
				_ = inst.Kill()
			}
			if err := gitpkg.ResetTaskBranch(m.activeRepoPath, freshEntry.Branch); err != nil {
				return err
			}
			switch taskfsm.Status(freshEntry.Status) {
			case taskfsm.StatusPlanning:
			case taskfsm.StatusDone:
				if err := m.fsm.Transition(planFile, taskfsm.StartOver); err != nil {
					return err
				}
			default:
				if err := m.fsm.Transition(planFile, taskfsm.Cancel); err != nil {
					return err
				}
				if err := m.fsm.Transition(planFile, taskfsm.Reopen); err != nil {
					return err
				}
			}
			m.audit(auditlog.EventPlanTransition, string(freshEntry.Status)+" → planning (start over)",
				auditlog.WithPlan(planFile),
				auditlog.WithDetail("start over: branch reset"))
			return startOverCompletedMsg{
				planFile:    planFile,
				planName:    planName,
				description: freshEntry.Description,
			}
		}
		return m, m.confirmAction(fmt.Sprintf("start over task '%s'? this resets the branch.", planName), startOverAction)

	case "restart_instance":
		selected := m.nav.GetSelectedInstance()
		if selected == nil {
			return m, nil
		}
		capturedTitle := selected.Title
		capturedAgent := selected.AgentType
		capturedPlan := selected.TaskFile
		return m, func() tea.Msg {
			err := selected.Restart()
			if err != nil {
				return err
			}
			m.audit(auditlog.EventAgentRestarted, "agent restarted",
				auditlog.WithInstance(capturedTitle),
				auditlog.WithAgent(capturedAgent),
				auditlog.WithPlan(capturedPlan),
			)
			_ = m.saveAllInstances()
			return instanceChangedMsg{}
		}

	case "toggle_auto_advance_planner":
		if m.appConfig == nil {
			return m, nil
		}
		m.appConfig.AutoAdvance = !m.appConfig.AutoAdvance
		label := "off"
		if m.appConfig.AutoAdvance {
			label = "on"
		}
		m.toastManager.Success(fmt.Sprintf("auto-advance planner: %s", label))
		return m, m.toastTickCmd()

	case "toggle_auto_advance", "toggle_auto_advance_waves":
		if m.appConfig == nil {
			return m, nil
		}
		m.appConfig.AutoAdvanceWaves = !m.appConfig.AutoAdvanceWaves
		label := "off"
		if m.appConfig.AutoAdvanceWaves {
			label = "on"
		}
		m.toastManager.Success(fmt.Sprintf("auto-advance waves: %s", label))
		return m, m.toastTickCmd()

	case "toggle_auto_review_fix":
		if m.appConfig == nil {
			return m, nil
		}
		m.appConfig.AutoReviewFix = !m.appConfig.AutoReviewFix
		label := "off"
		if m.appConfig.AutoReviewFix {
			label = "on"
		}
		m.toastManager.Success(fmt.Sprintf("auto review-fix loop: %s", label))
		return m, m.toastTickCmd()

	// ── Log-line context menu actions ──────────────────────────────────────
	// These are triggered from the audit pane cursor (stateAuditCursor).
	// m.pendingLogEvent holds the event that was selected.

	case "log_send_to_fixer":
		if m.pendingLogEvent == nil || m.pendingLogEvent.TaskFile == "" {
			return m, nil
		}
		planFile := m.pendingLogEvent.TaskFile
		m.pendingLogEvent = nil
		// Use spawnFixerWithFeedback with the event message as context so the
		// fixer knows what went wrong.
		return m, m.spawnFixerWithFeedback(planFile, "")

	case "log_retry_wave":
		if m.pendingLogEvent == nil || m.pendingLogEvent.TaskFile == "" {
			return m, nil
		}
		planFile := m.pendingLogEvent.TaskFile
		m.pendingLogEvent = nil
		orch, ok := m.waveOrchestrators[planFile]
		if !ok {
			m.toastManager.Error("no active wave orchestrator for this plan")
			return m, m.toastTickCmd()
		}
		if m.taskState == nil {
			return m, nil
		}
		entry, ok2 := m.taskState.Plans[planFile]
		if !ok2 {
			return m, nil
		}
		return m.retryFailedWaveTasks(orch, entry)

	case "log_advance_wave":
		if m.pendingLogEvent == nil || m.pendingLogEvent.TaskFile == "" {
			return m, nil
		}
		planFile := m.pendingLogEvent.TaskFile
		m.pendingLogEvent = nil
		return m, m.queueRecoveryAction(planFile, "advance_wave", "", "advance wave recovery queued")

	case "log_restart_agent":
		if m.pendingLogEvent == nil || m.pendingLogEvent.InstanceTitle == "" {
			return m, nil
		}
		if auditLogCleanupKill(m.pendingLogEvent) {
			m.pendingLogEvent = nil
			return m, nil
		}
		title := m.pendingLogEvent.InstanceTitle
		m.pendingLogEvent = nil
		for _, inst := range m.allInstances {
			if inst.Title == title {
				capturedTitle := inst.Title
				capturedAgent := inst.AgentType
				capturedPlan := inst.TaskFile
				return m, func() tea.Msg {
					err := inst.Restart()
					if err != nil {
						return err
					}
					m.audit(auditlog.EventAgentRestarted, "agent restarted via log action",
						auditlog.WithInstance(capturedTitle),
						auditlog.WithAgent(capturedAgent),
						auditlog.WithPlan(capturedPlan),
					)
					_ = m.saveAllInstances()
					return instanceChangedMsg{}
				}
			}
		}
		m.toastManager.Error(fmt.Sprintf("instance '%s' not found", title))
		return m, m.toastTickCmd()

	case "log_reopen_worktree":
		if m.pendingLogEvent == nil || m.pendingLogEvent.InstanceTitle == "" {
			return m, nil
		}
		if auditLogCleanupKill(m.pendingLogEvent) {
			m.pendingLogEvent = nil
			return m, nil
		}
		title := m.pendingLogEvent.InstanceTitle
		m.pendingLogEvent = nil
		for _, inst := range m.allInstances {
			if inst.Title == title {
				capturedTitle := inst.Title
				capturedAgent := inst.AgentType
				capturedPlan := inst.TaskFile
				return m, func() tea.Msg {
					if err := inst.Resume(); err != nil {
						return err
					}
					m.audit(auditlog.EventAgentResumed, "worktree reopened via log action",
						auditlog.WithInstance(capturedTitle),
						auditlog.WithAgent(capturedAgent),
						auditlog.WithPlan(capturedPlan),
					)
					_ = m.saveAllInstances()
					return instanceChangedMsg{}
				}
			}
		}
		m.toastManager.Error(fmt.Sprintf("instance '%s' not found", title))
		return m, m.toastTickCmd()

	case "log_start_review":
		if m.pendingLogEvent == nil || m.pendingLogEvent.TaskFile == "" {
			return m, nil
		}
		planFile := m.pendingLogEvent.TaskFile
		m.pendingLogEvent = nil
		return m.triggerTaskStage(planFile, "review")
	}

	return m, nil
}

func auditLogCleanupKill(e *ui.AuditEventDisplay) bool {
	if e == nil || e.Kind != "agent_killed" || e.DetailJSON == "" {
		return false
	}
	var detail struct {
		Cleanup bool `json:"cleanup"`
	}
	if err := json.Unmarshal([]byte(e.DetailJSON), &detail); err != nil {
		return false
	}
	return detail.Cleanup
}

func (m *home) mergeTaskToMain(planFile string) (tea.Model, tea.Cmd) {
	if planFile == "" || m.taskState == nil {
		return m, nil
	}
	entry, ok := m.taskState.Entry(planFile)
	if !ok {
		return m, m.handleError(fmt.Errorf("task not found: %s", planFile))
	}
	if entry.Branch == "" {
		return m, m.handleError(fmt.Errorf("plan has no branch to merge"))
	}
	planName := taskstate.DisplayName(planFile)
	mergeAction := func() tea.Msg {
		if err := gitpkg.PreflightMergeTaskBranch(m.activeRepoPath, entry.Branch); err != nil {
			return err
		}
		// Kill all instances bound to this plan.
		for i := len(m.allInstances) - 1; i >= 0; i-- {
			if m.allInstances[i].TaskFile == planFile {
				if err := m.allInstances[i].Kill(); err != nil {
					return err
				}
				m.allInstances = append(m.allInstances[:i], m.allInstances[i+1:]...)
			}
		}
		if err := gitpkg.MergeTaskBranch(m.activeRepoPath, entry.Branch); err != nil {
			return err
		}
		// Walk through FSM to done if not already there.
		if taskfsm.Status(entry.Status) != taskfsm.StatusDone {
			if taskfsm.Status(entry.Status) != taskfsm.StatusReviewing &&
				taskfsm.Status(entry.Status) != taskfsm.StatusVerifying {
				if err := m.fsmSetReviewing(planFile); err != nil {
					return err
				}
			}
			if taskfsm.Status(entry.Status) != taskfsm.StatusVerifying {
				if err := m.fsm.Transition(planFile, taskfsm.ReviewApproved); err != nil {
					return err
				}
			}
			if err := m.fsm.Transition(planFile, taskfsm.VerifyApproved); err != nil {
				return err
			}
		}
		if err := m.clearExecutionState(planFile); err != nil {
			return err
		}
		m.audit(auditlog.EventPlanMerged, "task merged to main: "+planName,
			auditlog.WithPlan(planFile))
		_ = m.saveAllInstances()
		m.loadTaskState()
		m.updateSidebarTasks()
		return taskRefreshMsg{}
	}
	return m, m.confirmAction(fmt.Sprintf("merge '%s' branch into main?", planName), mergeAction)
}

// fsmSetImplementing transitions the plan to implementing, handling the
// planning→ready→implementing two-step when called after a planner finishes.
func (m *home) fsmSetImplementing(planFile string) error {
	if m.taskState == nil {
		return fmt.Errorf("task state is not loaded")
	}
	entry, ok := m.taskState.Entry(planFile)
	if !ok {
		return fmt.Errorf("task not found: %s", planFile)
	}
	if taskstate.IsDraftReady(entry) {
		return fmt.Errorf("task is ready but not yet planned: %s", planFile)
	}
	current := taskfsm.Status(entry.Status)
	if current == taskfsm.StatusImplementing {
		return nil // already implementing (re-spawning coder), no status change
	}
	if current == taskfsm.StatusPlanning {
		// Planner finished without writing a sentinel — transition through ready.
		if err := m.fsm.Transition(planFile, taskfsm.PlannerFinished); err != nil {
			return err
		}
		m.audit(auditlog.EventPlanTransition, "planning → ready",
			auditlog.WithPlan(planFile))
	}
	if err := m.fsm.Transition(planFile, taskfsm.ImplementStart); err != nil {
		return err
	}
	m.audit(auditlog.EventPlanTransition, string(current)+" → implementing",
		auditlog.WithPlan(planFile))
	return nil
}

// fsmSetReviewing walks the FSM to reviewing from any earlier state.
// If already reviewing, it's a no-op (allows re-spawning a reviewer).
func (m *home) fsmSetReviewing(planFile string) error {
	if m.taskState == nil {
		return fmt.Errorf("task state is not loaded")
	}
	entry, ok := m.taskState.Entry(planFile)
	if !ok {
		return fmt.Errorf("task not found: %s", planFile)
	}
	current := taskfsm.Status(entry.Status)
	if current == taskfsm.StatusReviewing {
		return nil // already reviewing, no-op
	}
	// Walk through intermediate states to reach implementing first.
	if current != taskfsm.StatusImplementing {
		if err := m.fsmSetImplementing(planFile); err != nil {
			return err
		}
	}
	if err := m.fsm.Transition(planFile, taskfsm.ImplementFinished); err != nil {
		return err
	}
	m.audit(auditlog.EventPlanTransition, string(current)+" → reviewing",
		auditlog.WithPlan(planFile))
	return nil
}

// fsmRevertToPlanning moves the plan back to planning state from implementing.
// Used when implementation can't start (e.g., missing wave headers).
func (m *home) fsmRevertToPlanning(planFile string) error {
	if m.taskState == nil {
		return fmt.Errorf("task state is not loaded")
	}
	entry, ok := m.taskState.Entry(planFile)
	if !ok {
		return fmt.Errorf("task not found: %s", planFile)
	}
	if taskfsm.Status(entry.Status) == taskfsm.StatusPlanning {
		return nil // already there
	}
	if err := m.fsm.Transition(planFile, taskfsm.Cancel); err != nil {
		return err
	}
	return m.fsm.Transition(planFile, taskfsm.Reopen)
}

// fsmForceToPlanning moves the plan to planning from any current state.
// Used for start-over scenarios where branch history is reset.
func (m *home) fsmForceToPlanning(planFile string) error {
	if m.taskState == nil {
		return fmt.Errorf("task state is not loaded")
	}
	entry, ok := m.taskState.Entry(planFile)
	if !ok {
		return fmt.Errorf("task not found: %s", planFile)
	}
	switch taskfsm.Status(entry.Status) {
	case taskfsm.StatusPlanning:
		return nil
	case taskfsm.StatusCancelled:
		return m.fsm.Transition(planFile, taskfsm.Reopen)
	case taskfsm.StatusDone:
		return m.fsm.Transition(planFile, taskfsm.StartOver)
	default:
		// ready, planning, implementing, reviewing → Cancel then Reopen
		if err := m.fsm.Transition(planFile, taskfsm.Cancel); err != nil {
			return err
		}
		return m.fsm.Transition(planFile, taskfsm.Reopen)
	}
}

// refreshTaskEntry loads a fresh TaskEntry from the store for the given plan file.
// It re-runs loadTaskState() so the in-memory snapshot matches the DB, then calls
// updateSidebarTasks and updateInfoPane so nav/info surfaces stay consistent.
// Falls back to the cached m.taskState when no store is configured, so unit tests
// that don't wire up a real store continue to work.
func (m *home) refreshTaskEntry(planFile string) (taskstate.TaskEntry, bool) {
	if planFile == "" {
		return taskstate.TaskEntry{}, false
	}
	if m.taskStore != nil && m.taskStateDir != "" {
		m.loadTaskState()
		if m.taskState == nil {
			return taskstate.TaskEntry{}, false
		}
		entry, ok := m.taskState.Entry(planFile)
		if ok {
			m.updateSidebarTasks()
			m.updateInfoPane()
		}
		return entry, ok
	}
	// No store configured — fall back to cached snapshot.
	if m.taskState == nil {
		return taskstate.TaskEntry{}, false
	}
	return m.taskState.Entry(planFile)
}

// findTaskInstance returns the instance bound to the currently selected plan in the sidebar.
// Returns nil if no plan is selected or no instance is bound to it.
func (m *home) findTaskInstance() *session.Instance {
	planFile := m.nav.GetSelectedPlanFile()
	if planFile == "" {
		return nil
	}
	for _, inst := range m.nav.GetInstances() {
		if inst.TaskFile == planFile {
			return inst
		}
	}
	return nil
}

// taskLifecycleItems returns lifecycle agent-launch menu items for the given task,
// ordered from most-likely-needed to least, based on the current task status.
// Callers use splitPromotedItems to promote the leading items to root level.
func taskLifecycleItems(entry taskstate.TaskEntry) []overlay.ContextMenuItem {
	switch entry.Status {
	case taskstate.StatusPlanning:
		return []overlay.ContextMenuItem{
			{Label: "start planning", Action: "start_plan"},
			{Label: "start implement", Action: "start_implement"},
			{Label: "implement directly", Action: "start_implement_direct"},
			{Label: "start solo agent", Action: "start_solo"},
			{Label: "start review", Action: "start_review"},
		}
	case taskstate.StatusReady:
		if taskstate.IsDraftReady(entry) {
			return []overlay.ContextMenuItem{
				{Label: "start planning", Action: "start_plan"},
			}
		}
		if taskstate.IsPlannedReady(entry) {
			return []overlay.ContextMenuItem{
				{Label: "start implement", Action: "start_implement"},
				{Label: "implement directly", Action: "start_implement_direct"},
				{Label: "start planning", Action: "start_plan"},
				{Label: "start solo agent", Action: "start_solo"},
				{Label: "start review", Action: "start_review"},
			}
		}
	case taskstate.StatusImplementing:
		return []overlay.ContextMenuItem{
			{Label: "start review", Action: "start_review"},
			{Label: "start fixer", Action: "start_fixer"},
			{Label: "start verify", Action: "start_verify"},
			{Label: "start implement", Action: "start_implement"},
			{Label: "implement directly", Action: "start_implement_direct"},
			{Label: "start solo agent", Action: "start_solo"},
		}
	case taskstate.StatusReviewing:
		return []overlay.ContextMenuItem{
			{Label: "mark finished", Action: "mark_plan_done"},
			{Label: "start fixer", Action: "start_fixer"},
			{Label: "start verify", Action: "start_verify"},
			{Label: "start review", Action: "start_review"},
		}
	case taskstate.StatusVerifying:
		return []overlay.ContextMenuItem{
			{Label: "mark verify approved", Action: "mark_verify_approved"},
			{Label: "mark verify failed", Action: "mark_verify_failed"},
			{Label: "start verify", Action: "start_verify"},
			{Label: "start fixer", Action: "start_fixer"},
		}
	case taskstate.StatusDone:
		return []overlay.ContextMenuItem{
			{Label: "request review", Action: "request_review"},
			{Label: "resume implement", Action: "resume_implement"},
		}
	}
	return nil
}

// splitPromotedItems partitions items into the first limit items (promoted) and
// the remainder. When limit exceeds len(items) all items are returned as promoted.
func splitPromotedItems(items []overlay.ContextMenuItem, limit int) (promoted, remaining []overlay.ContextMenuItem) {
	if limit > len(items) {
		limit = len(items)
	}
	return items[:limit], items[limit:]
}

// actionAvailable reports whether any item (at root level) in items has the given action.
func actionAvailable(items []overlay.ContextMenuItem, action string) bool {
	for _, item := range items {
		if item.Action == action {
			return true
		}
	}
	return false
}

// taskActionStillAllowed returns true when the given task lifecycle action is
// offered by taskLifecycleItems for the fresh entry.  Use this to guard actions
// that were valid when the menu was built but may no longer be valid after the
// daemon advances the FSM while the overlay is open.
func taskActionStillAllowed(entry taskstate.TaskEntry, action string) bool {
	return actionAvailable(taskLifecycleItems(entry), action)
}

// instanceSignalStillAllowed returns true when the given signal action is still
// offered by instanceSignalItems for the fresh entry.
func instanceSignalStillAllowed(inst *session.Instance, entry taskstate.TaskEntry, action string) bool {
	return actionAvailable(instanceSignalItems(inst, entry, true), action)
}

// lifecycleActionRejected returns a Cmd that delivers a lifecycleActionRejectedMsg
// to the Update loop.  Use this to surface stale-state rejections as plain toasts
// rather than audit-log errors.
func lifecycleActionRejected(message string) tea.Cmd {
	return func() tea.Msg {
		return lifecycleActionRejectedMsg{message: message}
	}
}

func (m *home) spawnPlannersForTask(planFile, legacyPrompt, description string) (tea.Model, tea.Cmd) {
	if repoManagedByDaemon(m.activeRepoPath) {
		// The daemon owns profile fan-out for managed repos. Its start
		// handshake accepts both the legacy base title and profile-suffixed
		// planner titles produced by draft mode.
		return m.spawnTaskAgent(planFile, "plan", legacyPrompt)
	}
	if m.appConfig == nil {
		return m.spawnTaskAgent(planFile, "plan", legacyPrompt)
	}
	profiles := m.appConfig.PlannerProfileNames()
	if len(profiles) == 0 {
		return m.spawnTaskAgent(planFile, "plan", legacyPrompt)
	}
	if !m.requireDaemonForAgents() {
		return m, nil
	}
	if m.taskState == nil {
		return m, m.handleError(fmt.Errorf("no task state loaded"))
	}
	if _, ok := m.taskState.Entry(planFile); !ok {
		return m, m.handleError(fmt.Errorf("task not found: %s", planFile))
	}
	if err := scaffold.PatchWorktreeConfig(m.activeRepoPath, m.opencodeAgentConfigs()); err != nil {
		return m, m.handleError(err)
	}

	m.killExistingPlanAgent(planFile, session.AgentTypePlanner)
	// Drop any stale in-memory aggregation from a previous fan-out so the
	// new planners' draft signals aren't ignored. The FSM-driven path resets
	// this in ProcessFSMSignals(PlanStart), but UI flows (this file) and
	// AutoImplementAction-style retries call spawnPlannersForTask directly,
	// bypassing that signal-processing pass.
	if proc := m.ensureProcessor(); proc != nil {
		proc.ResetPlannerDraftAgg(planFile)
	}
	cacheDir := filepath.Join(m.activeRepoPath, ".kasmos", "cache")
	clearCmd := func() tea.Msg {
		if err := orchestration.ClearPlannerDraftCaches(cacheDir, planFile); err != nil {
			return err
		}
		return nil
	}

	var startCmds []tea.Cmd
	startGroupID := m.nextPlannerFanoutStartGroup(planFile)
	for i, profileName := range profiles {
		startCmd, err := m.spawnPlannerProfileForTask(planFile, profileName, i == 0, description, legacyPrompt, startGroupID)
		if err != nil {
			return m, m.handleError(err)
		}
		startCmds = append(startCmds, startCmd)
	}
	startCmds = append([]tea.Cmd{tea.RequestWindowSize}, startCmds...)
	return m, tea.Sequence(clearCmd, tea.Batch(startCmds...))
}

func (m *home) spawnPlannerProfileForTask(planFile, profileName string, primary bool, description, legacyPrompt, startGroupID string) (tea.Cmd, error) {
	profile, err := m.profileForNamedPlanner(profileName)
	if err != nil {
		return nil, err
	}
	cacheName, err := orchestration.PlannerDraftCacheFilename(planFile, profileName)
	if err != nil {
		return nil, err
	}
	spec := orchestration.BuildPlannerAgentSpecWithOptions(planFile, m.taskStoreProject, description, orchestration.PlannerAgentOptions{
		Profile:   profileName,
		Primary:   primary,
		DraftMode: true,
		CachePath: filepath.Join(".kasmos", "cache", cacheName),
	})
	inst, err := session.NewInstance(m.withRetentionOpts(session.InstanceOptions{
		Title:           spec.Title,
		Path:            m.activeRepoPath,
		Program:         buildHarnessAwareProgramCommand(profile),
		ExecutionMode:   session.ExecutionMode(config.NormalizeExecutionMode(profile.ExecutionMode)),
		SDKSpeedTier:    session.NormalizeSDKSpeedTier(profile.Tier),
		SkipPermissions: profile.ResolveSkipPermissions(false),
		TaskFile:        planFile,
		AgentType:       session.AgentTypePlanner,
		ClaudeNoFlicker: m.claudeNoFlicker(),
	}))
	if err != nil {
		return nil, err
	}
	inst.PlannerProfile = profileName
	inst.QueuedPrompt = orchestration.PlannerDraftPromptWithCallerPrompt(spec.Prompt, legacyPrompt)
	inst.SetStatus(session.Loading)
	inst.LoadingTotal = 5
	inst.LoadingMessage = "Preparing session..."
	m.addInstanceFinalizer(inst, m.nav.AddInstance(inst))
	m.nav.SelectInstance(inst)

	m.audit(auditlog.EventAgentSpawned, fmt.Sprintf("spawned planner %s for plan %s", profileName, taskstate.DisplayName(planFile)),
		auditlog.WithPlan(planFile),
		auditlog.WithInstance(spec.Title),
		auditlog.WithAgent(session.AgentTypePlanner),
	)
	plannerInst := inst
	return func() tea.Msg {
		return instanceStartedMsg{instance: plannerInst, err: plannerInst.StartOnMainBranch(), startGroupID: startGroupID}
	}, nil
}

// instanceSignalItems returns the promoted root-level items for an instance context
// menu: attachment controls (open/resume/kill/restart) and any task lifecycle
// signal actions that the instance is authorised to emit.
// entry and hasEntry must come from a freshly loaded TaskEntry so that lifecycle
// signal items reflect the DB state rather than the cached snapshot.
func instanceSignalItems(inst *session.Instance, entry taskstate.TaskEntry, hasEntry bool) []overlay.ContextMenuItem {
	var items []overlay.ContextMenuItem

	isAttachable := inst.Started() && !inst.Paused() && inst.TmuxAlive()
	isRetired := inst.Exited || (hasEntry && (entry.Status == taskstate.StatusDone || entry.Status == taskstate.StatusCancelled))
	if inst.Paused() {
		items = append(items, overlay.ContextMenuItem{Label: "resume", Action: "resume_instance"})
	} else if isAttachable {
		items = append(items, overlay.ContextMenuItem{Label: "open", Action: "open_instance"})
	}
	if isRetired {
		items = append(items, overlay.ContextMenuItem{Label: "cleanup", Action: "cleanup_instance"})
		if hasEntry && entry.Status == taskstate.StatusReviewing {
			items = append(items, overlay.ContextMenuItem{Label: "start review", Action: "start_review"})
		}
	} else {
		items = append(items, overlay.ContextMenuItem{Label: "kill", Action: "kill_instance"})
	}
	items = append(items, overlay.ContextMenuItem{Label: "restart", Action: "restart_instance"})

	// Task-owner lifecycle signal actions — only for the top-level task agent.
	// Wave-task rows (TaskNumber > 0) are managed by subtask completion, not FSM signals.
	// Guard every action with the fresh entry so stale menus don't offer invalid transitions.
	if inst.TaskFile != "" && inst.TaskNumber == 0 && hasEntry {
		switch inst.AgentType {
		case session.AgentTypeReviewer:
			if _, err := taskfsm.ApplyTransition(taskfsm.Status(entry.Status), taskfsm.ReviewApproved); err == nil {
				items = append(items,
					overlay.ContextMenuItem{Label: "mark review approved", Action: "mark_review_approved"},
					overlay.ContextMenuItem{Label: "mark changes requested", Action: "mark_review_changes_requested"},
				)
			}
		case session.AgentTypePlanner:
			if _, err := taskfsm.ApplyTransition(taskfsm.Status(entry.Status), taskfsm.PlannerFinished); err == nil {
				items = append(items, overlay.ContextMenuItem{Label: "mark planning finished", Action: "mark_planner_finished"})
			}
		case session.AgentTypeElaborator:
			if entry.Status == taskstate.StatusImplementing &&
				taskfsm.NormalizeExecutionPhase(entry.ExecutionState.Phase) == taskfsm.ExecutionPhaseArchitecting {
				items = append(items, overlay.ContextMenuItem{Label: "mark architect finished", Action: "mark_architect_finished"})
			}
		case session.AgentTypeCoder, session.AgentTypeFixer:
			if entry.Status == taskstate.StatusImplementing &&
				taskfsm.IsSingleAgentImplementingPhase(taskfsm.NormalizeExecutionPhase(entry.ExecutionState.Phase)) {
				items = append(items, overlay.ContextMenuItem{Label: "mark implement finished", Action: "mark_implement_finished"})
			}
		case session.AgentTypeMaster:
			if entry.Status == taskstate.StatusVerifying {
				items = append(items,
					overlay.ContextMenuItem{Label: "mark verify approved", Action: "mark_verify_approved"},
					overlay.ContextMenuItem{Label: "mark verify failed", Action: "mark_verify_failed"},
				)
			}
		}
	}

	return items
}

// openContextMenu builds a context menu for the currently focused/selected item
// (plan or instance) and positions it next to the selected item.
func (m *home) openContextMenu() (tea.Model, tea.Cmd) {
	if m.focusSlot == slotNav {
		// Nav panel focused — instance rows get the instance menu,
		// plan headers get the plan menu, everything else is a no-op.
		if inst := m.nav.GetSelectedInstance(); inst != nil {
			// fall through to instance context menu below
		} else if planFile := m.nav.GetSelectedPlanFile(); planFile != "" {
			return m.openTaskContextMenu()
		} else {
			return m, nil
		}
	}

	// Build instance context menu (reached from nav or other slots).
	selected := m.nav.GetSelectedInstance()
	if selected == nil {
		return m, nil
	}

	// For task-backed top-level agents, refresh the entry so lifecycle signal
	// items reflect DB state rather than the cached snapshot.
	var entry taskstate.TaskEntry
	var hasEntry bool
	if selected.TaskFile != "" && selected.TaskNumber == 0 {
		entry, hasEntry = m.refreshTaskEntry(selected.TaskFile)
		// Re-read selected in case the sidebar was updated by the refresh.
		selected = m.nav.GetSelectedInstance()
		if selected == nil {
			return m, nil
		}
	}

	// Promoted root-level items: attachment controls + task lifecycle signals.
	var items []overlay.ContextMenuItem
	items = append(items, instanceSignalItems(selected, entry, hasEntry)...)

	// session subgroup: secondary session controls (pause/focus).
	var sessionItems []overlay.ContextMenuItem
	if !selected.Paused() {
		sessionItems = append(sessionItems, overlay.ContextMenuItem{Label: "pause", Action: "pause_instance"})
	}
	if selected.Started() && !selected.Paused() {
		sessionItems = append(sessionItems, overlay.ContextMenuItem{Label: "focus agent", Action: "send_prompt_instance"})
	}
	if len(sessionItems) > 0 {
		items = append(items, overlay.ContextMenuItem{Label: "session", Children: sessionItems})
	}

	// sync group: branch and PR operations.
	syncItems := []overlay.ContextMenuItem{
		{Label: "push branch", Action: "push_instance"},
		{Label: "create pr", Action: "create_pr_instance"},
	}
	if selected.TaskFile != "" {
		syncItems = append(syncItems,
			overlay.ContextMenuItem{Label: "merge to main", Action: "merge_instance"},
			overlay.ContextMenuItem{Label: "open in browser", Action: "open_plan_browser"},
		)
	}
	items = append(items, overlay.ContextMenuItem{Label: "sync", Children: syncItems})

	// manage group: rename, reviewer cycle advance, and wave task completion.
	manageItems := []overlay.ContextMenuItem{
		{Label: "rename", Action: "rename_instance"},
	}
	if selected.TaskFile != "" && selected.TaskNumber == 0 && selected.AgentType == session.AgentTypeReviewer {
		manageItems = append(manageItems, overlay.ContextMenuItem{Label: "advance review cycle", Action: "advance_review_cycle"})
	}
	if selected.TaskNumber > 0 {
		if orch, ok := m.waveOrchestrators[selected.TaskFile]; ok {
			if !orch.IsTaskComplete(selected.TaskNumber) {
				manageItems = append(manageItems, overlay.ContextMenuItem{Label: "mark complete", Action: "mark_task_complete"})
			}
		} else {
			// No orchestrator — still offer manual completion for wave tasks.
			manageItems = append(manageItems, overlay.ContextMenuItem{Label: "mark complete", Action: "mark_task_complete"})
		}
	}
	if len(manageItems) > 0 {
		items = append(items, overlay.ContextMenuItem{Label: "manage", Children: manageItems})
	}

	// Track the lifecycle snapshot so the metadata tick can dismiss the menu
	// if the FSM state changes while it is open.
	if selected.TaskFile != "" && selected.TaskNumber == 0 && hasEntry {
		m.contextMenuTaskFile = selected.TaskFile
		m.contextMenuTaskStatus, m.contextMenuTaskPhase = lifecycleSnapshot(entry)
	}

	// Position at the left edge of the instance list (middle column).
	x := m.navWidth
	y := 1 + 4 + m.nav.GetSelectedIdx() // PaddingTop(1) + header rows + item offset
	m.overlays.ShowPositioned(overlay.NewContextMenu(items), x, y, false)
	m.state = stateContextMenu
	return m, nil
}

func (m *home) openTaskContextMenu() (tea.Model, tea.Cmd) {
	planFile := m.nav.GetSelectedPlanFile()
	if planFile == "" {
		return m, nil
	}

	// Build lifecycle items via the shared helper using a fresh store snapshot so
	// draft-ready vs planned-ready drift (and any other FSM change) is reflected
	// before the menu is constructed.
	entry, entryOk := m.refreshTaskEntry(planFile)
	var allLifecycle []overlay.ContextMenuItem
	if entryOk {
		allLifecycle = taskLifecycleItems(entry)
	}
	promoted, remaining := splitPromotedItems(allLifecycle, 2)

	var items []overlay.ContextMenuItem

	// Promoted lifecycle actions at root (fewer clicks for the happy path).
	items = append(items, promoted...)

	// Read-only inspection items at root.
	items = append(items,
		overlay.ContextMenuItem{Label: "view task", Action: "view_plan"},
		overlay.ContextMenuItem{Label: "chat about this", Action: "chat_about_plan"},
	)
	// History plans get an "inspect task" option to move them to the dead section.
	if m.nav.IsSelectedHistoryPlan() {
		items = append(items, overlay.ContextMenuItem{Label: "inspect task", Action: "inspect_plan"})
	}

	// start subgroup: remaining lifecycle items not promoted to root.
	if len(remaining) > 0 {
		items = append(items, overlay.ContextMenuItem{Label: "start", Children: remaining})
	}

	// sync group: branch and PR operations.
	syncItems := []overlay.ContextMenuItem{
		{Label: "create pr", Action: "create_plan_pr"},
		{Label: "merge to main", Action: "merge_plan"},
		{Label: "open in browser", Action: "open_plan_browser"},
	}
	items = append(items, overlay.ContextMenuItem{Label: "sync", Children: syncItems})

	// config group: task metadata and toggle options.
	autoPlannerLabel := "auto-advance planner: off"
	if m.appConfig != nil && m.appConfig.AutoAdvance {
		autoPlannerLabel = "auto-advance planner: on"
	}
	autoAdvanceLabel := "auto-advance waves: off"
	if m.appConfig != nil && m.appConfig.AutoAdvanceWaves {
		autoAdvanceLabel = "auto-advance waves: on"
	}
	autoReviewFixLabel := "auto review-fix loop: off"
	if m.appConfig != nil && m.appConfig.AutoReviewFix {
		autoReviewFixLabel = "auto review-fix loop: on"
	}
	configItems := []overlay.ContextMenuItem{
		{Label: "rename task", Action: "rename_plan"},
		{Label: "set topic", Action: "change_topic"},
		{Label: autoPlannerLabel, Action: "toggle_auto_advance_planner"},
		{Label: autoAdvanceLabel, Action: "toggle_auto_advance_waves"},
		{Label: autoReviewFixLabel, Action: "toggle_auto_review_fix"},
		{Label: "set status", Action: "set_status"},
	}
	items = append(items, overlay.ContextMenuItem{Label: "config", Children: configItems})

	// lifecycle group: destructive or terminal task transitions.
	lifecycleItems := []overlay.ContextMenuItem{
		{Label: "mark done", Action: "mark_plan_done"},
		{Label: "start over", Action: "start_over_plan"},
		{Label: "cancel task", Action: "cancel_plan"},
	}
	items = append(items, overlay.ContextMenuItem{Label: "lifecycle", Children: lifecycleItems})

	// Track lifecycle snapshot so the metadata tick can dismiss the menu on FSM change.
	if entryOk {
		m.contextMenuTaskFile = planFile
		m.contextMenuTaskStatus, m.contextMenuTaskPhase = lifecycleSnapshot(entry)
	}

	x := m.navWidth
	y := 1 + 4 + m.nav.GetSelectedIdx()
	m.overlays.ShowPositioned(overlay.NewContextMenu(items), x, y, false)
	m.state = stateContextMenu
	return m, nil
}

func isReviewFeedbackSignal(event taskfsm.Event) bool {
	return event == taskfsm.ReviewApproved || event == taskfsm.ReviewChangesRequested ||
		event == taskfsm.VerifyApproved || event == taskfsm.VerifyFailed
}

func gatewaySignalTypeForEvent(event taskfsm.Event) (string, error) {
	return taskfsm.GatewaySignalTypeForEvent(event)
}

func (m *home) validatePlannerCompletion(planFile string) error {
	if m.taskStore == nil || m.taskStoreProject == "" {
		return nil
	}
	content, err := m.taskStore.GetContent(m.taskStoreProject, planFile)
	if err != nil {
		return err
	}
	if strings.TrimSpace(content) == "" {
		return fmt.Errorf("plan content missing; save the plan before marking planning finished")
	}
	if _, err := taskparser.Parse(content); err != nil {
		return fmt.Errorf("plan is not implementation-ready: %w", err)
	}
	return nil
}

func (m *home) prepareSelectedInstanceSignal(selected *session.Instance, event taskfsm.Event) (string, string, error) {
	signalType, err := gatewaySignalTypeForEvent(event)
	if err != nil {
		return "", "", err
	}
	if event == taskfsm.PlannerFinished {
		if err := m.validatePlannerCompletion(selected.TaskFile); err != nil {
			return "", "", err
		}
	}
	if !isReviewFeedbackSignal(event) {
		return signalType, "", nil
	}
	if err := m.captureSelectedReviewFeedback(selected); err != nil {
		return "", "", err
	}
	return signalType, m.reviewFeedbackPayload(selected), nil
}

func (m *home) incrementReviewCycleAndRefresh(planFile, instanceTitle, agentType string) (int, error) {
	if planFile == "" {
		return 0, fmt.Errorf("task file is empty")
	}
	if m.taskState == nil {
		return 0, fmt.Errorf("task state is not loaded")
	}
	if err := m.taskState.IncrementReviewCycle(planFile); err != nil {
		return 0, err
	}
	m.loadTaskState()
	m.updateSidebarTasks()
	m.updateInfoPane()
	cycle, err := m.taskState.ReviewCycle(planFile)
	if err != nil {
		return 0, err
	}

	options := []auditlog.EventOption{auditlog.WithPlan(planFile)}
	if instanceTitle != "" {
		options = append(options, auditlog.WithInstance(instanceTitle))
	}
	if agentType != "" {
		options = append(options, auditlog.WithAgent(agentType))
	}
	m.audit(auditlog.EventPlanTransition, fmt.Sprintf("advanced review cycle to %d", cycle), options...)
	return cycle, nil
}

// eventToSignalAction maps a lifecycle FSM event to the action string that appears in
// instanceSignalItems so that emitSelectedInstanceSignal can validate with
// instanceSignalStillAllowed without duplicating the mapping table.
var eventToSignalAction = map[taskfsm.Event]string{
	taskfsm.PlannerFinished:        "mark_planner_finished",
	taskfsm.ArchitectFinished:      "mark_architect_finished",
	taskfsm.ImplementFinished:      "mark_implement_finished",
	taskfsm.ReviewApproved:         "mark_review_approved",
	taskfsm.ReviewChangesRequested: "mark_review_changes_requested",
	taskfsm.VerifyApproved:         "mark_verify_approved",
	taskfsm.VerifyFailed:           "mark_verify_failed",
}

func (m *home) emitSelectedInstanceSignal(event taskfsm.Event, successToast string) tea.Cmd {
	selected := m.nav.GetSelectedInstance()
	if selected == nil || selected.TaskFile == "" || m.taskStoreProject == "" {
		return nil
	}

	// Refresh task state for task-backed top-level agents before gating the signal.
	if selected.TaskFile != "" && selected.TaskNumber == 0 {
		entry, hasEntry := m.refreshTaskEntry(selected.TaskFile)
		// Re-read selected after a possible sidebar refresh.
		selected = m.nav.GetSelectedInstance()
		if selected == nil {
			return nil
		}
		if action, ok := eventToSignalAction[event]; ok {
			if !hasEntry || !instanceSignalStillAllowed(selected, entry, action) {
				return lifecycleActionRejected("task state changed; signal cancelled")
			}
		}
	}

	signalType, payload, err := m.prepareSelectedInstanceSignal(selected, event)
	if err != nil {
		return m.handleError(err)
	}
	planFile := selected.TaskFile
	instanceTitle := selected.Title
	agentType := selected.AgentType
	project := m.taskStoreProject
	gw := m.signalGateway
	return func() tea.Msg {
		if gw == nil {
			// Fall back to per-call open when no shared gateway is available
			// (e.g. remote task store mode).
			var err error
			gw, err = taskstore.OpenAuthoritativeSignalGateway(project)
			if err != nil {
				return manualSignalResultMsg{err: err}
			}
			defer gw.Close() //nolint:errcheck
		}
		if err := taskfsm.EmitGatewaySignal(gw, project, signalType, planFile, payload); err != nil {
			return manualSignalResultMsg{err: err}
		}
		return manualSignalResultMsg{
			signalType:    signalType,
			planFile:      planFile,
			instanceTitle: instanceTitle,
			agentType:     agentType,
			successToast:  successToast,
		}
	}
}

// emitSelectedRawSignal emits a raw gateway signal type for the selected
// instance's plan. Unlike emitSelectedInstanceSignal it bypasses the FSM-event
// → signal-type mapping and is used for signal types that have no FSM event
// constant (e.g. readiness_approved, readiness_changes_requested).
func (m *home) emitSelectedRawSignal(signalType, successToast string) tea.Cmd {
	selected := m.nav.GetSelectedInstance()
	if selected == nil || selected.TaskFile == "" || m.taskStoreProject == "" {
		return nil
	}
	planFile := selected.TaskFile
	instanceTitle := selected.Title
	agentType := selected.AgentType
	project := m.taskStoreProject
	gw := m.signalGateway
	return func() tea.Msg {
		if gw == nil {
			var err error
			gw, err = taskstore.OpenAuthoritativeSignalGateway(project)
			if err != nil {
				return manualSignalResultMsg{err: err}
			}
			defer gw.Close() //nolint:errcheck
		}
		if err := taskfsm.EmitGatewaySignal(gw, project, signalType, planFile, ""); err != nil {
			return manualSignalResultMsg{err: err}
		}
		return manualSignalResultMsg{
			signalType:    signalType,
			planFile:      planFile,
			instanceTitle: instanceTitle,
			agentType:     agentType,
			successToast:  successToast,
		}
	}
}

func (m *home) queueRecoveryAction(planFile, signalType, payload, successToast string) tea.Cmd {
	if planFile == "" || m.taskStoreProject == "" {
		return nil
	}
	project := m.taskStoreProject
	gw := m.signalGateway
	return func() tea.Msg {
		if gw == nil {
			var err error
			gw, err = taskstore.OpenAuthoritativeSignalGateway(project)
			if err != nil {
				return manualSignalResultMsg{err: err}
			}
			defer gw.Close() //nolint:errcheck
		}
		if err := taskfsm.EmitGatewaySignal(gw, project, signalType, planFile, payload); err != nil {
			return manualSignalResultMsg{err: err}
		}
		return manualSignalResultMsg{
			signalType:   signalType,
			planFile:     planFile,
			successToast: successToast,
		}
	}
}

// pushSelectedInstance pushes the selected instance's branch changes.
func (m *home) pushSelectedInstance() (tea.Model, tea.Cmd) {
	selected := m.nav.GetSelectedInstance()
	if selected == nil {
		return m, nil
	}
	capturedTitle := selected.Title
	capturedBranch := selected.Branch
	pushAction := func() tea.Msg {
		worktree, err := selected.GetGitWorktree()
		if err != nil {
			return err
		}
		commitMsg := "update from kas"
		if err := worktree.PushChanges(commitMsg, true); err != nil {
			return err
		}
		m.audit(auditlog.EventGitPush, fmt.Sprintf("pushed branch %s", capturedBranch),
			auditlog.WithInstance(capturedTitle),
		)
		return nil
	}
	message := "push changes from '" + selected.Title + "'?"
	return m, m.confirmAction(message, pushAction)
}

// triggerTaskStage handles a user action on a plan lifecycle stage row.
// It checks if the stage is locked, applies the concurrency gate for the
// implement stage, and then executes the stage transition.
func (m *home) triggerTaskStage(planFile, stage string) (tea.Model, tea.Cmd) {
	entry, ok := m.refreshTaskEntry(planFile)
	if !ok {
		return m, m.handleError(fmt.Errorf("missing task state for %s", planFile))
	}

	// Backfill branch name for plans created before the branch field existed.
	if entry.Branch == "" {
		entry.Branch = gitpkg.TaskBranchFromFile(planFile)
		if err := m.taskState.SetBranch(planFile, entry.Branch); err != nil {
			return m, m.handleError(fmt.Errorf("failed to assign branch for plan: %w", err))
		}
	}

	// Check if stage is locked
	if isLocked(entry.Status, stage) {
		prev := map[string]string{
			"implement": "plan",
			"review":    "implement",
			"finished":  "review",
		}[stage]
		m.toastManager.Error(fmt.Sprintf("complete '%s' first", prev))
		return m, m.toastTickCmd()
	}

	// Concurrency gate for coder stages
	if (stage == "implement" || stage == "solo") && entry.Topic != "" {
		if hasConflict, conflictPlan := m.taskState.HasRunningCoderInTopic(entry.Topic, planFile); hasConflict {
			conflictName := taskstate.DisplayName(conflictPlan)
			message := fmt.Sprintf("⚠ %s is already running in topic \"%s\"\n\nrunning both plans may cause issues.\ncontinue anyway?", conflictName, entry.Topic)
			proceedAction := func() tea.Msg {
				return taskStageConfirmedMsg{planFile: planFile, stage: stage}
			}
			return m, m.confirmAction(message, proceedAction)
		}
	}

	return m.executeTaskStage(planFile, stage)
}

// executeTaskStage runs the actual stage logic (agent spawn, wave orchestration)
// after all gates (lock check, concurrency) have passed. Called directly from
// triggerTaskStage on the normal path, and via taskStageConfirmedMsg when the
// user confirms past the topic-concurrency gate.
func (m *home) executeTaskStage(planFile, stage string) (tea.Model, tea.Cmd) {
	if m.taskState == nil {
		return m, m.handleError(fmt.Errorf("no task state loaded"))
	}
	switch stage {
	case "plan", "solo", "implement", "implement_direct", "review", "verify":
		if !m.requireDaemonForAgents() {
			return m, nil
		}
	}
	entry, ok := m.taskState.Plans[planFile]
	if !ok {
		return m, m.handleError(fmt.Errorf("missing task state for %s", planFile))
	}

	// Backfill branch name for plans created before the branch field existed.
	if entry.Branch == "" {
		entry.Branch = gitpkg.TaskBranchFromFile(planFile)
		if err := m.taskState.SetBranch(planFile, entry.Branch); err != nil {
			return m, m.handleError(fmt.Errorf("failed to assign branch for plan: %w", err))
		}
	}

	switch stage {
	case "plan":
		if err := m.fsm.Transition(planFile, taskfsm.PlanStart); err != nil {
			return m, m.handleError(err)
		}
		m.audit(auditlog.EventPlanTransition, string(entry.Status)+" → planning",
			auditlog.WithPlan(planFile))
		m.loadTaskState()
		m.updateSidebarTasks()
		return m.spawnPlannersForTask(planFile, buildPlanningPrompt(planFile, taskstate.DisplayName(planFile), entry.Description, m.taskStoreProject), entry.Description)
	case "solo":
		// Check store content before fsmSetImplementing — the FSM transition calls
		// store.Update which overwrites the content field with an empty string.
		// Reading before the transition preserves any ingested plan content.
		planName := taskstate.DisplayName(planFile)
		refFile := ""
		if m.taskStore != nil {
			if c, err := m.taskStore.GetContent(m.taskStoreProject, planFile); err == nil && c != "" {
				refFile = planFile
			}
		}
		if err := m.fsmSetImplementing(planFile); err != nil {
			return m, m.handleError(err)
		}
		m.audit(auditlog.EventPlanTransition, string(entry.Status)+" → implementing (solo)",
			auditlog.WithPlan(planFile))
		m.loadTaskState()
		m.updateSidebarTasks()
		prompt := buildSoloPrompt(planName, entry.Description, refFile, m.taskStoreProject)
		return m.spawnTaskAgent(planFile, "solo", prompt)
	case "implement":
		// If an orchestrator already exists (e.g. elaboration finished, or waves
		// are in progress), resume from where it left off instead of re-creating.
		if existingOrch, ok := m.waveOrchestrators[planFile]; ok {
			existingOrch.SetStore(m.taskStore, m.taskStoreProject)
			state := existingOrch.State()
			if state != orchestration.WaveStateElaborating {
				// Elaboration already done or waves running — start/resume next wave.
				mdl, cmd := m.startNextWave(existingOrch, entry)
				return mdl, cmd
			}
			// Still elaborating — don't re-spawn, just inform.
			m.toastManager.Info("architect pass still in progress — waiting to start the wave.")
			return m, nil
		}

		// Read and parse plan from the task store — this also validates wave headers.
		rawContent := ""
		if m.taskStore != nil {
			c, err := m.taskStore.GetContent(m.taskStoreProject, planFile)
			if err != nil {
				return m, m.handleError(err)
			}
			rawContent = c
		}
		if strings.TrimSpace(rawContent) == "" {
			if setErr := m.fsmRevertToPlanning(planFile); setErr != nil {
				return m, m.handleError(setErr)
			}
			m.loadTaskState()
			m.updateSidebarTasks()
			m.toastManager.Info("plan content missing — respawning planner to write plan content.")
			_, spawnCmd := m.spawnTaskAgent(planFile, "plan", buildPlanningPrompt(planFile, taskstate.DisplayName(planFile), entry.Description, m.taskStoreProject))
			return m, tea.Batch(m.toastTickCmd(), func() tea.Msg { return taskRefreshMsg{} }, spawnCmd)
		}
		plan, err := taskparser.Parse(rawContent)
		if err != nil {
			// No wave headers — revert to planning and respawn the planner with a
			// wave-annotation prompt so the agent adds the required ## Wave sections.
			if setErr := m.fsmRevertToPlanning(planFile); setErr != nil {
				return m, m.handleError(setErr)
			}
			m.loadTaskState()
			m.updateSidebarTasks()
			m.toastManager.Info("task needs ## Wave headers — respawning planner to annotate.")
			_, spawnCmd := m.spawnTaskAgent(planFile, "plan", orchestration.BuildWaveAnnotationPrompt(planFile, m.taskStoreProject))
			return m, tea.Batch(m.toastTickCmd(), func() tea.Msg { return taskRefreshMsg{} }, spawnCmd)
		}

		// Blueprint-skip: for small plans, bypass elaboration and wave orchestration.
		if orchestration.ShouldBlueprintSkip(plan, m.blueprintSkipThreshold()) {
			if m.hasActiveBlueprintSkipCoder(planFile) {
				m.toastManager.Info("implementation already running — waiting for single agent to finish.")
				return m, m.toastTickCmd()
			}
			return m.spawnBlueprintSkipAgent(planFile, plan)
		}

		persistedPhase := taskfsm.NormalizeExecutionPhase(entry.ExecutionState.Phase)
		if entry.Status == taskstate.StatusImplementing {
			switch persistedPhase {
			case taskfsm.ExecutionPhaseArchitecting:
				orch := orchestration.NewWaveOrchestrator(planFile, plan)
				orch.SetStore(m.taskStore, m.taskStoreProject)
				orch.SetElaborating()
				m.waveOrchestrators[planFile] = orch
				m.toastManager.Info("architect pass still in progress — waiting to start the wave.")
				return m, m.toastTickCmd()
			case taskfsm.ExecutionPhaseWaveRunning, taskfsm.ExecutionPhaseWaveWaiting:
				m.rebuildOrphanedOrchestrators()
				if existingOrch, ok := m.waveOrchestrators[planFile]; ok {
					return m.startNextWave(existingOrch, entry)
				}
				m.toastManager.Info("wave execution already in progress — waiting for active tasks.")
				return m, m.toastTickCmd()
			}
		}

		orch := orchestration.NewWaveOrchestrator(planFile, plan)
		orch.SetStore(m.taskStore, m.taskStoreProject)
		m.waveOrchestrators[planFile] = orch

		if err := m.fsmSetImplementing(planFile); err != nil {
			return m, m.handleError(err)
		}
		m.audit(auditlog.EventPlanTransition, string(entry.Status)+" → implementing",
			auditlog.WithPlan(planFile))
		m.loadTaskState()
		m.updateSidebarTasks()

		// Elaboration phase: spawn architect before starting wave 1.
		orch.SetElaborating()
		return m.spawnElaborator(planFile)

	case "implement_direct":
		// Same as implement but skips the elaboration phase — goes straight to wave 1.
		rawContent := ""
		if m.taskStore != nil {
			c, err := m.taskStore.GetContent(m.taskStoreProject, planFile)
			if err != nil {
				return m, m.handleError(err)
			}
			rawContent = c
		}
		if strings.TrimSpace(rawContent) == "" {
			if setErr := m.fsmRevertToPlanning(planFile); setErr != nil {
				return m, m.handleError(setErr)
			}
			m.loadTaskState()
			m.updateSidebarTasks()
			m.toastManager.Info("plan content missing — respawning planner to write plan content.")
			_, spawnCmd := m.spawnTaskAgent(planFile, "plan", buildPlanningPrompt(planFile, taskstate.DisplayName(planFile), entry.Description, m.taskStoreProject))
			return m, tea.Batch(m.toastTickCmd(), func() tea.Msg { return taskRefreshMsg{} }, spawnCmd)
		}
		plan, err := taskparser.Parse(rawContent)
		if err != nil {
			// No wave headers — revert to planning and respawn the planner.
			if setErr := m.fsmRevertToPlanning(planFile); setErr != nil {
				return m, m.handleError(setErr)
			}
			m.loadTaskState()
			m.updateSidebarTasks()
			m.toastManager.Info("task needs ## Wave headers — respawning planner to annotate.")
			_, spawnCmd := m.spawnTaskAgent(planFile, "plan", orchestration.BuildWaveAnnotationPrompt(planFile, m.taskStoreProject))
			return m, tea.Batch(m.toastTickCmd(), func() tea.Msg { return taskRefreshMsg{} }, spawnCmd)
		}

		// Blueprint-skip: for small plans, bypass elaboration and wave orchestration.
		if orchestration.ShouldBlueprintSkip(plan, m.blueprintSkipThreshold()) {
			m.clearWaveOrchestratorState(planFile)
			return m.spawnBlueprintSkipAgent(planFile, plan)
		}

		orch := orchestration.NewWaveOrchestrator(planFile, plan)
		orch.SetStore(m.taskStore, m.taskStoreProject)
		m.waveOrchestrators[planFile] = orch

		if err := m.fsmSetImplementing(planFile); err != nil {
			return m, m.handleError(err)
		}
		m.audit(auditlog.EventPlanTransition, string(entry.Status)+" → implementing (direct)",
			auditlog.WithPlan(planFile))
		m.loadTaskState()
		m.updateSidebarTasks()
		return m.startNextWave(orch, entry)
	case "review":
		if err := m.fsmSetReviewing(planFile); err != nil {
			return m, m.handleError(err)
		}
		m.audit(auditlog.EventPlanTransition, string(entry.Status)+" → reviewing",
			auditlog.WithPlan(planFile))
		m.loadTaskState()
		m.updateSidebarTasks()
		return m, m.spawnReviewer(planFile)
	case "verify":
		// Preempt any live coder/fixer for this plan before spawning the master.
		// spawnMaster only cleans up prior master/reviewer instances, and
		// fixer/coder sessions share the same task worktree — leaving them
		// running alongside the readiness pass would race on the working tree.
		m.killExistingPlanAgent(planFile, session.AgentTypeCoder)
		m.killExistingPlanAgent(planFile, session.AgentTypeFixer)
		m.clearWaveOrchestratorState(planFile)
		if entry.Status != taskstate.StatusReviewing && entry.Status != taskstate.StatusVerifying {
			if err := m.fsmSetReviewing(planFile); err != nil {
				return m, m.handleError(err)
			}
		}
		if entry.Status != taskstate.StatusVerifying {
			if err := m.fsm.Transition(planFile, taskfsm.ReviewApproved); err != nil {
				return m, m.handleError(err)
			}
			m.audit(auditlog.EventPlanTransition,
				string(entry.Status)+" → verifying (manual start verify)",
				auditlog.WithPlan(planFile))
		}
		m.clearLatestReviewFeedback(planFile)
		m.loadTaskState()
		m.updateSidebarTasks()
		return m, m.spawnMaster(planFile)
	}

	// Non-agent stages (finished): mark plan done via FSM.
	// Skip ReviewApproved if already in verifying — it has been applied already.
	if taskfsm.Status(entry.Status) != taskfsm.StatusVerifying {
		if err := m.fsm.Transition(planFile, taskfsm.ReviewApproved); err != nil {
			return m, m.handleError(err)
		}
	}
	if err := m.fsm.Transition(planFile, taskfsm.VerifyApproved); err != nil {
		return m, m.handleError(err)
	}
	if err := m.clearExecutionState(planFile); err != nil {
		return m, m.handleError(err)
	}
	m.audit(auditlog.EventPlanTransition, string(entry.Status)+" → done",
		auditlog.WithPlan(planFile))
	m.loadTaskState()
	m.updateSidebarTasks()
	return m, nil
}

// validatePlanContent checks if plan content has ## Wave headers.
// Returns an error if the plan lacks wave annotations or content is empty.
func validatePlanContent(content string) error {
	_, err := taskparser.Parse(content)
	return err
}

// handleTmuxBrowserAction dispatches actions from the tmux session browser overlay.
// browser is the TmuxBrowserOverlay captured BEFORE HandleKey was called (so SelectedItem is valid).
// action is the Result.Action string returned by HandleKey.
func (m *home) handleTmuxBrowserAction(browser *overlay.TmuxBrowserOverlay, action string) (tea.Model, tea.Cmd) {
	switch action {
	case "": // dismissed without action
		m.overlays.Dismiss()
		m.state = stateDefault
		return m, nil

	case "kill":
		if browser == nil {
			return m, nil
		}
		item := browser.SelectedItem()
		if item.Name == "" {
			return m, nil
		}
		name := item.Name
		browser.RemoveSelected()
		if browser.IsEmpty() {
			m.overlays.Dismiss()
			m.state = stateDefault
		}
		return m, func() tea.Msg {
			killCmd := exec.Command("tmux", "kill-session", "-t", name)
			err := killCmd.Run()
			return tmuxKillResultMsg{name: name, err: err}
		}

	case "adopt":
		if browser == nil {
			return m, nil
		}
		item := browser.SelectedItem()
		if item.Name == "" {
			return m, nil
		}
		m.state = stateDefault
		return m.adoptOrphanSession(overlay.TmuxBrowserItem{
			Name:  item.Name,
			Title: item.Title,
		})

	case "attach":
		if browser == nil {
			return m, nil
		}
		item := browser.SelectedItem()
		if item.Name == "" {
			return m, nil
		}
		m.state = stateDefault
		name := item.Name
		return m, tea.ExecProcess(exec.Command("tmux", "attach-session", "-t", name), func(err error) tea.Msg {
			return tmuxAttachReturnMsg{}
		})

	default:
		return m, nil
	}
}

// isLocked returns true if the given stage cannot be triggered given the current plan status.
// The context menu already gates which forward stages are offered, so this only
// guards against truly nonsensical transitions (e.g. marking "finished" when already done).
func isLocked(status taskstate.Status, stage string) bool {
	switch stage {
	case "plan", "implement", "implement_direct", "solo", "review", "verify":
		// Forward progression is always allowed — the FSM helpers
		// (fsmSetImplementing, fsmSetReviewing) walk through intermediate states.
		return false
	case "finished":
		return status != taskstate.StatusReviewing && status != taskstate.StatusVerifying
	default:
		return true
	}
}

// globalLauncherItems returns the always-available command launcher items that
// do not depend on the current selection (task or instance).
func globalLauncherItems() []overlay.LauncherItem {
	return []overlay.LauncherItem{
		{Label: "view keybinds", Hint: "?", Action: "view_keybinds"},
		{Label: "new plan", Hint: "n", Action: "new_plan"},
		{Label: "new instance", Hint: "N", Action: "new_instance"},
		{Label: "spawn agent", Hint: "S", Action: "spawn_agent"},
		{Label: "quick launch", Hint: "s", Action: "quick_launch"},
		{Label: "search", Hint: "/", Action: "search"},
		{Label: "tmux sessions", Hint: "t", Action: "tmux_browser"},
		{Label: "toggle sidebar", Hint: "ctrl+s", Action: "toggle_sidebar"},
		{Label: "toggle audit log", Hint: "L", Action: "toggle_audit"},
		{Label: "audit log actions", Hint: "A", Action: "audit_cursor"},
		{Label: "toggle info header", Hint: "I", Action: "info_tab"},
		{Label: "quit", Hint: "q", Action: "quit"},
	}
}

// taskLauncherItems builds launcher items for the currently selected task.
// It reuses taskLifecycleItems for lifecycle actions, then appends task-scoped
// management actions. planFile is used for reference only (not dispatched here).
func (m *home) taskLauncherItems(planFile string, entry taskstate.TaskEntry) []overlay.LauncherItem {
	var items []overlay.LauncherItem

	// Lifecycle actions from the shared builder (ordered by likelihood).
	for _, cm := range taskLifecycleItems(entry) {
		items = append(items, overlay.LauncherItem{Label: cm.Label, Action: cm.Action})
	}

	// Task-scoped management and sync actions.
	items = append(items,
		overlay.LauncherItem{Label: "view task", Action: "view_plan"},
		overlay.LauncherItem{Label: "chat about this", Action: "chat_about_plan"},
		overlay.LauncherItem{Label: "create pr", Action: "create_plan_pr"},
		overlay.LauncherItem{Label: "merge to main", Action: "merge_plan"},
		overlay.LauncherItem{Label: "open in browser", Action: "open_plan_browser"},
		overlay.LauncherItem{Label: "rename task", Action: "rename_plan"},
		overlay.LauncherItem{Label: "set topic", Action: "change_topic"},
		overlay.LauncherItem{Label: "cancel task", Action: "cancel_plan"},
	)
	return items
}

// instanceLauncherItems builds launcher items for the given instance.
// Context-sensitive items (attach controls, signals, send_yes) are prepended
// so the most actionable items appear first in the list.
func (m *home) instanceLauncherItems(inst *session.Instance) []overlay.LauncherItem {
	var items []overlay.LauncherItem

	// Attachment control: open when running with an attachable tmux session, resume when paused.
	if inst.Paused() {
		items = append(items, overlay.LauncherItem{Label: "resume", Action: "resume_instance"})
	} else if inst.Started() && !inst.Paused() && inst.TmuxAlive() {
		items = append(items, overlay.LauncherItem{Label: "open", Action: "open_instance"})
	}

	items = append(items,
		overlay.LauncherItem{Label: "kill", Action: "kill_instance"},
		overlay.LauncherItem{Label: "restart", Action: "restart_instance"},
	)

	if inst.Started() && !inst.Paused() {
		items = append(items, overlay.LauncherItem{Label: "focus agent", Action: "send_prompt_instance"})
	}

	items = append(items,
		overlay.LauncherItem{Label: "push branch", Action: "push_instance"},
		overlay.LauncherItem{Label: "create pr", Action: "create_pr_instance"},
	)

	if inst.TaskFile != "" {
		items = append(items, overlay.LauncherItem{Label: "merge to main", Action: "merge_instance"})
	}

	// Task-owner lifecycle signal actions (from the shared builder).
	// Refresh the task entry so that the launcher offers the same gated actions
	// as the context menu rather than stale cached state.
	var sigEntry taskstate.TaskEntry
	var sigHasEntry bool
	if inst.TaskFile != "" && inst.TaskNumber == 0 {
		sigEntry, sigHasEntry = m.refreshTaskEntry(inst.TaskFile)
	}
	for _, cm := range instanceSignalItems(inst, sigEntry, sigHasEntry) {
		switch cm.Action {
		// Skip items already emitted above to avoid duplicates.
		case "resume_instance", "open_instance", "kill_instance", "restart_instance":
			continue
		}
		items = append(items, overlay.LauncherItem{Label: cm.Label, Action: cm.Action})
	}

	// send_yes is launcher-only: emit only when the agent is waiting for input.
	if inst.Started() && !inst.Paused() && inst.PromptDetected {
		items = append(items, overlay.LauncherItem{Label: "send yes", Hint: "y", Action: "send_yes"})
	}

	return items
}

// buildLauncherItems constructs the full launcher item list: context-sensitive
// items for the current selection (task or instance) are prepended, then the
// always-available global items are appended.
func (m *home) buildLauncherItems() []overlay.LauncherItem {
	var items []overlay.LauncherItem

	if inst := m.nav.GetSelectedInstance(); inst != nil {
		items = append(items, m.instanceLauncherItems(inst)...)
	} else if planFile := m.nav.GetSelectedPlanFile(); planFile != "" {
		if m.taskState != nil {
			if entry, ok := m.taskState.Entry(planFile); ok {
				items = append(items, m.taskLauncherItems(planFile, entry)...)
			}
		}
	}

	items = append(items, globalLauncherItems()...)
	return items
}

// openCommandLauncher builds and shows the global command launcher overlay.
func (m *home) openCommandLauncher() (tea.Model, tea.Cmd) {
	launcher := overlay.NewCommandLauncherOverlay("commands", m.buildLauncherItems())
	m.overlays.Show(launcher)
	m.state = stateLauncher
	return m, nil
}

// openKeybindBrowser builds and shows a read-only keybind browser overlay
// using all configured keybinds from the keys package.
func (m *home) openKeybindBrowser() (tea.Model, tea.Cmd) {
	items := buildKeybindBrowserItems()
	browser := overlay.NewCommandLauncherOverlay("keybinds", items)
	m.overlays.Show(browser)
	m.state = stateKeybindBrowser
	return m, nil
}

// buildKeybindBrowserItems creates a sorted list of all global keybinds for
// the keybind browser. Uses keys.GlobalkeyBindings to get label and key text.
func buildKeybindBrowserItems() []overlay.LauncherItem {
	var items []overlay.LauncherItem
	for name, binding := range keys.GlobalkeyBindings {
		if name == keys.KeySubmitName {
			continue
		}
		help := binding.Help()
		if help.Key == "" || help.Desc == "" {
			continue
		}
		items = append(items, overlay.LauncherItem{
			Label: help.Desc,
			Hint:  help.Key,
		})
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].Label < items[j].Label
	})
	return items
}

// executeLauncherAction dispatches a command launcher action. Global-only actions
// are handled explicitly here; all other actions (task/instance lifecycle, signals)
// fall through to executeContextAction which owns the shared dispatch logic.
func (m *home) executeLauncherAction(action string) (tea.Model, tea.Cmd) {
	switch action {
	case "view_keybinds":
		return m.openKeybindBrowser()
	case "new_plan":
		m.state = stateNewPlan
		tio := overlay.NewTextInputOverlay("new plan", "")
		tio.SetMultiline(true)
		tio.SetPlaceholder("describe what you want to work on...")
		tio.SetSize(70, 8)
		m.overlays.Show(tio)
		return m, nil
	case "new_instance":
		requestedMode := m.standaloneExecutionMode(session.AgentTypeMaster, "claude")
		instance, err := m.newNamedAgentInstance("", m.activeRepoPath, "claude", requestedMode, "")
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
	case "spawn_agent":
		return m.beginSpawnAgentFlow()
	case "quick_launch":
		return m.quickLaunchAgent()
	case "search":
		m.nav.ActivateSearch()
		m.nav.SelectFirst()
		m.state = stateSearch
		m.setFocusSlot(slotNav)
		return m, nil
	case "tmux_browser":
		return m, m.discoverTmuxSessions()
	case "toggle_sidebar":
		if m.sidebarHidden {
			m.sidebarHidden = false
		} else {
			m.sidebarHidden = true
			if m.focusSlot == slotNav {
				m.setFocusSlot(slotAgent)
			}
		}
		return m, tea.RequestWindowSize
	case "toggle_audit":
		if m.auditPane != nil {
			m.auditPane.ToggleVisible()
		}
		return m, tea.RequestWindowSize
	case "audit_cursor":
		return m.enterAuditCursorMode()
	case "info_tab":
		m.tabbedWindow.SetShowInfo(!m.tabbedWindow.IsShowingInfo())
		return m, tea.RequestWindowSize
	case "quit":
		return m.handleQuit()

	// send_yes is launcher-only: queue a "yes" answer to the current agent prompt.
	// The selection and state guards match those in instanceLauncherItems.
	case "send_yes":
		selected := m.nav.GetSelectedInstance()
		if selected == nil || !selected.Started() || selected.Paused() || !selected.PromptDetected {
			return m, nil
		}
		selected.QueuedPrompt = "yes"
		selected.AwaitingWork = true
		return m, nil
	}

	// All remaining actions (task/instance lifecycle, signals, sync operations)
	// are handled by the shared context-action dispatcher so the logic is not
	// duplicated between the launcher and the context menu.
	return m.executeContextAction(action)
}
