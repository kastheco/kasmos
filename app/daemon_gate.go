package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/kastheco/kasmos/config"
	"github.com/kastheco/kasmos/config/taskstore"
	daemonpkg "github.com/kastheco/kasmos/daemon"
	"github.com/kastheco/kasmos/daemon/api"
	"github.com/kastheco/kasmos/internal/platform"
	"github.com/kastheco/kasmos/session"
	gitpkg "github.com/kastheco/kasmos/session/git"
	"github.com/kastheco/kasmos/session/tmux"
	"github.com/kastheco/kasmos/ui/overlay"

	tea "charm.land/bubbletea/v2"
)

type daemonStatusMsg struct {
	ready           bool
	message         string
	canRegisterRepo bool
	autoRegistered  bool
}

type daemonRepoRegisteredMsg struct {
	path string
}

var listDaemonInstances = func(project string) ([]api.InstanceStatus, error) {
	return daemonpkg.NewSocketClient(taskstore.ResolvedDaemonSocketPath()).ListInstances(project)
}

type daemonActionClient interface {
	SendInstancePrompt(project, title, prompt string) error
	SendInstancePromptWithLocalImages(project, title, prompt string, imagePaths []string) error
	KillInstance(project, title string) error
	SendInstancePermissionResponse(project, title string, choice tmux.PermissionChoice) error
}

var newDaemonActionClient = func() daemonActionClient {
	return daemonpkg.NewSocketClient(taskstore.ResolvedDaemonSocketPath())
}

// daemonStartCommand is a seam that tests can replace to verify the call site
// delegates to the platform package without depending on the host OS.
var daemonStartCommand = platform.DaemonStartCommand

// daemonRequiredMessage formats the user-facing message shown when the kasmos
// daemon is unavailable or not yet managing this repo. prefix describes the
// specific failure; repoPath is shown in the registration hint.
func daemonRequiredMessage(prefix, repoPath string) string {
	return fmt.Sprintf(
		"%s\n\nstart it with:\n  %s\n\nthen register this repo:\n  kas daemon add %s",
		prefix,
		daemonStartCommand(),
		repoPath,
	)
}

func canonicalRepoPath(repoPath string) string {
	if repoPath == "" {
		return ""
	}
	if root, err := config.ResolveRepoRoot(repoPath); err == nil && root != "" {
		repoPath = root
	}
	if realPath, err := filepath.EvalSymlinks(repoPath); err == nil && realPath != "" {
		repoPath = realPath
	}
	return filepath.Clean(repoPath)
}

func checkDaemonStatus(repoPath string) daemonStatusMsg {
	repoPath = canonicalRepoPath(repoPath)
	socketPath := taskstore.ResolvedDaemonSocketPath()
	transport := &http.Transport{
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			var d net.Dialer
			d.Timeout = 300 * time.Millisecond
			return d.DialContext(ctx, "unix", socketPath)
		},
	}
	client := &http.Client{Transport: transport, Timeout: 500 * time.Millisecond}

	resp, err := client.Get("http://daemon/v1/status")
	if err != nil {
		return daemonStatusMsg{
			message: daemonRequiredMessage(
				"agent workflows require the kasmos daemon.",
				repoPath,
			),
		}
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return daemonStatusMsg{
			message: daemonRequiredMessage(
				"agent workflows require the kasmos daemon, but the daemon status check failed.",
				repoPath,
			),
		}
	}

	var status api.StatusResponse
	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
		return daemonStatusMsg{
			message: daemonRequiredMessage(
				"agent workflows require the kasmos daemon, but its status response could not be read.",
				repoPath,
			),
		}
	}

	cleanRepoPath := canonicalRepoPath(repoPath)
	for _, repo := range status.Repos {
		if canonicalRepoPath(repo.Path) == cleanRepoPath {
			return daemonStatusMsg{ready: true}
		}
	}

	if err := registerRepoWithDaemon(repoPath); err == nil {
		return daemonStatusMsg{ready: true, autoRegistered: true}
	} else if repoManagedByDaemon(repoPath) {
		return daemonStatusMsg{ready: true, autoRegistered: true}
	}

	return daemonStatusMsg{
		canRegisterRepo: true,
		message: fmt.Sprintf(
			"the kasmos daemon is running, but this repo is not registered.\n\npress y to register it now, or run:\n  kas daemon add %s",
			repoPath,
		),
	}
}

func registerRepoWithDaemon(repoPath string) error {
	return daemonpkg.NewSocketClient(taskstore.ResolvedDaemonSocketPath()).AddRepo(canonicalRepoPath(repoPath))
}

func daemonInstanceData(repoPath string, status api.InstanceStatus) session.InstanceData {
	program := status.Program
	if program == "" {
		program = "opencode"
	}
	instStatus := session.Running
	if status.Loading {
		instStatus = session.Loading
	}
	// Honour the execution mode the daemon reports. Hardcoding tmux here
	// made the TUI wrap sdk-backed agents (codex/claude SDK) in a tmux
	// executionSession shim whose DoesSessionExist() asks `tmux
	// has-session` for a session that was never created — it's an SDK
	// subprocess, not a tmux pane. The sync loop then marked the restored
	// instance exited every tick and the agent never showed up in the
	// sidebar despite the daemon tracking it fine.
	mode := session.NormalizeExecutionMode(session.ExecutionMode(status.ExecutionMode))
	data := session.InstanceData{
		Title:         status.Title,
		Path:          repoPath,
		Branch:        status.Branch,
		Status:        instStatus,
		Program:       program,
		ExecutionMode: mode,
		AutoYes:       true,
		TaskFile:      status.Plan,
		AgentType:     status.Role,
		TaskNumber:    status.TaskNumber,
		WaveNumber:    status.WaveNumber,
		ReviewCycle:   status.ReviewCycle,
		WaveTaskIndex: status.WaveTaskIndex,
		WaveTaskCount: status.WaveTaskCount,
		SoloAgent:     status.SoloAgent,
		SDKSpeedTier:  status.SDKSpeedTier,
	}
	if status.Branch != "" {
		shared := gitpkg.NewSharedTaskWorktree(repoPath, status.Branch)
		data.Worktree = session.GitWorktreeData{
			RepoPath:     shared.GetRepoPath(),
			WorktreePath: shared.GetWorktreePath(),
			SessionName:  status.Title,
			BranchName:   status.Branch,
		}
	}
	return data
}

func daemonLoadingTotal(status api.InstanceStatus) int {
	switch {
	case status.TaskNumber > 0:
		return 8
	case status.Role == session.AgentTypePlanner, status.Role == session.AgentTypeElaborator:
		return 5
	default:
		return 6
	}
}

// newDaemonSDKInstance constructs a sidebar-only placeholder for an
// SDK-backed agent that is running inside the daemon. The returned
// Instance has no executionSession wired up (SDK sessions can't be
// mirrored across process boundaries); callers rely on the daemon for
// preview, send-prompt, and kill operations. Status mirrors what the
// daemon reports so the TUI doesn't flicker between running/ready.
func newDaemonSDKInstance(repoPath string, status api.InstanceStatus) (*session.Instance, error) {
	program := status.Program
	if program == "" {
		program = "opencode"
	}
	inst, err := session.NewInstance(session.InstanceOptions{
		Title:         status.Title,
		Path:          repoPath,
		Program:       program,
		ExecutionMode: session.ExecutionModeSDK,
		AutoYes:       true,
		TaskFile:      status.Plan,
		AgentType:     status.Role,
		TaskNumber:    status.TaskNumber,
		WaveNumber:    status.WaveNumber,
		ReviewCycle:   status.ReviewCycle,
		WaveTaskIndex: status.WaveTaskIndex,
		WaveTaskCount: status.WaveTaskCount,
		SDKSpeedTier:  status.SDKSpeedTier,
	})
	if err != nil {
		return nil, err
	}
	inst.SoloAgent = status.SoloAgent
	if status.Branch != "" {
		inst.BindSharedTaskWorktree(repoPath, status.Branch)
	}
	switch {
	case status.Ready:
		inst.SetStatus(session.Ready)
	case status.Loading:
		inst.SetStatus(session.Loading)
		inst.LoadingTotal = daemonLoadingTotal(status)
		inst.LoadingStage = 1
		inst.LoadingMessage = "waiting for session..."
	default:
		inst.SetStatus(session.Running)
	}
	return inst, nil
}

func newDaemonLoadingInstance(repoPath string, status api.InstanceStatus) (*session.Instance, error) {
	program := status.Program
	if program == "" {
		program = "opencode"
	}
	inst, err := session.NewInstance(session.InstanceOptions{
		Title:         status.Title,
		Path:          repoPath,
		Program:       program,
		ExecutionMode: session.ExecutionModeTmux,
		AutoYes:       true,
		TaskFile:      status.Plan,
		AgentType:     status.Role,
		TaskNumber:    status.TaskNumber,
		WaveNumber:    status.WaveNumber,
		ReviewCycle:   status.ReviewCycle,
		WaveTaskIndex: status.WaveTaskIndex,
		WaveTaskCount: status.WaveTaskCount,
	})
	if err != nil {
		return nil, err
	}
	if status.Branch != "" {
		inst.BindSharedTaskWorktree(repoPath, status.Branch)
	}
	inst.SetStatus(session.Loading)
	inst.LoadingTotal = daemonLoadingTotal(status)
	inst.LoadingStage = 1
	inst.LoadingMessage = "waiting for session..."
	return inst, nil
}

func restoreDaemonInstance(repoPath string, status api.InstanceStatus) (*session.Instance, error) {
	// SDK-backed agents (codex/claude SDK) live as subprocesses inside the
	// daemon; the TUI cannot meaningfully replicate their ExecutionSession
	// because the JSON-RPC transport lives in the daemon process. Trying
	// restoreInstanceFromData anyway produces a fresh SDK Session with
	// alive=false, DoesSessionExist reports false, and FromInstanceData
	// marks the restored instance Exited → sidebar drops it every tick.
	// Mirror the daemon's state with a display-only placeholder instead so
	// the sidebar shows the agent while the daemon keeps driving it.
	if status.Active && session.NormalizeExecutionMode(session.ExecutionMode(status.ExecutionMode)) == session.ExecutionModeSDK {
		placeholder, placeholderErr := newDaemonSDKInstance(repoPath, status)
		if placeholderErr == nil {
			return placeholder, nil
		}
		return nil, fmt.Errorf("restore daemon sdk instance %q: %w", status.Title, placeholderErr)
	}
	inst, err := restoreInstanceFromData(daemonInstanceData(repoPath, status))
	if err == nil && inst != nil && !inst.Exited {
		return inst, nil
	}
	if status.Active && status.Loading {
		placeholder, placeholderErr := newDaemonLoadingInstance(repoPath, status)
		if placeholderErr == nil {
			return placeholder, nil
		}
		if err != nil {
			return nil, fmt.Errorf("restore daemon loading instance %q: %w (placeholder: %v)", status.Title, err, placeholderErr)
		}
		return nil, placeholderErr
	}
	if err != nil {
		return nil, err
	}
	if inst != nil && inst.Exited {
		return nil, fmt.Errorf("daemon instance %q restored as exited", status.Title)
	}
	return nil, fmt.Errorf("daemon instance %q unavailable", status.Title)
}

// isDaemonSDKPlaceholder reports whether inst is a display-only entry
// constructed by newDaemonSDKInstance. Those instances have no local
// executionSession — all lifecycle ops (send/kill/pause/restart, preview
// capture) need to reach the daemon via the control socket instead of
// running locally on the empty placeholder. Detection heuristic: SDK
// execution mode + inst.Started() == false (the placeholder never calls
// inst.Start since there's no real transport to attach to).
func (m *home) isDaemonSDKPlaceholder(inst *session.Instance) bool {
	if inst == nil {
		return false
	}
	if session.NormalizeExecutionMode(inst.ExecutionMode) != session.ExecutionModeSDK {
		return false
	}
	return !inst.Started()
}

// daemonRouteSend routes a compose-prompt submit through the daemon API
// when inst is an SDK placeholder. Returns (handled, err): handled=true
// means the caller should stop (we did the work); handled=false means
// fall through to the local inst.SendPrompt path.
func (m *home) daemonRouteSend(inst *session.Instance, prompt string) (bool, error) {
	if !m.isDaemonSDKPlaceholder(inst) {
		return false, nil
	}
	project := m.taskStoreProject
	if project == "" {
		return true, fmt.Errorf("daemon route: no project for %q", inst.Title)
	}
	client := newDaemonActionClient()
	return true, client.SendInstancePrompt(project, inst.Title, prompt)
}

// daemonRouteSendCmd retries daemon prompt delivery for SDK placeholders in a
// background tea.Cmd so Bubble Tea's Update path stays non-blocking while the
// daemon finishes registering a newly spawned placeholder.
func (m *home) daemonRouteSendCmd(inst *session.Instance, prompt, auditMsg string) tea.Cmd {
	if !m.isDaemonSDKPlaceholder(inst) {
		return nil
	}
	project := m.taskStoreProject
	return func() tea.Msg {
		if project == "" {
			return promptSubmittedMsg{
				instance: inst,
				auditMsg: auditMsg,
				err:      fmt.Errorf("daemon route: no project for %q", inst.Title),
			}
		}
		client := newDaemonActionClient()
		deadline := time.Now().Add(plannerInstanceWaitTimeout)
		for {
			err := client.SendInstancePrompt(project, inst.Title, prompt)
			if err == nil {
				return promptSubmittedMsg{instance: inst, auditMsg: auditMsg}
			}
			var statusErr *daemonpkg.ClientStatusError
			if errors.As(err, &statusErr) &&
				inst.Status == session.Loading &&
				(statusErr.StatusCode == http.StatusNotFound || statusErr.StatusCode == http.StatusConflict) &&
				time.Now().Before(deadline) {
				time.Sleep(plannerInstancePollInterval)
				continue
			}
			return promptSubmittedMsg{
				instance: inst,
				auditMsg: auditMsg,
				err:      err,
			}
		}
	}
}

func (m *home) daemonRouteSendImagesCmd(inst *session.Instance, prompt string, imagePaths []string, auditMsg string) tea.Cmd {
	if !m.isDaemonSDKPlaceholder(inst) {
		return nil
	}
	project := m.taskStoreProject
	filtered := make([]string, 0, len(imagePaths))
	for _, imagePath := range imagePaths {
		if trimmed := strings.TrimSpace(imagePath); trimmed != "" {
			filtered = append(filtered, trimmed)
		}
	}
	return func() tea.Msg {
		defer removeLocalFiles(filtered)
		if project == "" {
			return promptSubmittedMsg{
				instance: inst,
				auditMsg: auditMsg,
				err:      fmt.Errorf("daemon route: no project for %q", inst.Title),
			}
		}
		client := newDaemonActionClient()
		deadline := time.Now().Add(plannerInstanceWaitTimeout)
		for {
			err := client.SendInstancePromptWithLocalImages(project, inst.Title, prompt, filtered)
			if err == nil {
				return promptSubmittedMsg{instance: inst, auditMsg: auditMsg}
			}
			var statusErr *daemonpkg.ClientStatusError
			if errors.As(err, &statusErr) &&
				inst.Status == session.Loading &&
				(statusErr.StatusCode == http.StatusNotFound || statusErr.StatusCode == http.StatusConflict) &&
				time.Now().Before(deadline) {
				time.Sleep(plannerInstancePollInterval)
				continue
			}
			return promptSubmittedMsg{
				instance: inst,
				auditMsg: auditMsg,
				err:      err,
			}
		}
	}
}

// daemonRouteKill routes a kill action through the daemon API for SDK
// placeholder instances. Same (handled, err) contract as daemonRouteSend.
func (m *home) daemonRouteKill(inst *session.Instance) (bool, error) {
	if !m.isDaemonSDKPlaceholder(inst) {
		return false, nil
	}
	project := m.taskStoreProject
	if project == "" {
		return true, fmt.Errorf("daemon route: no project for %q", inst.Title)
	}
	client := newDaemonActionClient()
	return true, client.KillInstance(project, inst.Title)
}

// daemonRoutePermissionResponse routes a permission-overlay response through
// the daemon API for SDK placeholder instances. Same (handled, err) contract as
// daemonRouteSend.
func (m *home) daemonRoutePermissionResponse(inst *session.Instance, choice tmux.PermissionChoice) (bool, error) {
	if !m.isDaemonSDKPlaceholder(inst) {
		return false, nil
	}
	project := m.taskStoreProject
	if project == "" {
		return true, fmt.Errorf("daemon route: no project for %q", inst.Title)
	}
	client := newDaemonActionClient()
	return true, client.SendInstancePermissionResponse(project, inst.Title, choice)
}

func (m *home) daemonStartupCheckCmd() tea.Cmd {
	if m.daemonStatusChecker == nil {
		return nil
	}
	repoPath := m.activeRepoPath
	checker := m.daemonStatusChecker
	return func() tea.Msg {
		return checker(repoPath)
	}
}

func (m *home) requireDaemonForAgents() bool {
	if m.daemonStatusChecker == nil {
		return true
	}
	status := m.daemonStatusChecker(m.activeRepoPath)
	if status.ready {
		return true
	}
	m.showDaemonRequiredDialog(status)
	return false
}

func (m *home) showDaemonRequiredDialog(status daemonStatusMsg) {
	if m.overlays == nil {
		m.overlays = overlay.NewManager()
	}
	m.state = stateConfirm
	m.pendingConfirmAction = nil
	if status.canRegisterRepo && m.daemonRepoRegistrar != nil {
		repoPath := m.activeRepoPath
		registrar := m.daemonRepoRegistrar
		m.pendingConfirmAction = func() tea.Msg {
			if err := registrar(repoPath); err != nil {
				return err
			}
			return daemonRepoRegisteredMsg{path: repoPath}
		}
	}
	co := overlay.NewConfirmationOverlay(status.message)
	co.SetSize(76, 0)
	m.overlays.Show(co)
}
