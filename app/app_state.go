package app

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/glamour"
	cmd2 "github.com/kastheco/kasmos/cmd"
	"github.com/kastheco/kasmos/config"
	"github.com/kastheco/kasmos/config/auditlog"
	"github.com/kastheco/kasmos/config/taskfsm"
	"github.com/kastheco/kasmos/config/taskparser"
	"github.com/kastheco/kasmos/config/taskstate"
	"github.com/kastheco/kasmos/config/taskstore"
	daemonpkg "github.com/kastheco/kasmos/daemon"
	"github.com/kastheco/kasmos/daemon/api"
	"github.com/kastheco/kasmos/internal/clickup"
	"github.com/kastheco/kasmos/internal/initcmd/harness"
	"github.com/kastheco/kasmos/internal/initcmd/scaffold"
	"github.com/kastheco/kasmos/internal/opencodesession"
	"github.com/kastheco/kasmos/log"
	"github.com/kastheco/kasmos/orchestration"
	"github.com/kastheco/kasmos/orchestration/loop"
	"github.com/kastheco/kasmos/session"
	"github.com/kastheco/kasmos/session/common"
	gitpkg "github.com/kastheco/kasmos/session/git"
	"github.com/kastheco/kasmos/session/sdk"
	"github.com/kastheco/kasmos/session/tmux"
	"github.com/kastheco/kasmos/ui"
	"github.com/kastheco/kasmos/ui/overlay"

	tea "charm.land/bubbletea/v2"
)

var restoreInstanceFromData = session.FromInstanceData

var (
	plannerInstancePollInterval      = 50 * time.Millisecond
	plannerInstanceWaitTimeout       = 5 * time.Second
	quickLaunchTitleSyncInitialDelay = 500 * time.Millisecond
	quickLaunchTitleSyncMaxDelay     = 5 * time.Second
	quickLaunchTitleSyncTimeout      = 30 * time.Second
	quickLaunchTitleSyncMultiplier   = 1.2
)

func waitForDaemonPlannerInstance(project string, data session.InstanceData) (*session.Instance, error) {
	var lastErr error
	deadline := time.Now().Add(plannerInstanceWaitTimeout)
	for time.Now().Before(deadline) {
		if project != "" {
			statuses, err := listDaemonInstances(project)
			if err == nil {
				for _, status := range statuses {
					if status.Title != data.Title || !status.Active {
						continue
					}
					inst, restoreErr := restoreDaemonInstance(data.Path, status)
					if restoreErr == nil && inst != nil {
						return inst, nil
					}
					lastErr = restoreErr
					break
				}
			} else {
				lastErr = err
			}
		}

		inst, err := restoreInstanceFromData(data)
		if err == nil {
			if inst != nil && !inst.Exited {
				return inst, nil
			}
			lastErr = fmt.Errorf("planner session not live yet")
		} else {
			lastErr = err
		}
		time.Sleep(plannerInstancePollInterval)
	}
	if lastErr != nil {
		return nil, lastErr
	}
	return nil, fmt.Errorf("planner session did not appear")
}

// spawnSoloWithDaemon is a seam for tests. It POSTs to the daemon's
// /instances/solo endpoint and waits until the title appears in daemon
// ListInstances before returning. Returns a non-nil error on timeout so the
// instanceStartedMsg error path removes the local placeholder.
var spawnSoloWithDaemon = func(project string, req api.SpawnSoloRequest) error {
	client := daemonpkg.NewSocketClient(taskstore.ResolvedDaemonSocketPath())
	if err := client.SpawnSolo(project, req); err != nil {
		return err
	}
	return waitForDaemonTitle(project, req.Title)
}

// waitForDaemonTitle polls ListInstances until the given title appears as an
// active entry (loading counts as active). Reuses plannerInstancePollInterval
// and plannerInstanceWaitTimeout so solo and planner waits share the same knob.
func waitForDaemonTitle(project, title string) error {
	deadline := time.Now().Add(plannerInstanceWaitTimeout)
	for time.Now().Before(deadline) {
		statuses, err := listDaemonInstances(project)
		if err == nil {
			for _, s := range statuses {
				if s.Title == title && s.Active {
					return nil
				}
			}
		}
		time.Sleep(plannerInstancePollInterval)
	}
	return fmt.Errorf("daemon did not register %q within timeout", title)
}

var spawnPlannerWithDaemon = func(repoPath, project, planFile, title, prompt, program string) (*session.Instance, error) {
	client := daemonpkg.NewSocketClient(taskstore.ResolvedDaemonSocketPath())
	if err := client.StartPlan(project, planFile, prompt, program); err != nil {
		return nil, err
	}

	data := session.InstanceData{
		Title:         title,
		Path:          repoPath,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
		Program:       program,
		ExecutionMode: session.ExecutionModeTmux,
		TaskFile:      planFile,
		AgentType:     session.AgentTypePlanner,
		Status:        session.Loading,
	}

	return waitForDaemonPlannerInstance(project, data)
}

var quickLaunchStartOnMain = func(inst *session.Instance) error {
	return inst.StartOnMainBranch()
}

var readQuickLaunchSessionTitle = opencodesession.ReadSessionTitle

var quickLaunchPlaceholderTitleRE = regexp.MustCompile(`^(?:.+-)?agent-(\d+)$`)
var quickLaunchDisplayTitleRE = regexp.MustCompile(`[^a-z0-9]+`)

type daemonPlannerStartedMsg struct {
	instance *session.Instance
	err      error
}

type instanceTitleSyncMsg struct {
	instance *session.Instance
	newTitle string
}

// shouldCreatePR returns true when a plan entry is eligible for automatic PR creation:
// the plan is done, has a branch, and does not already have a PR URL.
func shouldCreatePR(entry taskstore.TaskEntry) bool {
	return entry.Status == taskstore.StatusDone && entry.Branch != "" && entry.PRURL == ""
}

// instancePresentation categorises an instance for default-view rendering.
// It is a pure derivation over session.Instance and taskstore.TaskEntry;
// callers pass whatever subset they already have in-context. Returning a
// compact enum keeps renderers decoupled from the FSM.
type instancePresentation int

const (
	presentationActive  instancePresentation = iota // running, loading, ready, blocked
	presentationRetired                             // task Done or Cancelled, or instance Exited
	presentationIdle                                // paused with nothing pending
)

func deriveInstancePresentation(inst *session.Instance, entry taskstore.TaskEntry, hasEntry bool) instancePresentation {
	if inst != nil && inst.Exited {
		return presentationRetired
	}
	if hasEntry {
		switch entry.Status {
		case taskstore.StatusDone, taskstore.StatusCancelled:
			return presentationRetired
		}
	}
	if inst != nil && inst.Status == session.Paused {
		if !hasEntry || (entry.Status != taskstore.StatusReviewing && entry.Status != taskstore.StatusImplementing && entry.Status != taskstore.StatusVerifying) {
			return presentationIdle
		}
	}
	return presentationActive
}

// toTaskFSMHooks converts a slice of config.TOMLHook to taskfsm.HookConfig entries.
func toTaskFSMHooks(entries []config.TOMLHook) []taskfsm.HookConfig {
	out := make([]taskfsm.HookConfig, len(entries))
	for i, h := range entries {
		out[i] = taskfsm.HookConfig{
			Type:    h.Type,
			URL:     h.URL,
			Headers: h.Headers,
			Command: h.Command,
			Events:  h.Events,
		}
	}
	return out
}

// ensureProcessor lazily initializes and returns the signal Processor.
// Returns nil when taskStore is not set (for example in narrow tests that do
// not exercise signal processing), in which case the caller uses the inline
// fallback path in app.Update.
func (m *home) ensureProcessor() *loop.Processor {
	autoReviewFix := false
	maxCycles := 0
	autoReadinessReview := false
	if m.appConfig != nil {
		autoReviewFix = m.appConfig.AutoReviewFix
		maxCycles = m.appConfig.MaxReviewFixCycles
		autoReadinessReview = m.appConfig.AutoReadinessReview
	}
	if m.processor != nil {
		m.processor.SetReviewFixConfig(autoReviewFix, maxCycles)
		m.processor.SetReadinessReviewConfig(autoReadinessReview)
		return m.processor
	}
	if m.taskStore == nil {
		return nil
	}
	var hooks *taskfsm.HookRegistry
	if m.appConfig != nil {
		if len(m.appConfig.Hooks) > 0 {
			hooks = taskfsm.BuildHookRegistry(toTaskFSMHooks(m.appConfig.Hooks))
		}
	}
	m.processor = loop.NewProcessor(loop.ProcessorConfig{
		AutoReviewFix:       autoReviewFix,
		AutoReadinessReview: autoReadinessReview,
		Store:               m.taskStore,
		Project:             m.taskStoreProject,
		Dir:                 m.taskStateDir,
		MaxReviewFixCycles:  maxCycles,
		Hooks:               hooks,
	})
	return m.processor
}

func (m *home) handleReviewChangesRequested(planFile, feedback string) tea.Cmd {
	m.pendingReviewFeedback[planFile] = feedback
	if m.taskState != nil {
		if err := m.taskState.SetLatestReviewFeedback(planFile, feedback); err != nil {
			log.WarningLog.Printf("could not persist latest review feedback for %q: %v", planFile, err)
		}
	}

	var cmds []tea.Cmd
	truncated := feedback
	if len(truncated) > 200 {
		truncated = truncated[:200] + "..."
	}
	if cmd := m.postClickUpProgress(planFile, "review_changes_requested", truncated); cmd != nil {
		cmds = append(cmds, cmd)
	}
	for _, inst := range m.nav.GetInstances() {
		if inst.TaskFile == planFile && isReviewerInstance(inst) {
			_ = inst.Pause()
			break
		}
	}
	for _, inst := range m.nav.GetInstances() {
		if inst.TaskFile == planFile && inst.AgentType == session.AgentTypeMaster {
			_ = inst.Pause()
			break
		}
	}
	if len(cmds) == 0 {
		return nil
	}
	return tea.Batch(cmds...)
}

func (m *home) reviewRound(planFile string) int {
	if m.taskState != nil {
		if cycle, err := m.taskState.ReviewCycle(planFile); err == nil && cycle >= 0 {
			return cycle + 1
		}
	}
	return 1
}

func (m *home) fixerRound(planFile string) int {
	if m.taskState != nil {
		if cycle, err := m.taskState.ReviewCycle(planFile); err == nil && cycle > 0 {
			return cycle
		}
	}
	return 1
}

func (m *home) latestReviewFeedback(planFile string) string {
	if feedback := strings.TrimSpace(m.pendingReviewFeedback[planFile]); feedback != "" {
		return feedback
	}
	if m.taskState != nil {
		if entry, ok := m.taskState.Entry(planFile); ok {
			return strings.TrimSpace(entry.LatestReviewFeedback)
		}
	}
	return ""
}

func (m *home) recoveryCandidatesForTask(filename string, entry taskstate.TaskEntry) []orchestration.RecoveryCandidate {
	storeEntry := taskstore.TaskEntry{
		Filename:             filename,
		Status:               taskstore.Status(entry.Status),
		Branch:               entry.Branch,
		ReviewCycle:          entry.ReviewCycle,
		LatestReviewFeedback: entry.LatestReviewFeedback,
		ExecutionState:       entry.ExecutionState,
	}

	content := ""
	phase := taskfsm.NormalizeExecutionPhase(entry.ExecutionState.Phase)
	if taskfsm.IsWaveExecutionPhase(phase) && m.taskStore != nil {
		if stored, err := m.taskStore.GetContent(m.taskStoreProject, filename); err == nil {
			content = stored
		}
	}

	return orchestration.BuildRecoveryCandidates(storeEntry, content)
}

func (m *home) recoveryCandidateForTitle(filename string, entry taskstate.TaskEntry, title string) (orchestration.RecoveryCandidate, bool) {
	storeEntry := taskstore.TaskEntry{
		Filename:             filename,
		Status:               taskstore.Status(entry.Status),
		Branch:               entry.Branch,
		ReviewCycle:          entry.ReviewCycle,
		LatestReviewFeedback: entry.LatestReviewFeedback,
		ExecutionState:       entry.ExecutionState,
	}

	content := ""
	phase := taskfsm.NormalizeExecutionPhase(entry.ExecutionState.Phase)
	if taskfsm.IsWaveExecutionPhase(phase) && m.taskStore != nil {
		if stored, err := m.taskStore.GetContent(m.taskStoreProject, filename); err == nil {
			content = stored
		}
	}

	return orchestration.MatchRecoveryCandidateByTitle(storeEntry, content, title)
}

func (m *home) clearLatestReviewFeedback(planFile string) {
	delete(m.pendingReviewFeedback, planFile)
	if m.taskState != nil {
		if err := m.taskState.ClearLatestReviewFeedback(planFile); err != nil {
			log.WarningLog.Printf("could not clear latest review feedback for %q: %v", planFile, err)
		}
	}
}

func (m *home) reviewFeedbackPayload(inst *session.Instance) string {
	if inst == nil {
		return ""
	}
	if text := strings.TrimSpace(inst.CachedContent); text != "" {
		return text
	}
	return strings.TrimSpace(m.latestReviewFeedback(inst.TaskFile))
}

func (m *home) captureSelectedReviewFeedback(inst *session.Instance) error {
	if inst == nil || inst.TaskFile == "" || m.taskState == nil {
		return nil
	}
	feedback := m.reviewFeedbackPayload(inst)
	if feedback == "" {
		return nil
	}
	if err := m.taskState.SetLatestReviewFeedback(inst.TaskFile, feedback); err != nil {
		return fmt.Errorf("persist review feedback: %w", err)
	}
	m.pendingReviewFeedback[inst.TaskFile] = feedback
	return nil
}

// mapPRReviewDecision maps GitHub review decision strings to internal representation.
func mapPRReviewDecision(ghValue string) string {
	switch ghValue {
	case "APPROVED":
		return "approved"
	case "CHANGES_REQUESTED":
		return "changes_requested"
	default:
		return "pending"
	}
}

// mapPRCheckStatus maps GitHub check status strings to internal representation.
func mapPRCheckStatus(ghValue string) string {
	switch ghValue {
	case "SUCCESS":
		return "passing"
	case "FAILURE", "ERROR":
		return "failing"
	default:
		return "pending"
	}
}

// createPRAfterApproval returns an async tea.Cmd that creates a GitHub PR for the given
// plan file, posts an approving review with the reviewer's body, and reports the URL back.
func (m *home) createPRAfterApproval(planFile, reviewBody string) tea.Cmd {
	repoPath := m.activeRepoPath
	store := m.taskStore
	project := m.taskStoreProject
	planName := taskstate.DisplayName(planFile)

	return func() tea.Msg {
		entry, err := store.Get(project, planFile)
		if err != nil {
			log.WarningLog.Printf("createPRAfterApproval: could not get entry for %q: %v", planFile, err)
			return nil
		}
		if entry.Branch == "" {
			log.WarningLog.Printf("createPRAfterApproval: no branch for %q — skipping PR creation", planFile)
			return nil
		}

		shared := gitpkg.NewSharedTaskWorktree(repoPath, entry.Branch)
		if err := shared.Setup(); err != nil {
			log.WarningLog.Printf("createPRAfterApproval: worktree setup failed for %q: %v", planFile, err)
			return nil
		}

		subtasks := []taskstore.SubtaskEntry(nil)
		if subtasksFromStore, err := store.GetSubtasks(project, planFile); err == nil {
			subtasks = subtasksFromStore
		} else {
			log.WarningLog.Printf("createPRAfterApproval: failed to load subtasks for %q: %v", planFile, err)
		}

		base := shared.GetBaseCommitSHA()
		gitChanges, gitCommits, gitStats := "", "", ""
		if base != "" {
			if files, err := exec.Command("git", "-C", shared.GetWorktreePath(), "diff", "--name-only", base).CombinedOutput(); err == nil {
				gitChanges = strings.TrimSpace(string(files))
			}
			if commits, err := exec.Command("git", "-C", shared.GetWorktreePath(), "log", "--oneline", base+"..HEAD").CombinedOutput(); err == nil {
				gitCommits = strings.TrimSpace(string(commits))
			}
			if stats, err := exec.Command("git", "-C", shared.GetWorktreePath(), "diff", "--stat", base).CombinedOutput(); err == nil {
				gitStats = strings.TrimSpace(string(stats))
			}
		}

		meta := gitpkg.AssemblePRMetadata(entry, subtasks, reviewBody, entry.ReviewCycle, gitChanges, gitCommits, gitStats)
		title := gitpkg.BuildPRTitle(entry.Description, planName)
		body := gitpkg.BuildPRBody(meta)
		commitMsg := fmt.Sprintf("[kas] implementation of '%s'", planName)
		if err := shared.CreatePR(title, body, commitMsg); err != nil {
			log.WarningLog.Printf("createPRAfterApproval: PR creation failed for %q: %v", planFile, err)
			return nil
		}

		state, err := shared.QueryPRState()
		if err != nil {
			log.WarningLog.Printf("createPRAfterApproval: QueryPRState failed for %q: %v", planFile, err)
			return nil
		}
		if state.URL == "" {
			log.WarningLog.Printf("createPRAfterApproval: empty URL for %q after PR creation", planFile)
			return nil
		}

		if state.Number > 0 {
			if err := shared.PostGitHubReview(state.Number, body, true); err != nil {
				log.WarningLog.Printf("createPRAfterApproval: PostGitHubReview failed for %q: %v", planFile, err)
				// Non-fatal — PR was created, review posting failed.
			}
		}

		return prCreatedForPlanMsg{planFile: planFile, url: state.URL}
	}
}

func mergeTopicStatus(status ui.TopicStatus, inst *session.Instance, started bool) ui.TopicStatus {
	if started && !inst.Paused() && !inst.PromptDetected {
		status.HasRunning = true
	}
	if inst.Notified {
		status.HasNotification = true
	}
	return status
}

func mergePlanStatus(status ui.TopicStatus, inst *session.Instance, started bool) ui.TopicStatus {
	if inst.TaskFile == "" {
		return status
	}
	if started && !inst.Paused() {
		if isReviewerInstance(inst) {
			status.HasNotification = true
		} else if (inst.Status == session.Running || inst.Status == session.Loading) && !inst.PromptDetected {
			status.HasRunning = true
		}
	}
	if inst.Notified && isReviewerInstance(inst) {
		status.HasNotification = true
	}
	return status
}

func isReviewerInstance(inst *session.Instance) bool {
	return inst != nil && (inst.AgentType == session.AgentTypeReviewer || inst.IsReviewer)
}

// computeStatusBarData builds the StatusBarData from the current app state.
func (m *home) computeStatusBarData() ui.StatusBarData {
	data := ui.StatusBarData{
		FocusMode:        m.state == stateFocusAgent,
		Version:          m.version,
		TmuxSessionCount: m.tmuxSessionCount,
		ProjectDir:       filepath.Base(m.activeRepoPath),
	}

	if m.nav == nil {
		if data.Branch == "" {
			data.Branch = currentBranch(m.activeRepoPath)
		}
		return data
	}

	planFile := m.nav.GetSelectedPlanFile()
	selected := m.nav.GetSelectedInstance()

	switch {
	case planFile != "" && m.taskState != nil:
		entry, ok := m.taskState.Entry(planFile)
		if ok {
			data.Branch = entry.Branch
			data.PlanName = taskstate.DisplayName(planFile)
			data.PlanStatus = string(entry.Status)

			if orch, orchOK := m.waveOrchestrators[planFile]; orchOK {
				waveNum := orch.CurrentWaveNumber()
				totalWaves := orch.TotalWaves()
				if waveNum > 0 {
					data.WaveLabel = fmt.Sprintf("wave %d/%d", waveNum, totalWaves)
					tasks := orch.CurrentWaveTasks()
					data.TaskGlyphs = make([]ui.TaskGlyph, len(tasks))
					for i, task := range tasks {
						switch {
						case orch.IsTaskComplete(task.Number):
							data.TaskGlyphs[i] = ui.TaskGlyphComplete
						case orch.IsTaskFailed(task.Number):
							data.TaskGlyphs[i] = ui.TaskGlyphFailed
						case orch.IsTaskRunning(task.Number):
							data.TaskGlyphs[i] = ui.TaskGlyphRunning
						default:
							data.TaskGlyphs[i] = ui.TaskGlyphPending
						}
					}
				}
			}

			// Populate PR state from the task store (has full PR metadata).
			if m.taskStore != nil {
				if storeEntry, err := m.taskStore.Get(m.taskStoreProject, planFile); err == nil && storeEntry.PRURL != "" {
					data.PRState = mapPRReviewDecision(storeEntry.PRReviewDecision)
					data.PRChecks = mapPRCheckStatus(storeEntry.PRCheckStatus)
				}
			}
		}
	case selected != nil && selected.Branch != "":
		data.Branch = selected.Branch
		if selected.TaskFile != "" && m.taskState != nil {
			entry, ok := m.taskState.Entry(selected.TaskFile)
			if ok {
				data.PlanName = taskstate.DisplayName(selected.TaskFile)
				data.PlanStatus = string(entry.Status)

				// Populate PR state from the task store.
				if m.taskStore != nil {
					if storeEntry, err := m.taskStore.Get(m.taskStoreProject, selected.TaskFile); err == nil && storeEntry.PRURL != "" {
						data.PRState = mapPRReviewDecision(storeEntry.PRReviewDecision)
						data.PRChecks = mapPRCheckStatus(storeEntry.PRCheckStatus)
					}
				}
			}
		}
	}

	if data.Branch == "" {
		data.Branch = currentBranch(m.activeRepoPath)
	}

	return data
}

// currentBranch returns the name of the currently checked-out branch in repoPath.
// Falls back to "main" if the branch cannot be determined (e.g. detached HEAD).
func currentBranch(repoPath string) string {
	cmd := exec.Command("git", "branch", "--show-current")
	cmd.Dir = repoPath
	out, err := cmd.Output()
	if err != nil {
		return "main"
	}
	branch := strings.TrimSpace(string(out))
	if branch == "" {
		return "main" // detached HEAD
	}
	return branch
}

// computePlanStatuses builds per-plan instance status flags (running/notification)
// from the current instance list.
func (m *home) computePlanStatuses() map[string]ui.TopicStatus {
	planStatuses := make(map[string]ui.TopicStatus)
	for _, inst := range m.nav.GetInstances() {
		if inst.TaskFile == "" {
			continue
		}
		planSt := planStatuses[inst.TaskFile]
		planStatuses[inst.TaskFile] = mergePlanStatus(planSt, inst, inst.Started())
	}
	return planStatuses
}

// updateNavPanelStatus recomputes plan instance statuses and triggers a row
// rebuild. Use this after instance mutations (kill, remove, pause) where the
// plan list itself hasn't changed. When updateSidebarTasks is also called,
// skip this — updateSidebarTasks already includes plan statuses in its rebuild.
func (m *home) updateNavPanelStatus() {
	m.nav.SetItems(nil, nil, 0, nil, nil, m.computePlanStatuses())
}

// focusSlot constants for readability.
// 0=nav, 1=tabs (center pane).
const (
	slotNav   = 0
	slotAgent = 1
	slotCount = 2
)

// setFocusSlot updates which pane has focus and syncs visual state.
func (m *home) setFocusSlot(slot int) {
	m.focusSlot = slot
	m.nav.SetFocused(slot == slotNav)
	m.menu.SetFocusSlot(slot)
	m.tabbedWindow.SetFocused(slot == slotAgent)
}

func asyncClosePreviewTerminal(term *session.EmbeddedTerminal) tea.Cmd {
	if term == nil {
		return nil
	}
	return func() tea.Msg {
		term.Close()
		return nil
	}
}

func previewIdentityKey(inst *session.Instance) string {
	if inst == nil {
		return ""
	}
	return inst.IdentityKey()
}

func (m *home) shouldAttachPreviewTerminal(selected *session.Instance) bool {
	return m.previewRequested &&
		selected != nil &&
		selected.Started() &&
		selected.Status != session.Paused &&
		selected.Status != session.Loading &&
		!selected.Exited &&
		session.NormalizeExecutionMode(selected.ExecutionMode) == session.ExecutionModeTmux
}

func (m *home) spawnPreviewTerminal(selected *session.Instance) tea.Cmd {
	if selected == nil {
		return nil
	}
	cols, rows := m.tabbedWindow.GetPreviewSize()
	if cols < 10 {
		cols = 80
	}
	if rows < 5 {
		rows = 24
	}
	capturedKey := previewIdentityKey(selected)
	capturedInstance := selected
	return func() tea.Msg {
		term, err := capturedInstance.NewEmbeddedTerminalForInstance(cols, rows)
		return previewTerminalReadyMsg{term: term, instanceKey: capturedKey, err: err}
	}
}

func (m *home) syncPreviewTerminal() tea.Cmd {
	selected := m.nav.GetSelectedInstance()
	needsPreview := m.shouldAttachPreviewTerminal(selected)
	currentMatches := needsPreview && m.previewTerminal != nil && m.previewTerminalInstance == previewIdentityKey(selected)

	var cmds []tea.Cmd
	if m.previewTerminal != nil && !currentMatches {
		oldTerm := m.previewTerminal
		m.previewTerminal = nil
		m.previewTerminalInstance = ""
		m.previewClipboardPending = false
		m.previewClipboardTarget = 0
		cmds = append(cmds, asyncClosePreviewTerminal(oldTerm))
	}

	if needsPreview && !currentMatches {
		cmds = append(cmds, m.spawnPreviewTerminal(selected))
	}

	if len(cmds) == 0 {
		return nil
	}
	if len(cmds) == 1 {
		return cmds[0]
	}
	return tea.Batch(cmds...)
}

func (m *home) refreshSelectedPreview() {
	if m.nav == nil || m.tabbedWindow == nil || m.tabbedWindow.IsDocumentMode() {
		return
	}
	selected := m.nav.GetSelectedInstance()
	if m.previewTerminal != nil && (selected == nil || session.NormalizeExecutionMode(selected.ExecutionMode) == session.ExecutionModeTmux) {
		return
	}
	if m.shouldAttachPreviewTerminal(selected) {
		m.tabbedWindow.SetConnectingState()
		return
	}
	if err := m.tabbedWindow.UpdatePreview(selected); err != nil {
		log.ErrorLog.Printf("preview update error: %v", err)
	}
}

// enterFocusMode enters focus/insert mode and starts the fast preview ticker.
// enterFocusMode reuses the existing previewTerminal if it is already attached to
// the selected instance — entering focus just toggles key forwarding to the same
// terminal. Only spawns a new terminal if none is attached yet (rare fallback).
func (m *home) enterFocusMode() tea.Cmd {
	m.tabbedWindow.ClearDocumentMode()
	m.previewRequested = true
	selected := m.nav.GetSelectedInstance()
	if selected == nil || selected.Status == session.Paused {
		return nil
	}
	if !selected.Started() && session.NormalizeExecutionMode(selected.ExecutionMode) != session.ExecutionModeSDK {
		return nil
	}

	// If previewTerminal is already attached to this instance, just enter focus mode.
	if m.previewTerminal != nil && m.previewTerminalInstance == previewIdentityKey(selected) {
		m.state = stateFocusAgent
		m.tabbedWindow.SetFocusMode(true)
		m.menu.SetFocusMode(true)
		return tea.RequestWindowSize
	}

	// No terminal yet — attach asynchronously so focus transitions don't block.
	cmd := m.syncPreviewTerminal()
	m.state = stateFocusAgent
	m.tabbedWindow.SetFocusMode(true)
	m.menu.SetFocusMode(true)

	return tea.Batch(tea.RequestWindowSize, cmd)
}

// exclamationAutoFocus handles the `!` key in stateDefault: it enters
// interactive/focus mode and sends '!' to the harness terminal so the agent's
// shell mode is activated in one keystroke.
//
// No-ops gracefully when no selected/running instance exists.
func (m *home) exclamationAutoFocus() (tea.Model, tea.Cmd) {
	m.previewRequested = true

	// Resolve selected instance.
	selected := m.nav.GetSelectedInstance()
	if selected == nil {
		if pf := m.nav.GetSelectedPlanFile(); pf != "" {
			if best := m.nav.FindPlanInstance(pf); best != nil {
				m.nav.SelectInstance(best)
				selected = best
			}
		}
	}

	// Guard: no valid target — don't enter focus mode.
	if selected == nil || !selected.Started() || selected.Paused() {
		return m, nil
	}

	// Enter focus mode using the existing API.
	cmd := m.enterFocusMode()

	// Only send '!' if focus mode was actually entered and the tmux session is live.
	// This prevents nil-PTY panics on DummyTerminal (tests) and stale terminals.
	if m.state != stateFocusAgent {
		return m, cmd
	}
	if m.previewTerminal != nil && selected.TmuxAlive() {
		if err := m.previewTerminal.SendKey([]byte("!")); err != nil {
			return m, m.handleError(err)
		}
	}

	return m, cmd
}

// exitFocusMode resets focus state. previewTerminal stays alive — it continues
// rendering in normal preview mode after the user exits focus/insert mode.
func (m *home) exitFocusMode() {
	if m.tabbedWindow.IsPreviewInScrollMode() {
		if selected := m.nav.GetSelectedInstance(); selected != nil {
			if err := m.tabbedWindow.ResetPreviewToNormalMode(selected); err != nil && log.ErrorLog != nil {
				log.ErrorLog.Printf("exitFocusMode: reset preview scroll mode: %v", err)
			}
		}
	}
	// previewTerminal stays alive — it continues rendering in normal preview mode.
	m.state = stateDefault
	m.tabbedWindow.SetFocusMode(false)
	m.menu.SetFocusMode(false)
}

func (m *home) showAllCompletePrompt(planFile string) {
	planName := taskstate.DisplayName(planFile)
	delete(m.allCompleteDismissed, planFile)
	m.resolveDeferredToast(m.deferredWaveToastIDs, planFile, overlay.ToastInfo,
		fmt.Sprintf("wave decision ready for '%s'", planName))
	m.resolveDeferredToast(m.deferredPlannerToastIDs, planFile, overlay.ToastInfo,
		fmt.Sprintf("implementation prompt ready for '%s'", planName))
	m.resolveDeferredToast(m.deferredCoderPushToastIDs, planFile, overlay.ToastInfo,
		fmt.Sprintf("review prompt ready for '%s'", planName))
	m.pendingAllCompleteTaskFile = planFile
	m.confirmAction(
		fmt.Sprintf("all waves complete for '%s'. push branch and start review?", planName),
		func() tea.Msg {
			return waveAllCompleteMsg{planFile: planFile}
		},
	)
}

func (m *home) ensureAllCompleteToast(planFile string) {
	if m.state != stateFocusAgent || m.toastManager == nil {
		return
	}
	if m.allCompleteToastIDs == nil {
		m.allCompleteToastIDs = make(map[string]string)
	}
	if m.allCompleteToastIDs[planFile] != "" {
		return
	}
	planName := taskstate.DisplayName(planFile)
	m.allCompleteToastIDs[planFile] = m.toastManager.Loading(
		fmt.Sprintf("all waves complete for '%s'. leave focus mode to review", planName),
	)
}

func (m *home) resolveAllCompleteToast(planFile string, typ overlay.ToastType, msg string) {
	if m.toastManager == nil || m.allCompleteToastIDs == nil {
		return
	}
	id := m.allCompleteToastIDs[planFile]
	if id == "" {
		return
	}
	m.toastManager.Resolve(id, typ, msg)
	delete(m.allCompleteToastIDs, planFile)
}

func (m *home) queueAllCompletePrompt(planFile string) {
	if m.allCompleteAdvancing != nil && m.allCompleteAdvancing[planFile] {
		return
	}
	if m.pendingAllCompleteTaskFile != planFile {
		alreadyQueued := false
		for _, pf := range m.pendingAllComplete {
			if pf == planFile {
				alreadyQueued = true
				break
			}
		}
		if !alreadyQueued {
			m.pendingAllComplete = append(m.pendingAllComplete, planFile)
		}
	}
	m.ensureAllCompleteToast(planFile)
}

func appendUniqueString(values []string, value string) []string {
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}

func (m *home) queueDeferredToast(toasts map[string]string, key, msg string) {
	if m.state != stateFocusAgent || m.toastManager == nil || toasts == nil || key == "" {
		return
	}
	if toasts[key] != "" {
		return
	}
	toasts[key] = m.toastManager.Loading(msg)
}

func (m *home) resolveDeferredToast(toasts map[string]string, key string, typ overlay.ToastType, msg string) {
	if m.toastManager == nil || toasts == nil || key == "" {
		return
	}
	id := toasts[key]
	if id == "" {
		return
	}
	m.toastManager.Resolve(id, typ, msg)
	delete(toasts, key)
}

func (m *home) queuePlannerDialog(planFile string) {
	m.deferredPlannerDialogs = appendUniqueString(m.deferredPlannerDialogs, planFile)
	m.queueDeferredToast(m.deferredPlannerToastIDs, planFile,
		fmt.Sprintf("task '%s' is ready. leave focus mode to start implementation", taskstate.DisplayName(planFile)))
}

func (m *home) showPlannerDialog(planFile string) tea.Cmd {
	m.resolveDeferredToast(m.deferredPlannerToastIDs, planFile, overlay.ToastInfo,
		fmt.Sprintf("implementation prompt ready for '%s'", taskstate.DisplayName(planFile)))
	for _, inst := range m.nav.GetInstances() {
		if inst.TaskFile == planFile && inst.AgentType == session.AgentTypePlanner {
			m.pendingPlannerInstanceTitle = inst.Title
			if cmd := m.focusInstanceForOverlay(inst); cmd != nil {
				m.pendingPlannerTaskFile = planFile
				m.confirmAction(
					fmt.Sprintf("task '%s' is ready. start implementation?", taskstate.DisplayName(planFile)),
					func() tea.Msg { return plannerCompleteMsg{planFile: planFile} },
				)
				return cmd
			}
			break
		}
	}
	m.pendingPlannerTaskFile = planFile
	m.confirmAction(
		fmt.Sprintf("task '%s' is ready. start implementation?", taskstate.DisplayName(planFile)),
		func() tea.Msg { return plannerCompleteMsg{planFile: planFile} },
	)
	return nil
}

func (m *home) queueCoderPushDialog(planFile string) {
	m.deferredCoderPushDialogs = appendUniqueString(m.deferredCoderPushDialogs, planFile)
	m.queueDeferredToast(m.deferredCoderPushToastIDs, planFile,
		fmt.Sprintf("implementation finished for '%s'. leave focus mode to start review", taskstate.DisplayName(planFile)))
}

func (m *home) findLifecycleImplementer(planFile string) *session.Instance {
	var fallback *session.Instance
	for _, inst := range m.nav.GetInstances() {
		if inst.TaskFile != planFile {
			continue
		}
		if inst.AgentType != session.AgentTypeCoder && inst.AgentType != session.AgentTypeFixer {
			continue
		}
		if fallback == nil {
			fallback = inst
		}
		if !inst.Paused() && !inst.Exited {
			return inst
		}
	}
	return fallback
}

func (m *home) showCoderPushDialog(planFile string) tea.Cmd {
	m.resolveDeferredToast(m.deferredCoderPushToastIDs, planFile, overlay.ToastInfo,
		fmt.Sprintf("review prompt ready for '%s'", taskstate.DisplayName(planFile)))
	inst := m.findLifecycleImplementer(planFile)
	if inst == nil {
		return nil
	}
	if cmd := m.focusInstanceForOverlay(inst); cmd != nil {
		_ = m.promptPushBranchThenAdvance(inst)
		return cmd
	}
	return m.promptPushBranchThenAdvance(inst)
}

func (m *home) queueWaveDialog(planFile string) {
	m.deferredWaveDialogs = appendUniqueString(m.deferredWaveDialogs, planFile)
	m.queueDeferredToast(m.deferredWaveToastIDs, planFile,
		fmt.Sprintf("wave complete for '%s'. dismiss overlay to continue", taskstate.DisplayName(planFile)))
}

// showWaveDialog is the single source of truth for intermediate wave decisions.
// It handles all pre-work (toast, ClickUp, focus, persistence, audit) before
// showing a WaveDecisionOverlay and transitioning to stateWaveDecision.
func (m *home) showWaveDialog(planFile string, orch *orchestration.WaveOrchestrator) []tea.Cmd {
	if orch == nil || orch.State() != orchestration.WaveStateWaveComplete {
		return nil
	}
	waveNum := orch.CurrentWaveNumber()
	completed := orch.CompletedTaskCount()
	failed := orch.FailedTaskCount()
	total := completed + failed
	planName := taskstate.DisplayName(planFile)

	m.resolveDeferredToast(m.deferredWaveToastIDs, planFile, overlay.ToastInfo,
		fmt.Sprintf("wave decision ready for '%s'", planName))

	cmds := make([]tea.Cmd, 0, 4)

	// Post intermediate ClickUp comment for successful waves.
	if failed == 0 && orch.ShouldPostWaveCompleteComment() {
		detail := fmt.Sprintf("%d/%d: %d/%d tasks", waveNum, orch.TotalWaves(), completed, total)
		if cmd := m.postClickUpProgress(planFile, "wave_complete", detail); cmd != nil {
			cmds = append(cmds, cmd)
		}
	}

	// Auto-advance path: emit audit, toast, and dispatch advance without overlay.
	if failed == 0 && m.appConfig != nil && m.appConfig.AutoAdvanceWaves {
		if orch.ClaimWaveOutcome() {
			m.audit(auditlog.EventWaveCompleted,
				fmt.Sprintf("wave %d complete: %d/%d tasks (auto-advancing)", waveNum, completed, total),
				auditlog.WithPlan(planFile),
				auditlog.WithWave(waveNum, 0),
				auditlog.WithWaveOutcome("wave_success", failed, total, "next_wave|review", orch.RetryGeneration()))
		}
		m.toastManager.Info(fmt.Sprintf("%s — wave %d complete, auto-advancing...", planName, waveNum))
		entry, _ := m.taskState.Entry(planFile)
		capturedPlanFile := planFile
		capturedEntry := entry
		cmds = append(cmds, func() tea.Msg {
			return waveAdvanceMsg{planFile: capturedPlanFile, entry: capturedEntry}
		})
		return cmds
	}

	// Persist wave-waiting execution state.
	if err := m.setExecutionState(planFile, taskstore.ExecutionState{
		Phase:           string(taskfsm.ExecutionPhaseWaveWaiting),
		ActiveAgentType: session.AgentTypeCoder,
		ActiveWave:      waveNum,
	}); err != nil {
		log.WarningLog.Printf("could not persist wave waiting state for %q: %v", planFile, err)
	}

	// Emit audit event before showing the overlay.
	if orch.ClaimWaveOutcome() {
		if failed > 0 {
			m.audit(auditlog.EventWaveFailed,
				fmt.Sprintf("wave %d needs a decision — %d of %d tasks failed", waveNum, failed, total),
				auditlog.WithPlan(planFile),
				auditlog.WithWave(waveNum, 0),
				auditlog.WithWaveOutcome("wave_decision", failed, total, "retry|advance|abort", orch.RetryGeneration()),
				auditlog.WithLevel("warn"))
		} else {
			m.audit(auditlog.EventWaveCompleted,
				fmt.Sprintf("wave %d complete: %d/%d tasks", waveNum, completed, total),
				auditlog.WithPlan(planFile),
				auditlog.WithWave(waveNum, 0),
				auditlog.WithWaveOutcome("wave_success", failed, total, "next_wave|review", orch.RetryGeneration()))
		}
	}

	// Focus a task instance so the user can see agent output behind the overlay.
	if cmd := m.focusPlanInstanceForOverlay(planFile); cmd != nil {
		cmds = append(cmds, cmd)
	}

	// Show the dedicated wave decision overlay.
	wo := overlay.NewWaveDecisionOverlay(overlay.WaveDecisionInput{
		PlanFile:   planFile,
		PlanName:   planName,
		WaveNumber: waveNum,
		TotalWaves: orch.TotalWaves(),
		Completed:  completed,
		Failed:     failed,
		Total:      total,
	})
	m.overlays.Show(wo)
	m.state = stateWaveDecision
	return cmds
}

func (m *home) queuePermissionPrompt(inst *session.Instance, pattern, desc string) {
	if inst == nil {
		return
	}
	// An instance can have at most one active permission prompt. Dedup on
	// instance identity and update the latest pattern/desc in place so
	// minor pane-content drift between ticks doesn't grow the queue.
	for i, existing := range m.deferredPermissionPrompts {
		if existing.instance == inst {
			m.deferredPermissionPrompts[i].pattern = pattern
			m.deferredPermissionPrompts[i].desc = desc
			return
		}
	}
	m.deferredPermissionPrompts = append(m.deferredPermissionPrompts, deferredPermissionPrompt{instance: inst, pattern: pattern, desc: desc})
	m.queueDeferredToast(m.deferredPermissionToastIDs, inst.Title,
		fmt.Sprintf("permission prompt waiting for %s. leave focus mode to respond", inst.Title))
}

func (m *home) clearDeferredPermissionPrompt(inst *session.Instance) {
	if inst == nil {
		return
	}
	filtered := m.deferredPermissionPrompts[:0]
	for _, deferred := range m.deferredPermissionPrompts {
		if deferred.instance != inst {
			filtered = append(filtered, deferred)
		}
	}
	m.deferredPermissionPrompts = filtered
	m.resolveDeferredToast(m.deferredPermissionToastIDs, inst.Title, overlay.ToastInfo,
		fmt.Sprintf("permission prompt cleared for %s", inst.Title))
}

func (m *home) showPermissionPrompt(deferred deferredPermissionPrompt) tea.Cmd {
	inst := deferred.instance
	if inst == nil {
		return nil
	}
	m.resolveDeferredToast(m.deferredPermissionToastIDs, inst.Title, overlay.ToastInfo,
		fmt.Sprintf("permission prompt ready for %s", inst.Title))
	// Save the current nav row id before focusing away (first-write-wins).
	// Capturing the row id — rather than the selected instance pointer —
	// means plan/history selections (where GetSelectedInstance() is nil)
	// still restore correctly when the overlay is dismissed.
	// Skip capture+focus when the prompt is on the already-selected instance —
	// no restoration needed and avoids unnecessary instanceChanged() side effects.
	var focusCmd tea.Cmd
	if m.nav.GetSelectedInstance() != inst {
		if !m.preOverlayCaptured {
			m.preOverlayNavID = m.nav.GetSelectedID()
			m.preOverlayCaptured = true
		}
		focusCmd = m.focusInstanceForOverlay(inst)
	}
	m.pendingPermissionPattern = deferred.pattern
	m.pendingPermissionDesc = deferred.desc
	m.overlays.Show(overlay.NewPermissionOverlay(inst.Title, deferred.desc, deferred.pattern))
	m.pendingPermissionInstance = inst
	m.state = statePermission
	return focusCmd
}

// snapshotPaneOnCompletion writes the instance's pane content to a log file
// when a wave task is marked complete. This provides debugging data for
// diagnosing false-positive completion detection. Only writes when the
// instance has cached content and a .kasmos directory exists.
func (m *home) snapshotPaneOnCompletion(inst *session.Instance, planFile string, taskNumber, waveNumber int) {
	if !inst.CachedContentSet || inst.CachedContent == "" {
		return
	}
	logDir := filepath.Join(m.activeRepoPath, ".kasmos", "logs", "completions")
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		return
	}
	slug := taskstate.DisplayName(planFile)
	filename := fmt.Sprintf("%s-W%d-T%d-%d.log", slug, waveNumber, taskNumber, time.Now().Unix())
	_ = os.WriteFile(filepath.Join(logDir, filename), []byte(inst.CachedContent), 0o644)
}

// saveAllInstances saves allInstances (all repos) to storage.
// Daemon SDK placeholders are excluded: they have no local execution session
// and would appear as stale "standalone sdk" rows after the daemon restarts.
// No-ops gracefully when storage is nil (e.g. in unit tests).
func (m *home) saveAllInstances() error {
	if m.storage == nil {
		return nil
	}
	toSave := make([]*session.Instance, 0, len(m.allInstances))
	for _, inst := range m.allInstances {
		if !m.isDaemonSDKPlaceholder(inst) {
			toSave = append(toSave, inst)
		}
	}
	return m.storage.SaveInstances(toSave)
}

func (m *home) syncInstanceDisplayTitle(inst *session.Instance, rawTitle string) error {
	if inst == nil {
		return nil
	}

	newTitle := slugify(rawTitle)
	if newTitle == "" || inst.DisplayName() == newTitle {
		return nil
	}

	for _, other := range m.nav.GetInstances() {
		if other != nil && other != inst && other.DisplayName() == newTitle {
			return nil
		}
	}
	for _, other := range m.allInstances {
		if other != nil && other != inst && other.DisplayName() == newTitle {
			return nil
		}
	}
	for other := range m.instanceFinalizers {
		if other != nil && other != inst && other.DisplayName() == newTitle {
			return nil
		}
	}

	inst.DisplayTitle = newTitle
	if selected := m.nav.GetSelectedInstance(); selected == inst {
		m.updateInfoPane()
	}
	m.updateNavPanelStatus()
	return m.saveAllInstances()
}

// removeFromAllInstances removes an instance from the master list by title.
func (m *home) removeFromAllInstances(title string) {
	filtered := m.allInstances[:0]
	for _, inst := range m.allInstances {
		if inst.Title != title {
			filtered = append(filtered, inst)
		}
	}
	m.allInstances = filtered
}

// dismissInstanceFromList removes inst from the sidebar, allInstances, and
// persistence in a single step. Both the delete/backspace path and the k+k+k
// triple-tap path use this helper so list-mutation semantics stay single-sourced.
func (m *home) dismissInstanceFromList(inst *session.Instance) tea.Cmd {
	if inst == nil || strings.TrimSpace(inst.Title) == "" {
		return nil
	}
	m.markInstanceTitleDismissed(inst.Title)
	m.nav.RemoveByTitle(inst.Title)
	m.removeFromAllInstances(inst.Title)
	_ = m.saveAllInstances()
	m.updateNavPanelStatus()
	return tea.Batch(tea.RequestWindowSize, m.instanceChanged())
}

func (m *home) hasLiveOrPendingInstance(planFile, agentType, title string) bool {
	matches := func(inst *session.Instance) bool {
		if inst == nil || inst.TaskFile != planFile || inst.AgentType != agentType {
			return false
		}
		if title != "" && inst.Title != title {
			return false
		}
		return !inst.Exited && !inst.Paused()
	}

	for _, inst := range m.nav.GetInstances() {
		if matches(inst) {
			return true
		}
	}
	for _, inst := range m.allInstances {
		if matches(inst) {
			return true
		}
	}
	for inst := range m.instanceFinalizers {
		if matches(inst) {
			return true
		}
	}
	return false
}

// cleanupPausedDoneReviewers is retained for compatibility with older call
// sites. Completed reviewers stay visible now; presentation derives their
// non-actionable state instead of deleting evidence from the instance list.
func (m *home) cleanupPausedDoneReviewers(selected *session.Instance) {
	_ = selected
}

// instanceChanged updates the preview pane, menu, and diff pane based on the selected instance.
// It returns a tea.Cmd when an async operation is needed (terminal spawn).
func (m *home) instanceChanged() tea.Cmd {
	// selected may be nil
	selected := m.nav.GetSelectedInstance()
	m.previewRequested = true
	m.cleanupPausedDoneReviewers(selected)
	selected = m.nav.GetSelectedInstance() // refresh in case list mutation changed selection

	// Clear notification on the previously-viewed instance when the user
	// navigates away. This prevents the item from jumping out of "attention"
	// while the user is still looking at it.
	if m.seenNotified != nil && m.seenNotified != selected {
		m.seenNotified.Notified = false
		m.seenNotified = nil
		m.updateNavPanelStatus()
	}
	if selected != nil && selected.Notified {
		m.seenNotified = selected
	}

	previewCmd := m.syncPreviewTerminal()

	m.tabbedWindow.SetInstance(selected)
	m.updateInfoPane()
	// Update menu with current instance.
	m.menu.SetInstance(selected)
	m.refreshSelectedPreview()

	// Collect async commands.
	var cmds []tea.Cmd
	if previewCmd != nil {
		cmds = append(cmds, previewCmd)
	}

	if len(cmds) == 0 {
		return nil
	}
	if len(cmds) == 1 {
		return cmds[0]
	}
	return tea.Batch(cmds...)
}

// focusInstanceForOverlay selects the given instance in the nav panel and
// switches the preview to show its output behind an overlay dialog. The user
// can see the agent's terminal behind the overlay to understand context before
// responding. Returns a tea.Cmd if an async terminal spawn is needed.
func (m *home) focusInstanceForOverlay(inst *session.Instance) tea.Cmd {
	if inst == nil {
		return nil
	}
	m.nav.SelectInstance(inst)
	return m.instanceChanged()
}

// focusPlanInstanceForOverlay selects the best instance belonging to the given
// plan file, preferring running instances. This is used before showing overlay
// dialogs about plan lifecycle events (wave completion, etc.) so the user can
// see the agent output behind the overlay.
func (m *home) focusPlanInstanceForOverlay(planFile string) tea.Cmd {
	var best *session.Instance
	for _, inst := range m.nav.GetInstances() {
		if inst.TaskFile != planFile {
			continue
		}
		if best == nil {
			best = inst
		}
		// Prefer running instances over ready/paused ones.
		if inst.Status == session.Running && best.Status != session.Running {
			best = inst
		}
	}
	return m.focusInstanceForOverlay(best)
}

// findTaskTitle returns the title of the task with the given number in the plan, or "".
func findTaskTitle(plan *taskparser.Plan, taskNumber int) string {
	if plan == nil || taskNumber == 0 {
		return ""
	}
	for _, wave := range plan.Waves {
		for _, task := range wave.Tasks {
			if task.Number == taskNumber {
				return task.Title
			}
		}
	}
	return ""
}

// buildSubtaskProgress fetches subtasks from the store and groups them by wave using the
// orchestrator's plan structure. On error, prior* values are returned unchanged so that a
// transient store failure does not blank out previously displayed subtask data.
func (m *home) buildSubtaskProgress(
	planFile string,
	orch *orchestration.WaveOrchestrator,
	priorCompleted, priorTotal int,
	priorGroups []ui.WaveSubtaskGroup,
) (completed, total int, groups []ui.WaveSubtaskGroup) {
	if m.taskState == nil || orch == nil {
		return priorCompleted, priorTotal, priorGroups
	}
	subtasks, err := m.taskState.GetSubtasks(planFile)
	if err != nil {
		log.WarningLog.Printf("updateInfoPane: could not read subtasks for %q: %v", planFile, err)
		return priorCompleted, priorTotal, priorGroups
	}

	// Index subtasks by task number.
	byNumber := make(map[int]taskstore.SubtaskEntry, len(subtasks))
	for _, s := range subtasks {
		byNumber[s.TaskNumber] = s
	}

	plan := orch.Plan()
	if plan == nil {
		return priorCompleted, priorTotal, priorGroups
	}

	total = len(subtasks)
	for _, s := range subtasks {
		switch string(s.Status) {
		case "complete", "done", "closed":
			completed++
		}
	}

	groups = make([]ui.WaveSubtaskGroup, 0, len(plan.Waves))
	for _, wave := range plan.Waves {
		group := ui.WaveSubtaskGroup{WaveNumber: wave.Number}
		for _, task := range wave.Tasks {
			entry, ok := byNumber[task.Number]
			statusStr := "pending"
			if ok {
				statusStr = string(entry.Status)
			}
			title := task.Title
			if ok && entry.Title != "" {
				title = entry.Title
			}
			group.Subtasks = append(group.Subtasks, ui.SubtaskDisplay{
				Number: task.Number,
				Title:  title,
				Status: statusStr,
			})
		}
		groups = append(groups, group)
	}
	return completed, total, groups
}

func statusString(s session.Status) string {
	switch s {
	case session.Running:
		return "running"
	case session.Ready:
		return "ready"
	case session.Loading:
		return "loading"
	case session.Paused:
		return "paused"
	default:
		return "unknown"
	}
}

// updateInfoPaneForPlanHeader populates the info tab when a plan header is selected
// (no instance). Shows plan metadata and instance summary counts.
func (m *home) updateInfoPaneForPlanHeader() {
	planFile := m.nav.GetSelectedPlanFile()
	if planFile == "" || m.taskState == nil {
		m.tabbedWindow.SetInfoData(ui.InfoData{IsPlanHeaderSelected: true})
		return
	}
	entry, ok := m.taskState.Entry(planFile)
	if !ok {
		m.tabbedWindow.SetInfoData(ui.InfoData{IsPlanHeaderSelected: true})
		return
	}
	data := ui.InfoData{
		IsPlanHeaderSelected: true,
		PlanName:             taskstate.DisplayName(planFile),
		PlanDescription:      entry.Description,
		PlanStatus:           string(entry.Status),
		PlanTopic:            entry.Topic,
		PlanBranch:           entry.Branch,
		ExecutionPhase:       strings.TrimSpace(entry.ExecutionState.Phase),
		ActiveAgentType:      strings.TrimSpace(entry.ExecutionState.ActiveAgentType),
		ActiveWave:           entry.ExecutionState.ActiveWave,
	}
	switch taskfsm.NormalizeExecutionPhase(entry.ExecutionState.Phase) {
	case taskfsm.ExecutionPhaseFixing, taskfsm.ExecutionPhaseReviewing:
		data.ActiveRound = entry.ReviewCycle + 1
	}
	if !entry.CreatedAt.IsZero() {
		data.PlanCreated = entry.CreatedAt.Format("2006-01-02")
	}
	// Count instances belonging to this plan.
	for _, inst := range m.nav.GetInstances() {
		if inst.TaskFile != planFile {
			continue
		}
		data.PlanInstanceCount++
		switch {
		case inst.Status == session.Running || inst.Status == session.Loading:
			data.PlanRunningCount++
		case inst.Status == session.Paused:
			data.PlanPausedCount++
		default:
			data.PlanReadyCount++
		}
	}
	// Enrich with goal and lifecycle timestamps.
	data.PlanGoal = entry.Goal
	data.PlanningAt = entry.PlanningAt
	data.ImplementingAt = entry.ImplementingAt
	data.ReviewingAt = entry.ReviewingAt
	data.VerifyingAt = entry.VerifyingAt
	data.DoneAt = entry.DoneAt

	// Include wave progress if an orchestrator exists for this plan.
	var orch *orchestration.WaveOrchestrator
	if o, ok := m.waveOrchestrators[planFile]; ok {
		orch = o
		data.TotalWaves = orch.TotalWaves()
		data.TotalTasks = orch.TotalTasks()
		tasks := orch.CurrentWaveTasks()
		data.WaveTasks = make([]ui.WaveTaskInfo, len(tasks))
		for i, task := range tasks {
			state := "pending"
			if orch.IsTaskComplete(task.Number) {
				state = "complete"
			} else if orch.IsTaskFailed(task.Number) {
				state = "failed"
			} else if orch.IsTaskRunning(task.Number) {
				state = "running"
			}
			data.WaveTasks[i] = ui.WaveTaskInfo{Number: task.Number, State: state}
		}
	}

	// Subtask progress (preserve prior zeros as initial values — plan header has no prior state).
	data.CompletedTasks, data.TotalSubtasks, data.AllWaveSubtasks =
		m.buildSubtaskProgress(planFile, orch, 0, 0, nil)

	// Review outcome — shown when the plan has been approved.
	if entry.Status == taskstate.StatusDone {
		data.ReviewOutcome = "approved"
		data.ReviewCycle = 1 // default display cycle
		if cycle, err := m.taskState.ReviewCycle(planFile); err == nil {
			data.ReviewCycle = cycle + 1
		}
	}
	if m.appConfig != nil && m.appConfig.MaxReviewFixCycles > 0 {
		data.MaxReviewFixCycles = m.appConfig.MaxReviewFixCycles
	}
	// Verify-round counter for the terminal-attempt label in the info pane.
	if entry.Status == taskstate.StatusVerifying && m.appConfig != nil {
		if cycle, err := m.taskState.ReviewCycle(planFile); err == nil {
			data.VerifyRound = cycle + 1
		}
		data.ReadinessMaxVerifyCycles = m.appConfig.ReadinessMaxVerifyCycles
	}

	m.tabbedWindow.SetInfoData(data)
}

// updateInfoPane refreshes the info tab data from the selected instance or plan header.
func (m *home) updateInfoPane() {
	selected := m.nav.GetSelectedInstance()
	if selected == nil {
		// No instance selected — check if a plan header is selected.
		if m.nav.IsSelectedPlanHeader() {
			m.updateInfoPaneForPlanHeader()
			return
		}
		m.tabbedWindow.SetInfoData(ui.InfoData{HasInstance: false})
		return
	}

	data := ui.InfoData{
		HasInstance:   true,
		Title:         selected.DisplayName(),
		Program:       selected.Program,
		Branch:        selected.Branch,
		Path:          selected.Path,
		Status:        statusString(selected.Status),
		AgentType:     selected.AgentType,
		TaskNumber:    selected.TaskNumber,
		WaveNumber:    selected.WaveNumber,
		WaveTaskIndex: selected.WaveTaskIndex,
		WaveTaskCount: selected.WaveTaskCount,
		ExecutionMode: string(selected.ExecutionMode),
		SDKSpeedTier:  selected.SDKSpeedTier,
	}

	if !selected.CreatedAt.IsZero() {
		data.Created = selected.CreatedAt.Format("2006-01-02 15:04")
	}

	// Capture prior subtask data from the current pane so we can preserve it on error.
	prior := m.tabbedWindow.GetInfoData()

	if selected.TaskFile != "" {
		var orch *orchestration.WaveOrchestrator
		if m.taskState != nil {
			entry, ok := m.taskState.Entry(selected.TaskFile)
			if ok {
				data.HasPlan = true
				data.PlanName = taskstate.DisplayName(selected.TaskFile)
				data.PlanDescription = entry.Description
				data.PlanStatus = string(entry.Status)
				data.PlanTopic = entry.Topic
				data.PlanBranch = entry.Branch
				data.ExecutionPhase = strings.TrimSpace(entry.ExecutionState.Phase)
				data.ActiveAgentType = strings.TrimSpace(entry.ExecutionState.ActiveAgentType)
				data.ActiveWave = entry.ExecutionState.ActiveWave
				switch taskfsm.NormalizeExecutionPhase(entry.ExecutionState.Phase) {
				case taskfsm.ExecutionPhaseFixing, taskfsm.ExecutionPhaseReviewing:
					data.ActiveRound = entry.ReviewCycle + 1
				}
				if !entry.CreatedAt.IsZero() {
					data.PlanCreated = entry.CreatedAt.Format("2006-01-02")
				}
				// Enrich with goal and lifecycle timestamps.
				data.PlanGoal = entry.Goal
				data.PlanningAt = entry.PlanningAt
				data.ImplementingAt = entry.ImplementingAt
				data.ReviewingAt = entry.ReviewingAt
				data.VerifyingAt = entry.VerifyingAt
				data.DoneAt = entry.DoneAt
				// Review outcome — shown when the plan has been approved.
				if entry.Status == taskstate.StatusDone {
					data.ReviewOutcome = "approved"
					data.ReviewCycle = 1 // default display cycle
					if cycle, err := m.taskState.ReviewCycle(selected.TaskFile); err == nil {
						data.ReviewCycle = cycle + 1
					}
				}
				// Verify-round counter for the terminal-attempt label in the info pane.
				if entry.Status == taskstate.StatusVerifying && m.appConfig != nil {
					if cycle, err := m.taskState.ReviewCycle(selected.TaskFile); err == nil {
						data.VerifyRound = cycle + 1
					}
					data.ReadinessMaxVerifyCycles = m.appConfig.ReadinessMaxVerifyCycles
				}
			}
		}

		if o, ok := m.waveOrchestrators[selected.TaskFile]; ok {
			orch = o
			data.TotalWaves = orch.TotalWaves()
			data.TotalTasks = orch.TotalTasks()
			tasks := orch.CurrentWaveTasks()
			data.WaveTasks = make([]ui.WaveTaskInfo, len(tasks))
			for i, task := range tasks {
				state := "pending"
				if orch.IsTaskComplete(task.Number) {
					state = "complete"
				} else if orch.IsTaskFailed(task.Number) {
					state = "failed"
				} else if orch.IsTaskRunning(task.Number) {
					state = "running"
				}
				data.WaveTasks[i] = ui.WaveTaskInfo{Number: task.Number, State: state}
			}
			// Populate TaskTitle from the plan structure.
			data.TaskTitle = findTaskTitle(orch.Plan(), selected.TaskNumber)
		}

		// Subtask progress — preserve prior values if GetSubtasks fails.
		data.CompletedTasks, data.TotalSubtasks, data.AllWaveSubtasks =
			m.buildSubtaskProgress(selected.TaskFile, orch,
				prior.CompletedTasks, prior.TotalSubtasks, prior.AllWaveSubtasks)
	}

	m.tabbedWindow.SetInfoData(data)
}

// loadTaskState reads plan state from the store for the active repo.
// Called on user-triggered events (plan creation, repo switch, etc.). The periodic
// metadata tick loads plan state in its goroutine instead.
// Silently no-ops if the store is not configured.
func (m *home) loadTaskState() {
	if m.taskStateDir == "" || m.taskStore == nil {
		return
	}
	ps, err := taskstate.Load(m.taskStore, m.taskStoreProject, m.taskStateDir)
	if err != nil {
		log.WarningLog.Printf("could not load plan state: %v", err)
		if m.toastManager != nil {
			m.toastManager.Error("task store error: " + err.Error())
		}
		return
	}
	m.taskState = ps
	m.taskStateLoadedAt = time.Now().UTC()
	m.cachedPlanFile = ""
	m.cachedPlanRendered = ""
}

// updateSidebarTasks pushes the current plans into the sidebar using the three-level tree API.
func (m *home) updateSidebarTasks() {
	if m.taskState == nil {
		m.nav.SetTopicsAndPlans(nil, nil, nil)
		return
	}

	// Build topic displays
	topicInfos := m.taskState.Topics()
	topics := make([]ui.TopicDisplay, 0, len(topicInfos))
	for _, t := range topicInfos {
		plans := m.taskState.TasksByTopic(t.Name)
		planDisplays := make([]ui.PlanDisplay, 0, len(plans))
		for _, p := range plans {
			if p.Status == taskstate.StatusDone || p.Status == taskstate.StatusCancelled {
				continue // finished/cancelled plans handled separately
			}
			entry := m.taskState.Plans[p.Filename]
			activeRound := 0
			switch taskfsm.NormalizeExecutionPhase(entry.ExecutionState.Phase) {
			case taskfsm.ExecutionPhaseFixing, taskfsm.ExecutionPhaseReviewing, "readiness_reviewing":
				activeRound = entry.ReviewCycle + 1
			}
			if activeRound == 0 && p.Status == taskstate.StatusVerifying && entry.ExecutionState.ActiveAgentType == session.AgentTypeMaster {
				activeRound = entry.ReviewCycle + 1
			}
			planDisplays = append(planDisplays, ui.PlanDisplay{
				Filename:    p.Filename,
				Status:      string(p.Status),
				Description: p.Description,
				Branch:      p.Branch,
				Topic:       p.Topic,
				Phase:       strings.TrimSpace(entry.ExecutionState.Phase),
				AgentType:   strings.TrimSpace(entry.ExecutionState.ActiveAgentType),
				ActiveWave:  entry.ExecutionState.ActiveWave,
				ActiveRound: activeRound,
			})
		}
		if len(planDisplays) > 0 {
			topics = append(topics, ui.TopicDisplay{Name: t.Name, Plans: planDisplays})
		}
	}

	// Build ungrouped plans
	ungroupedInfos := m.taskState.UngroupedTasks()
	ungrouped := make([]ui.PlanDisplay, 0, len(ungroupedInfos))
	for _, p := range ungroupedInfos {
		entry := m.taskState.Plans[p.Filename]
		activeRound := 0
		switch taskfsm.NormalizeExecutionPhase(entry.ExecutionState.Phase) {
		case taskfsm.ExecutionPhaseFixing, taskfsm.ExecutionPhaseReviewing, "readiness_reviewing":
			activeRound = entry.ReviewCycle + 1
		}
		if activeRound == 0 && p.Status == taskstate.StatusVerifying && entry.ExecutionState.ActiveAgentType == session.AgentTypeMaster {
			activeRound = entry.ReviewCycle + 1
		}
		ungrouped = append(ungrouped, ui.PlanDisplay{
			Filename:    p.Filename,
			Status:      string(p.Status),
			Description: p.Description,
			Branch:      p.Branch,
			Phase:       strings.TrimSpace(entry.ExecutionState.Phase),
			AgentType:   strings.TrimSpace(entry.ExecutionState.ActiveAgentType),
			ActiveWave:  entry.ExecutionState.ActiveWave,
			ActiveRound: activeRound,
		})
	}

	// Flatten single-plan topics where topic name matches the plan display name.
	// These don't benefit from a topic header — show the plan directly as ungrouped.
	filtered := topics[:0]
	for _, t := range topics {
		if len(t.Plans) == 1 && t.Name == taskstate.DisplayName(t.Plans[0].Filename) {
			ungrouped = append(ungrouped, t.Plans[0])
		} else {
			filtered = append(filtered, t)
		}
	}
	topics = filtered

	// Build history
	finishedInfos := m.taskState.Finished()
	history := make([]ui.PlanDisplay, 0, len(finishedInfos))
	for _, p := range finishedInfos {
		entry := m.taskState.Plans[p.Filename]
		history = append(history, ui.PlanDisplay{
			Filename:    p.Filename,
			Status:      string(p.Status),
			Description: p.Description,
			Branch:      p.Branch,
			Topic:       p.Topic,
			Phase:       strings.TrimSpace(entry.ExecutionState.Phase),
			AgentType:   strings.TrimSpace(entry.ExecutionState.ActiveAgentType),
			ActiveWave:  entry.ExecutionState.ActiveWave,
		})
	}

	// Build cancelled
	cancelledInfos := m.taskState.Cancelled()
	cancelled := make([]ui.PlanDisplay, 0, len(cancelledInfos))
	for _, p := range cancelledInfos {
		entry := m.taskState.Plans[p.Filename]
		cancelled = append(cancelled, ui.PlanDisplay{
			Filename:    p.Filename,
			Status:      string(p.Status),
			Description: p.Description,
			Branch:      p.Branch,
			Topic:       p.Topic,
			Phase:       strings.TrimSpace(entry.ExecutionState.Phase),
			AgentType:   strings.TrimSpace(entry.ExecutionState.ActiveAgentType),
			ActiveWave:  entry.ExecutionState.ActiveWave,
		})
	}

	// Set plan statuses before the rebuild so navPlanSortKey uses
	// up-to-date running/notification flags in a single pass.
	m.nav.SetPlanStatuses(m.computePlanStatuses())

	m.nav.SetTopicsAndPlans(topics, ungrouped, history, cancelled)

}

// checkPlanCompletion scans running coder instances for plans that have been
// marked "done" by the agent and, if found, transitions them to reviewer sessions.
// Returns a cmd to start the reviewer (may be nil).
func (m *home) checkPlanCompletion() tea.Cmd {
	if m.taskState == nil {
		return nil
	}
	// Guard: if a reviewer already exists for a plan, do not spawn another.
	// The async metadata tick can overwrite m.taskState with a stale snapshot
	// that still shows StatusDone after transitionToReview already ran and set
	// StatusReviewing. Without this guard, a second reviewer is spawned.
	reviewerPlans := make(map[string]bool)
	for _, inst := range m.nav.GetInstances() {
		if isReviewerInstance(inst) && inst.TaskFile != "" {
			reviewerPlans[inst.TaskFile] = true
		}
	}
	for _, inst := range m.nav.GetInstances() {
		if inst.TaskFile == "" || isReviewerInstance(inst) {
			continue
		}
		if inst.ImplementationComplete {
			continue // already went through review cycle — don't re-trigger
		}
		if reviewerPlans[inst.TaskFile] {
			continue // reviewer already spawned; skip regardless of stale plan state
		}
		entry, ok := m.taskState.Entry(inst.TaskFile)
		if !ok {
			continue
		}
		if entry.Status != taskstate.StatusReviewing || taskfsm.NormalizeExecutionPhase(entry.ExecutionState.Phase) != taskfsm.ExecutionPhaseReviewing {
			continue
		}
		return m.transitionToReview(inst)
	}
	return nil
}

// transitionToReview marks a plan as "reviewing", pauses the coder session,
// spawns a reviewer session with the reviewer profile, and returns the start cmd.
func (m *home) transitionToReview(coderInst *session.Instance) tea.Cmd {
	// Guard: transition via FSM before next tick re-reads disk, preventing double-spawn.
	planFile := coderInst.TaskFile
	if entry, ok := m.taskState.Entry(planFile); !ok || entry.Status != taskstate.StatusReviewing {
		if err := m.fsm.Transition(planFile, taskfsm.ImplementFinished); err != nil {
			log.WarningLog.Printf("could not set plan %q to reviewing: %v", planFile, err)
			// Mark complete to break the retry loop — checkPlanCompletion fires
			// every tick and would re-attempt this transition indefinitely.
			coderInst.ImplementationComplete = true
			return nil // FSM rejected — plan is not in implementing state, don't spawn reviewer.
		}
	}

	// Auto-pause the coder instance — its work is done.
	coderInst.ImplementationComplete = true
	if err := coderInst.Pause(); err != nil {
		log.WarningLog.Printf("could not pause coder instance for %q: %v", planFile, err)
	}

	return m.spawnReviewer(planFile)
}

// spawnReviewer creates and starts a reviewer session for the given plan,
// using the plan's shared worktree so it reviews the actual implementation branch.
// Does NOT perform any FSM transition — the caller is responsible for that.
// Solo agent plans are excluded — the user ends those manually.
func (m *home) spawnReviewer(planFile string) tea.Cmd {
	if !m.requireDaemonForAgents() {
		return nil
	}
	for _, inst := range m.nav.GetInstances() {
		if inst.TaskFile == planFile && inst.SoloAgent {
			return nil
		}
	}
	cycle, _ := m.taskState.ReviewCycle(planFile)
	planName := taskstate.DisplayName(planFile)
	spec := orchestration.BuildReviewerAgentSpec(planFile, m.taskStoreProject, cycle, m.latestReviewFeedback(planFile))
	title := spec.Title
	if m.hasLiveOrPendingInstance(planFile, session.AgentTypeReviewer, title) {
		return nil
	}
	if err := m.setExecutionState(planFile, taskstore.ExecutionState{
		Phase:           string(taskfsm.ExecutionPhaseReviewing),
		ActiveAgentType: session.AgentTypeReviewer,
	}); err != nil {
		log.WarningLog.Printf("could not persist reviewer execution state for %q: %v", planFile, err)
		return nil
	}

	// Kill any previous reviewer for this plan so the new session gets a fresh
	// tmux session instead of reattaching to a stale/errored one.
	m.killExistingPlanAgent(planFile, session.AgentTypeReviewer)

	// Resolve the plan's branch for the shared worktree.
	branch := m.taskBranch(planFile)
	if branch == "" {
		log.WarningLog.Printf("could not resolve branch for plan %q", planFile)
		return nil
	}

	reviewerInst, err := session.NewInstance(m.withRetentionOpts(session.InstanceOptions{
		Title:           title,
		Path:            m.activeRepoPath,
		Program:         m.programForAgent(session.AgentTypeReviewer),
		ExecutionMode:   m.executionModeForAgent(session.AgentTypeReviewer),
		SDKSpeedTier:    m.sdkSpeedTierForAgent(session.AgentTypeReviewer),
		SkipPermissions: m.skipPermissionsForAgent(session.AgentTypeReviewer),
		TaskFile:        planFile,
		AgentType:       session.AgentTypeReviewer,
		ReviewCycle:     spec.ReviewCycle,
		ClaudeNoFlicker: m.claudeNoFlicker(),
	}))
	if err != nil {
		log.WarningLog.Printf("could not create reviewer instance for %q: %v", planFile, err)
		return nil
	}
	reviewerInst.QueuedPrompt = spec.Prompt
	reviewerInst.SetStatus(session.Loading)

	m.addInstanceFinalizer(reviewerInst, m.nav.AddInstance(reviewerInst))
	m.nav.SelectInstance(reviewerInst) // sort-order safe, unlike index arithmetic

	m.audit(auditlog.EventAgentSpawned, fmt.Sprintf("spawned reviewer for %s", planName),
		auditlog.WithPlan(planFile),
		auditlog.WithInstance(reviewerInst.Title),
		auditlog.WithAgent(session.AgentTypeReviewer),
	)

	m.toastManager.Success(fmt.Sprintf("implementation complete → review started for %s", planName))

	shared := gitpkg.NewSharedTaskWorktree(m.activeRepoPath, branch)
	return func() tea.Msg {
		if err := shared.Setup(); err != nil {
			return instanceStartedMsg{instance: reviewerInst, err: err}
		}
		if err := m.syncSharedWorktreeScaffold(shared.GetWorktreePath()); err != nil {
			return instanceStartedMsg{instance: reviewerInst, err: err}
		}
		err := reviewerInst.StartInSharedWorktree(shared, branch)
		return instanceStartedMsg{instance: reviewerInst, err: err}
	}
}

// spawnMaster creates and starts the master readiness agent for the given plan.
// It persists verifying-compatible execution metadata (AgentTypeMaster), kills
// any existing master and reviewer instances, and launches the master in the
// plan's shared worktree.
func (m *home) spawnMaster(planFile string) tea.Cmd {
	if !m.requireDaemonForAgents() {
		return nil
	}
	for _, inst := range m.nav.GetInstances() {
		if inst.TaskFile == planFile && inst.SoloAgent {
			return nil
		}
	}
	planName := taskstate.DisplayName(planFile)
	reviewCycle := 0
	if m.taskState != nil {
		reviewCycle, _ = m.taskState.ReviewCycle(planFile)
	}
	spec := orchestration.BuildMasterAgentSpec(planFile, m.taskStoreProject, reviewCycle)
	title := spec.Title
	if m.hasLiveOrPendingInstance(planFile, session.AgentTypeMaster, title) {
		return nil
	}
	if err := m.setExecutionState(planFile, taskstore.ExecutionState{
		ActiveAgentType: session.AgentTypeMaster,
	}); err != nil {
		log.WarningLog.Printf("could not persist master execution state for %q: %v", planFile, err)
		return nil
	}

	// Kill any previous master and reviewer so the new session starts fresh.
	m.killExistingPlanAgent(planFile, session.AgentTypeMaster)
	m.killExistingPlanAgent(planFile, session.AgentTypeReviewer)

	// Resolve the plan's branch for the shared worktree.
	branch := m.taskBranch(planFile)
	if branch == "" {
		log.WarningLog.Printf("could not resolve branch for plan %q", planFile)
		return nil
	}

	masterInst, err := session.NewInstance(m.withRetentionOpts(session.InstanceOptions{
		Title:           title,
		Path:            m.activeRepoPath,
		Program:         m.programForAgent(session.AgentTypeMaster),
		ExecutionMode:   m.executionModeForAgent(session.AgentTypeMaster),
		SDKSpeedTier:    m.sdkSpeedTierForAgent(session.AgentTypeMaster),
		SkipPermissions: m.skipPermissionsForAgent(session.AgentTypeMaster),
		TaskFile:        planFile,
		AgentType:       session.AgentTypeMaster,
		ClaudeNoFlicker: m.claudeNoFlicker(),
	}))
	if err != nil {
		log.WarningLog.Printf("could not create master instance for %q: %v", planFile, err)
		return nil
	}
	masterInst.QueuedPrompt = spec.Prompt
	masterInst.SetStatus(session.Loading)

	m.addInstanceFinalizer(masterInst, m.nav.AddInstance(masterInst))
	m.nav.SelectInstance(masterInst)

	m.audit(auditlog.EventAgentSpawned, fmt.Sprintf("spawned master readiness agent for %s", planName),
		auditlog.WithPlan(planFile),
		auditlog.WithInstance(masterInst.Title),
		auditlog.WithAgent(session.AgentTypeMaster),
	)

	m.toastManager.Info(fmt.Sprintf("review approved → readiness check started for %s", planName))

	shared := gitpkg.NewSharedTaskWorktree(m.activeRepoPath, branch)
	return func() tea.Msg {
		if err := shared.Setup(); err != nil {
			return instanceStartedMsg{instance: masterInst, err: err}
		}
		if err := m.syncSharedWorktreeScaffold(shared.GetWorktreePath()); err != nil {
			return instanceStartedMsg{instance: masterInst, err: err}
		}
		err := masterInst.StartInSharedWorktree(shared, branch)
		return instanceStartedMsg{instance: masterInst, err: err}
	}
}

func withOpenCodeModelFlag(program, model string) string {
	model = normalizeOpenCodeModelID(model)
	if model == "" {
		return program
	}

	tokens := strings.Fields(program)
	if len(tokens) == 0 {
		return program
	}
	if filepath.Base(tokens[0]) != "opencode" {
		return program
	}

	for i, tok := range tokens {
		if tok == "--model" || tok == "-m" {
			if i+1 < len(tokens) {
				return program
			}
			return program
		}
		if strings.HasPrefix(tok, "--model=") {
			return program
		}
	}

	return program + " --model " + model
}

func buildHarnessAwareProgramCommand(profile config.AgentProfile) string {
	program := strings.TrimSpace(profile.Program)
	if program == "" {
		return ""
	}

	// Preserve inline commands verbatim rather than trying to reinterpret them.
	if strings.Contains(program, " ") {
		return profile.BuildCommand()
	}

	registry := harness.NewRegistry()
	adapter := registry.Get(filepath.Base(program))
	if adapter == nil {
		return profile.BuildCommand()
	}

	agentCfg := harness.AgentConfig{
		Harness:     adapter.Name(),
		Model:       profile.Model,
		Effort:      profile.Effort,
		Temperature: profile.Temperature,
		ExtraFlags:  profile.Flags,
	}
	flags := adapter.BuildFlags(agentCfg)
	if len(flags) == 0 {
		return program
	}
	return program + " " + strings.Join(flags, " ")
}

func (m *home) profileForAgent(agentType string) config.AgentProfile {
	if m.appConfig == nil {
		return config.AgentProfile{Program: m.program, ExecutionMode: config.ExecutionModeTmux}
	}

	var profile config.AgentProfile
	switch agentType {
	case session.AgentTypeCoder:
		profile = m.appConfig.ResolveProfile("implementing", m.program)
	case session.AgentTypePlanner:
		profile = m.appConfig.ResolveProfile("planning", m.program)
	case session.AgentTypeReviewer:
		profile = m.appConfig.ResolveProfile("quality_review", m.program)
	case session.AgentTypeFixer:
		profile = m.appConfig.ResolveProfile("fixer", m.program)
	case session.AgentTypeElaborator:
		profile = m.appConfig.ResolveProfile("elaborating", m.program)
	case session.AgentTypeMaster:
		profile = m.appConfig.ResolveProfile("readiness_review", m.program)
	default:
		if p, ok := m.appConfig.Profiles["chat"]; ok && p.Enabled && p.Program != "" {
			profile = p
		} else {
			return config.AgentProfile{Program: m.program, ExecutionMode: config.ExecutionModeTmux}
		}
	}
	profile.ExecutionMode = config.NormalizeExecutionMode(profile.ExecutionMode)
	return profile
}

// programForAgent resolves the program command for a given agent type
// (e.g. "coder", "planner") using the kasmos config profile. Falls back to
// m.program if no profile is configured.
//
// Commands are built via harness adapters so claude/codex pick up their
// configured model/effort flags automatically. opencode still relies on the
// project config, so only ad-hoc opencode sessions need the explicit --model
// append below.
func (m *home) programForAgent(agentType string) string {
	profile := m.profileForAgent(agentType)
	program := buildHarnessAwareProgramCommand(profile)
	if agentType == "" {
		return withOpenCodeModelFlag(program, profile.Model)
	}
	return program
}

func (m *home) executionModeForAgent(agentType string) session.ExecutionMode {
	return session.ExecutionMode(config.NormalizeExecutionMode(m.profileForAgent(agentType).ExecutionMode))
}

// skipPermissionsForAgent resolves the configured permission default for a
// given agent type. Local/ad-hoc spawns default to false (prompt) when
// the profile inherits, matching the TUI's current behaviour where
// non-daemon spawns always prompted. daemon-backed requests convert this
// via the SpawnSoloRequest.SkipPermissions pointer (see daemon_gate
// spawn paths).
func (m *home) skipPermissionsForAgent(agentType string) bool {
	return m.profileForAgent(agentType).ResolveSkipPermissions(false)
}

func (m *home) sdkSpeedTierForAgent(agentType string) string {
	return session.NormalizeSDKSpeedTier(m.profileForAgent(agentType).Tier)
}

// sdkTranscriptRetentionOptions returns the SDK transcript retention limits from
// the active app config. configured is true when the config is non-nil (so
// callers can distinguish "no config" from "config with zero/unlimited values").
func (m *home) sdkTranscriptRetentionOptions() (maxBytes, maxTurns int64, configured bool) {
	if m.appConfig == nil {
		return 0, 0, false
	}
	return m.appConfig.SDK.TranscriptMaxBytes, m.appConfig.SDK.TranscriptMaxTurns, true
}

// withRetentionOpts copies SDK transcript retention limits from config into opts
// and sets SDKTranscriptLimitsSet so the SDK renderer applies them. Call this
// on every InstanceOptions that may resolve to an SDK execution session.
func (m *home) withRetentionOpts(opts session.InstanceOptions) session.InstanceOptions {
	maxBytes, maxTurns, configured := m.sdkTranscriptRetentionOptions()
	if !configured {
		return opts
	}
	opts.SDKTranscriptLimitsSet = true
	opts.SDKTranscriptMaxBytes = maxBytes
	opts.SDKTranscriptMaxTurns = maxTurns
	return opts
}

// standaloneExecutionMode resolves the execution mode for a standalone ad-hoc
// agent spawn (KeyPrompt, new_instance, quickLaunchAgent, spawnAdHocAgent).
// It reads the profile for agentType, normalises the requested mode via
// config.NormalizeExecutionMode, and demotes sdk to tmux when the concrete
// program does not support the SDK transport.
func (m *home) standaloneExecutionMode(agentType, program string) session.ExecutionMode {
	profile := m.profileForAgent(agentType)
	normalised := config.NormalizeExecutionMode(profile.ExecutionMode)
	requested := session.ExecutionMode(normalised)
	return session.ResolveExecutionMode(requested, program)
}

func (m *home) standaloneSDKSpeedTier(agentType, program string) string {
	if m.standaloneExecutionMode(agentType, program) != session.ExecutionModeSDK {
		return ""
	}
	if common.DetectProgramKind(program) != common.ProgramCodex {
		return ""
	}
	return m.sdkSpeedTierForAgent(agentType)
}

// standaloneExecutionModeLimitError enforces the standalone tmux-session cap.
// SDK standalones do not create tmux sessions and are exempt from this limit.
func (m *home) standaloneExecutionModeLimitError(mode session.ExecutionMode) error {
	if session.NormalizeExecutionMode(mode) != session.ExecutionModeTmux {
		return nil
	}
	if m.tmuxSessionCount < GlobalInstanceLimit {
		return nil
	}
	return fmt.Errorf("you can't create more than %d instances (%d tmux sessions active)", GlobalInstanceLimit, m.tmuxSessionCount)
}

// claudeNoFlicker returns the configured CLAUDE_CODE_NO_FLICKER value.
// Defaults to false when the config is nil.
func (m *home) claudeNoFlicker() bool {
	if m.appConfig == nil {
		return false
	}
	return m.appConfig.ClaudeNoFlicker
}

func normalizeOpenCodeModelID(model string) string {
	model = strings.TrimSpace(model)
	if model == "" || strings.Contains(model, "/") {
		return model
	}
	if strings.HasPrefix(model, "claude-") {
		return "anthropic/" + model
	}
	return model
}

func (m *home) opencodeAgentConfigs() []harness.AgentConfig {
	if m.appConfig == nil {
		return nil
	}

	configsByRole := make(map[string]harness.AgentConfig)
	resolve := func(phase, fallbackRole string) {
		profile := m.appConfig.ResolveProfile(phase, m.program)
		if !isOpenCodeProfile(profile) || !profile.Enabled {
			return
		}

		role := fallbackRole
		if mappedRole, ok := m.appConfig.PhaseRoles[phase]; ok && mappedRole != "" {
			role = mappedRole
		}

		if role == "" {
			return
		}

		if profile.Model == "" && profile.Temperature == nil && profile.Effort == "" {
			return
		}

		programFields := strings.Fields(profile.Program)
		if len(programFields) == 0 {
			return
		}

		configsByRole[role] = harness.AgentConfig{
			Role:        role,
			Harness:     filepath.Base(programFields[0]),
			Model:       normalizeOpenCodeModelID(profile.Model),
			Temperature: profile.Temperature,
			Effort:      profile.Effort,
			Enabled:     profile.Enabled,
		}
	}

	orderedPhases := []struct {
		phase string
		role  string
	}{
		{phase: "implementing", role: session.AgentTypeCoder},
		{phase: "planning", role: session.AgentTypePlanner},
		{phase: "quality_review", role: session.AgentTypeReviewer},
		{phase: "fixer", role: session.AgentTypeFixer},
		{phase: "elaborating", role: session.AgentTypeElaborator},
	}
	for _, item := range orderedPhases {
		resolve(item.phase, item.role)
	}

	if len(configsByRole) == 0 {
		return nil
	}

	configs := make([]harness.AgentConfig, 0, len(configsByRole))
	for _, item := range orderedPhases {
		mappedRole := item.role
		if mappedRoleFromConfig, ok := m.appConfig.PhaseRoles[item.phase]; ok && mappedRoleFromConfig != "" {
			mappedRole = mappedRoleFromConfig
		}
		if cfg, ok := configsByRole[mappedRole]; ok {
			configs = append(configs, cfg)
		}
	}

	return configs
}

func scaffoldModelForHarness(harnessName, model string) string {
	if harnessName == "opencode" {
		return normalizeOpenCodeModelID(model)
	}
	return strings.TrimSpace(model)
}

func (m *home) scaffoldAgentConfigs() []harness.AgentConfig {
	if m.appConfig == nil || len(m.appConfig.Profiles) == 0 {
		return nil
	}

	harnessSet := map[string]struct{}{}
	for role, profile := range m.appConfig.Profiles {
		if role == "chat" {
			continue
		}
		fields := strings.Fields(profile.Program)
		if len(fields) == 0 {
			continue
		}
		harnessSet[filepath.Base(fields[0])] = struct{}{}
	}

	roles := make([]string, 0, len(m.appConfig.Profiles))
	for role := range m.appConfig.Profiles {
		roles = append(roles, role)
	}
	sort.Strings(roles)

	configs := make([]harness.AgentConfig, 0, len(roles))
	for _, role := range roles {
		profile := m.appConfig.Profiles[role]
		if !profile.Enabled {
			continue
		}

		fields := strings.Fields(profile.Program)
		if role != "chat" && len(fields) == 0 {
			continue
		}

		if role == "chat" {
			chatHarnesses := make([]string, 0, len(harnessSet))
			for harnessName := range harnessSet {
				chatHarnesses = append(chatHarnesses, harnessName)
			}
			sort.Strings(chatHarnesses)
			if len(chatHarnesses) == 0 && len(fields) > 0 {
				chatHarnesses = []string{filepath.Base(fields[0])}
			}
			for _, harnessName := range chatHarnesses {
				configs = append(configs, harness.AgentConfig{
					Role:        role,
					Harness:     harnessName,
					Model:       scaffoldModelForHarness(harnessName, profile.Model),
					Temperature: profile.Temperature,
					Effort:      profile.Effort,
					Enabled:     profile.Enabled,
					ExtraFlags:  profile.Flags,
				})
			}
			continue
		}

		harnessName := filepath.Base(fields[0])
		configs = append(configs, harness.AgentConfig{
			Role:        role,
			Harness:     harnessName,
			Model:       scaffoldModelForHarness(harnessName, profile.Model),
			Temperature: profile.Temperature,
			Effort:      profile.Effort,
			Enabled:     profile.Enabled,
			ExtraFlags:  profile.Flags,
		})
	}

	return configs
}

func (m *home) syncSharedWorktreeScaffold(worktreePath string) error {
	_, err := scaffold.SyncScaffold(worktreePath, m.scaffoldAgentConfigs())
	return err
}

func isOpenCodeProfile(profile config.AgentProfile) bool {
	fields := strings.Fields(profile.Program)
	if len(fields) == 0 {
		return false
	}

	return filepath.Base(fields[0]) == "opencode"
}

// killExistingPlanAgent finds and kills any existing instance for the given plan
// and agent type, removing it from both the UI list and persistence list.
//
// IMPORTANT: Instances are removed from both lists BEFORE killing the tmux
// session. This prevents the metadata-tick death-detection from seeing a dead
// reviewer in the list and auto-firing ReviewApproved (which would prematurely
// mark the plan as done).
func (m *home) killExistingPlanAgent(planFile, agentType string) {
	// First pass: identify matching instances by title.
	var titles []string
	for _, inst := range m.nav.GetInstances() {
		if inst.TaskFile != planFile {
			continue
		}
		if inst.AgentType == agentType {
			titles = append(titles, inst.Title)
		}
	}

	// Second pass: remove from both lists, then kill tmux.
	// Removal first ensures the death-detection tick cannot see the dead instance.
	for _, title := range titles {
		inst := m.nav.RemoveByTitle(title)
		m.removeFromAllInstances(title)
		// Invalidate the preview terminal cache so that a replacement instance
		// with the same title (e.g. a new "applying fixes" fixer) gets a fresh
		// terminal instead of showing stale output from the killed session.
		if inst != nil && inst.IdentityKey() == m.previewTerminalInstance {
			if m.previewTerminal != nil {
				m.previewTerminal.Close()
			}
			m.previewTerminal = nil
			m.previewTerminalInstance = ""
		}
		if inst != nil {
			if err := inst.Kill(); err != nil {
				log.WarningLog.Printf("could not kill old %s for %q: %v", agentType, planFile, err)
			}
		}
	}
}

// spawnFixerWithFeedback creates and starts a fixer session for the given plan,
// injecting reviewer feedback into a dedicated fixer prompt. Uses the plan's
// shared worktree so fixes are applied to the actual implementation branch.
// Does NOT perform any FSM transition — the caller is responsible for that.
func (m *home) spawnFixerWithFeedback(planFile, feedback string) tea.Cmd {
	if !m.requireDaemonForAgents() {
		return nil
	}
	feedback = strings.TrimSpace(feedback)
	if feedback == "" {
		feedback = m.latestReviewFeedback(planFile)
	}
	cycle, _ := m.taskState.ReviewCycle(planFile)
	planName := taskstate.DisplayName(planFile)
	spec := orchestration.BuildFixerAgentSpec(planFile, m.taskStoreProject, cycle, feedback)

	// Kill any previous fixer (and any legacy feedback-coder) for this plan so
	// the new session gets a fresh tmux session instead of reattaching to a
	// stale/errored one.
	m.killExistingPlanAgent(planFile, session.AgentTypeFixer)
	m.killExistingPlanAgent(planFile, session.AgentTypeCoder)

	// Clear the push-prompt dedup flag so this new coder round can trigger
	// the push dialog when it finishes.
	delete(m.coderPushPrompted, planFile)

	// Resolve the plan's branch for the shared worktree.
	branch := m.taskBranch(planFile)
	if branch == "" {
		log.WarningLog.Printf("could not resolve branch for plan %q", planFile)
		return nil
	}

	title := spec.Title
	if m.hasLiveOrPendingInstance(planFile, session.AgentTypeFixer, title) {
		return nil
	}
	if err := m.setExecutionState(planFile, taskstore.ExecutionState{
		Phase:           string(taskfsm.ExecutionPhaseFixing),
		ActiveAgentType: session.AgentTypeFixer,
	}); err != nil {
		log.WarningLog.Printf("could not persist fixer execution state for %q: %v", planFile, err)
		return nil
	}
	fixerInst, err := session.NewInstance(m.withRetentionOpts(session.InstanceOptions{
		Title:           title,
		Path:            m.activeRepoPath,
		Program:         m.programForAgent(session.AgentTypeFixer),
		ExecutionMode:   m.executionModeForAgent(session.AgentTypeFixer),
		SDKSpeedTier:    m.sdkSpeedTierForAgent(session.AgentTypeFixer),
		SkipPermissions: m.skipPermissionsForAgent(session.AgentTypeFixer),
		TaskFile:        planFile,
		AgentType:       session.AgentTypeFixer,
		ReviewCycle:     spec.ReviewCycle,
		ClaudeNoFlicker: m.claudeNoFlicker(),
	}))
	if err != nil {
		log.WarningLog.Printf("could not create fixer instance for %q: %v", planFile, err)
		return nil
	}
	fixerInst.QueuedPrompt = spec.Prompt
	fixerInst.SetStatus(session.Loading)

	m.addInstanceFinalizer(fixerInst, m.nav.AddInstance(fixerInst))
	m.nav.SelectInstance(fixerInst)

	detail := ""
	if feedback != "" {
		if len(feedback) > 200 {
			detail = feedback[:200] + "..."
		} else {
			detail = feedback
		}
	}
	m.audit(auditlog.EventAgentSpawned, fmt.Sprintf("spawned fixer with reviewer feedback for %s", planName),
		auditlog.WithPlan(planFile),
		auditlog.WithInstance(fixerInst.Title),
		auditlog.WithAgent(session.AgentTypeFixer),
		auditlog.WithDetail(detail),
	)

	m.toastManager.Info(fmt.Sprintf("review changes requested → applying fixes to %s", planName))

	shared := gitpkg.NewSharedTaskWorktree(m.activeRepoPath, branch)
	return func() tea.Msg {
		if err := shared.Setup(); err != nil {
			return instanceStartedMsg{instance: fixerInst, err: err}
		}
		if err := m.syncSharedWorktreeScaffold(shared.GetWorktreePath()); err != nil {
			return instanceStartedMsg{instance: fixerInst, err: err}
		}
		err := fixerInst.StartInSharedWorktree(shared, branch)
		return instanceStartedMsg{instance: fixerInst, err: err}
	}
}

// spawnElaborator creates and starts the architect pass for the given plan.
// The architect runs on the main branch since it updates the task store without
// editing implementation files. When it finishes, it writes the retained
// elaborator_finished sentinel consumed by the metadata tick to advance wave orchestration.
func (m *home) spawnElaborator(planFile string) (tea.Model, tea.Cmd) {
	if !m.requireDaemonForAgents() {
		return m, nil
	}
	planName := taskstate.DisplayName(planFile)
	description := ""
	if m.taskState != nil {
		if entry, ok := m.taskState.Entry(planFile); ok {
			description = entry.Description
		}
	}
	spec := orchestration.BuildArchitectAgentSpecWithOptions(planFile, m.taskStoreProject, orchestration.ArchitectPromptOptions{
		ParallelBaseline: m.parallelPlannerArchitectEnabled(),
		DescriptionHash:  orchestration.ArchitectBaselineDescriptionHash(description),
	})

	// Clear any stale elaborator_finished sentinel before starting a new architect pass.
	// Signal processing is edge-unaware, so a stale file would advance the current
	// orchestrator immediately instead of waiting for this architect run to finish.
	taskfsm.ClearElaborationSignal(m.signalsDir, planFile)

	inst, err := session.NewInstance(m.withRetentionOpts(session.InstanceOptions{
		Title:           spec.Title,
		Path:            m.activeRepoPath,
		Program:         m.programForAgent(session.AgentTypeElaborator),
		ExecutionMode:   m.executionModeForAgent(session.AgentTypeElaborator),
		SDKSpeedTier:    m.sdkSpeedTierForAgent(session.AgentTypeElaborator),
		SkipPermissions: m.skipPermissionsForAgent(session.AgentTypeElaborator),
		TaskFile:        planFile,
		AgentType:       session.AgentTypeElaborator,
		ClaudeNoFlicker: m.claudeNoFlicker(),
	}))
	if err != nil {
		return m, m.handleError(err)
	}
	inst.QueuedPrompt = spec.Prompt
	inst.SetStatus(session.Loading)
	inst.LoadingTotal = 6
	inst.LoadingMessage = "running architect pass..."
	if err := m.setExecutionState(planFile, taskstore.ExecutionState{
		Phase:           string(taskfsm.ExecutionPhaseArchitecting),
		ActiveAgentType: session.AgentTypeElaborator,
	}); err != nil {
		return m, m.handleError(err)
	}

	m.addInstanceFinalizer(inst, m.nav.AddInstance(inst))

	if err := scaffold.PatchWorktreeConfig(m.activeRepoPath, m.opencodeAgentConfigs()); err != nil {
		return m, m.handleError(err)
	}

	startCmd := func() tea.Msg {
		return instanceStartedMsg{instance: inst, err: inst.StartOnMainBranch()}
	}

	m.audit(auditlog.EventAgentSpawned, fmt.Sprintf("spawned architect for %s", planName),
		auditlog.WithPlan(planFile),
		auditlog.WithAgent(session.AgentTypeElaborator))

	m.toastManager.Info(fmt.Sprintf("running architect pass for '%s' before implementation", planName))
	return m, tea.Batch(tea.RequestWindowSize, startCmd, m.toastTickCmd())
}

func (m home) parallelPlannerArchitectEnabled() bool {
	return m.appConfig != nil && m.appConfig.ParallelPlannerArchitect
}

func (m home) architectBaselineCacheDir() string {
	return filepath.Join(m.activeRepoPath, ".kasmos", "cache")
}

func (m home) clearArchitectBaselineCmd(planFile string) tea.Cmd {
	if !m.parallelPlannerArchitectEnabled() || repoManagedByDaemon(m.activeRepoPath) {
		return nil
	}
	cacheDir := m.architectBaselineCacheDir()
	return func() tea.Msg {
		if err := orchestration.ClearArchitectBaseline(cacheDir, planFile); err != nil {
			return err
		}
		return nil
	}
}

func (m home) spawnArchitectBaseline(planFile, description string) (tea.Model, tea.Cmd) {
	if !m.requireDaemonForAgents() {
		return &m, nil
	}
	if repoManagedByDaemon(m.activeRepoPath) {
		return &m, nil
	}

	spec := orchestration.BuildArchitectBaselineAgentSpec(planFile, m.taskStoreProject, description)
	agentType := session.AgentTypeArchitectBaseline
	if err := scaffold.PatchWorktreeConfig(m.activeRepoPath, m.opencodeAgentConfigs()); err != nil {
		return &m, m.handleError(err)
	}
	m.killExistingPlanAgent(planFile, agentType)

	inst, err := session.NewInstance(m.withRetentionOpts(session.InstanceOptions{
		Title:           spec.Title,
		Path:            m.activeRepoPath,
		Program:         m.programForAgent(session.AgentTypeElaborator),
		ExecutionMode:   m.executionModeForAgent(session.AgentTypeElaborator),
		SDKSpeedTier:    m.sdkSpeedTierForAgent(session.AgentTypeElaborator),
		SkipPermissions: m.skipPermissionsForAgent(session.AgentTypeElaborator),
		TaskFile:        planFile,
		AgentType:       agentType,
		ClaudeNoFlicker: m.claudeNoFlicker(),
	}))
	if err != nil {
		return &m, m.handleError(err)
	}
	inst.QueuedPrompt = spec.Prompt
	inst.SetStatus(session.Loading)
	inst.LoadingTotal = 5
	inst.LoadingMessage = "Preparing session..."

	m.audit(auditlog.EventAgentSpawned, fmt.Sprintf("spawned architect-baseline for plan %s", taskstate.DisplayName(planFile)),
		auditlog.WithPlan(planFile),
		auditlog.WithInstance(spec.Title),
		auditlog.WithAgent(agentType),
	)

	m.addInstanceFinalizer(inst, m.nav.AddInstance(inst))
	m.nav.SelectInstance(inst)
	startCmd := func() tea.Msg {
		return instanceStartedMsg{instance: inst, err: inst.StartOnMainBranch()}
	}
	return &m, tea.Batch(tea.RequestWindowSize, startCmd)
}

// blueprintSkipThreshold returns the configured threshold for blueprint-skip mode.
// When the plan's total task count is <= this value, elaboration and wave orchestration
// are skipped and a single coder agent implements all tasks sequentially.
// Returns the default of 2 when appConfig is nil or not explicitly configured.
func (m *home) blueprintSkipThreshold() int {
	if m.appConfig == nil {
		return 2
	}
	return m.appConfig.BlueprintSkipThreshold()
}

// isSubtaskPersistedComplete checks whether the store has the given subtask
// marked as complete/done/closed. Used as a fallback when the TUI can't detect
// completion from live instance state (e.g. daemon-managed repos where the
// daemon marked the task complete before the TUI saw it working).
func (m *home) isSubtaskPersistedComplete(planFile string, taskNumber int) bool {
	if m.taskStore == nil || m.taskStoreProject == "" {
		return false
	}
	subtasks, err := m.taskStore.GetSubtasks(m.taskStoreProject, planFile)
	if err != nil {
		return false
	}
	for _, st := range subtasks {
		if st.TaskNumber == taskNumber {
			switch st.Status {
			case taskstore.SubtaskStatusComplete, taskstore.SubtaskStatusDone, taskstore.SubtaskStatusClosed:
				return true
			}
			return false
		}
	}
	return false
}

// clearWaveOrchestratorState removes any wave-orchestrator bookkeeping for the
// given plan from both the home model and the processor-backed signal gate.
// This is required before switching an implementing plan onto the single-agent
// blueprint-skip path so later implement_finished signals are not suppressed.
func (m *home) clearWaveOrchestratorState(planFile string) {
	delete(m.waveOrchestrators, planFile)
	if proc := m.ensureProcessor(); proc != nil {
		proc.SetWaveOrchestratorActive(planFile, false)
	}
}

func (m *home) setExecutionState(planFile string, state taskstore.ExecutionState) error {
	if m.taskState == nil {
		return fmt.Errorf("task state is not loaded")
	}
	return m.taskState.SetExecutionState(planFile, state)
}

func (m *home) clearExecutionState(planFile string) error {
	if m.taskState == nil {
		return fmt.Errorf("task state is not loaded")
	}
	return m.taskState.ClearExecutionState(planFile)
}

// hasActiveBlueprintSkipCoder reports whether a non-wave coder is already
// active for this plan. Used to prevent duplicate small-plan implement spawns
// when the user re-triggers "implement" while a single-agent implementation is
// already in flight.
func (m *home) hasActiveBlueprintSkipCoder(planFile string) bool {
	for _, inst := range m.nav.GetInstances() {
		if inst.TaskFile != planFile || inst.AgentType != session.AgentTypeCoder {
			continue
		}
		if inst.WaveNumber != 0 || inst.SoloAgent {
			continue
		}
		if inst.ImplementationComplete || inst.Exited || inst.Paused() {
			continue
		}
		return true
	}
	return false
}

// spawnBlueprintSkipAgent transitions the plan to implementing and spawns a single
// coder agent to implement all tasks sequentially. Used when the plan's task count
// is at or below blueprintSkipThreshold(). No WaveOrchestrator is created — the
// agent signals implement_finished directly, which triggers the existing review flow.
func (m *home) spawnBlueprintSkipAgent(planFile string, plan *taskparser.Plan) (tea.Model, tea.Cmd) {
	if err := m.fsmSetImplementing(planFile); err != nil {
		return m, m.handleError(err)
	}
	m.loadTaskState()
	if err := m.setExecutionState(planFile, taskstore.ExecutionState{
		Phase:           string(taskfsm.ExecutionPhaseSingleAgentImplementing),
		ActiveAgentType: session.AgentTypeCoder,
	}); err != nil {
		return m, m.handleError(err)
	}
	m.updateSidebarTasks()

	totalTasks := 0
	for _, wave := range plan.Waves {
		totalTasks += len(wave.Tasks)
	}
	m.toastManager.Info(fmt.Sprintf("small plan (%d tasks) - running single agent", totalTasks))

	model, cmd := m.spawnTaskAgent(planFile, "implement", orchestration.BuildBlueprintSkipPrompt(planFile, plan, m.taskStoreProject))
	return model, tea.Batch(cmd, m.toastTickCmd())
}

// viewSelectedPlan renders the selected plan's markdown in the preview pane.
// The rendered output is cached; on cache miss the glamour render runs async
// via a tea.Cmd so the UI stays responsive.
func (m *home) viewSelectedPlan() (tea.Model, tea.Cmd) {
	planFile := m.nav.GetSelectedPlanFile()
	if planFile == "" {
		return m, nil
	}

	// Cache hit — reuse previously rendered content (instant).
	if planFile == m.cachedPlanFile && m.cachedPlanRendered != "" {
		m.previewRequested = false
		m.tabbedWindow.SetDocumentContent(m.cachedPlanRendered)
		return m, nil
	}

	// Cache miss — render async so the UI doesn't freeze.
	previewWidth, _ := m.tabbedWindow.GetPreviewSize()
	wordWrap := previewWidth - 4
	if wordWrap < 40 {
		wordWrap = 40
	}

	return m, func() tea.Msg {
		data, err := m.taskStore.GetContent(m.taskStoreProject, planFile)
		if err != nil {
			return planRenderedMsg{err: fmt.Errorf("could not read plan %s: %w", planFile, err)}
		}

		renderer, err := glamour.NewTermRenderer(
			glamour.WithStandardStyle("dark"),
			glamour.WithWordWrap(wordWrap),
		)
		if err != nil {
			return planRenderedMsg{err: fmt.Errorf("could not create markdown renderer: %w", err)}
		}

		rendered, err := renderer.Render(data)
		if err != nil {
			return planRenderedMsg{err: fmt.Errorf("could not render markdown: %w", err)}
		}

		return planRenderedMsg{planFile: planFile, rendered: rendered}
	}
}

// createTaskEntry creates a new plan entry in the store.
func (m *home) createTaskEntry(name, description, topic string) error {
	if m.taskState == nil || !m.taskState.HasStore() {
		if m.taskStore == nil {
			return fmt.Errorf("task store not configured")
		}
		ps, err := taskstate.Load(m.taskStore, m.taskStoreProject, m.taskStateDir)
		if err != nil {
			return err
		}
		m.taskState = ps
	}

	slug := slugifyPlanName(name)
	filename := slug
	branch := "plan/" + slug
	if err := m.taskState.Create(filename, description, branch, topic, time.Now().UTC()); err != nil {
		if m.toastManager != nil {
			m.toastManager.Error("task store error: " + err.Error())
		}
		return err
	}
	if err := m.taskState.SetContent(filename, renderPlanStub(name, description, filename)); err != nil {
		if m.toastManager != nil {
			m.toastManager.Error("task store error: " + err.Error())
		}
		return err
	}
	m.audit(auditlog.EventPlanCreated, "created plan", auditlog.WithPlan(filename))
	m.updateSidebarTasks()
	return nil
}

// slugifyPlanName converts a plan name to a URL-safe slug.
func slugifyPlanName(name string) string {
	name = strings.ToLower(strings.TrimSpace(name))
	name = regexp.MustCompile(`[^a-z0-9]+`).ReplaceAllString(name, "-")
	return strings.Trim(name, "-")
}

// buildPlanFilename derives the plan filename from a human name.
// "Auth Refactor" → "auth-refactor"
func buildPlanFilename(name string, _ time.Time) string {
	slug := slugifyPlanName(name)
	if slug == "" {
		slug = "plan"
	}
	return slug
}

// renderPlanStub returns the initial markdown content for a new plan file.
func renderPlanStub(name, description, filename string) string {
	return fmt.Sprintf("# %s\n\n## Context\n\n%s\n\n## Notes\n\n- Created by kas lifecycle flow\n- Plan file: %s\n", name, description, filename)
}

// createPlanRecord registers the plan in the store.
func (m *home) createPlanRecord(planFile, description, branch string, now time.Time) error {
	if m.taskState == nil || !m.taskState.HasStore() {
		if m.taskStore == nil {
			return fmt.Errorf("task store not configured")
		}
		ps, err := taskstate.Load(m.taskStore, m.taskStoreProject, m.taskStateDir)
		if err != nil {
			return err
		}
		m.taskState = ps
	}
	if err := m.taskState.Register(planFile, description, branch, now); err != nil {
		if m.toastManager != nil {
			m.toastManager.Error("task store error: " + err.Error())
		}
		return err
	}
	return nil
}

// finalizePlanCreation writes the plan stub content to the store, registers it,
// and creates the feature branch. Called at the end of the plan creation wizard.
func (m *home) finalizePlanCreation(name, description string) error {
	now := time.Now().UTC()
	planFile := buildPlanFilename(name, now)
	branch := gitpkg.TaskBranchFromFile(planFile)
	content := renderPlanStub(name, description, planFile)
	if err := m.createPlanRecord(planFile, description, branch, now); err != nil {
		return err
	}
	if err := m.taskState.SetContent(planFile, content); err != nil {
		return err
	}
	if err := gitpkg.EnsureTaskBranch(m.activeRepoPath, branch); err != nil {
		return err
	}

	m.loadTaskState()
	m.updateSidebarTasks()
	return nil
}

func (m *home) importClickUpTask(task *clickup.Task) (tea.Model, tea.Cmd) {
	if task == nil {
		m.toastManager.Error("clickup fetch failed: empty task payload")
		return m, m.toastTickCmd()
	}

	filename := clickup.ScaffoldFilename(task.Name)

	if m.taskState == nil {
		m.loadTaskState()
	}
	if m.taskState == nil {
		m.toastManager.Error("failed to register imported plan: plan state unavailable")
		return m, m.toastTickCmd()
	}

	filename = dedupePlanFilenameInState(m.taskState, filename)

	scaffold := clickup.ScaffoldPlan(*task)

	branch := gitpkg.TaskBranchFromFile(filename)
	if err := m.taskState.Register(filename, task.Name, branch, time.Now()); err != nil {
		m.toastManager.Error("failed to register imported plan: " + err.Error())
		return m, m.toastTickCmd()
	}
	if err := m.taskState.SetContent(filename, scaffold); err != nil {
		m.toastManager.Error("failed to save imported plan content: " + err.Error())
		return m, m.toastTickCmd()
	}
	if task.ID != "" {
		if err := m.taskState.SetClickUpTaskID(filename, task.ID); err != nil {
			log.WarningLog.Printf("importClickUpTask: failed to set clickup task id for %q: %v", filename, err)
		}
	}

	if err := m.fsm.Transition(filename, taskfsm.PlanStart); err != nil {
		log.WarningLog.Printf("clickup import transition failed for %q: %v", filename, err)
	}

	m.loadTaskState()
	m.updateSidebarTasks()

	prompt := fmt.Sprintf(`Analyze this imported ClickUp task. The task details and subtasks are included as reference in the plan.

Determine if the task is well-specified enough for implementation or needs further analysis. Write a proper implementation plan with ## Wave sections, task breakdowns, architecture notes, and tech stack. Use the ClickUp subtasks as a starting point but reorganize into waves based on dependencies.

Retrieve the current plan content with: kas task show %s`, filename)

	m.toastManager.Success("imported! spawning planner...")
	model, cmd := m.spawnPlannerWithOptionalBaseline(filename, prompt, task.Description)
	if cmd == nil {
		return model, m.toastTickCmd()
	}
	return model, tea.Batch(cmd, m.toastTickCmd())
}

func dedupePlanFilename(plansDir, filename string) string {
	planPath := filepath.Join(plansDir, filename)
	if _, err := os.Stat(planPath); os.IsNotExist(err) {
		return filename
	}

	for i := 2; i < 100; i++ {
		candidate := fmt.Sprintf("%s-%d", filename, i)
		if _, err := os.Stat(filepath.Join(plansDir, candidate)); os.IsNotExist(err) {
			return candidate
		}
	}

	return filename
}

func dedupePlanFilenameInState(ps *taskstate.TaskState, filename string) string {
	if ps == nil {
		return filename
	}
	if _, ok := ps.Entry(filename); !ok {
		return filename
	}

	for i := 2; i < 100; i++ {
		candidate := fmt.Sprintf("%s-%d", filename, i)
		if _, ok := ps.Entry(candidate); !ok {
			return candidate
		}
	}

	return filename
}

// promptPushBranchThenAdvance shows a confirmation overlay asking the user to
// push the implementation branch, then advances the plan to reviewing and
// spawns a reviewer agent via coderCompleteMsg.
func (m *home) promptPushBranchThenAdvance(inst *session.Instance) tea.Cmd {
	capturedPlanFile := inst.TaskFile
	// Mark as prompted so the metadata tick doesn't re-trigger the dialog
	// while the user is deciding or after they dismiss it.
	if m.coderPushPrompted == nil {
		m.coderPushPrompted = make(map[string]bool)
	}
	m.coderPushPrompted[capturedPlanFile] = true
	message := fmt.Sprintf("[!] implementation finished for '%s'. push branch now?", taskstate.DisplayName(capturedPlanFile))
	pushAction := func() tea.Msg {
		worktree, err := inst.GetGitWorktree()
		if err == nil {
			_ = worktree.Push(false)
		}
		return coderCompleteMsg{planFile: capturedPlanFile}
	}
	return m.confirmAction(message, func() tea.Msg { return pushAction() })
}

// taskBranch resolves the branch name for a plan, backfilling if needed.
func (m *home) taskBranch(planFile string) string {
	if m.taskState == nil {
		return ""
	}
	entry, ok := m.taskState.Entry(planFile)
	if !ok {
		return ""
	}
	if entry.Branch == "" {
		entry.Branch = gitpkg.TaskBranchFromFile(planFile)
		_ = m.taskState.SetBranch(planFile, entry.Branch)
	}
	return entry.Branch
}

// buildPlanningPrompt returns the initial prompt for a planner agent session.
// It delegates to orchestration.BuildPlannerPrompt so the TUI and daemon
// (via SpawnPlannerAction) share the exact same planner prompt.
func buildPlanningPrompt(planFile, planName, description, project string) string {
	return orchestration.BuildPlannerPrompt(planFile, planName, description, project)
}

// buildImplementPrompt returns the prompt for a coder agent session.
// Agents retrieve plan content from the task store via MCP or CLI and execute all tasks.
func buildImplementPrompt(planFile, project string) string {
	return fmt.Sprintf(
		"Implement %s. Retrieve the full plan with MCP `task_show` (filename: \"%[1]s\", project: \"%[2]s\") and execute all tasks sequentially. "+
			"If MCP is unavailable, fall back to `kas task show %[1]s`. "+
			"Use rg/sd/fd instead of grep/sed/find. Scope tests with -run TestName. Do not load skills.",
		planFile, project,
	)
}

// buildSoloPrompt returns a minimal prompt for a solo agent session.
// If planFile is non-empty, it references the plan via MCP task_show with project, or CLI fallback.
func buildSoloPrompt(planName, description, planFile, project string) string {
	const rules = "Commit with task number in message. Use rg/sd/fd instead of grep/sed/find. Scope tests with -run TestName. Do not load skills."
	if planFile != "" {
		return fmt.Sprintf(
			"Implement %s. Goal: %s. Retrieve the full plan with MCP `task_show` (filename: %q, project: %q) — fall back to `kas task show %s`. %s",
			planName, description, planFile, project, planFile, rules,
		)
	}
	return fmt.Sprintf("Implement %s. Goal: %s. %s", planName, description, rules)
}

// buildModifyTaskPrompt returns the prompt for modifying an existing plan.
func buildModifyTaskPrompt(planFile, project string) string {
	return fmt.Sprintf(
		"Modify existing plan %s. Retrieve current content with MCP `task_show` (filename: %q, project: %q) — fall back to `kas task show %s`. "+
			"Keep the same filename and update only what changed.",
		planFile, planFile, project, planFile,
	)
}

// agentTypeForSubItem maps a sidebar stage name to the corresponding AgentType constant.
func agentTypeForSubItem(action string) (string, bool) {
	switch action {
	case "plan":
		return session.AgentTypePlanner, true
	case "implement", "solo":
		return session.AgentTypeCoder, true
	case "review":
		return session.AgentTypeReviewer, true
	default:
		return "", false
	}
}

func (m *home) nextPlaceholderName() string {
	maxUsed := 0
	collect := func(inst *session.Instance) {
		if inst == nil {
			return
		}
		matches := quickLaunchPlaceholderTitleRE.FindStringSubmatch(inst.Title)
		if len(matches) != 2 {
			return
		}
		n, err := strconv.Atoi(matches[1])
		if err != nil || n < 1 {
			return
		}
		if n > maxUsed {
			maxUsed = n
		}
	}

	for _, inst := range m.nav.GetInstances() {
		collect(inst)
	}
	for _, inst := range m.allInstances {
		collect(inst)
	}
	for inst := range m.instanceFinalizers {
		collect(inst)
	}

	prefix := filepath.Base(m.activeRepoPath)
	return fmt.Sprintf("%s-agent-%d", prefix, maxUsed+1)
}

func (m *home) quickLaunchTitleSyncCmd(inst *session.Instance) tea.Cmd {
	if inst == nil ||
		inst.TaskFile != "" ||
		(inst.AgentType != "" && inst.AgentType != session.AgentTypeFixer) ||
		!strings.HasSuffix(inst.Program, "opencode") ||
		!quickLaunchPlaceholderTitleRE.MatchString(inst.Title) ||
		strings.TrimSpace(inst.Path) == "" {
		return nil
	}

	return func() tea.Msg {
		delay := quickLaunchTitleSyncInitialDelay
		deadline := time.Now().Add(quickLaunchTitleSyncTimeout)
		for {
			title, err := readQuickLaunchSessionTitle(inst.Path, inst.CreatedAt)
			if err != nil {
				log.WarningLog.Printf("quick launch title sync: %v", err)
				return nil
			}

			title = strings.TrimSpace(title)
			if title != "" && !strings.HasPrefix(title, "kas: ") {
				return instanceTitleSyncMsg{instance: inst, newTitle: title}
			}

			if !time.Now().Before(deadline) {
				return nil
			}
			time.Sleep(delay)

			delay = time.Duration(float64(delay) * quickLaunchTitleSyncMultiplier)
			if delay > quickLaunchTitleSyncMaxDelay {
				delay = quickLaunchTitleSyncMaxDelay
			}
		}
	}
}

func slugify(title string) string {
	title = strings.ToLower(strings.TrimSpace(title))
	title = quickLaunchDisplayTitleRE.ReplaceAllString(title, "-")
	title = strings.Trim(title, "-")
	if len(title) > 40 {
		title = strings.Trim(title[:40], "-")
	}
	return title
}

func (m *home) quickLaunchAgent() (tea.Model, tea.Cmd) {
	if !m.requireDaemonForAgents() {
		return m, nil
	}
	fixerProgram := m.programForAgent(session.AgentTypeFixer)
	requestedMode := m.standaloneExecutionMode(session.AgentTypeFixer, fixerProgram)
	if err := m.standaloneExecutionModeLimitError(requestedMode); err != nil {
		return m, m.handleError(err)
	}

	title := m.nextPlaceholderName()
	inst, err := session.NewInstance(m.withRetentionOpts(session.InstanceOptions{
		Title:           title,
		Path:            m.activeRepoPath,
		Program:         fixerProgram,
		ExecutionMode:   requestedMode,
		SDKSpeedTier:    m.sdkSpeedTierForAgent(session.AgentTypeFixer),
		SkipPermissions: m.skipPermissionsForAgent(session.AgentTypeFixer),
		AgentType:       session.AgentTypeFixer,
		ClaudeNoFlicker: m.claudeNoFlicker(),
	}))
	if err != nil {
		return m, m.handleError(err)
	}

	inst.SetStatus(session.Loading)
	inst.LoadingTotal = 5
	inst.LoadingMessage = "preparing session..."

	m.state = stateDefault
	m.menu.SetState(ui.StateDefault)
	m.addInstanceFinalizer(inst, m.nav.AddInstance(inst))
	m.nav.SelectInstance(inst)

	var startCmd tea.Cmd
	if session.NormalizeExecutionMode(inst.ExecutionMode) == session.ExecutionModeSDK &&
		repoManagedByDaemon(m.activeRepoPath) {
		capturedInst := inst
		capturedProject := m.taskStoreProject
		startCmd = func() tea.Msg {
			skip := capturedInst.SkipPermissions
			req := api.SpawnSoloRequest{
				Title:           capturedInst.Title,
				Program:         capturedInst.Program,
				AgentType:       capturedInst.AgentType,
				SDKSpeedTier:    capturedInst.SDKSpeedTier,
				SkipPermissions: &skip,
			}
			return instanceStartedMsg{instance: capturedInst, err: spawnSoloWithDaemon(capturedProject, req)}
		}
	} else {
		startCmd = func() tea.Msg {
			return instanceStartedMsg{instance: inst, err: quickLaunchStartOnMain(inst)}
		}
	}
	return m, tea.Batch(tea.RequestWindowSize, startCmd)
}

var builtInSpawnHarnessLabels = []string{"claude", "codex", "opencode"}

func normalizeSpawnProgramLabel(program string) string {
	switch common.DetectProgramKind(program) {
	case common.ProgramClaude:
		return "claude"
	case common.ProgramCodex:
		return "codex"
	case common.ProgramOpenCode:
		return "opencode"
	default:
		return common.ProgramBase(program)
	}
}

func (m *home) configuredSpawnProgramsByLabel() map[string]string {
	programs := make(map[string]string)
	if m.appConfig != nil {
		if dp := strings.TrimSpace(m.appConfig.DefaultProgram); dp != "" {
			if label := normalizeSpawnProgramLabel(dp); label != "" {
				programs[label] = dp
			}
		}
		for _, profile := range m.appConfig.Profiles {
			if !profile.Enabled {
				continue
			}
			if program := strings.TrimSpace(profile.Program); program != "" {
				if label := normalizeSpawnProgramLabel(program); label != "" {
					if _, exists := programs[label]; !exists {
						programs[label] = program
					}
				}
			}
		}
	}
	if len(programs) == 0 {
		program := strings.TrimSpace(m.program)
		if program == "" {
			program = "claude"
		}
		if label := normalizeSpawnProgramLabel(program); label != "" {
			programs[label] = program
		}
	}
	return programs
}

// availableSpawnPrograms returns the clean picker labels for ad-hoc spawn targets.
// The three built-in harnesses are always shown, and any additional configured
// non-built-in program labels are appended after them.
func (m *home) availableSpawnPrograms() []string {
	configured := m.configuredSpawnProgramsByLabel()
	seen := make(map[string]struct{}, len(configured)+len(builtInSpawnHarnessLabels))
	programs := make([]string, 0, len(configured)+len(builtInSpawnHarnessLabels))
	for _, label := range builtInSpawnHarnessLabels {
		seen[label] = struct{}{}
		programs = append(programs, label)
	}
	var extras []string
	for label := range configured {
		if _, exists := seen[label]; exists {
			continue
		}
		extras = append(extras, label)
	}
	sort.Strings(extras)
	programs = append(programs, extras...)
	return programs
}

func (m *home) defaultSpawnProgramLabel() string {
	if m.appConfig != nil {
		if label := normalizeSpawnProgramLabel(m.appConfig.DefaultProgram); label != "" {
			return label
		}
	}
	if label := normalizeSpawnProgramLabel(m.program); label != "" {
		return label
	}
	return "claude"
}

func (m *home) resolveSpawnProgram(label string) string {
	configured := m.configuredSpawnProgramsByLabel()
	if program, ok := configured[label]; ok {
		return program
	}
	return label
}

// resetPendingSpawnFlow clears all pending spawn state so an abandoned picker
// or form never leaks an sdk selection into the next spawn.
func (m *home) resetPendingSpawnFlow() {
	m.pendingSpawnProgram = ""
	m.pendingSpawnExecutionMode = ""
	m.pendingSpawnSpeedTier = ""
}

// showSpawnExecutionModePicker sets the pending spawn program and opens the
// execution-mode picker, preselecting the default resolved mode for the program.
func (m *home) showSpawnExecutionModePicker(program string) {
	m.pendingSpawnProgram = program
	m.pendingSpawnExecutionMode = m.standaloneExecutionMode(session.AgentTypeMaster, program)
	var options []string
	defaultSelection := string(m.pendingSpawnExecutionMode)
	if common.DetectProgramKind(program) == common.ProgramCodex {
		options = []string{"tmux", "sdk", "sdk-fast"}
		if m.pendingSpawnExecutionMode == session.ExecutionModeSDK &&
			m.standaloneSDKSpeedTier(session.AgentTypeMaster, program) == "fast" {
			defaultSelection = "sdk-fast"
		}
	} else {
		options = []string{"tmux", "sdk"}
	}
	picker := overlay.NewPickerOverlay("execution mode", options)
	picker.SetSelectedValue(defaultSelection)
	m.overlays.Show(picker)
	m.state = stateSpawnExecutionModePicker
}

// continueSpawnAgentFlow is called after a harness program has been selected
// (or when only one program is available). When the program supports the SDK
// transport it shows the execution-mode picker; otherwise it sets tmux mode and
// proceeds directly to the spawn name form.
func (m *home) continueSpawnAgentFlow(selection string) (tea.Model, tea.Cmd) {
	program := m.resolveSpawnProgram(selection)
	if sdk.SupportsProgram(program) {
		m.showSpawnExecutionModePicker(program)
		return m, nil
	}
	return m.continueSpawnAgentFlowWithMode(program, session.ExecutionModeTmux, "")
}

// continueSpawnAgentFlowWithMode validates the selected execution mode against
// the tmux-session limit, then proceeds to the spawn form.
func (m *home) continueSpawnAgentFlowWithMode(program string, mode session.ExecutionMode, speedTier string) (tea.Model, tea.Cmd) {
	if err := m.standaloneExecutionModeLimitError(mode); err != nil {
		m.resetPendingSpawnFlow()
		m.state = stateDefault
		m.menu.SetState(ui.StateDefault)
		return m, m.handleError(err)
	}
	m.pendingSpawnExecutionMode = mode
	m.pendingSpawnSpeedTier = speedTier
	m.showSpawnAgentForm(program)
	return m, nil
}

// beginSpawnAgentFlow is the shared entry point for both the S-key binding and the
// "spawn_agent" launcher action. It either shows the harness picker (multiple
// programs available) or continues to the next step (exactly one program available).
func (m *home) beginSpawnAgentFlow() (tea.Model, tea.Cmd) {
	programs := m.availableSpawnPrograms()
	picker := overlay.NewPickerOverlay("select harness", programs)
	picker.SetSelectedValue(m.defaultSpawnProgramLabel())
	m.overlays.Show(picker)
	m.state = stateSpawnHarnessPicker
	return m, nil
}

// showSpawnAgentForm stores the selected harness program and opens the spawn name form.
// When the pending execution mode is SDK, footer hints are shown (including speed tier
// when set). Multiple hints are joined with a newline.
func (m *home) showSpawnAgentForm(program string) {
	m.pendingSpawnProgram = program
	m.state = stateSpawnAgent
	formOverlay := overlay.NewSpawnFormOverlay("spawn agent", 60)
	var hints []string
	if m.pendingSpawnSpeedTier == "fast" {
		hints = append(hints, "fast tier consumes 2x usage")
	}
	if len(hints) > 0 {
		formOverlay.SetFooterHint(strings.Join(hints, "\n"))
	}
	m.overlays.Show(formOverlay)
}

// newNamedAgentInstance builds the interactive ad-hoc session used by the
// launcher "new instance" flow and the explicit spawn-agent form.
//
// These sessions launch the given program as a master agent. If program is empty,
// "claude" is used as a safe default so existing callers remain unaffected.
// requestedMode is the pre-resolved execution mode from standaloneExecutionMode;
// NewInstance will call ResolveExecutionMode again on the final program, which is
// idempotent.
func (m *home) newNamedAgentInstance(title, path, program string, requestedMode session.ExecutionMode, speedTier string) (*session.Instance, error) {
	if strings.TrimSpace(path) == "" {
		path = m.activeRepoPath
	}
	p := strings.TrimSpace(program)
	if p == "" {
		p = "claude"
	}
	if err := m.standaloneExecutionModeLimitError(session.ResolveExecutionMode(requestedMode, p)); err != nil {
		return nil, err
	}
	return session.NewInstance(m.withRetentionOpts(session.InstanceOptions{
		Title:           title,
		Path:            path,
		Program:         p,
		ExecutionMode:   requestedMode,
		SDKSpeedTier:    speedTier,
		SkipPermissions: m.skipPermissionsForAgent(session.AgentTypeMaster),
		AgentType:       session.AgentTypeMaster,
		ClaudeNoFlicker: m.claudeNoFlicker(),
	}))
}

// spawnAdHocAgent creates and starts an ad-hoc agent session (no plan, no lifecycle).
// branch and workPath are optional overrides - empty strings use defaults.
// program selects which harness binary to launch; empty falls back to "claude".
// requestedMode is the execution mode selected by the user (or resolved via
// standaloneExecutionMode); the actual inst.ExecutionMode may differ if the
// program does not support the requested transport.
func (m *home) spawnAdHocAgent(name, branch, workPath, program string, requestedMode session.ExecutionMode, speedTier string) (tea.Model, tea.Cmd) {
	if !m.requireDaemonForAgents() {
		return m, nil
	}
	path := m.activeRepoPath
	if workPath != "" {
		path = workPath
	}

	inst, err := m.newNamedAgentInstance(name, path, program, requestedMode, speedTier)
	if err != nil {
		return m, m.handleError(err)
	}

	inst.SetStatus(session.Loading)
	inst.LoadingTotal = 8
	inst.LoadingMessage = "preparing session..."

	m.state = stateDefault
	m.menu.SetState(ui.StateDefault)

	var startCmd tea.Cmd
	if session.NormalizeExecutionMode(inst.ExecutionMode) == session.ExecutionModeSDK &&
		repoManagedByDaemon(m.activeRepoPath) {
		capturedInst := inst
		capturedProject := m.taskStoreProject
		capturedBranch := branch
		capturedWorkPath := workPath
		startCmd = func() tea.Msg {
			skip := capturedInst.SkipPermissions
			req := api.SpawnSoloRequest{
				Title:           capturedInst.Title,
				Program:         capturedInst.Program,
				AgentType:       capturedInst.AgentType,
				Branch:          capturedBranch,
				WorkPath:        capturedWorkPath,
				SDKSpeedTier:    capturedInst.SDKSpeedTier,
				SkipPermissions: &skip,
			}
			return instanceStartedMsg{instance: capturedInst, err: spawnSoloWithDaemon(capturedProject, req)}
		}
	} else {
		switch {
		case workPath != "" && branch == "":
			// Path override only - run in-place on main branch (no worktree)
			startCmd = func() tea.Msg {
				return instanceStartedMsg{instance: inst, err: inst.StartOnMainBranch()}
			}

		case branch != "":
			// Branch override - create worktree on specified branch
			startCmd = func() tea.Msg {
				return instanceStartedMsg{instance: inst, err: inst.StartOnBranch(branch)}
			}

		default:
			// No overrides - run in-place on current branch (no worktree)
			startCmd = func() tea.Msg {
				return instanceStartedMsg{instance: inst, err: inst.StartOnMainBranch()}
			}
		}
	}

	m.audit(auditlog.EventAgentSpawned, fmt.Sprintf("spawned %s agent: %s", session.AgentTypeMaster, name),
		auditlog.WithInstance(name),
		auditlog.WithAgent(session.AgentTypeMaster),
		auditlog.WithExecutionMode(string(inst.ExecutionMode)),
		auditlog.WithSpeedTier(inst.SDKSpeedTier),
	)

	m.addInstanceFinalizer(inst, m.nav.AddInstance(inst))
	m.nav.SelectInstance(inst)
	return m, tea.Batch(tea.RequestWindowSize, startCmd)
}

// spawnTaskAgent creates and starts an agent session for the given plan and action.
func (m *home) spawnTaskAgent(planFile, action, prompt string) (tea.Model, tea.Cmd) {
	if !m.requireDaemonForAgents() {
		return m, nil
	}
	entry, ok := m.taskState.Entry(planFile)
	if !ok {
		return m, m.handleError(fmt.Errorf("task not found: %s", planFile))
	}

	agentType, ok := agentTypeForSubItem(action)
	if !ok {
		return m, m.handleError(fmt.Errorf("unknown plan action: %s", action))
	}

	// Kill any existing instance of the same type for this plan to prevent
	// duplicates (e.g. user triggers "start review" when a reviewer already exists).
	if agentType == session.AgentTypeReviewer || agentType == session.AgentTypePlanner {
		m.killExistingPlanAgent(planFile, agentType)
	}

	planName := taskstate.DisplayName(planFile)
	title := planName + "-" + action
	// reviewCycle is resolved once and reused for both the title and the instance field.
	var reviewCycle int
	if action == "solo" {
		// Solo sessions run on main branch, so their tmux session names must stay
		// unique to avoid accidentally reattaching to another solo session.
		title = planName + "-solo"
	} else if action == "review" {
		// Use the shared reviewer builder so every manual and automated review
		// round uses the same title/cycle numbering.
		spec := orchestration.BuildReviewerAgentSpec(planFile, m.taskStoreProject, entry.ReviewCycle, m.latestReviewFeedback(planFile))
		reviewCycle = spec.ReviewCycle
		title = spec.Title
		if strings.TrimSpace(prompt) == "" {
			prompt = spec.Prompt
		}
	}
	inst, err := session.NewInstance(m.withRetentionOpts(session.InstanceOptions{
		Title:           title,
		Path:            m.activeRepoPath,
		Program:         m.programForAgent(agentType),
		ExecutionMode:   m.executionModeForAgent(agentType),
		SDKSpeedTier:    m.sdkSpeedTierForAgent(agentType),
		SkipPermissions: m.skipPermissionsForAgent(agentType),
		TaskFile:        planFile,
		AgentType:       agentType,
		ClaudeNoFlicker: m.claudeNoFlicker(),
	}))
	if err != nil {
		return m, m.handleError(err)
	}
	if agentType == session.AgentTypeReviewer {
		// Set ReviewCycle so the instance carries the same cycle number used in the title.
		inst.ReviewCycle = reviewCycle
	}
	if action == "solo" {
		inst.SoloAgent = true
	}
	inst.QueuedPrompt = prompt

	// Set loading state immediately so the UI shows the progress bar
	// instead of the idle banner while the async start runs.
	inst.SetStatus(session.Loading)
	inst.LoadingTotal = 5
	inst.LoadingMessage = "Preparing session..."

	var startCmd tea.Cmd
	deferAddInstance := true
	if action == "plan" || action == "solo" {
		// Planner and solo agent run on main branch — no worktree created.
		if err := scaffold.PatchWorktreeConfig(m.activeRepoPath, m.opencodeAgentConfigs()); err != nil {
			return m, m.handleError(err)
		}
		if action == "plan" && repoManagedByDaemon(m.activeRepoPath) {
			deferAddInstance = false
			capturedRepoPath := m.activeRepoPath
			capturedProject := m.taskStoreProject
			capturedPlanFile := planFile
			capturedTitle := title
			capturedPrompt := prompt
			capturedProgram := m.programForAgent(agentType)
			startCmd = func() tea.Msg {
				inst, err := spawnPlannerWithDaemon(capturedRepoPath, capturedProject, capturedPlanFile, capturedTitle, capturedPrompt, capturedProgram)
				return daemonPlannerStartedMsg{instance: inst, err: err}
			}
		} else if action == "solo" &&
			session.NormalizeExecutionMode(inst.ExecutionMode) == session.ExecutionModeSDK &&
			repoManagedByDaemon(m.activeRepoPath) {
			capturedInst := inst
			capturedProject := m.taskStoreProject
			capturedPlanFile := planFile
			capturedAgentType := agentType
			startCmd = func() tea.Msg {
				skip := capturedInst.SkipPermissions
				req := api.SpawnSoloRequest{
					Title:           capturedInst.Title,
					Program:         capturedInst.Program,
					Prompt:          capturedInst.QueuedPrompt,
					TaskFile:        capturedPlanFile,
					AgentType:       capturedAgentType,
					SoloAgent:       true,
					SDKSpeedTier:    capturedInst.SDKSpeedTier,
					SkipPermissions: &skip,
				}
				return instanceStartedMsg{instance: capturedInst, err: spawnSoloWithDaemon(capturedProject, req)}
			}
		} else {
			startCmd = func() tea.Msg {
				return instanceStartedMsg{instance: inst, err: inst.StartOnMainBranch()}
			}
		}
	} else {
		// Backfill branch name for plans created before the branch field was introduced.
		if entry.Branch == "" {
			entry.Branch = gitpkg.TaskBranchFromFile(planFile)
			if err := m.taskState.SetBranch(planFile, entry.Branch); err != nil {
				return m, m.handleError(fmt.Errorf("failed to assign branch for plan: %w", err))
			}
		}

		// Coder and reviewer share the plan's feature branch worktree
		shared := gitpkg.NewSharedTaskWorktree(m.activeRepoPath, entry.Branch)
		if err := shared.Setup(); err != nil {
			return m, m.handleError(err)
		}
		if err := m.syncSharedWorktreeScaffold(shared.GetWorktreePath()); err != nil {
			return m, m.handleError(err)
		}
		startCmd = func() tea.Msg {
			err := inst.StartInSharedWorktree(shared, entry.Branch)
			return instanceStartedMsg{instance: inst, err: err}
		}
	}

	m.audit(auditlog.EventAgentSpawned, fmt.Sprintf("spawned %s for plan %s", agentType, taskstate.DisplayName(planFile)),
		auditlog.WithPlan(planFile),
		auditlog.WithInstance(title),
		auditlog.WithAgent(agentType),
	)

	if deferAddInstance {
		m.addInstanceFinalizer(inst, m.nav.AddInstance(inst))
		m.nav.SelectInstance(inst)
	}
	return m, tea.Batch(tea.RequestWindowSize, startCmd)
}

// getTopicNames returns existing topic names for the picker.
func (m *home) getTopicNames() []string {
	if m.taskState == nil {
		return nil
	}
	topics := m.taskState.Topics()
	names := make([]string, len(topics))
	for i, t := range topics {
		names[i] = t.Name
	}
	return names
}

// rebuildOrphanedOrchestrators reconstructs in-memory WaveOrchestrators for plans that
// were mid-wave when kasmos was restarted. Without this, wave completion/retry/advance
// flows can strand a plan in wave_running or wave_waiting after restart.
//
// Recovery prefers persisted execution/subtask state from the task store so it still
// works when no live task tmux sessions survive. When persisted subtask state is not
// available, it falls back to rebuilding from live task instances.
func (m *home) rebuildOrphanedOrchestrators() {
	if m.taskState == nil || m.taskStateDir == "" || m.taskStore == nil || m.taskStoreProject == "" {
		return
	}

	// Group task instances by plan file.
	type taskInst struct {
		taskNumber int
		waveNumber int
		title      string
		paused     bool
		exited     bool
	}
	byPlan := make(map[string]map[string]taskInst)
	hasActiveByPlan := make(map[string]bool)
	for _, inst := range m.nav.GetInstances() {
		if inst.TaskNumber == 0 || inst.TaskFile == "" {
			continue
		}
		if !inst.Started() {
			continue
		}
		if byPlan[inst.TaskFile] == nil {
			byPlan[inst.TaskFile] = make(map[string]taskInst)
		}
		key := fmt.Sprintf("w%d:t%d", inst.WaveNumber, inst.TaskNumber)
		candidate := taskInst{
			taskNumber: inst.TaskNumber,
			waveNumber: inst.WaveNumber,
			title:      inst.Title,
			paused:     inst.Paused(),
			exited:     inst.Exited,
		}
		if existing, ok := byPlan[inst.TaskFile][key]; !ok || ((existing.paused || existing.exited) && !(candidate.paused || candidate.exited)) || ((existing.paused || existing.exited) == (candidate.paused || candidate.exited) && existing.title < candidate.title) {
			byPlan[inst.TaskFile][key] = candidate
		}
		if !inst.Paused() && !inst.Exited {
			hasActiveByPlan[inst.TaskFile] = true
		}
	}

	for planFile, entry := range m.taskState.Plans {
		// Skip if orchestrator already exists.
		if _, exists := m.waveOrchestrators[planFile]; exists {
			continue
		}
		// Skip if user dismissed the all-waves-complete prompt for this plan.
		if m.allCompleteDismissed[planFile] {
			continue
		}
		// Skip if the user already accepted the final review prompt and the push /
		// review transition is still in flight.
		if m.allCompleteAdvancing[planFile] {
			continue
		}
		// Only reconstruct plans that are explicitly on a wave-execution branch.
		if entry.Status != taskstate.StatusImplementing || !taskfsm.IsWaveExecutionPhase(taskfsm.NormalizeExecutionPhase(entry.ExecutionState.Phase)) {
			continue
		}

		// Parse the plan content from store.
		content, err := m.taskStore.GetContent(m.taskStoreProject, planFile)
		if err != nil {
			log.WarningLog.Printf("rebuildOrphanedOrchestrators: cannot read %s: %v", planFile, err)
			continue
		}
		plan, err := taskparser.Parse(content)
		if err != nil {
			log.WarningLog.Printf("rebuildOrphanedOrchestrators: cannot parse %s: %v", planFile, err)
			continue
		}

		taskToWave := make(map[int]int)
		for _, wave := range plan.Waves {
			for _, task := range wave.Tasks {
				taskToWave[task.Number] = wave.Number
			}
		}

		// Prefer the persisted active wave so restart recovery does not depend on
		// every surviving instance still carrying accurate metadata.
		targetWave := entry.ExecutionState.ActiveWave
		if targetWave <= 0 {
			for _, t := range byPlan[planFile] {
				if t.waveNumber > targetWave {
					targetWave = t.waveNumber
				}
			}
		}

		// Guard against malformed legacy task instances with no wave metadata.
		if targetWave <= 0 || targetWave > len(plan.Waves) {
			log.WarningLog.Printf("rebuildOrphanedOrchestrators: skipping %s — invalid target wave %d", planFile, targetWave)
			continue
		}

		completedTasks := make([]int, 0)
		failedTasks := make([]int, 0)
		hasPersistedWaveState := false
		if subtasks, err := m.taskStore.GetSubtasks(m.taskStoreProject, planFile); err == nil {
			for _, subtask := range subtasks {
				if taskToWave[subtask.TaskNumber] != targetWave {
					continue
				}

				switch subtask.Status {
				case taskstore.SubtaskStatusRunning,
					taskstore.SubtaskStatusComplete,
					taskstore.SubtaskStatusDone,
					taskstore.SubtaskStatusClosed,
					taskstore.SubtaskStatusFailed:
					hasPersistedWaveState = true
				}

				switch subtask.Status {
				case taskstore.SubtaskStatusComplete, taskstore.SubtaskStatusDone, taskstore.SubtaskStatusClosed:
					completedTasks = append(completedTasks, subtask.TaskNumber)
				case taskstore.SubtaskStatusFailed:
					failedTasks = append(failedTasks, subtask.TaskNumber)
				}
			}
		} else {
			log.WarningLog.Printf("rebuildOrphanedOrchestrators: cannot read subtasks for %s: %v", planFile, err)
		}

		if !hasPersistedWaveState {
			taskSet := byPlan[planFile]
			if len(taskSet) == 0 {
				continue
			}
			if !hasActiveByPlan[planFile] {
				log.WarningLog.Printf("rebuildOrphanedOrchestrators: skipping %s — no persisted wave state and no active wave instances", planFile)
				continue
			}
			completedTasks = completedTasks[:0]
			for _, task := range taskSet {
				if task.waveNumber == targetWave && task.paused {
					completedTasks = append(completedTasks, task.taskNumber)
				}
			}
		}

		orch := orchestration.NewWaveOrchestrator(planFile, plan)
		orch.SetStore(m.taskStore, m.taskStoreProject)

		// Fast-forward the orchestrator to the target wave, marking earlier waves
		// as complete and applying actual task states for the target wave.
		orch.RestoreToWave(targetWave, completedTasks)
		for _, taskNumber := range failedTasks {
			orch.MarkTaskFailed(taskNumber)
		}

		// If the restored orchestrator is already AllComplete, all tasks finished
		// before the restart (or before the orchestrator was deleted). Don't add
		// it to waveOrchestrators — the metadata tick would immediately re-show the
		// "push branch and start review?" prompt on every tick. Queue a single
		// deferred prompt instead, but only if the prompt isn't already showing
		// (pendingAllCompleteTaskFile) and the plan isn't already queued.
		if orch.State() == orchestration.WaveStateAllComplete {
			if m.pendingAllCompleteTaskFile != planFile {
				m.queueAllCompletePrompt(planFile)
				log.WarningLog.Printf("rebuildOrphanedOrchestrators: %s already all-complete — queued prompt", planFile)
			}
			continue
		}

		m.waveOrchestrators[planFile] = orch
		log.WarningLog.Printf("rebuildOrphanedOrchestrators: restored orchestrator for %s (wave %d)",
			planFile, targetWave)
	}
}

// spawnWaveTasks creates and starts instances for the given task list within an orchestrator.
// Used by both startNextWave (initial spawn) and retryFailedWaveTasks (re-spawn failed tasks).
func (m *home) spawnWaveTasks(orch *orchestration.WaveOrchestrator, tasks []taskparser.Task, entry taskstate.TaskEntry) (tea.Model, tea.Cmd) {
	if !m.requireDaemonForAgents() {
		return m, nil
	}
	planFile := orch.TaskFile()
	if err := m.setExecutionState(planFile, taskstore.ExecutionState{
		Phase:           string(taskfsm.ExecutionPhaseWaveRunning),
		ActiveAgentType: session.AgentTypeCoder,
		ActiveWave:      orch.CurrentWaveNumber(),
	}); err != nil {
		return m, m.handleError(err)
	}
	// Set up shared worktree for all tasks in this batch.
	shared := gitpkg.NewSharedTaskWorktree(m.activeRepoPath, entry.Branch)
	if err := shared.Setup(); err != nil {
		return m, m.handleError(err)
	}
	if err := m.syncSharedWorktreeScaffold(shared.GetWorktreePath()); err != nil {
		return m, m.handleError(err)
	}

	// Derive WaveTaskIndex/WaveTaskCount from the full current wave so that
	// retried tasks (a subset of the wave) still carry the correct position.
	allWaveTasks := orch.CurrentWaveTasks()
	waveTaskCount := len(allWaveTasks)
	waveTaskPos := make(map[int]int, len(allWaveTasks))
	for i, t := range allWaveTasks {
		waveTaskPos[t.Number] = i + 1
	}

	var cmds []tea.Cmd
	for _, task := range tasks {
		title := orchestration.BuildWaveTaskTitle(planFile, orch.CurrentWaveNumber(), task.Number)

		// Skip if an instance with this title already exists (e.g. daemon
		// already spawned the wave task before the TUI's auto-advance fired).
		alreadyExists := false
		for _, existing := range m.nav.GetInstances() {
			if existing.Title == title {
				alreadyExists = true
				break
			}
		}
		if alreadyExists {
			continue
		}

		prompt := orch.BuildTaskPrompt(task, len(tasks))

		inst, err := session.NewInstance(m.withRetentionOpts(session.InstanceOptions{
			Title:           title,
			Path:            m.activeRepoPath,
			Program:         m.programForAgent(session.AgentTypeCoder),
			ExecutionMode:   m.executionModeForAgent(session.AgentTypeCoder),
			SDKSpeedTier:    m.sdkSpeedTierForAgent(session.AgentTypeCoder),
			SkipPermissions: m.skipPermissionsForAgent(session.AgentTypeCoder),
			TaskFile:        planFile,
			AgentType:       session.AgentTypeCoder,
			TaskNumber:      task.Number,
			WaveNumber:      orch.CurrentWaveNumber(),
			PeerCount:       len(tasks),
			WaveTaskIndex:   waveTaskPos[task.Number], // 0 (unknown) if task not in current wave — safe, never happens in practice
			WaveTaskCount:   waveTaskCount,
		}))
		if err != nil {
			return m, m.handleError(err)
		}
		inst.QueuedPrompt = prompt
		inst.SetStatus(session.Loading)
		inst.LoadingTotal = 6
		inst.LoadingMessage = "Connecting to shared worktree..."

		// AddInstance registers in the list immediately; finalizer sets repo name after start.
		m.addInstanceFinalizer(inst, m.nav.AddInstance(inst))

		taskInst := inst // capture for closure
		startCmd := func() tea.Msg {
			err := taskInst.StartInSharedWorktree(shared, entry.Branch)
			return instanceStartedMsg{instance: taskInst, err: err}
		}
		cmds = append(cmds, startCmd)
	}

	cmds = append(cmds, tea.RequestWindowSize, m.toastTickCmd())
	return m, tea.Batch(cmds...)
}

func (m *home) applyAdvanceWaveAction(action loop.AdvanceWaveAction, preStartToast, agentTypeToStop string) (tea.Model, tea.Cmd) {
	if agentTypeToStop != "" {
		for _, inst := range m.nav.GetInstances() {
			if inst.TaskFile == action.PlanFile && inst.AgentType == agentTypeToStop {
				_ = inst.Kill()
				break
			}
		}
	}

	orch, exists := m.waveOrchestrators[action.PlanFile]
	if !exists {
		return m, nil
	}
	entry, ok := m.taskState.Entry(action.PlanFile)
	if !ok {
		return m, nil
	}
	if preStartToast != "" {
		m.toastManager.Info(preStartToast)
	}
	return m.startNextWave(orch, entry)
}

// startNextWave advances the orchestrator to the next wave and spawns its task instances.
func (m *home) startNextWave(orch *orchestration.WaveOrchestrator, entry taskstate.TaskEntry) (tea.Model, tea.Cmd) {
	tasks := orch.StartNextWave()
	if len(tasks) == 0 {
		return m, nil
	}

	waveNum := orch.CurrentWaveNumber()
	m.toastManager.Info(fmt.Sprintf("wave %d started: %d task(s) running", waveNum, len(tasks)))
	m.audit(auditlog.EventWaveStarted,
		fmt.Sprintf("wave %d started: %d task(s)", waveNum, len(tasks)),
		auditlog.WithPlan(orch.TaskFile()),
		auditlog.WithWave(waveNum, 0))
	return m.spawnWaveTasks(orch, tasks, entry)
}

// retryFailedWaveTasks retries all failed tasks in the current wave by re-spawning them.
// Old failed instances are removed first to prevent ghost duplicates that accumulate
// across retries and all get marked ImplementationComplete when waves finish.
func (m *home) retryFailedWaveTasks(orch *orchestration.WaveOrchestrator, entry taskstate.TaskEntry) (tea.Model, tea.Cmd) {
	tasks := orch.RetryFailedTasks()
	if len(tasks) == 0 {
		return m, nil
	}

	// Build a set of task numbers being retried for fast lookup.
	retryingTasks := make(map[int]bool, len(tasks))
	for _, t := range tasks {
		retryingTasks[t.Number] = true
	}

	// Remove old failed instances for the tasks being retried.
	// Collect first to avoid mutating the list while iterating.
	planFile := orch.TaskFile()
	var staleInsts []*session.Instance
	for _, inst := range m.nav.GetInstances() {
		if inst.TaskFile == planFile && retryingTasks[inst.TaskNumber] {
			staleInsts = append(staleInsts, inst)
		}
	}
	for _, inst := range staleInsts {
		m.nav.RemoveByTitle(inst.Title)
		m.removeFromAllInstances(inst.Title)
	}

	m.toastManager.Info(fmt.Sprintf("retrying %d failed task(s) in wave %d",
		len(tasks), orch.CurrentWaveNumber()))
	return m.spawnWaveTasks(orch, tasks, entry)
}

// discoverTmuxSessions returns a tea.Cmd that lists all kas_ tmux sessions (managed + orphaned).
func (m *home) discoverTmuxSessions() tea.Cmd {
	knownNames := make([]string, 0, len(m.allInstances))
	for _, inst := range m.allInstances {
		if inst.Started() && inst.TmuxAlive() {
			knownNames = append(knownNames, tmux.ToKasTmuxNamePublic(inst.Title))
		}
	}
	return func() tea.Msg {
		sessions, err := tmux.DiscoverAll(cmd2.MakeExecutor(), knownNames)
		return tmuxSessionsMsg{sessions: sessions, err: err}
	}
}

// buildChatAboutTaskPrompt builds the custodian prompt for a chat-about-plan session.
func buildChatAboutTaskPrompt(planFile, project string, entry taskstate.TaskEntry, question string) string {
	name := taskstate.DisplayName(planFile)
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("You are answering a question about the plan '%s'.\n\n", name))
	sb.WriteString("## Plan Context\n\n")
	sb.WriteString(fmt.Sprintf("- **Plan:** %s\n", planFile))
	sb.WriteString(fmt.Sprintf("- **Status:** %s\n", entry.Status))
	if entry.Description != "" {
		sb.WriteString(fmt.Sprintf("- **Description:** %s\n", entry.Description))
	}
	if entry.Branch != "" {
		sb.WriteString(fmt.Sprintf("- **Branch:** %s\n", entry.Branch))
	}
	if entry.Topic != "" {
		sb.WriteString(fmt.Sprintf("- **Topic:** %s\n", entry.Topic))
	}
	sb.WriteString(fmt.Sprintf("\nRetrieve the full plan with MCP `task_show` (filename: %q, project: %q) for details. If that tool is unavailable, fall back to `kas task show %s`.\n\n", planFile, project, planFile))
	sb.WriteString("## User Question\n\n")
	sb.WriteString(question)
	return sb.String()
}

// spawnChatAboutTask spawns a custodian agent pre-loaded with the plan context and user question.
func (m *home) spawnChatAboutTask(planFile, question string) (tea.Model, tea.Cmd) {
	if !m.requireDaemonForAgents() {
		return m, nil
	}
	if m.taskState == nil {
		return m, m.handleError(fmt.Errorf("no task state loaded"))
	}
	entry, ok := m.taskState.Entry(planFile)
	if !ok {
		return m, m.handleError(fmt.Errorf("task not found: %s", planFile))
	}
	prompt := buildChatAboutTaskPrompt(planFile, m.taskStoreProject, entry, question)
	planName := taskstate.DisplayName(planFile)
	title := planName + "-chat"

	inst, err := session.NewInstance(m.withRetentionOpts(session.InstanceOptions{
		Title:           title,
		Path:            m.activeRepoPath,
		Program:         m.programForAgent(session.AgentTypeFixer),
		ExecutionMode:   m.executionModeForAgent(session.AgentTypeFixer),
		SDKSpeedTier:    m.sdkSpeedTierForAgent(session.AgentTypeFixer),
		SkipPermissions: m.skipPermissionsForAgent(session.AgentTypeFixer),
		TaskFile:        planFile,
		AgentType:       session.AgentTypeFixer,
		ClaudeNoFlicker: m.claudeNoFlicker(),
	}))
	if err != nil {
		return m, m.handleError(err)
	}
	inst.QueuedPrompt = prompt
	inst.SetStatus(session.Loading)
	inst.LoadingTotal = 5
	inst.LoadingMessage = "preparing chat..."

	// Use the plan's branch worktree if available, otherwise main.
	var startCmd tea.Cmd
	branch := m.taskBranch(planFile)
	if branch != "" {
		shared := gitpkg.NewSharedTaskWorktree(m.activeRepoPath, branch)
		startCmd = func() tea.Msg {
			if err := shared.Setup(); err != nil {
				return instanceStartedMsg{instance: inst, err: err}
			}
			err := inst.StartInSharedWorktree(shared, branch)
			return instanceStartedMsg{instance: inst, err: err}
		}
	} else {
		startCmd = func() tea.Msg {
			return instanceStartedMsg{instance: inst, err: inst.StartOnMainBranch()}
		}
	}

	m.audit(auditlog.EventAgentSpawned, fmt.Sprintf("spawned custodian chat for %s", taskstate.DisplayName(planFile)),
		auditlog.WithPlan(planFile),
		auditlog.WithInstance(title),
		auditlog.WithAgent(session.AgentTypeFixer),
	)

	m.addInstanceFinalizer(inst, m.nav.AddInstance(inst))
	m.nav.SelectInstance(inst)
	return m, tea.Batch(tea.RequestWindowSize, startCmd)
}

// adoptOrphanSession creates a new Instance backed by an existing orphaned tmux session.
func (m *home) adoptOrphanSession(item overlay.TmuxBrowserItem) (tea.Model, tea.Cmd) {
	var (
		candidate orchestration.RecoveryCandidate
		bound     bool
	)
	if m.taskState != nil {
		for filename, entry := range m.taskState.Plans {
			if current, ok := m.recoveryCandidateForTitle(filename, entry, item.Title); ok {
				candidate = current
				bound = true
			}
			if bound {
				break
			}
		}
	}
	if bound && m.hasLiveOrPendingInstance(candidate.TaskFile, candidate.AgentType, candidate.Title) {
		for _, inst := range m.nav.GetInstances() {
			if inst.Title == candidate.Title {
				m.nav.SelectInstance(inst)
				break
			}
		}
		return m, nil
	}
	for _, inst := range m.nav.GetInstances() {
		if inst.Title == item.Title {
			m.nav.SelectInstance(inst)
			return m, nil
		}
	}

	program := m.program
	if bound && candidate.AgentType != "" {
		program = m.programForAgent(candidate.AgentType)
	}
	inst, err := session.NewInstance(m.withRetentionOpts(session.InstanceOptions{
		Title:           item.Title,
		Path:            m.activeRepoPath,
		Program:         program,
		SkipPermissions: m.skipPermissionsForAgent(candidate.AgentType),
		TaskFile:        candidate.TaskFile,
		AgentType:       candidate.AgentType,
		TaskNumber:      candidate.TaskNumber,
		WaveNumber:      candidate.WaveNumber,
		ReviewCycle:     candidate.ReviewCycle,
		ClaudeNoFlicker: m.claudeNoFlicker(),
	}))
	if err != nil {
		return m, m.handleError(err)
	}
	if bound {
		inst.TaskFile = candidate.TaskFile
		inst.Branch = candidate.Branch
		if candidate.Branch != "" {
			inst.BindSharedTaskWorktree(m.activeRepoPath, candidate.Branch)
		}
	}

	m.addInstanceFinalizer(inst, m.nav.AddInstance(inst))
	m.nav.SelectInstance(inst)

	m.audit(auditlog.EventAgentSpawned, fmt.Sprintf("adopted orphan session: %s", item.Title),
		auditlog.WithInstance(item.Title),
	)

	m.toastManager.Info(fmt.Sprintf("adopting session '%s'", item.Title))

	return m, func() tea.Msg {
		err := inst.AdoptOrphanTmuxSession(item.Name)
		return instanceStartedMsg{instance: inst, err: err}
	}
}

// audit emits a structured audit event, automatically filling in the Project
// field from m.taskStoreProject. Optional fields (PlanFile, InstanceTitle,
// AgentType, WaveNumber, TaskNumber, Detail, Level) can be set via EventOption
// functional options: WithPlan, WithInstance, WithAgent, WithWave, WithDetail,
// WithLevel.
func (m *home) audit(kind auditlog.EventKind, msg string, opts ...auditlog.EventOption) {
	if m.auditLogger == nil {
		return
	}
	e := auditlog.Event{
		Kind:    kind,
		Project: m.taskStoreProject,
		Message: msg,
	}
	for _, opt := range opts {
		opt(&e)
	}
	m.auditLogger.Emit(e)
	m.refreshAuditPane()
}

// refreshAuditPane queries the audit logger and updates the audit pane display.
// Shows a global activity feed — not filtered by sidebar selection.
func (m *home) refreshAuditPane() {
	if m.auditPane == nil || m.auditLogger == nil {
		return
	}

	filter := auditlog.QueryFilter{
		Project: m.taskStoreProject,
		Limit:   200,
	}

	events, err := m.auditLogger.Query(filter)
	if err != nil {
		return
	}

	displays := make([]ui.AuditEventDisplay, 0, len(events))
	for _, e := range events {
		icon, color := ui.EventKindIcon(string(e.Kind))
		timeStr := e.Timestamp.Local().Format("15:04")
		msg := e.Message
		// Prepend [plan-name] when the event has a plan context and the message
		// doesn't already embed the plan name (some messages include it inline).
		if e.TaskFile != "" {
			label := taskstate.DisplayName(e.TaskFile)
			if !strings.Contains(msg, label) {
				msg = "[" + label + "] " + msg
			}
		}
		displays = append(displays, ui.AuditEventDisplay{
			Time:          timeStr,
			Kind:          string(e.Kind),
			Icon:          icon,
			Message:       msg,
			Color:         color,
			Level:         e.Level,
			TaskFile:      e.TaskFile,
			InstanceTitle: e.InstanceTitle,
			AgentType:     e.AgentType,
			GroupKey:      auditEventGroupKey(e.Detail),
			DetailJSON:    e.Detail,
		})
	}

	// Coalesce consecutive stopped+started pairs (newest-first order) with the
	// same HH:MM timestamp into a single "kasmos restarted" event.
	displays = coalesceRestarts(displays)

	m.auditPane.SetEvents(displays)

	// Push updated audit view into the nav panel.
	if m.nav != nil && m.auditPane.Visible() {
		m.nav.SetAuditView(m.auditPane.String(), m.auditPane.ContentLines())
	}
}

// buildClickUpProgressComment formats a concise markdown comment for ClickUp.
// Prefixes with "🤖 kasmos:" so comments are identifiable.
// Events:
//   - plan_ready: "plan finalized — {detail}"
//   - wave_complete: "wave {detail} complete"
//   - review_approved: "review approved — implementation complete"
//   - review_changes_requested: "review: changes requested — {detail}"
//   - fixer_complete: "fixer agent completed — {detail}"
func buildClickUpProgressComment(event, planName, detail string) string {
	var body string
	switch event {
	case "plan_ready":
		if detail != "" {
			body = "plan finalized — " + detail
		} else {
			body = "plan finalized"
		}
	case "wave_complete":
		if detail != "" {
			body = "wave " + detail + " complete"
		} else {
			body = "wave complete"
		}
	case "review_approved":
		body = "review approved — implementation complete"
	case "review_changes_requested":
		if detail != "" {
			body = "review: changes requested — " + detail
		} else {
			body = "review: changes requested"
		}
	case "fixer_complete":
		if detail != "" {
			body = "fixer agent completed — " + detail
		} else {
			body = "fixer agent completed"
		}
	default:
		if detail != "" {
			body = event + " — " + detail
		} else {
			body = event
		}
	}
	return "🤖 kasmos: **" + planName + "** — " + body
}

// postClickUpProgress resolves the ClickUp task ID for the given plan and posts
// a progress comment. Returns a fire-and-forget tea.Cmd. Returns nil if no task
// ID is associated with the plan or the commenter is unavailable. All errors
// are logged, never surfaced to the user.
func (m *home) postClickUpProgress(planFile, event, detail string) tea.Cmd {
	if m.taskState == nil {
		return nil
	}
	entry, ok := m.taskState.Entry(planFile)
	if !ok {
		return nil
	}

	// Fetch content for fallback task ID resolution only when the field is empty.
	var content string
	if entry.ClickUpTaskID == "" && m.taskStore != nil {
		content, _ = m.taskStore.GetContent(m.taskStoreProject, planFile)
	}
	taskID := resolveClickUpTaskID(entry, content)

	planName := taskstate.DisplayName(planFile)
	comment := buildClickUpProgressComment(event, planName, detail)

	commenter := m.getOrCreateCommenter(m.ctx)
	return postClickUpProgress(commenter, taskID, comment)
}

// getOrCreateCommenter returns a Commenter backed by the same MCP client as
// the Importer if it already exists. Returns nil when no MCP client has been
// initialized yet — progress comments are best-effort and the importer is
// always initialized before plans acquire ClickUp task IDs, so this fallback
// path is never hit in practice. Lazy initialization via getOrCreateImporter
// is deliberately avoided: that call does blocking I/O (MCP subprocess spawn)
// and must not run inside the synchronous Update() path.
func (m *home) getOrCreateCommenter(_ context.Context) *clickup.Commenter {
	if m.clickUpCommenter != nil {
		return m.clickUpCommenter
	}
	if m.clickUpConfig == nil || m.clickUpMCPClient == nil {
		return nil
	}

	// Reuse the shared MCP client initialized by the importer.
	m.clickUpCommenter = clickup.NewCommenter(m.clickUpMCPClient)
	if projCfg := clickup.LoadProjectConfig(m.activeRepoPath); projCfg.WorkspaceID != "" {
		m.clickUpCommenter.SetWorkspaceID(projCfg.WorkspaceID)
	}
	return m.clickUpCommenter
}

// coalesceRestarts merges adjacent session_started + session_stopped pairs
// (in newest-first order) that share the same HH:MM into a single "restarted"
// event. The slice is newest-first, so started appears before stopped.
func coalesceRestarts(displays []ui.AuditEventDisplay) []ui.AuditEventDisplay {
	if len(displays) < 2 {
		return displays
	}
	out := make([]ui.AuditEventDisplay, 0, len(displays))
	i := 0
	for i < len(displays) {
		// Newest-first: started at [i], stopped at [i+1].
		if i+1 < len(displays) &&
			displays[i].Kind == "session_started" &&
			displays[i+1].Kind == "session_stopped" &&
			displays[i].Time == displays[i+1].Time {
			icon, color := ui.EventKindIcon("session_started")
			out = append(out, ui.AuditEventDisplay{
				Time:    displays[i].Time,
				Kind:    "session_restarted",
				Icon:    icon,
				Message: "kasmos restarted",
				Color:   color,
				Level:   "info",
			})
			i += 2
			continue
		}
		out = append(out, displays[i])
		i++
	}
	return out
}

func auditEventGroupKey(detailJSON string) string {
	if detailJSON == "" {
		return ""
	}
	var detail struct {
		GroupKey string `json:"group_key"`
	}
	if err := json.Unmarshal([]byte(detailJSON), &detail); err != nil {
		return ""
	}
	return detail.GroupKey
}
