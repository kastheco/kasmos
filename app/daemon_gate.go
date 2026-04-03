package app

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"path/filepath"
	"time"

	"github.com/kastheco/kasmos/config"
	"github.com/kastheco/kasmos/config/taskstore"
	daemonpkg "github.com/kastheco/kasmos/daemon"
	"github.com/kastheco/kasmos/daemon/api"
	"github.com/kastheco/kasmos/session"
	gitpkg "github.com/kastheco/kasmos/session/git"
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
			message: fmt.Sprintf(
				"agent workflows require the kasmos daemon.\n\nstart it with:\n  systemctl --user start kasmos\n\nthen register this repo:\n  kas daemon add %s",
				repoPath,
			),
		}
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return daemonStatusMsg{
			message: fmt.Sprintf(
				"agent workflows require the kasmos daemon, but the daemon status check failed.\n\nstart it with:\n  systemctl --user start kasmos\n\nthen register this repo:\n  kas daemon add %s",
				repoPath,
			),
		}
	}

	var status api.StatusResponse
	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
		return daemonStatusMsg{
			message: fmt.Sprintf(
				"agent workflows require the kasmos daemon, but its status response could not be read.\n\nstart it with:\n  systemctl --user start kasmos\n\nthen register this repo:\n  kas daemon add %s",
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
	data := session.InstanceData{
		Title:         status.Title,
		Path:          repoPath,
		Branch:        status.Branch,
		Status:        instStatus,
		Program:       program,
		ExecutionMode: session.ExecutionModeTmux,
		AutoYes:       true,
		TaskFile:      status.Plan,
		AgentType:     status.Role,
		TaskNumber:    status.TaskNumber,
		WaveNumber:    status.WaveNumber,
		ReviewCycle:   status.ReviewCycle,
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
