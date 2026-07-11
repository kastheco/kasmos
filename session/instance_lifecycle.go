package session

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/kastheco/kasmos/config/taskfsm"
	"github.com/kastheco/kasmos/config/taskstate"
	"github.com/kastheco/kasmos/config/taskstore"
	"github.com/kastheco/kasmos/internal/mcpclient"
	"github.com/kastheco/kasmos/internal/opencodesession"
	"github.com/kastheco/kasmos/internal/platform"
	"github.com/kastheco/kasmos/log"
	"github.com/kastheco/kasmos/session/common"
	"github.com/kastheco/kasmos/session/git"
	"github.com/kastheco/kasmos/session/tmux"
)

// kasmosManagedPrograms lists executable basenames that depend on the shared
// kasmos MCP HTTP endpoint. Absolute-path variants are handled by extracting
// the basename before lookup.
var kasmosManagedPrograms = map[string]bool{
	"claude":   true,
	"opencode": true,
	"codex":    true,
}

// probeMCPFunc is the probe seam. Tests may replace it to avoid real network
// calls while still exercising the gate logic in the launch methods.
var probeMCPFunc = func() error {
	return mcpclient.ProbeHTTP(context.Background(), mcpclient.SharedEndpointURL)
}

var staleThreshold = 24 * time.Hour

// IsStale reports whether a paused instance has gone without persisted activity
// long enough to warrant operator recovery guidance.
func (i *Instance) IsStale(now time.Time) bool {
	if i == nil || i.Status != Paused || i.UpdatedAt.IsZero() {
		return false
	}
	return now.Sub(i.UpdatedAt) > staleThreshold
}

// usesManagedKasmosMCP reports whether program depends on the shared kasmos
// MCP HTTP endpoint. It strips command-line flags and extracts the executable
// basename so both "claude" and "/usr/local/bin/claude --flag" are recognised.
func usesManagedKasmosMCP(program string) bool {
	fields := strings.Fields(program)
	if len(fields) == 0 {
		return false
	}
	base := filepath.Base(fields[0])
	return kasmosManagedPrograms[base]
}

// ensureSharedKasmosMCP gates launch on the shared MCP endpoint being
// reachable. Returns nil immediately for programs that do not use the endpoint.
// Errors include an actionable service start command for the current platform.
func (i *Instance) ensureSharedKasmosMCP() error {
	if !usesManagedKasmosMCP(i.Program) {
		return nil
	}
	if err := probeMCPFunc(); err != nil {
		return fmt.Errorf("kasmos mcp endpoint not reachable — run `%s` to start the shared mcp host: %w",
			platform.RestartServicesCommand(), err)
	}
	return nil
}

// prepareExecutionSession returns the existing execution session if already wired, otherwise
// allocates a fresh one from the instance configuration.
func (i *Instance) prepareExecutionSession() ExecutionSession {
	if i.executionSession != nil {
		return i.executionSession
	}
	return newExecutionSession(i.ExecutionMode, i.Title, i.Program, i.SkipPermissions)
}

// transferPromptToCli moves QueuedPrompt into the execution session's initialPrompt
// when the backend can deliver a startup prompt itself.
//
// SDK sessions deliver their initial prompt through the transport immediately
// after startup; tmux-backed sessions bake it into the CLI command line for
// programs that support prompt injection. Programs that do not support startup
// prompt delivery leave QueuedPrompt intact so a later send-keys fallback can
// deliver it after the session becomes ready.
func (i *Instance) transferPromptToCli() {
	if i.QueuedPrompt == "" {
		return
	}
	if NormalizeExecutionMode(i.ExecutionMode) == ExecutionModeSDK {
		// Keep QueuedPrompt until startup succeeds so SDK bootstrap fallback can
		// retry in tmux without losing the first task prompt.
		i.executionSession.SetInitialPrompt(i.QueuedPrompt)
		i.deliveredPrompt = i.QueuedPrompt
		return
	}
	if programSupportsCliPrompt(i.Program) {
		i.executionSession.SetInitialPrompt(i.QueuedPrompt)
		i.deliveredPrompt = i.QueuedPrompt
		i.QueuedPrompt = ""
	}
}

// projectName derives the repository base name from the instance's attached
// worktree. Returns "" when no worktree is attached or the repo path is empty.
func (i *Instance) projectName() string {
	if i.gitWorktree != nil {
		if p := i.gitWorktree.GetRepoPath(); p != "" {
			return filepath.Base(filepath.Clean(p))
		}
	}
	return ""
}

// setExecutionTaskEnv pushes wave/task/peer identity and the project name into
// the execution session environment so that agents spawned inside the session
// inherit the orchestration context.
func (i *Instance) setExecutionTaskEnv() {
	if i.executionSession == nil {
		return
	}
	if i.TaskNumber > 0 {
		i.executionSession.SetTaskEnv(i.TaskNumber, i.WaveNumber, i.PeerCount)
	}
	if project := i.projectName(); project != "" {
		i.executionSession.SetProject(project)
	}
}

// buildTitleOpts converts an Instance's metadata fields into the TitleOpts
// structure consumed by the opencodesession title builder.
func buildTitleOpts(inst *Instance) opencodesession.TitleOpts {
	displayName := ""
	if inst.TaskFile != "" {
		displayName = taskstate.DisplayName(inst.TaskFile)
	}
	return opencodesession.TitleOpts{
		PlanName:      displayName,
		AgentType:     inst.AgentType,
		WaveNumber:    inst.WaveNumber,
		TaskNumber:    inst.TaskNumber,
		InstanceTitle: inst.Title,
		ReviewCycle:   inst.ReviewCycle,
	}
}

// configureSessionTitle derives a session title from the instance metadata and
// registers a callback that writes it to the opencode database when the session
// becomes ready. It is a no-op for non-opencode programs.
func (i *Instance) configureSessionTitle() {
	if i.executionSession == nil || !strings.HasSuffix(i.Program, "opencode") {
		return
	}
	opts := buildTitleOpts(i)
	title := opencodesession.BuildTitle(opts)
	i.executionSession.SetSessionTitle(title)
	i.executionSession.SetTitleFunc(func(workDir string, beforeStart time.Time, t string) {
		if err := opencodesession.SetTitleDirect(workDir, beforeStart, t); err != nil {
			log.ErrorLog.Printf("opencodesession: set title: %v", err)
		}
	})
}

func dirtyWorktreeContext(worktreePath string) string {
	if strings.TrimSpace(worktreePath) == "" {
		return ""
	}
	out, err := exec.Command("git", "-C", worktreePath, "status", "--short").CombinedOutput()
	if err != nil {
		return ""
	}
	trimmed := strings.TrimSpace(string(out))
	if trimmed == "" {
		return ""
	}
	lines := strings.Split(trimmed, "\n")
	preview := lines
	if len(preview) > 5 {
		preview = preview[:5]
	}
	if len(lines) > len(preview) {
		return fmt.Sprintf(" (%s (+%d more))", strings.Join(preview, ", "), len(lines)-len(preview))
	}
	return fmt.Sprintf(" (%s)", strings.Join(preview, ", "))
}

func sdkLogPath(workDir, title string) string {
	return filepath.Join(workDir, ".kasmos", "logs", common.SanitizeSessionName(title)+".log")
}

func sdkLogSize(workDir, title string) int64 {
	info, err := os.Stat(sdkLogPath(workDir, title))
	if err != nil {
		return 0
	}
	return info.Size()
}

func sdkBootstrapFlagUnsupported(output string) bool {
	lower := strings.ToLower(output)
	hasBootstrapFlag := strings.Contains(lower, "--app-server") || strings.Contains(lower, "--server")
	rejected := strings.Contains(lower, "unknown option") ||
		strings.Contains(lower, "unexpected argument") ||
		strings.Contains(lower, "unrecognized option")
	return hasBootstrapFlag && rejected
}

func (i *Instance) shouldFallbackSDKStart(workDir string, logOffset int64) bool {
	if NormalizeExecutionMode(i.ExecutionMode) != ExecutionModeSDK {
		return false
	}

	data, err := os.ReadFile(sdkLogPath(workDir, i.Title))
	if err != nil {
		return false
	}
	if logOffset < 0 {
		logOffset = 0
	}
	if logOffset > int64(len(data)) {
		logOffset = int64(len(data))
	}
	return sdkBootstrapFlagUnsupported(string(data[logOffset:]))
}

func (i *Instance) startExecutionSessionWithFallback(workDir string, prepare func()) error {
	logOffset := sdkLogSize(workDir, i.Title)
	if err := i.executionSession.Start(workDir); err != nil {
		if !i.shouldFallbackSDKStart(workDir, logOffset) {
			return err
		}
		log.WarningLog.Printf("sdk bootstrap unsupported for %q (%s); falling back to tmux", i.Title, i.Program)
		_ = i.executionSession.Close()
		i.ExecutionMode = ExecutionModeTmux
		i.executionSession = nil
		prepare()
		if retryErr := i.executionSession.Start(workDir); retryErr != nil {
			return fmt.Errorf("%v (tmux fallback failed: %w)", err, retryErr)
		}
	}
	if NormalizeExecutionMode(i.ExecutionMode) == ExecutionModeSDK {
		i.QueuedPrompt = ""
	}
	return nil
}

// ShouldAutoAdvanceLifecycleImplementer reports whether the instance matches
// the persisted single-agent implementation state and has clearly finished its
// queued work, so the lifecycle can advance to review.
func ShouldAutoAdvanceLifecycleImplementer(status string, state taskstore.ExecutionState, inst *Instance, tmuxAlive bool) bool {
	if inst == nil || inst.TaskFile == "" {
		return false
	}
	if inst.AgentType != AgentTypeCoder && inst.AgentType != AgentTypeFixer {
		return false
	}
	if inst.SoloAgent || inst.TaskNumber > 0 {
		return false
	}
	if strings.TrimSpace(status) != string(taskfsm.StatusImplementing) {
		return false
	}
	phase := taskfsm.NormalizeExecutionPhase(state.Phase)
	if !taskfsm.IsSingleAgentImplementingPhase(phase) {
		return false
	}
	if state.ActiveAgentType != "" && state.ActiveAgentType != inst.AgentType {
		return false
	}
	if !tmuxAlive {
		return true
	}
	if NormalizeExecutionMode(inst.ExecutionMode) == ExecutionModeSDK && inst.Exited {
		return true
	}
	now := time.Now()
	inst.UpdateCompletionPromptState(now)
	return inst.HasStableCompletionPrompt(now)
}

// IsStuck reports whether an exited instance is stranded in an implementing
// execution phase that the daemon cannot auto-advance.
func IsStuck(entry taskstore.TaskEntry, inst *Instance, tmuxAlive bool) bool {
	if inst == nil || inst.TaskFile == "" || inst.Paused() {
		return false
	}
	if strings.TrimSpace(string(entry.Status)) != string(taskfsm.StatusImplementing) {
		return false
	}
	if ShouldAutoAdvanceLifecycleImplementer(string(entry.Status), entry.ExecutionState, inst, tmuxAlive) {
		return false
	}

	phase := taskfsm.NormalizeExecutionPhase(entry.ExecutionState.Phase)
	switch phase {
	case taskfsm.ExecutionPhaseArchitecting,
		taskfsm.ExecutionPhaseWaveRunning,
		taskfsm.ExecutionPhaseWaveWaiting,
		taskfsm.ExecutionPhaseSingleAgentImplementing,
		taskfsm.ExecutionPhaseFixing:
	default:
		return false
	}

	if entry.ExecutionState.ActiveAgentType != "" && inst.AgentType != "" && entry.ExecutionState.ActiveAgentType != inst.AgentType {
		return false
	}

	if !tmuxAlive {
		return true
	}

	return NormalizeExecutionMode(inst.ExecutionMode) == ExecutionModeSDK && inst.Exited
}

// setProgressFunc injects a progress hook into the execution session if it
// implements progressReporter (i.e. tmuxExecutionSession). No-op otherwise.
func (i *Instance) setProgressFunc(fn func(int, string)) {
	if pr, ok := i.executionSession.(progressReporter); ok {
		pr.SetProgressFunc(fn)
	}
}

// applyResourceControlsToSession forwards the resolved resource-control policy to the
// execution session. For tmux-backed sessions this arms the nice/ionice wrapper for
// Start(); for SDK sessions it is a no-op.
func (i *Instance) applyResourceControlsToSession() {
	if i.executionSession == nil {
		return
	}
	i.executionSession.SetResourceControls(i.ResourceControls)
}

// applySDKRetentionToSession forwards transcript limits to the execution session
// when SDKTranscriptLimitsSet is true and the session implements rendererRetentionSetter.
// No-op for tmux-backed sessions (which do not implement the interface).
func (i *Instance) applySDKRetentionToSession() {
	if !i.SDKTranscriptLimitsSet || i.executionSession == nil {
		return
	}
	if rrs, ok := i.executionSession.(rendererRetentionSetter); ok {
		rrs.SetRendererRetention(i.SDKTranscriptMaxBytes, i.SDKTranscriptMaxTurns)
	}
}

// Start launches the instance. When firstTimeSetup is true a fresh git worktree is
// created and the execution session starts inside it. When false the instance was loaded
// from storage and the existing session is restored instead.
func (i *Instance) Start(firstTimeSetup bool) error {
	if i.Title == "" {
		return fmt.Errorf("instance title cannot be empty")
	}
	// Probe only when spawning a fresh harness. Restore-only attach (firstTimeSetup=false,
	// used by FromInstanceData) reconnects to an already-running process and must not
	// fail when the shared endpoint is temporarily down.
	if firstTimeSetup {
		if err := i.ensureSharedKasmosMCP(); err != nil {
			return err
		}
	}

	if firstTimeSetup {
		i.LoadingTotal = 8
	} else {
		i.LoadingTotal = 6
	}
	i.LoadingStage = 0
	i.LoadingMessage = "Initializing..."

	i.setLoadingProgress(1, "Preparing session...")
	stageBase := 3
	if !firstTimeSetup {
		stageBase = 1
	}
	prepareSession := func() {
		i.executionSession = i.prepareExecutionSession()
		i.executionSession.SetAgentType(i.AgentType)
		i.executionSession.SetNoFlicker(i.ClaudeNoFlicker)
		i.executionSession.SetSDKSpeedTier(i.SDKSpeedTier)
		i.applyResourceControlsToSession()
		i.applySDKRetentionToSession()
		i.setExecutionTaskEnv()
		i.configureSessionTitle()
		i.setProgressFunc(func(stage int, desc string) {
			i.setLoadingProgress(stageBase+stage, desc)
			// Stage 3 ("Configuring session...") fires after the tmux session has
			// been created and the initial tmux options applied. The pane is live
			// at this point — flip Status to Running immediately so the admin UI
			// and the TUI can see the agent as "running" even while the backend
			// is still polling up to 30s for a per-harness ready signal (claude's
			// trust prompt, opencode's "Ask anything", etc.). Without this, every
			// daemon-spawned agent looks stuck on "loading" for half a minute.
			if stage >= 3 {
				i.SetStatus(Running)
			}
		})
		i.transferPromptToCli()
	}
	prepareSession()

	if firstTimeSetup {
		i.setLoadingProgress(2, "Creating git worktree...")
		worktree, branch, err := git.NewGitWorktree(i.Path, i.Title)
		if err != nil {
			return fmt.Errorf("failed to create git worktree: %w", err)
		}
		i.gitWorktree = worktree
		i.Branch = branch
	}

	// Clean up on any failure after this point.
	var startErr error
	defer func() {
		if startErr != nil {
			if killErr := i.Kill(); killErr != nil {
				startErr = fmt.Errorf("%v (cleanup: %v)", startErr, killErr)
			}
		} else {
			i.started = true
		}
	}()

	if firstTimeSetup {
		i.setLoadingProgress(3, "Setting up git worktree...")
		if err := i.gitWorktree.Setup(); err != nil {
			startErr = fmt.Errorf("failed to setup git worktree: %w", err)
			return startErr
		}
		i.setLoadingProgress(4, "Starting session...")
		if err := i.startExecutionSessionWithFallback(i.gitWorktree.GetWorktreePath(), prepareSession); err != nil {
			if cleanupErr := i.gitWorktree.Cleanup(); cleanupErr != nil {
				err = fmt.Errorf("%v (cleanup: %v)", err, cleanupErr)
			}
			startErr = fmt.Errorf("failed to start session: %w", err)
			return startErr
		}
	} else {
		i.setLoadingProgress(2, "Restoring session...")
		if err := i.executionSession.Restore(); err != nil {
			startErr = fmt.Errorf("failed to restore existing session: %w", err)
			return startErr
		}
	}

	i.SetStatus(Running)
	return nil
}

// StartOnMainBranch launches the instance directly in the repository root without
// creating a git worktree. Intended for planner agents that operate on main.
func (i *Instance) StartOnMainBranch() error {
	if i.Title == "" {
		return fmt.Errorf("instance title cannot be empty")
	}
	if err := i.ensureSharedKasmosMCP(); err != nil {
		return err
	}

	i.LoadingTotal = 5
	i.LoadingStage = 0
	i.LoadingMessage = "Initializing..."

	i.setLoadingProgress(1, "Preparing session...")
	prepareSession := func() {
		i.executionSession = i.prepareExecutionSession()
		i.executionSession.SetAgentType(i.AgentType)
		i.executionSession.SetNoFlicker(i.ClaudeNoFlicker)
		i.executionSession.SetSDKSpeedTier(i.SDKSpeedTier)
		i.applyResourceControlsToSession()
		i.applySDKRetentionToSession()
		i.setExecutionTaskEnv()
		i.configureSessionTitle()
		i.setProgressFunc(func(stage int, desc string) {
			i.setLoadingProgress(1+stage, desc)
			// See Instance.Start for why stage 3 flips status to Running eagerly.
			if stage >= 3 {
				i.SetStatus(Running)
			}
		})
		i.transferPromptToCli()
	}
	prepareSession()

	var startErr error
	defer func() {
		if startErr != nil {
			if killErr := i.Kill(); killErr != nil {
				startErr = fmt.Errorf("%v (cleanup: %v)", startErr, killErr)
			}
		} else {
			i.started = true
		}
	}()

	if err := i.startExecutionSessionWithFallback(i.Path, prepareSession); err != nil {
		startErr = fmt.Errorf("failed to start session on main branch: %w", err)
		return startErr
	}

	// Safety net: if tmux never reached stage 3 (e.g. a non-tmux execution
	// backend that doesn't report progress), make sure Status ends up at
	// Running on successful start.
	i.SetStatus(Running)
	return nil
}

// StartOnBranch creates a worktree on the specified branch (reusing an existing
// branch when it already exists) and starts the execution session inside it.
func (i *Instance) StartOnBranch(branch string) error {
	if i.Title == "" {
		return fmt.Errorf("instance title cannot be empty")
	}
	if err := i.ensureSharedKasmosMCP(); err != nil {
		return err
	}

	i.LoadingTotal = 8
	i.LoadingStage = 0
	i.LoadingMessage = "Initializing..."

	i.setLoadingProgress(1, "Preparing session...")
	prepareSession := func() {
		i.executionSession = i.prepareExecutionSession()
		i.executionSession.SetAgentType(i.AgentType)
		i.executionSession.SetNoFlicker(i.ClaudeNoFlicker)
		i.executionSession.SetSDKSpeedTier(i.SDKSpeedTier)
		i.applyResourceControlsToSession()
		i.applySDKRetentionToSession()
		i.setExecutionTaskEnv()
		i.configureSessionTitle()
		i.setProgressFunc(func(stage int, desc string) {
			i.setLoadingProgress(3+stage, desc)
			// See Instance.Start for why stage 3 flips status to Running eagerly.
			if stage >= 3 {
				i.SetStatus(Running)
			}
		})
		i.transferPromptToCli()
	}
	prepareSession()

	i.setLoadingProgress(2, "Creating git worktree...")
	worktree, branchName, err := git.NewGitWorktreeOnBranch(i.Path, i.Title, branch)
	if err != nil {
		return fmt.Errorf("failed to create git worktree on branch %s: %w", branch, err)
	}
	i.gitWorktree = worktree
	i.Branch = branchName

	var startErr error
	defer func() {
		if startErr != nil {
			if killErr := i.Kill(); killErr != nil {
				startErr = fmt.Errorf("%v (cleanup: %v)", startErr, killErr)
			}
		} else {
			i.started = true
		}
	}()

	i.setLoadingProgress(3, "Setting up git worktree...")
	if err := i.gitWorktree.Setup(); err != nil {
		startErr = fmt.Errorf("failed to setup git worktree: %w", err)
		return startErr
	}

	i.setLoadingProgress(4, "Starting session...")
	if err := i.startExecutionSessionWithFallback(i.gitWorktree.GetWorktreePath(), prepareSession); err != nil {
		if cleanupErr := i.gitWorktree.Cleanup(); cleanupErr != nil {
			err = fmt.Errorf("%v (cleanup: %v)", err, cleanupErr)
		}
		startErr = fmt.Errorf("failed to start session: %w", err)
		return startErr
	}

	i.SetStatus(Running)
	return nil
}

// StartInSharedWorktree connects the instance to a topic-owned worktree. No new
// worktree is created; the instance borrows the one passed by the caller.
func (i *Instance) StartInSharedWorktree(worktree *git.GitWorktree, branch string) error {
	if i.Title == "" {
		return fmt.Errorf("instance title cannot be empty")
	}
	if err := i.ensureSharedKasmosMCP(); err != nil {
		return err
	}

	i.LoadingTotal = 6
	i.setLoadingProgress(1, "Connecting to shared worktree...")

	i.gitWorktree = worktree
	i.Branch = branch
	i.sharedWorktree = true

	prepareSession := func() {
		i.executionSession = i.prepareExecutionSession()
		i.executionSession.SetAgentType(i.AgentType)
		i.executionSession.SetSDKSpeedTier(i.SDKSpeedTier)
		i.applyResourceControlsToSession()
		i.applySDKRetentionToSession()
		i.setExecutionTaskEnv()
		i.configureSessionTitle()
		i.setProgressFunc(func(stage int, desc string) {
			i.setLoadingProgress(1+stage, desc)
			// See Instance.Start for why stage 3 flips status to Running eagerly.
			if stage >= 3 {
				i.SetStatus(Running)
			}
		})
		i.transferPromptToCli()
	}
	prepareSession()

	i.setLoadingProgress(2, "Starting session...")
	if err := i.startExecutionSessionWithFallback(worktree.GetWorktreePath(), prepareSession); err != nil {
		return fmt.Errorf("failed to start session in shared worktree: %w", err)
	}

	i.started = true
	i.SetStatus(Running)
	return nil
}

// Kill terminates the execution session and removes the git worktree.
// The git branch is preserved so the instance can be inspected or resumed later.
// Returns nil for instances that were never started.
func (i *Instance) Kill() error {
	if !i.started {
		return nil
	}
	if i.gitWorktree != nil && !i.sharedWorktree {
		dirty, err := i.gitWorktree.IsDirty()
		if err != nil {
			return fmt.Errorf("failed to check if worktree is dirty: %w", err)
		}
		if dirty {
			return fmt.Errorf("cannot kill instance with uncommitted changes%s; commit or stash first", dirtyWorktreeContext(i.gitWorktree.GetWorktreePath()))
		}
	}

	var errs []error

	// Close the execution session first — it may hold an open handle to the worktree directory.
	if i.executionSession != nil {
		if err := i.executionSession.Close(); err != nil {
			errs = append(errs, fmt.Errorf("failed to close session: %w", err))
		}
	}

	// Shared worktrees are owned by the topic, not the instance.
	if i.gitWorktree != nil && !i.sharedWorktree {
		if err := i.gitWorktree.Remove(); err != nil {
			errs = append(errs, fmt.Errorf("failed to remove git worktree: %w", err))
		}
		if err := i.gitWorktree.Prune(); err != nil {
			errs = append(errs, fmt.Errorf("failed to prune git worktrees: %w", err))
		}
	}

	return errors.Join(errs...)
}

// StopTmux closes the underlying execution session without touching the worktree or
// any other instance state. The instance remains in the list as stopped.
func (i *Instance) StopTmux() {
	if i.executionSession != nil {
		_ = i.executionSession.Close()
	}
}

// Pause detaches from the session and removes the git worktree, preserving
// the branch for a later Resume.
func (i *Instance) Pause() error {
	if !i.started {
		return fmt.Errorf("cannot pause instance that has not been started")
	}
	if i.Status == Paused {
		return fmt.Errorf("instance is already paused")
	}
	if i.gitWorktree != nil && !i.sharedWorktree {
		dirty, err := i.gitWorktree.IsDirty()
		if err != nil {
			return fmt.Errorf("failed to check if worktree is dirty: %w", err)
		}
		if dirty {
			return fmt.Errorf("cannot pause instance with uncommitted changes%s; commit or stash first", dirtyWorktreeContext(i.gitWorktree.GetWorktreePath()))
		}
	}

	var errs []error

	if err := i.executionSession.DetachSafely(); err != nil {
		errs = append(errs, fmt.Errorf("failed to detach session: %w", err))
		log.ErrorLog.Print(err)
	}

	if !i.sharedWorktree && i.gitWorktree != nil {
		worktreePath := i.gitWorktree.GetWorktreePath()
		if _, statErr := os.Stat(worktreePath); statErr == nil {
			if removeErr := i.gitWorktree.Remove(); removeErr != nil {
				errs = append(errs, fmt.Errorf("failed to remove git worktree: %w", removeErr))
				log.ErrorLog.Print(removeErr)
				return errors.Join(errs...)
			}
			if pruneErr := i.gitWorktree.Prune(); pruneErr != nil {
				errs = append(errs, fmt.Errorf("failed to prune git worktrees: %w", pruneErr))
				log.ErrorLog.Print(pruneErr)
				return errors.Join(errs...)
			}
		}
	}

	if joined := errors.Join(errs...); joined != nil {
		log.ErrorLog.Print(joined)
		return joined
	}

	i.SetStatus(Paused)
	return nil
}

// AdoptOrphanTmuxSession wires the instance to an existing tmux session that was
// not created through the normal lifecycle. No worktree is involved.
func (i *Instance) AdoptOrphanTmuxSession(tmuxName string) error {
	if NormalizeExecutionMode(i.ExecutionMode) != ExecutionModeTmux {
		return fmt.Errorf("adopting orphan sessions is only supported for tmux execution")
	}
	ts := tmux.NewTmuxSessionFromExisting(tmuxName, i.Program, i.SkipPermissions)
	w := &tmuxExecutionSession{s: ts}
	i.executionSession = w
	if err := ts.Restore(); err != nil {
		return fmt.Errorf("failed to adopt orphan session %s: %w", tmuxName, err)
	}
	i.started = true
	i.SetStatus(Ready)
	return nil
}

// resetExecutionSession creates a fresh execution session for Restart().
// For tmux sessions, the underlying TmuxSession.NewReset preserves injected
// test dependencies (ptyFactory, cmdExec). For all other modes a new session
// is constructed via NewExecutionSession.
func (i *Instance) resetExecutionSession() ExecutionSession {
	if ts, ok := i.executionSession.(*tmuxExecutionSession); ok {
		return &tmuxExecutionSession{s: ts.s.NewReset(i.Title, i.Program, i.SkipPermissions)}
	}
	return newExecutionSession(i.ExecutionMode, i.Title, i.Program, i.SkipPermissions)
}

// Restart closes the current execution session (best-effort) and launches a fresh one
// with the same configuration. The worktree and branch are preserved. Ephemeral
// per-run flags are reset so the instance appears freshly started.
func (i *Instance) Restart() error {
	if !i.started {
		return fmt.Errorf("cannot restart instance that has not been started")
	}
	if i.Status == Paused {
		return fmt.Errorf("cannot restart paused instance; resume it first")
	}
	if err := i.ensureSharedKasmosMCP(); err != nil {
		return err
	}

	// Best-effort: session may already be dead.
	if i.executionSession != nil {
		_ = i.executionSession.Close()
	}

	// Allocate a new session object, carrying over injected test dependencies.
	prepareSession := func() {
		i.executionSession = i.resetExecutionSession()
		i.executionSession.SetAgentType(i.AgentType)
		i.executionSession.SetSDKSpeedTier(i.SDKSpeedTier)
		i.applyResourceControlsToSession()
		i.applySDKRetentionToSession()
		i.setExecutionTaskEnv()
		i.configureSessionTitle()
	}
	prepareSession()

	workDir := i.Path
	if i.gitWorktree != nil {
		workDir = i.gitWorktree.GetWorktreePath()
	}

	if err := i.startExecutionSessionWithFallback(workDir, prepareSession); err != nil {
		return fmt.Errorf("failed to restart session: %w", err)
	}

	// Reset ephemeral per-run state.
	i.resetEphemeralState()

	i.SetStatus(Running)
	return nil
}

// Resume recreates the worktree for a paused instance and reconnects or starts
// a fresh execution session inside it. The behaviour depends on worktree ownership:
//   - nil worktree (main-branch / planner): uses i.Path, no worktree setup/cleanup
//   - shared worktree: reuses existing worktree path, no setup/cleanup (Pause never removed it)
//   - owned worktree: recreates via Setup(), cleans up on failure
func (i *Instance) Resume() error {
	if !i.started {
		return fmt.Errorf("cannot resume instance that has not been started")
	}
	if i.Status != Paused {
		return fmt.Errorf("can only resume paused instances")
	}

	// Determine working directory and perform worktree setup based on ownership.
	var workDir string
	switch {
	case i.gitWorktree == nil:
		// Main-branch / planner instance — no worktree involved.
		workDir = i.Path

	case i.sharedWorktree:
		// Shared worktree — Pause() never removed it, so just reuse.
		workDir = i.gitWorktree.GetWorktreePath()

	default:
		// Owned worktree — guard against branch conflicts and recreate.
		checked, err := i.gitWorktree.IsBranchCheckedOut()
		if err != nil {
			log.ErrorLog.Print(err)
			return fmt.Errorf("failed to check if branch is checked out: %w", err)
		}
		if checked {
			return fmt.Errorf("cannot resume: branch is checked out, please switch to a different branch")
		}

		if err := i.gitWorktree.Setup(); err != nil {
			log.ErrorLog.Print(err)
			return fmt.Errorf("failed to setup git worktree: %w", err)
		}
		workDir = i.gitWorktree.GetWorktreePath()
	}

	// Reconnect or start a fresh execution session.
	if i.executionSession.DoesSessionExist() {
		if restoreErr := i.executionSession.Restore(); restoreErr != nil {
			log.ErrorLog.Print(restoreErr)
			// Fall back to a fresh session start — probe the shared endpoint first.
			if probeErr := i.ensureSharedKasmosMCP(); probeErr != nil {
				return probeErr
			}
			prepareSession := func() {
				i.executionSession = i.resetExecutionSession()
				i.executionSession.SetAgentType(i.AgentType)
				i.executionSession.SetSDKSpeedTier(i.SDKSpeedTier)
				i.applyResourceControlsToSession()
				i.applySDKRetentionToSession()
				i.setExecutionTaskEnv()
				i.configureSessionTitle()
			}
			prepareSession()
			if startErr := i.startExecutionSessionWithFallback(workDir, prepareSession); startErr != nil {
				log.ErrorLog.Print(startErr)
				if i.gitWorktree != nil && !i.sharedWorktree {
					if cleanupErr := i.gitWorktree.Cleanup(); cleanupErr != nil {
						startErr = fmt.Errorf("%v (cleanup: %v)", startErr, cleanupErr)
						log.ErrorLog.Print(startErr)
					}
				}
				return fmt.Errorf("failed to start new session: %w", startErr)
			}
		}
	} else {
		if err := i.ensureSharedKasmosMCP(); err != nil {
			return err
		}
		prepareSession := func() {
			i.executionSession = i.resetExecutionSession()
			i.executionSession.SetAgentType(i.AgentType)
			i.executionSession.SetSDKSpeedTier(i.SDKSpeedTier)
			i.applyResourceControlsToSession()
			i.applySDKRetentionToSession()
			i.setExecutionTaskEnv()
			i.configureSessionTitle()
		}
		prepareSession()
		if err := i.startExecutionSessionWithFallback(workDir, prepareSession); err != nil {
			log.ErrorLog.Print(err)
			if i.gitWorktree != nil && !i.sharedWorktree {
				if cleanupErr := i.gitWorktree.Cleanup(); cleanupErr != nil {
					err = fmt.Errorf("%v (cleanup: %v)", err, cleanupErr)
					log.ErrorLog.Print(err)
				}
			}
			return fmt.Errorf("failed to start new session: %w", err)
		}
	}

	// Reset ephemeral per-run state (mirrors Restart).
	i.resetEphemeralState()

	i.SetStatus(Running)
	return nil
}

// resetEphemeralState zeroes transient per-run flags so an instance appears
// freshly started. Used by both Restart() and Resume().
func (i *Instance) resetEphemeralState() {
	i.Exited = false
	i.PromptDetected = false
	i.HasWorked = false
	i.AwaitingWork = false
	i.Notified = false
	i.CachedContentSet = false
	i.CachedContent = ""
	i.PermissionBlocked = false
	i.CompletionPromptSince = time.Time{}
}
