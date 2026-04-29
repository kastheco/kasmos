package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/kastheco/kasmos/cmd"
	"github.com/kastheco/kasmos/config"
	"github.com/kastheco/kasmos/config/taskfsm"
	"github.com/kastheco/kasmos/config/taskparser"
	"github.com/kastheco/kasmos/config/taskstore"
	"github.com/kastheco/kasmos/daemon/api"
	"github.com/kastheco/kasmos/internal/theme"
	"github.com/kastheco/kasmos/orchestration"
	"github.com/kastheco/kasmos/orchestration/loop"
	"github.com/kastheco/kasmos/session"
	sessionsdk "github.com/kastheco/kasmos/session/sdk"
	tmuxpkg "github.com/kastheco/kasmos/session/tmux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDaemon_TaskComplete_ReplayedFailedWaveEmitsOnce(t *testing.T) {
	store := taskstore.NewTestStore(t)
	project := "test-project"
	planFile := "failed-wave.md"
	require.NoError(t, store.Create(project, taskstore.TaskEntry{
		Filename: planFile,
		Status:   taskstore.StatusImplementing,
	}))

	proc := loop.NewProcessor(loop.ProcessorConfig{Store: store, Project: project})
	proc.RegisterOrchestrator(planFile, 1, []int{1, 2})
	orch := proc.WaveOrchestrator(planFile)
	require.NotNil(t, orch)
	orch.MarkTaskFailed(1)

	actions := proc.ProcessTaskSignals([]taskfsm.TaskSignal{{
		TaskFile:   planFile,
		WaveNumber: 1,
		TaskNumber: 2,
	}})
	require.Len(t, actions, 1)
	taskAction, ok := actions[0].(loop.TaskCompleteAction)
	require.True(t, ok)

	broadcaster := api.NewEventBroadcaster()
	sub := broadcaster.Subscribe()
	t.Cleanup(func() {
		broadcaster.Unsubscribe(sub)
		broadcaster.Close()
	})
	d := &Daemon{
		cfg:         &DaemonConfig{},
		spawner:     NewTmuxSpawner(),
		logger:      slog.Default(),
		broadcaster: broadcaster,
		killWaveAgents: func(repoPath, pf string, wave int) error {
			return nil
		},
	}
	e := RepoEntry{
		Path:      t.TempDir(),
		Project:   project,
		Store:     store,
		Processor: proc,
	}

	require.NoError(t, d.executeAction(context.Background(), e, taskAction))
	require.NoError(t, d.executeAction(context.Background(), e, taskAction))

	var waveFailed []api.Event
	timeout := time.After(200 * time.Millisecond)
	for {
		select {
		case ev := <-sub:
			if ev.Kind == "wave_failed" {
				waveFailed = append(waveFailed, ev)
			}
		case <-timeout:
			require.Len(t, waveFailed, 1)
			assert.Equal(t, "failed-wave.md: wave 1 needs a decision (1 of 2 tasks failed)", waveFailed[0].Message)
			var detail map[string]any
			require.NoError(t, json.Unmarshal([]byte(waveFailed[0].Detail), &detail))
			assert.Equal(t, "wave_terminal", detail["outcome"])
			assert.Equal(t, float64(0), detail["retry_generation"])
			return
		}
	}
}

func TestDaemon_StartStop(t *testing.T) {
	dir := t.TempDir()
	cfg := &DaemonConfig{
		PollInterval: 100 * time.Millisecond,
		SocketPath:   filepath.Join(dir, "kas.sock"),
	}
	d, err := NewDaemon(cfg)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() {
		errCh <- d.Run(ctx)
	}()

	time.Sleep(250 * time.Millisecond)
	cancel()

	err = <-errCh
	assert.NoError(t, err)
}

func TestDaemon_ControlSocket(t *testing.T) {
	dir := t.TempDir()
	cfg := &DaemonConfig{
		PollInterval: 100 * time.Millisecond,
		SocketPath:   filepath.Join(dir, "kas.sock"),
	}
	d, err := NewDaemon(cfg)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() {
		errCh <- d.Run(ctx)
	}()

	// Wait for socket to appear
	require.Eventually(t, func() bool {
		_, err := os.Stat(cfg.SocketPath)
		return err == nil
	}, 2*time.Second, 50*time.Millisecond)

	// Connect and query status
	client := NewSocketClient(cfg.SocketPath)
	status, err := client.Status()
	require.NoError(t, err)
	assert.True(t, status.Running)

	cancel()
	<-errCh
}

func TestDaemon_RunRejectsSecondInstanceForSameSocket(t *testing.T) {
	dir := t.TempDir()
	cfg := &DaemonConfig{
		PollInterval: 100 * time.Millisecond,
		SocketPath:   filepath.Join(dir, "kas.sock"),
	}

	first, err := NewDaemon(cfg)
	require.NoError(t, err)

	ctx1, cancel1 := context.WithCancel(context.Background())
	errCh1 := make(chan error, 1)
	go func() {
		errCh1 <- first.Run(ctx1)
	}()

	require.Eventually(t, func() bool {
		_, err := os.Stat(cfg.SocketPath)
		return err == nil
	}, 2*time.Second, 50*time.Millisecond)

	second, err := NewDaemon(cfg)
	require.NoError(t, err)
	err = second.Run(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "daemon already running")

	cancel1()
	assert.NoError(t, <-errCh1)
}

func TestDaemon_AddRepo(t *testing.T) {
	dir := t.TempDir()
	cfg := &DaemonConfig{
		PollInterval: 100 * time.Millisecond,
		SocketPath:   filepath.Join(dir, "kas.sock"),
	}
	d, err := NewDaemon(cfg)
	require.NoError(t, err)
	// Use an isolated test store so the test does not touch the real global DB.
	d.repos = newTestRepoManager(t)

	tmpDir := t.TempDir()
	err = d.AddRepo(tmpDir)
	assert.NoError(t, err)

	repos := d.ListRepos()
	assert.Len(t, repos, 1)
}

func TestDaemonStateAdapter_ListTasksMapsEntries(t *testing.T) {
	const project = "proj"
	store := taskstore.NewTestStore(t)
	require.NoError(t, store.Create(project, taskstore.TaskEntry{
		Filename:    "feature.md",
		Status:      taskstore.StatusReviewing,
		Description: "ship feature",
		Branch:      "plan/feature",
		Topic:       "core",
		ReviewCycle: 3,
		PRURL:       "https://example.com/pr/1",
		ExecutionState: taskstore.ExecutionState{
			Phase:           string(taskfsm.ExecutionPhaseReviewing),
			ActiveAgentType: session.AgentTypeReviewer,
		},
	}))

	adapter := &daemonStateAdapter{d: &Daemon{repos: NewRepoManager()}}
	adapter.d.repos.repos = []RepoEntry{{Project: project, Store: store}}

	tasks, err := adapter.ListTasks(project)
	require.NoError(t, err)
	require.Len(t, tasks, 1)
	assert.Equal(t, []api.TaskStatus{{
		Filename:    "feature.md",
		Status:      string(taskstore.StatusReviewing),
		Description: "ship feature",
		Branch:      "plan/feature",
		Topic:       "core",
		ReviewCycle: 3,
		PRURL:       "https://example.com/pr/1",
		ExecutionState: taskstore.ExecutionState{
			Phase:           string(taskfsm.ExecutionPhaseReviewing),
			ActiveAgentType: session.AgentTypeReviewer,
		},
	}}, tasks)
}

// TestDaemonStateAdapter_InstanceActionErrorMapping verifies that the adapter
// wraps spawner sentinel errors as api.ErrInstanceNotFound and api.ErrInvalidTransition,
// which is the end-to-end contract the daemon HTTP handler relies on to return
// 404 for missing titles and 409 for invalid pause/resume/restart transitions.
func TestDaemonStateAdapter_InstanceActionErrorMapping(t *testing.T) {
	const (
		project = "proj"
		repo    = "/tmp/proj"
	)

	newAdapter := func() (*daemonStateAdapter, *TmuxSpawner) {
		spawner := NewTmuxSpawner()
		d := &Daemon{
			repos:       NewRepoManager(),
			spawner:     spawner,
			logger:      slog.Default(),
			broadcaster: api.NewEventBroadcaster(),
		}
		d.repos.repos = []RepoEntry{{Path: repo, Project: project}}
		return &daemonStateAdapter{d: d}, spawner
	}

	trackInstance := func(s *TmuxSpawner, title string, started bool, status session.Status) {
		key := instanceKey(repo, "plan.md", session.AgentTypeReviewer)
		inst := &session.Instance{Title: title, Path: repo, Status: status}
		if started {
			inst.MarkStartedForTest()
		}
		s.mu.Lock()
		s.instances[key] = inst
		s.planFileByKey[key] = "plan.md"
		s.agentTypeByKey[key] = session.AgentTypeReviewer
		s.projectByKey[key] = project
		s.mu.Unlock()
	}

	t.Run("pause missing title returns ErrInstanceNotFound", func(t *testing.T) {
		adapter, _ := newAdapter()
		err := adapter.PauseInstance(project, "missing")
		require.Error(t, err)
		assert.ErrorIs(t, err, api.ErrInstanceNotFound)
	})

	t.Run("resume missing title returns ErrInstanceNotFound", func(t *testing.T) {
		adapter, _ := newAdapter()
		err := adapter.ResumeInstance(project, "missing")
		require.Error(t, err)
		assert.ErrorIs(t, err, api.ErrInstanceNotFound)
	})

	t.Run("restart missing title returns ErrInstanceNotFound", func(t *testing.T) {
		adapter, _ := newAdapter()
		err := adapter.RestartInstance(project, "missing")
		require.Error(t, err)
		assert.ErrorIs(t, err, api.ErrInstanceNotFound)
	})

	t.Run("kill missing title returns ErrInstanceNotFound", func(t *testing.T) {
		adapter, _ := newAdapter()
		err := adapter.KillInstance(project, "missing")
		require.Error(t, err)
		assert.ErrorIs(t, err, api.ErrInstanceNotFound)
	})

	t.Run("pause on paused instance returns ErrInvalidTransition", func(t *testing.T) {
		adapter, spawner := newAdapter()
		trackInstance(spawner, "agent-1", true, session.Paused)
		err := adapter.PauseInstance(project, "agent-1")
		require.Error(t, err)
		assert.ErrorIs(t, err, api.ErrInvalidTransition)
	})

	t.Run("pause on ready instance returns ErrInvalidTransition", func(t *testing.T) {
		// Ready rows only expose restart/kill in the web action matrix, so a
		// crafted POST that reaches the adapter must still fail at the
		// spawner guard rather than detach an idle-but-available agent.
		adapter, spawner := newAdapter()
		trackInstance(spawner, "agent-1", true, session.Ready)
		err := adapter.PauseInstance(project, "agent-1")
		require.Error(t, err)
		assert.ErrorIs(t, err, api.ErrInvalidTransition)
	})

	t.Run("resume on running instance returns ErrInvalidTransition", func(t *testing.T) {
		adapter, spawner := newAdapter()
		trackInstance(spawner, "agent-1", true, session.Running)
		err := adapter.ResumeInstance(project, "agent-1")
		require.Error(t, err)
		assert.ErrorIs(t, err, api.ErrInvalidTransition)
	})

	t.Run("restart on paused instance returns ErrInvalidTransition", func(t *testing.T) {
		adapter, spawner := newAdapter()
		trackInstance(spawner, "agent-1", true, session.Paused)
		err := adapter.RestartInstance(project, "agent-1")
		require.Error(t, err)
		assert.ErrorIs(t, err, api.ErrInvalidTransition)
	})
}

type shellCommandExecutionSession struct {
	command string
	err     error
	stats   sessionsdk.RendererStats
	turns   []*sessionsdk.PresentationTurn
}

var _ session.ExecutionSession = (*shellCommandExecutionSession)(nil)

func (s *shellCommandExecutionSession) Start(string) error     { return nil }
func (s *shellCommandExecutionSession) Restore() error         { return nil }
func (s *shellCommandExecutionSession) Close() error           { return nil }
func (s *shellCommandExecutionSession) DoesSessionExist() bool { return true }
func (s *shellCommandExecutionSession) SendKeys(string) error  { return nil }
func (s *shellCommandExecutionSession) TapEnter() error        { return nil }
func (s *shellCommandExecutionSession) SendPermissionResponse(tmuxpkg.PermissionChoice) error {
	return nil
}
func (s *shellCommandExecutionSession) CapturePaneContent() (string, error) { return "", nil }
func (s *shellCommandExecutionSession) CapturePaneContentWithOptions(string, string) (string, error) {
	return "", nil
}
func (s *shellCommandExecutionSession) HasUpdated() (bool, bool) { return false, false }
func (s *shellCommandExecutionSession) HasUpdatedWithContent() (bool, bool, string, bool) {
	return false, false, "", false
}
func (s *shellCommandExecutionSession) GetPanePID() (int, error)       { return 0, nil }
func (s *shellCommandExecutionSession) Attach() (chan struct{}, error) { return nil, nil }
func (s *shellCommandExecutionSession) DetachSafely() error            { return nil }
func (s *shellCommandExecutionSession) SetDetachedSize(int, int) error { return nil }
func (s *shellCommandExecutionSession) GetSanitizedName() string       { return "" }
func (s *shellCommandExecutionSession) SetAgentType(string)            {}
func (s *shellCommandExecutionSession) SetInitialPrompt(string)        {}
func (s *shellCommandExecutionSession) SetNoFlicker(bool)              {}
func (s *shellCommandExecutionSession) SetTaskEnv(int, int, int)       {}
func (s *shellCommandExecutionSession) SetProject(string)              {}
func (s *shellCommandExecutionSession) SetSessionTitle(string)         {}
func (s *shellCommandExecutionSession) SetTitleFunc(func(string, time.Time, string)) {
}
func (s *shellCommandExecutionSession) SetSDKSpeedTier(string)                              {}
func (s *shellCommandExecutionSession) SetResourceControls(config.ResolvedResourceControls) {}
func (s *shellCommandExecutionSession) RendererStats() sessionsdk.RendererStats {
	return s.stats
}
func (s *shellCommandExecutionSession) CapturePresentation() []*sessionsdk.PresentationTurn {
	return s.turns
}
func (s *shellCommandExecutionSession) RunShellCommand(_ context.Context, command string) error {
	s.command = command
	return s.err
}

func TestDaemonStateAdapter_CaptureInstance_RangeUsesRepoPaletteForSDK(t *testing.T) {
	const (
		project  = "proj"
		repoPath = "/tmp/proj"
		title    = "sdk-agent"
	)
	t.Cleanup(func() {
		theme.SetCurrent(theme.DefaultPalette())
	})
	globalPalette := theme.DefaultPalette()
	globalPalette.Text = "#aabbcc"
	theme.SetCurrent(globalPalette)
	repoPalette := theme.DefaultPalette()
	repoPalette.Text = "#112233"

	spawner := NewTmuxSpawner()
	broadcaster := api.NewEventBroadcaster()
	t.Cleanup(func() { broadcaster.Close() })
	d := &Daemon{
		repos:       NewRepoManager(),
		spawner:     spawner,
		logger:      slog.Default(),
		broadcaster: broadcaster,
	}
	d.repos.repos = []RepoEntry{{
		Path:    repoPath,
		Project: project,
		Theme:   theme.Result{Palette: repoPalette},
	}}

	execSession := &shellCommandExecutionSession{
		turns: []*sessionsdk.PresentationTurn{
			{
				ID:     "t1",
				Number: 1,
				Rows: []sessionsdk.PresentationRow{
					{Kind: sessionsdk.RowResponse},
					{Kind: sessionsdk.RowProse, Text: "daemon ranged text"},
				},
			},
		},
	}
	inst := &session.Instance{
		Title:         title,
		Path:          repoPath,
		ExecutionMode: session.ExecutionModeSDK,
		Status:        session.Running,
	}
	inst.MarkStartedForTest()
	inst.SetExecutionSessionForTest(execSession)
	spawner.commitInstance(
		instanceKey(repoPath, "plan.md", session.AgentTypeCoder),
		"plan.md",
		session.AgentTypeCoder,
		project,
		inst,
	)

	preview, err := (&daemonStateAdapter{d: d}).CaptureInstance(project, title, "0", "999")
	require.NoError(t, err)
	assert.Contains(t, preview, lipgloss.NewStyle().Foreground(lipgloss.Color(string(repoPalette.Text))).Render("daemon ranged text"))
	assert.NotContains(t, preview, lipgloss.NewStyle().Foreground(lipgloss.Color(string(globalPalette.Text))).Render("daemon ranged text"))
}

func TestDaemonStateAdapter_RunInstanceShellCommand_DelegatesToTrackedSDKInstance(t *testing.T) {
	const (
		project  = "proj"
		repoPath = "/tmp/proj"
		title    = "sdk-agent"
	)

	spawner := NewTmuxSpawner()
	d := &Daemon{
		repos:       NewRepoManager(),
		spawner:     spawner,
		logger:      slog.Default(),
		broadcaster: api.NewEventBroadcaster(),
	}
	d.repos.repos = []RepoEntry{{Path: repoPath, Project: project}}

	execSession := &shellCommandExecutionSession{}
	inst := &session.Instance{
		Title:         title,
		Path:          repoPath,
		ExecutionMode: session.ExecutionModeSDK,
		Status:        session.Running,
	}
	inst.MarkStartedForTest()
	inst.SetExecutionSessionForTest(execSession)
	spawner.commitInstance(
		instanceKey(repoPath, "plan.md", session.AgentTypeCoder),
		"plan.md",
		session.AgentTypeCoder,
		project,
		inst,
	)

	adapter := &daemonStateAdapter{d: d}
	err := adapter.RunInstanceShellCommand(project, title, "echo")
	require.NoError(t, err)
	assert.Equal(t, "echo", execSession.command)
}

func TestDaemonStateAdapter_RunInstanceShellCommand_NonSDKInstanceInvalidRequest(t *testing.T) {
	const (
		project  = "proj"
		repoPath = "/tmp/proj"
		title    = "tmux-agent"
	)

	spawner := NewTmuxSpawner()
	d := &Daemon{
		repos:       NewRepoManager(),
		spawner:     spawner,
		logger:      slog.Default(),
		broadcaster: api.NewEventBroadcaster(),
	}
	d.repos.repos = []RepoEntry{{Path: repoPath, Project: project}}

	inst := &session.Instance{
		Title:         title,
		Path:          repoPath,
		ExecutionMode: session.ExecutionModeTmux,
		Status:        session.Running,
	}
	inst.MarkStartedForTest()
	spawner.commitInstance(
		instanceKey(repoPath, "plan.md", session.AgentTypeCoder),
		"plan.md",
		session.AgentTypeCoder,
		project,
		inst,
	)

	adapter := &daemonStateAdapter{d: d}
	err := adapter.RunInstanceShellCommand(project, title, "echo")

	require.ErrorIs(t, err, api.ErrInvalidRequest)
}

func TestDaemon_MonitorRunningInstances_CachesSDKRendererStats(t *testing.T) {
	const (
		project  = "proj"
		repoPath = "/tmp/proj"
		title    = "sdk-agent"
	)

	stats := sessionsdk.RendererStats{
		Bytes:         8192,
		Lines:         64,
		Turns:         7,
		MaxBytes:      8 << 20,
		MaxTurns:      3000,
		EvictedTurns:  3,
		EvictedLines:  10,
		EvictedBytes:  2048,
		TruncatedRows: 1,
	}
	execSession := &shellCommandExecutionSession{stats: stats}
	inst := &session.Instance{
		Title:         title,
		Path:          repoPath,
		ExecutionMode: session.ExecutionModeSDK,
		Status:        session.Running,
	}
	inst.MarkStartedForTest()
	inst.SetExecutionSessionForTest(execSession)

	spawner := NewTmuxSpawner()
	spawner.commitInstance(
		instanceKey(repoPath, "plan.md", session.AgentTypeCoder),
		"plan.md",
		session.AgentTypeCoder,
		project,
		inst,
	)
	d := &Daemon{
		spawner:     spawner,
		logger:      slog.Default(),
		broadcaster: api.NewEventBroadcaster(),
	}

	d.monitorRunningInstances(context.Background(), RepoEntry{Path: repoPath, Project: project})

	assert.Equal(t, stats, inst.RendererStats)
}

func TestDaemon_GracefulShutdown_DrainsAgents(t *testing.T) {
	dir := t.TempDir()
	cfg := &DaemonConfig{
		PollInterval: 100 * time.Millisecond,
		SocketPath:   filepath.Join(dir, "kas.sock"),
	}
	d, err := NewDaemon(cfg)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() {
		errCh <- d.Run(ctx)
	}()

	time.Sleep(200 * time.Millisecond)
	cancel()

	err = <-errCh
	assert.NoError(t, err)
	assert.Empty(t, d.spawner.RunningInstances())
}

func TestDaemon_RecoverOnRestart(t *testing.T) {
	sockDir := t.TempDir()
	cfg := &DaemonConfig{
		PollInterval: 100 * time.Millisecond,
		SocketPath:   filepath.Join(sockDir, "kas.sock"),
	}
	d, err := NewDaemon(cfg)
	require.NoError(t, err)
	// Use an isolated test store so the test does not touch the real global DB.
	d.repos = newTestRepoManager(t)

	repoDir := t.TempDir()
	require.NoError(t, d.AddRepo(repoDir))

	recovered, err := d.RecoverSessions()
	assert.NoError(t, err)
	assert.Equal(t, 0, recovered)
}

func TestDaemon_RecoverSessions_AdoptsTrackedInstances(t *testing.T) {
	project := "proj"
	store := taskstore.NewTestStore(t)
	require.NoError(t, store.Create(project, taskstore.TaskEntry{
		Filename: "feature.md",
		Status:   taskstore.StatusImplementing,
		Branch:   "plan/feature",
		ExecutionState: taskstore.ExecutionState{
			Phase:           string(taskfsm.ExecutionPhaseSingleAgentImplementing),
			ActiveAgentType: session.AgentTypeCoder,
		},
	}))

	d := &Daemon{
		repos:       NewRepoManager(),
		spawner:     NewTmuxSpawner(),
		logger:      slog.Default(),
		broadcaster: api.NewEventBroadcaster(),
	}
	d.repos.repos = []RepoEntry{{
		Path:    "/tmp/proj",
		Project: project,
		Store:   store,
	}}
	d.spawner.discoverOrphans = func(_ []string) ([]tmuxpkg.SessionInfo, error) {
		return []tmuxpkg.SessionInfo{{Title: "feature.md-coder"}}, nil
	}
	d.spawner.restoreInstance = func(data session.InstanceData) (*session.Instance, error) {
		return &session.Instance{
			Title:     data.Title,
			Path:      data.Path,
			TaskFile:  data.TaskFile,
			AgentType: data.AgentType,
		}, nil
	}

	recovered, err := d.RecoverSessions()
	require.NoError(t, err)
	assert.Equal(t, 1, recovered)

	running := d.spawner.RunningInstances()
	require.Len(t, running, 1)
	assert.Equal(t, "feature.md", running[0].PlanFile)
	assert.Equal(t, session.AgentTypeCoder, running[0].AgentType)
	assert.Equal(t, project, running[0].Project)
}

func TestDaemon_RecoverSessions_AdoptsNumberedReviewerSessions(t *testing.T) {
	project := "proj"
	store := taskstore.NewTestStore(t)
	require.NoError(t, store.Create(project, taskstore.TaskEntry{
		Filename:    "feature",
		Status:      taskstore.StatusReviewing,
		Branch:      "plan/feature",
		ReviewCycle: 5,
		ExecutionState: taskstore.ExecutionState{
			Phase:           string(taskfsm.ExecutionPhaseReviewing),
			ActiveAgentType: session.AgentTypeReviewer,
		},
	}))

	d := &Daemon{
		repos:       NewRepoManager(),
		spawner:     NewTmuxSpawner(),
		logger:      slog.Default(),
		broadcaster: api.NewEventBroadcaster(),
	}
	d.repos.repos = []RepoEntry{{
		Path:    "/tmp/proj",
		Project: project,
		Store:   store,
	}}
	d.spawner.discoverOrphans = func(_ []string) ([]tmuxpkg.SessionInfo, error) {
		return []tmuxpkg.SessionInfo{{Title: "feature-review-6"}}, nil
	}
	var restored session.InstanceData
	d.spawner.restoreInstance = func(data session.InstanceData) (*session.Instance, error) {
		restored = data
		return &session.Instance{Title: data.Title, Path: data.Path, TaskFile: data.TaskFile, AgentType: data.AgentType}, nil
	}

	recovered, err := d.RecoverSessions()
	require.NoError(t, err)
	assert.Equal(t, 1, recovered)

	running := d.spawner.RunningInstances()
	require.Len(t, running, 1)
	assert.Equal(t, "feature", running[0].PlanFile)
	assert.Equal(t, session.AgentTypeReviewer, running[0].AgentType)
	assert.Equal(t, "feature-review-6", restored.Title)
	assert.Equal(t, 6, restored.ReviewCycle)
}

func TestDaemon_RecoverSessions_AdoptsPlannerDraftProfiles(t *testing.T) {
	project := "proj"
	store := taskstore.NewTestStore(t)
	require.NoError(t, store.Create(project, taskstore.TaskEntry{
		Filename: "feature",
		Status:   taskstore.StatusPlanning,
	}))

	d := &Daemon{
		repos:       NewRepoManager(),
		spawner:     NewTmuxSpawner(),
		logger:      slog.Default(),
		broadcaster: api.NewEventBroadcaster(),
	}
	d.repos.repos = []RepoEntry{{
		Path:    "/tmp/proj",
		Project: project,
		Store:   store,
	}}
	d.spawner.discoverOrphans = func(_ []string) ([]tmuxpkg.SessionInfo, error) {
		return []tmuxpkg.SessionInfo{
			{Title: "feature-plan-planner-a"},
			{Title: "feature-plan-planner-b"},
		}, nil
	}
	var restoredProfiles []string
	d.spawner.restoreInstance = func(data session.InstanceData) (*session.Instance, error) {
		restoredProfiles = append(restoredProfiles, data.PlannerProfile)
		return &session.Instance{
			Title:          data.Title,
			Path:           data.Path,
			TaskFile:       data.TaskFile,
			AgentType:      data.AgentType,
			PlannerProfile: data.PlannerProfile,
		}, nil
	}

	recovered, err := d.RecoverSessions()
	require.NoError(t, err)
	assert.Equal(t, 2, recovered)
	assert.ElementsMatch(t, []string{"planner-a", "planner-b"}, restoredProfiles)

	running := d.spawner.RunningInstances()
	require.Len(t, running, 2)
	assert.ElementsMatch(t, []string{
		"/tmp/proj:feature:planner:planner-a",
		"/tmp/proj:feature:planner:planner-b",
	}, []string{running[0].Key, running[1].Key})
}

func TestDaemon_StartPlan_ReturnsBeforeSpawnCompletes(t *testing.T) {
	project := "proj"
	store := taskstore.NewTestStore(t)
	require.NoError(t, store.Create(project, taskstore.TaskEntry{
		Filename: "feature.md",
		Status:   taskstore.StatusPlanning,
	}))

	started := make(chan struct{}, 1)
	release := make(chan struct{})
	d := &Daemon{
		repos:       NewRepoManager(),
		logger:      slog.Default(),
		broadcaster: api.NewEventBroadcaster(),
		killAgent: func(repoPath, planFile, agentType string) error {
			return nil
		},
		spawnPlanner: func(_ context.Context, opts loop.SpawnOpts) error {
			select {
			case started <- struct{}{}:
			default:
			}
			<-release
			return nil
		},
	}
	d.repos.repos = []RepoEntry{{
		Path:    "/tmp/proj",
		Project: project,
		Store:   store,
	}}

	errCh := make(chan error, 1)
	go func() {
		errCh <- d.StartPlan(project, "feature.md", "plan prompt", "opencode")
	}()

	select {
	case err := <-errCh:
		require.NoError(t, err)
	case <-time.After(200 * time.Millisecond):
		t.Fatal("StartPlan should return before async spawn completes")
	}

	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("background planner spawn did not start")
	}

	close(release)
}

func TestDaemon_StartPlan_SpawnsLegacyPlannerWhenPlannerProfilesUnset(t *testing.T) {
	project := "proj"
	store := taskstore.NewTestStore(t)
	require.NoError(t, store.Create(project, taskstore.TaskEntry{
		Filename:    "feature.md",
		Status:      taskstore.StatusPlanning,
		Description: "ship feature",
	}))

	spawnedPlanner := make(chan loop.SpawnOpts, 1)
	d := &Daemon{
		repos:       NewRepoManager(),
		logger:      slog.Default(),
		broadcaster: api.NewEventBroadcaster(),
		killAgent: func(repoPath, planFile, agentType string) error {
			return nil
		},
		spawnPlanner: func(_ context.Context, opts loop.SpawnOpts) error {
			spawnedPlanner <- opts
			return nil
		},
	}
	t.Cleanup(func() { d.broadcaster.Close() })
	d.repos.repos = []RepoEntry{{
		Path:    t.TempDir(),
		Project: project,
		Store:   store,
	}}

	require.NoError(t, d.StartPlan(project, "feature.md", "plan prompt", "opencode"))

	select {
	case opts := <-spawnedPlanner:
		assert.Equal(t, "feature.md", opts.PlanFile)
		assert.Equal(t, "plan prompt", opts.Prompt)
		assert.Equal(t, "opencode", opts.Program)
		assert.False(t, opts.PlannerDraftMode)
	case <-time.After(time.Second):
		t.Fatal("planner spawn did not run")
	}
}

func TestDaemon_StartPlan_SpawnsPlannerDraftProfiles(t *testing.T) {
	project := "proj"
	repoPath := t.TempDir()
	kasmosDir := filepath.Join(repoPath, ".kasmos")
	require.NoError(t, os.MkdirAll(kasmosDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(kasmosDir, "config.toml"), []byte(`
[orchestration]
planners = ["planner-a", "planner-b"]

[agents.planner-a]
program = "codex"
enabled = true
execution_mode = "sdk"
tier = "fast"
permission_default = "prompt"

[agents.planner-b]
program = "opencode"
enabled = true
execution_mode = "tmux"
permission_default = "bypass"
`), 0o644))

	store := taskstore.NewTestStore(t)
	require.NoError(t, store.Create(project, taskstore.TaskEntry{
		Filename:    "feature.md",
		Status:      taskstore.StatusPlanning,
		Description: "ship feature",
	}))

	type spawnRecord struct {
		name string
		opts loop.SpawnOpts
	}
	spawned := make(chan spawnRecord, 2)
	killed := make(chan string, 1)
	cacheDir := filepath.Join(kasmosDir, "cache")
	require.NoError(t, os.MkdirAll(cacheDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(cacheDir, "feature.md-planner-old.md"), []byte("stale"), 0o644))
	d := &Daemon{
		repos:       NewRepoManager(),
		logger:      slog.Default(),
		broadcaster: api.NewEventBroadcaster(),
		killAgent: func(repoPath, planFile, agentType string) error {
			killed <- agentType
			return nil
		},
		spawnPlanner: func(_ context.Context, opts loop.SpawnOpts) error {
			spawned <- spawnRecord{name: "planner", opts: opts}
			return nil
		},
	}
	t.Cleanup(func() { d.broadcaster.Close() })
	d.repos.repos = []RepoEntry{{
		Path:             repoPath,
		Project:          project,
		Store:            store,
		PlannerProfiles:  []string{"planner-a", "planner-b"},
		PlannerDraftMode: true,
		CacheDir:         cacheDir,
	}}

	require.NoError(t, d.StartPlan(project, "feature.md", "caller-specific repair prompt", "legacy-program-ignored"))

	var killedAgents []string
	require.Eventually(t, func() bool {
		for {
			select {
			case agentType := <-killed:
				killedAgents = append(killedAgents, agentType)
			default:
				return len(killedAgents) == 1
			}
		}
	}, time.Second, 5*time.Millisecond)
	assert.Equal(t, []string{session.AgentTypePlanner}, killedAgents)

	var records []spawnRecord
	require.Eventually(t, func() bool {
		for {
			select {
			case record := <-spawned:
				records = append(records, record)
			default:
				return len(records) == 2
			}
		}
	}, time.Second, 5*time.Millisecond)
	require.Len(t, records, 2)
	assert.Equal(t, "planner", records[0].name)
	assert.Equal(t, "planner-a", records[0].opts.PlannerProfile)
	assert.True(t, records[0].opts.PlannerPrimary)
	assert.True(t, records[0].opts.PlannerDraftMode)
	assert.Equal(t, "codex", records[0].opts.Program)
	assert.Equal(t, "sdk", records[0].opts.ExecutionMode)
	assert.Equal(t, "fast", records[0].opts.SDKSpeedTier)
	assert.False(t, records[0].opts.SkipPermissions)
	assert.Contains(t, records[0].opts.Prompt, ".kasmos/cache/feature.md-planner-planner-a.md")
	assert.Contains(t, records[0].opts.Prompt, "caller-specific repair prompt")
	assert.Contains(t, records[0].opts.Prompt, "planner_draft_finished")
	assert.Equal(t, "planner-b", records[1].opts.PlannerProfile)
	assert.False(t, records[1].opts.PlannerPrimary)
	assert.True(t, records[1].opts.PlannerDraftMode)
	assert.Equal(t, "opencode", records[1].opts.Program)
	assert.Equal(t, "tmux", records[1].opts.ExecutionMode)
	assert.True(t, records[1].opts.SkipPermissions)
	assert.Contains(t, records[1].opts.Prompt, "caller-specific repair prompt")
	assert.NoFileExists(t, filepath.Join(cacheDir, "feature.md-planner-old.md"))
}

func TestDaemon_StartPlan_DraftModeResetsProcessorAggregation(t *testing.T) {
	project := "proj"
	repoPath := t.TempDir()
	kasmosDir := filepath.Join(repoPath, ".kasmos")
	require.NoError(t, os.MkdirAll(kasmosDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(kasmosDir, "config.toml"), []byte(`
[orchestration]
planners = ["planner-a", "planner-b"]

[agents.planner-a]
program = "opencode"
enabled = true

[agents.planner-b]
program = "opencode"
enabled = true
`), 0o644))

	const planFile = "feature.md"
	store := taskstore.NewTestStore(t)
	require.NoError(t, store.Create(project, taskstore.TaskEntry{
		Filename:    planFile,
		Status:      taskstore.StatusPlanning,
		Description: "ship feature",
	}))
	proc := loop.NewProcessor(loop.ProcessorConfig{
		Store:            store,
		Project:          project,
		PlannerProfiles:  []string{"planner-a", "planner-b"},
		PlannerDraftMode: true,
	})

	assert.Empty(t, proc.ProcessPlannerDraftSignals([]taskfsm.PlannerDraftSignal{{TaskFile: planFile, PlannerID: "planner-a"}}))
	require.NotEmpty(t, proc.ProcessPlannerDraftSignals([]taskfsm.PlannerDraftSignal{{TaskFile: planFile, PlannerID: "planner-b"}}))
	entry, err := store.Get(project, planFile)
	require.NoError(t, err)
	entry.Status = taskstore.StatusPlanning
	entry.ExecutionState = taskstore.ExecutionState{}
	require.NoError(t, store.Update(project, planFile, entry))

	spawned := make(chan loop.SpawnOpts, 2)
	cacheDir := filepath.Join(kasmosDir, "cache")
	require.NoError(t, os.MkdirAll(cacheDir, 0o755))
	d := &Daemon{
		repos:       NewRepoManager(),
		logger:      slog.Default(),
		broadcaster: api.NewEventBroadcaster(),
		killAgent: func(repoPath, planFile, agentType string) error {
			return nil
		},
		spawnPlanner: func(_ context.Context, opts loop.SpawnOpts) error {
			spawned <- opts
			return nil
		},
	}
	t.Cleanup(func() { d.broadcaster.Close() })
	d.repos.repos = []RepoEntry{{
		Path:             repoPath,
		Project:          project,
		Store:            store,
		Processor:        proc,
		PlannerProfiles:  []string{"planner-a", "planner-b"},
		PlannerDraftMode: true,
		CacheDir:         cacheDir,
	}}

	require.NoError(t, d.StartPlan(project, planFile, "legacy", "legacy"))
	require.Eventually(t, func() bool {
		return len(spawned) == 2
	}, time.Second, 5*time.Millisecond)

	assert.Empty(t, proc.ProcessPlannerDraftSignals([]taskfsm.PlannerDraftSignal{{TaskFile: planFile, PlannerID: "planner-a"}}))
	actions := proc.ProcessPlannerDraftSignals([]taskfsm.PlannerDraftSignal{{TaskFile: planFile, PlannerID: "planner-b"}})
	require.NotEmpty(t, actions, "fresh daemon fan-out must not be blocked by stale agg.done")
	assert.Equal(t, "planner_complete", actions[0].Kind())
}

// TestDaemon_StartPlan_PartialFanOutFailureCleansUpSiblings verifies that when
// the second planner spawn errors, the daemon also kills the previously
// successful spawn so the aggregator doesn't hang waiting on a draft that
// will never arrive.
func TestDaemon_StartPlan_PartialFanOutFailureCleansUpSiblings(t *testing.T) {
	project := "proj"
	repoPath := t.TempDir()
	kasmosDir := filepath.Join(repoPath, ".kasmos")
	require.NoError(t, os.MkdirAll(kasmosDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(kasmosDir, "config.toml"), []byte(`
[orchestration]
planners = ["planner-a", "planner-b"]

[agents.planner-a]
program = "codex"
enabled = true

[agents.planner-b]
program = "opencode"
enabled = true
`), 0o644))

	store := taskstore.NewTestStore(t)
	require.NoError(t, store.Create(project, taskstore.TaskEntry{
		Filename:    "feature.md",
		Status:      taskstore.StatusPlanning,
		Description: "ship feature",
	}))

	killed := make(chan string, 4)
	cacheDir := filepath.Join(kasmosDir, "cache")
	require.NoError(t, os.MkdirAll(cacheDir, 0o755))
	var spawnCount int32
	d := &Daemon{
		repos:       NewRepoManager(),
		logger:      slog.Default(),
		broadcaster: api.NewEventBroadcaster(),
		killAgent: func(repoPath, planFile, agentType string) error {
			killed <- agentType
			return nil
		},
		spawnPlanner: func(_ context.Context, opts loop.SpawnOpts) error {
			n := atomic.AddInt32(&spawnCount, 1)
			if n == 2 {
				return errors.New("simulated spawn failure on second profile")
			}
			return nil
		},
	}
	t.Cleanup(func() { d.broadcaster.Close() })
	d.repos.repos = []RepoEntry{{
		Path:             repoPath,
		Project:          project,
		Store:            store,
		PlannerProfiles:  []string{"planner-a", "planner-b"},
		PlannerDraftMode: true,
		CacheDir:         cacheDir,
	}}

	require.NoError(t, d.StartPlan(project, "feature.md", "legacy", "legacy"))

	// We expect at least two killAgent invocations: the pre-spawn cleanup of
	// any prior planner and the post-failure cleanup of the partial fan-out.
	var killedAgents []string
	require.Eventually(t, func() bool {
		for {
			select {
			case agentType := <-killed:
				killedAgents = append(killedAgents, agentType)
			default:
				return len(killedAgents) >= 2
			}
		}
	}, time.Second, 5*time.Millisecond)
	assert.GreaterOrEqual(t, len(killedAgents), 2,
		"daemon must kill the partial fan-out on spawn failure (initial kill + cleanup kill)")
}

func TestDaemon_ExecuteAction_DraftPlannerDoesNotAppendGeneratedLegacyPrompt(t *testing.T) {
	project := "proj"
	planFile := "feature.md"
	store := taskstore.NewTestStore(t)
	require.NoError(t, store.Create(project, taskstore.TaskEntry{
		Filename:    planFile,
		Status:      taskstore.StatusPlanning,
		Description: "ship feature",
	}))

	spawned := make(chan loop.SpawnOpts, 1)
	d := &Daemon{
		logger:      slog.Default(),
		broadcaster: api.NewEventBroadcaster(),
		killAgent: func(repoPath, planFile, agentType string) error {
			return nil
		},
		spawnPlanner: func(_ context.Context, opts loop.SpawnOpts) error {
			spawned <- opts
			return nil
		},
	}
	t.Cleanup(func() { d.broadcaster.Close() })

	cacheDir := filepath.Join(t.TempDir(), "cache")
	require.NoError(t, os.MkdirAll(cacheDir, 0o755))
	err := d.executeAction(context.Background(), RepoEntry{
		Path:     t.TempDir(),
		Project:  project,
		Store:    store,
		CacheDir: cacheDir,
	}, loop.SpawnPlannerAction{
		PlanFile:       planFile,
		PlannerProfile: "planner-a",
		Primary:        true,
		DraftMode:      true,
	})
	require.NoError(t, err)

	select {
	case opts := <-spawned:
		assert.Equal(t, "planner-a", opts.PlannerProfile)
		assert.True(t, opts.PlannerDraftMode)
		assert.Contains(t, opts.Prompt, filepath.Join(cacheDir, "feature.md-planner-planner-a.md"))
		assert.Contains(t, opts.Prompt, "planner_draft_finished")
		assert.NotContains(t, opts.Prompt, "## caller-provided prompt")
	case <-time.After(time.Second):
		t.Fatal("planner spawn did not run")
	}
}

func TestDaemon_ExecuteAction_DraftPlannerStoreUnavailableReturnsError(t *testing.T) {
	var spawnCalled bool
	d := &Daemon{
		logger:      slog.Default(),
		broadcaster: api.NewEventBroadcaster(),
		killAgent: func(repoPath, planFile, agentType string) error {
			return nil
		},
		spawnPlanner: func(_ context.Context, opts loop.SpawnOpts) error {
			spawnCalled = true
			return nil
		},
	}
	t.Cleanup(func() { d.broadcaster.Close() })

	var err error
	require.NotPanics(t, func() {
		err = d.executeAction(context.Background(), RepoEntry{
			Path:    t.TempDir(),
			Project: "proj",
		}, loop.SpawnPlannerAction{
			PlanFile:       "feature.md",
			PlannerProfile: "planner-a",
			Primary:        true,
			DraftMode:      true,
		})
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "task store unavailable for feature.md")
	assert.False(t, spawnCalled)
}

func TestDaemon_ExecuteClearPlannerDraftsAction(t *testing.T) {
	project := "proj"
	planFile := "feature.md"
	store := taskstore.NewTestStore(t)
	require.NoError(t, store.Create(project, taskstore.TaskEntry{
		Filename:    planFile,
		Status:      taskstore.StatusPlanning,
		Description: "ship feature",
	}))

	d := &Daemon{
		spawner:     NewTmuxSpawner(),
		logger:      slog.Default(),
		broadcaster: api.NewEventBroadcaster(),
	}
	t.Cleanup(func() { d.broadcaster.Close() })

	cacheDir := filepath.Join(t.TempDir(), "cache")
	require.NoError(t, os.MkdirAll(cacheDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(cacheDir, "feature.md-planner-old.md"), []byte("stale"), 0o644))
	repo := RepoEntry{
		Path:     t.TempDir(),
		Project:  project,
		Store:    store,
		CacheDir: cacheDir,
	}
	require.NoError(t, d.executeAction(context.Background(), repo, loop.ClearPlannerDraftsAction{PlanFile: planFile}))
	assert.NoFileExists(t, filepath.Join(cacheDir, "feature.md-planner-old.md"))
}

func TestDaemon_AutoAdvanceCompletedImplementer_TransitionsToReviewing(t *testing.T) {
	project := "proj"
	store := taskstore.NewTestStore(t)
	require.NoError(t, store.Create(project, taskstore.TaskEntry{
		Filename: "feature.md",
		Status:   taskstore.StatusImplementing,
		Branch:   "plan/feature",
		ExecutionState: taskstore.ExecutionState{
			Phase:           string(taskfsm.ExecutionPhaseFixing),
			ActiveAgentType: session.AgentTypeFixer,
		},
	}))

	pushCalled := false
	d := &Daemon{
		logger:      slog.Default(),
		broadcaster: api.NewEventBroadcaster(),
		pushBranch: func(*session.Instance) error {
			pushCalled = true
			return nil
		},
	}
	e := RepoEntry{Project: project, Store: store}
	inst := &session.Instance{
		Title:                 "feature-fixer",
		TaskFile:              "feature.md",
		AgentType:             session.AgentTypeFixer,
		PromptDetected:        true,
		CompletionPromptSince: time.Now().Add(-(session.CompletionPromptStabilityWindow + 10*time.Millisecond)),
	}

	advanced, err := d.autoAdvanceCompletedImplementer(e, inst, true)
	require.NoError(t, err)
	assert.True(t, advanced)
	assert.True(t, pushCalled)

	entry, err := store.Get(project, "feature.md")
	require.NoError(t, err)
	assert.Equal(t, taskstore.StatusReviewing, entry.Status)
	assert.Equal(t, taskstore.ExecutionState{Phase: string(taskfsm.ExecutionPhaseReviewing), ActiveAgentType: session.AgentTypeReviewer}, entry.ExecutionState)
}

func TestDaemon_MonitorRunningInstances_EmitsStuckDetectedOncePerExit(t *testing.T) {
	repo := t.TempDir()
	project := filepath.Base(repo)
	store := taskstore.NewTestStore(t)
	planFile := "feature.md"
	require.NoError(t, store.Create(project, taskstore.TaskEntry{
		Filename: planFile,
		Status:   taskstore.StatusImplementing,
		ExecutionState: taskstore.ExecutionState{
			Phase:           string(taskfsm.ExecutionPhaseArchitecting),
			ActiveAgentType: session.AgentTypeElaborator,
		},
	}))

	inst, err := session.NewInstance(session.InstanceOptions{
		Title:         "feature-architect",
		Path:          repo,
		Program:       "claude",
		ExecutionMode: session.ExecutionModeSDK,
		TaskFile:      planFile,
		AgentType:     session.AgentTypeElaborator,
	})
	require.NoError(t, err)
	// Simulate an already-exited agent without spawning real tmux/sdk
	// subprocesses — CI hosts do not have tmux installed.
	inst.MarkStartedDeadForTest()
	require.False(t, inst.TmuxAlive())

	d := &Daemon{
		spawner:     NewTmuxSpawner(),
		logger:      slog.Default(),
		broadcaster: api.NewEventBroadcaster(),
	}
	e := RepoEntry{Path: repo, Project: project, Store: store}

	key := instanceKey(repo, planFile, session.AgentTypeElaborator)
	d.spawner.mu.Lock()
	d.spawner.instances[key] = inst
	d.spawner.planFileByKey[key] = planFile
	d.spawner.agentTypeByKey[key] = session.AgentTypeElaborator
	d.spawner.projectByKey[key] = project
	d.spawner.mu.Unlock()

	sub := d.broadcaster.Subscribe()
	t.Cleanup(func() { d.broadcaster.Unsubscribe(sub) })

	d.monitorRunningInstances(context.Background(), e)

	select {
	case ev := <-sub:
		assert.Equal(t, api.EventKindStuckDetected, ev.Kind)
		assert.Equal(t, "agent exited without an auto-advance path for "+planFile, ev.Message)
		assert.Equal(t, repo, ev.Repo)
		assert.Equal(t, planFile, ev.PlanFile)
		assert.Equal(t, session.AgentTypeElaborator, ev.AgentType)
	case <-time.After(time.Second):
		t.Fatal("expected stuck_detected event")
	}

	assert.True(t, inst.Exited)
	assert.Equal(t, session.Ready, inst.Status)

	d.monitorRunningInstances(context.Background(), e)

	select {
	case ev := <-sub:
		t.Fatalf("unexpected extra event: %+v", ev)
	default:
	}
}

func TestDaemon_MonitorRunningInstances_CompletesExitedWaveTask(t *testing.T) {
	repo := t.TempDir()
	project := filepath.Base(repo)
	planFile := "feature.md"
	store := taskstore.NewTestStore(t)
	require.NoError(t, store.Create(project, taskstore.TaskEntry{
		Filename: planFile,
		Status:   taskstore.StatusImplementing,
		Branch:   "plan/feature",
		ExecutionState: taskstore.ExecutionState{
			Phase:           string(taskfsm.ExecutionPhaseArchitecting),
			ActiveAgentType: session.AgentTypeCoder,
		},
	}))
	require.NoError(t, store.SetContent(project, planFile, `# Feature Plan

**Goal:** test wave completion

**Architecture:** test

**Tech Stack:** go

---

## Wave 1

### Task 1: First Thing

Do the first thing.
`))

	proc := loop.NewProcessor(loop.ProcessorConfig{Store: store, Project: project, AutoAdvance: true})
	actions := proc.ProcessWaveSignals([]taskfsm.WaveSignal{{TaskFile: planFile, WaveNumber: 1}})
	require.Len(t, actions, 1)
	require.NoError(t, setRepoExecutionState(RepoEntry{Project: project, Store: store}, planFile, taskstore.ExecutionState{
		Phase:           string(taskfsm.ExecutionPhaseWaveRunning),
		ActiveAgentType: session.AgentTypeCoder,
		ActiveWave:      1,
	}))

	inst, err := session.NewInstance(session.InstanceOptions{
		Title:         "feature-W1-T1",
		Path:          repo,
		Program:       "claude",
		ExecutionMode: session.ExecutionModeSDK,
		TaskFile:      planFile,
		AgentType:     session.AgentTypeCoder,
		WaveNumber:    1,
		TaskNumber:    1,
	})
	require.NoError(t, err)
	inst.MarkStartedDeadForTest()
	require.False(t, inst.TmuxAlive())
	inst.HasWorked = true

	reviewerSpawned := 0
	killedWaveAgents := 0
	d := &Daemon{
		cfg:         &DaemonConfig{AutoAdvanceWaves: true},
		spawner:     NewTmuxSpawner(),
		logger:      slog.Default(),
		broadcaster: api.NewEventBroadcaster(),
		spawnReviewer: func(context.Context, loop.SpawnOpts) error {
			reviewerSpawned++
			return nil
		},
		killWaveAgents: func(repoPath, pf string, wave int) error {
			killedWaveAgents++
			assert.Equal(t, repo, repoPath)
			assert.Equal(t, planFile, pf)
			assert.Equal(t, 1, wave)
			return nil
		},
	}
	e := RepoEntry{Path: repo, Project: project, Store: store, Processor: proc}

	key := instanceKeyForTask(repo, planFile, session.AgentTypeCoder, 1, 1)
	d.spawner.mu.Lock()
	d.spawner.instances[key] = inst
	d.spawner.planFileByKey[key] = planFile
	d.spawner.agentTypeByKey[key] = session.AgentTypeCoder
	d.spawner.projectByKey[key] = project
	d.spawner.mu.Unlock()

	sub := d.broadcaster.Subscribe()
	t.Cleanup(func() { d.broadcaster.Unsubscribe(sub) })

	d.monitorRunningInstances(context.Background(), e)

	entry, err := store.Get(project, planFile)
	require.NoError(t, err)
	assert.Equal(t, taskstore.StatusReviewing, entry.Status)
	assert.Equal(t, taskstore.ExecutionState{Phase: string(taskfsm.ExecutionPhaseReviewing), ActiveAgentType: session.AgentTypeReviewer}, entry.ExecutionState)
	assert.True(t, inst.ImplementationComplete)
	assert.True(t, inst.Exited)
	assert.Equal(t, 1, reviewerSpawned)
	assert.Equal(t, 1, killedWaveAgents)
	assert.Nil(t, proc.WaveOrchestrator(planFile))

	for {
		select {
		case ev := <-sub:
			assert.NotEqual(t, api.EventKindStuckDetected, ev.Kind)
		default:
			return
		}
	}
}

func TestDaemon_MonitorRunningInstances_SetsCompletionPromptSinceWhenPromptDetected(t *testing.T) {
	repo := t.TempDir()
	project := filepath.Base(repo)
	store := taskstore.NewTestStore(t)
	planFile := "feature.md"
	require.NoError(t, store.Create(project, taskstore.TaskEntry{
		Filename: planFile,
		Status:   taskstore.StatusImplementing,
		ExecutionState: taskstore.ExecutionState{
			Phase:           string(taskfsm.ExecutionPhaseArchitecting),
			ActiveAgentType: session.AgentTypeElaborator,
		},
	}))

	inst, err := session.NewInstance(session.InstanceOptions{
		Title:         "feature-architect",
		Path:          repo,
		Program:       "claude",
		ExecutionMode: session.ExecutionModeSDK,
		TaskFile:      planFile,
		AgentType:     session.AgentTypeElaborator,
	})
	require.NoError(t, err)
	inst.MarkStartedDeadForTest()
	require.False(t, inst.TmuxAlive())

	// Pre-set prompt state so UpdateCompletionPromptState has something to record.
	inst.PromptDetected = true

	d := &Daemon{
		spawner:     NewTmuxSpawner(),
		logger:      slog.Default(),
		broadcaster: api.NewEventBroadcaster(),
	}
	e := RepoEntry{Path: repo, Project: project, Store: store}

	key := instanceKey(repo, planFile, session.AgentTypeElaborator)
	d.spawner.mu.Lock()
	d.spawner.instances[key] = inst
	d.spawner.planFileByKey[key] = planFile
	d.spawner.agentTypeByKey[key] = session.AgentTypeElaborator
	d.spawner.projectByKey[key] = project
	d.spawner.mu.Unlock()

	d.monitorRunningInstances(context.Background(), e)

	assert.False(t, inst.CompletionPromptSince.IsZero(),
		"UpdateCompletionPromptState should populate CompletionPromptSince when prompt is detected and not blocked")
}

func TestDaemon_AutoAdvancePlannerFinished_StartsBlueprintSkipImplementation(t *testing.T) {
	project := "proj"
	planFile := "feature.md"
	store := taskstore.NewTestStore(t)
	require.NoError(t, store.Create(project, taskstore.TaskEntry{
		Filename: planFile,
		Status:   taskstore.StatusPlanning,
		Branch:   "plan/feature",
	}))
	require.NoError(t, store.SetContent(project, planFile, "# Plan\n\n**Goal:** test auto advance\n\n**Architecture:** test\n\n**Tech Stack:** go\n\n**Size:** Small\n\n---\n\n## Wave 1\n\n### Task 1: First\n\nImplement the first task."))

	proc := loop.NewProcessor(loop.ProcessorConfig{Store: store, Project: project, AutoAdvance: true})

	var spawned loop.SpawnOpts
	d := &Daemon{
		cfg:         &DaemonConfig{AutoAdvance: true},
		spawner:     NewTmuxSpawner(),
		logger:      slog.Default(),
		broadcaster: api.NewEventBroadcaster(),
		killAgent: func(repoPath, taskFile, agentType string) error {
			return nil
		},
		spawnCoder: func(_ context.Context, opts loop.SpawnOpts) error {
			spawned = opts
			return nil
		},
	}
	e := RepoEntry{
		Path:      t.TempDir(),
		Project:   project,
		Store:     store,
		Processor: proc,
	}

	actions := proc.ProcessFSMSignals([]taskfsm.Signal{{Event: taskfsm.PlannerFinished, TaskFile: planFile}})
	require.Len(t, actions, 2)

	for _, action := range actions {
		require.NoError(t, d.executeAction(context.Background(), e, action))
	}

	entry, err := store.Get(project, planFile)
	require.NoError(t, err)
	assert.Equal(t, taskstore.StatusImplementing, entry.Status)
	assert.Equal(t, taskstore.ExecutionState{
		Phase:           string(taskfsm.ExecutionPhaseSingleAgentImplementing),
		ActiveAgentType: session.AgentTypeCoder,
	}, entry.ExecutionState)
	assert.Equal(t, planFile, spawned.PlanFile)
	assert.Equal(t, e.Path, spawned.RepoPath)
	assert.Equal(t, project, spawned.Project)
	assert.Equal(t, "plan/feature", spawned.Branch)
	assert.Contains(t, spawned.Prompt, "Implement all 1 task(s) for plan: feature.md")
}

func TestDaemon_AutoAdvancePlannerFinished_StartsArchitectImplementation(t *testing.T) {
	project := "proj"
	planFile := "feature.md"
	store := taskstore.NewTestStore(t)
	require.NoError(t, store.Create(project, taskstore.TaskEntry{
		Filename: planFile,
		Status:   taskstore.StatusPlanning,
	}))
	require.NoError(t, store.SetContent(project, planFile, "# Plan\n\n**Goal:** test auto advance\n\n**Architecture:** test\n\n**Tech Stack:** go\n\n**Size:** Large\n\n---\n\n## Wave 1\n\n### Task 1: First\n\nImplement the first task.\n\n### Task 2: Second\n\nImplement the second task.\n\n### Task 3: Third\n\nImplement the third task."))

	proc := loop.NewProcessor(loop.ProcessorConfig{Store: store, Project: project, AutoAdvance: true})

	var spawned loop.SpawnOpts
	d := &Daemon{
		cfg:         &DaemonConfig{AutoAdvance: true},
		spawner:     NewTmuxSpawner(),
		logger:      slog.Default(),
		broadcaster: api.NewEventBroadcaster(),
		killAgent: func(repoPath, taskFile, agentType string) error {
			return nil
		},
		spawnElaborator: func(_ context.Context, opts loop.SpawnOpts) error {
			spawned = opts
			return nil
		},
	}
	e := RepoEntry{
		Path:      t.TempDir(),
		Project:   project,
		Store:     store,
		Processor: proc,
	}

	actions := proc.ProcessFSMSignals([]taskfsm.Signal{{Event: taskfsm.PlannerFinished, TaskFile: planFile}})
	require.Len(t, actions, 2)

	for _, action := range actions {
		require.NoError(t, d.executeAction(context.Background(), e, action))
	}

	entry, err := store.Get(project, planFile)
	require.NoError(t, err)
	assert.Equal(t, taskstore.StatusImplementing, entry.Status)
	assert.Equal(t, taskstore.ExecutionState{
		Phase:           string(taskfsm.ExecutionPhaseArchitecting),
		ActiveAgentType: session.AgentTypeElaborator,
	}, entry.ExecutionState)
	assert.Equal(t, planFile, spawned.PlanFile)
	assert.Equal(t, e.Path, spawned.RepoPath)
	assert.Equal(t, project, spawned.Project)
	assert.Contains(t, spawned.Prompt, "You are the architect agent")
}

func TestDaemon_AutoImplementPlan_KeepsPlannerWithAttachedClients(t *testing.T) {
	project := "proj"
	planFile := "feature.md"
	store := taskstore.NewTestStore(t)
	// StatusReady is the state after PlannerFinished - autoImplementPlan transitions
	// it to StatusImplementing via ImplementStart.
	require.NoError(t, store.Create(project, taskstore.TaskEntry{
		Filename: planFile,
		Status:   taskstore.StatusReady,
	}))
	// Large plan so the architect (elaborator) path is taken.
	require.NoError(t, store.SetContent(project, planFile, "# Plan\n\n**Goal:** test planner handoff\n\n**Architecture:** test\n\n**Tech Stack:** go\n\n**Size:** Large\n\n---\n\n## Wave 1\n\n### Task 1: First\n\nFirst.\n\n### Task 2: Second\n\nSecond.\n\n### Task 3: Third\n\nThird."))

	// Set up a spawner where a client is attached and a planner is tracked.
	spawner := NewTmuxSpawner()
	spawner.hasAttachedClients = func(_ cmd.Executor, _ string) bool { return true }
	spawner.sleep = func(_ time.Duration) {}
	spawner.kill = func(_ *session.Instance) error {
		require.FailNow(t, "planner must not be killed during architect handoff")
		return nil
	}

	const repoPath = "/tmp/repo"
	plannerKey := instanceKey(repoPath, planFile, session.AgentTypePlanner)
	plannerInst := &session.Instance{Title: "feature-plan"}
	spawner.mu.Lock()
	spawner.instances[plannerKey] = plannerInst
	spawner.planFileByKey[plannerKey] = planFile
	spawner.agentTypeByKey[plannerKey] = session.AgentTypePlanner
	spawner.projectByKey[plannerKey] = project
	spawner.mu.Unlock()

	var spawned loop.SpawnOpts
	d := &Daemon{
		cfg:         &DaemonConfig{AutoAdvance: true},
		spawner:     spawner,
		logger:      slog.Default(),
		broadcaster: api.NewEventBroadcaster(),
		spawnElaborator: func(_ context.Context, opts loop.SpawnOpts) error {
			spawned = opts
			return nil
		},
	}
	e := RepoEntry{
		Path:    repoPath,
		Project: project,
		Store:   store,
	}

	err := d.autoImplementPlan(context.Background(), e, planFile)
	require.NoError(t, err)
	spawner.mu.Lock()
	trackedPlanner := spawner.instances[plannerKey]
	spawner.mu.Unlock()
	assert.Same(t, plannerInst, trackedPlanner, "planner must remain tracked during architect handoff")
	assert.Equal(t, planFile, spawned.PlanFile, "architect must be spawned while planner remains available")
}

func TestDaemon_TickScansSharedWorktreeSignals(t *testing.T) {
	repo := t.TempDir()
	project := filepath.Base(repo)
	store := taskstore.NewTestStore(t)
	require.NoError(t, store.Create(project, taskstore.TaskEntry{
		Filename: "plan.md",
		Status:   taskstore.StatusImplementing,
		Branch:   "plan/plan",
	}))

	d := &Daemon{
		repos:       NewRepoManager(),
		spawner:     NewTmuxSpawner(),
		logger:      slog.Default(),
		broadcaster: api.NewEventBroadcaster(),
	}
	d.repos.repos = []RepoEntry{{
		Path:      repo,
		Project:   project,
		Store:     store,
		Processor: loop.NewProcessor(loop.ProcessorConfig{Store: store, Project: project}),
	}}

	wtSignals := filepath.Join(repo, ".worktrees", "plan-plan", ".kasmos", "signals")
	require.NoError(t, taskfsm.EnsureSignalDirs(wtSignals))
	require.NoError(t, os.WriteFile(filepath.Join(wtSignals, "implement-finished-plan.md"), nil, 0o644))

	d.tick(context.Background())

	entry, err := store.Get(project, "plan.md")
	require.NoError(t, err)
	assert.Equal(t, taskstore.StatusReviewing, entry.Status)

	// The signal should not be stuck in processing after tick completes.
	_, err = os.Stat(filepath.Join(wtSignals, "processing", "implement-finished-plan.md"))
	assert.True(t, os.IsNotExist(err), "processing file should not exist after tick")
}

func TestDaemon_Tick_AtomicProcessing(t *testing.T) {
	repo := t.TempDir()
	project := filepath.Base(repo)
	store := taskstore.NewTestStore(t)
	require.NoError(t, store.Create(project, taskstore.TaskEntry{
		Filename: "atomic-plan.md",
		Status:   taskstore.StatusPlanning,
	}))

	d := &Daemon{
		repos:       NewRepoManager(),
		spawner:     NewTmuxSpawner(),
		logger:      slog.Default(),
		broadcaster: api.NewEventBroadcaster(),
	}
	d.repos.repos = []RepoEntry{{
		Path:      repo,
		Project:   project,
		Store:     store,
		Processor: loop.NewProcessor(loop.ProcessorConfig{Store: store, Project: project}),
	}}

	signalsDir := filepath.Join(repo, ".kasmos", "signals")
	require.NoError(t, taskfsm.EnsureSignalDirs(signalsDir))
	signalFile := "planner-finished-atomic-plan.md"
	require.NoError(t, os.WriteFile(filepath.Join(signalsDir, signalFile), nil, 0o644))

	d.tick(context.Background())

	// Base signal file must be gone.
	_, err := os.Stat(filepath.Join(signalsDir, signalFile))
	assert.True(t, os.IsNotExist(err), "base signal file should be gone after tick")

	// Processing file must be gone (consumed on success).
	_, err = os.Stat(filepath.Join(signalsDir, "processing", signalFile))
	assert.True(t, os.IsNotExist(err), "processing file should be gone after successful processing")

	// Failed file must be absent (signal was handled successfully).
	_, err = os.Stat(filepath.Join(signalsDir, "failed", signalFile))
	assert.True(t, os.IsNotExist(err), "failed file should not exist for a successfully processed signal")

	// Store status must have transitioned to ready.
	entry, err := store.Get(project, "atomic-plan.md")
	require.NoError(t, err)
	assert.Equal(t, taskstore.StatusReady, entry.Status)
}

func TestDaemon_RecoverInFlight_OnStartup(t *testing.T) {
	signalsDir := t.TempDir()
	require.NoError(t, taskfsm.EnsureSignalDirs(signalsDir))

	// Simulate a crash: place a file in processing/
	staleFile := "planner-finished-stale.md"
	processingPath := filepath.Join(signalsDir, "processing", staleFile)
	require.NoError(t, os.WriteFile(processingPath, nil, 0o644))

	n := taskfsm.RecoverInFlight(signalsDir)
	assert.Equal(t, 1, n, "should recover exactly one in-flight signal")

	// File must have moved back to the base signals dir.
	_, err := os.Stat(filepath.Join(signalsDir, staleFile))
	assert.NoError(t, err, "recovered signal should be in the base signals dir")

	// Processing file must be gone.
	_, err = os.Stat(processingPath)
	assert.True(t, os.IsNotExist(err), "processing file should be gone after recovery")
}

func TestCoderSpawnOpts_ForwardsFeedbackAsPrompt(t *testing.T) {
	repo := RepoEntry{Path: "/tmp/repo", Project: "repo"}
	opts := coderSpawnOpts(repo, "plan.md", "plan/plan", "apply requested fixes")

	assert.Equal(t, "apply requested fixes", opts.Prompt)
	assert.Equal(t, "apply requested fixes", opts.Feedback)
	assert.Equal(t, "/tmp/repo", opts.RepoPath)
	assert.Equal(t, "plan/plan", opts.Branch)
}

func TestFixerSpawnOpts_UsesFixerPrompt(t *testing.T) {
	store := taskstore.NewTestStore(t)
	require.NoError(t, store.Create("repo", taskstore.TaskEntry{Filename: "plan.md", ReviewCycle: 4}))
	repo := RepoEntry{Path: "/tmp/repo", Project: "repo", Store: store}
	opts := fixerSpawnOpts(repo, "plan.md", "plan/plan", "- [app.go:42] address reviewer feedback")

	assert.Contains(t, opts.Prompt, "Address reviewer feedback for plan: plan.md")
	assert.Contains(t, opts.Prompt, "Current fix round: 4")
	assert.Contains(t, opts.Prompt, "not an implementer")
	assert.Contains(t, opts.Prompt, "address reviewer feedback")
	assert.NotContains(t, opts.Prompt, "execute all tasks sequentially")
	assert.Equal(t, "- [app.go:42] address reviewer feedback", opts.Feedback)
	assert.Equal(t, "/tmp/repo", opts.RepoPath)
	assert.Equal(t, "plan/plan", opts.Branch)
	assert.Equal(t, 4, opts.ReviewCycle)
}

func TestReviewerSpawnOpts_UsesReviewRoundPrompt(t *testing.T) {
	repo := RepoEntry{Path: "/tmp/repo", Project: "repo"}
	entry := taskstore.TaskEntry{Filename: "plan.md", Branch: "plan/plan", ReviewCycle: 5, LatestReviewFeedback: "round 5 findings"}
	opts := reviewerSpawnOpts(repo, entry)

	assert.Equal(t, "/tmp/repo", opts.RepoPath)
	assert.Equal(t, "plan/plan", opts.Branch)
	assert.Equal(t, 6, opts.ReviewCycle)
	assert.Contains(t, opts.Prompt, "Current review round: 6")
	assert.Contains(t, opts.Prompt, "round 5 findings")
}

func TestDaemon_ExecuteAction_ReviewChanges_PersistsLatestFeedback(t *testing.T) {
	store := taskstore.NewTestStore(t)
	require.NoError(t, store.Create("proj", taskstore.TaskEntry{Filename: "plan.md", Status: taskstore.StatusReviewing}))
	d := &Daemon{logger: slog.Default()}
	e := RepoEntry{Project: "proj", Store: store}
	require.NoError(t, d.executeAction(context.Background(), e, loop.ReviewChangesAction{PlanFile: "plan.md", Feedback: "new review findings"}))
	entry, err := store.Get("proj", "plan.md")
	require.NoError(t, err)
	assert.Equal(t, "new review findings", entry.LatestReviewFeedback)
}

func TestDaemon_ExecuteAction_VerifyApproved_ClearsExecutionStateAndEmitsEvent(t *testing.T) {
	store := taskstore.NewTestStore(t)
	require.NoError(t, store.Create("proj", taskstore.TaskEntry{
		Filename: "plan.md",
		Status:   taskstore.StatusVerifying,
		ExecutionState: taskstore.ExecutionState{
			Phase:           string(taskfsm.ExecutionPhaseReviewing),
			ActiveAgentType: session.AgentTypeReviewer,
		},
	}))
	broadcaster := api.NewEventBroadcaster()
	sub := broadcaster.Subscribe()
	t.Cleanup(func() {
		broadcaster.Unsubscribe(sub)
		broadcaster.Close()
	})
	d := &Daemon{logger: slog.Default(), broadcaster: broadcaster}
	e := RepoEntry{Project: "proj", Store: store}

	require.NoError(t, d.executeAction(context.Background(), e, loop.VerifyApprovedAction{PlanFile: "plan.md", ReviewBody: "ship it"}))

	entry, err := store.Get("proj", "plan.md")
	require.NoError(t, err)
	assert.Equal(t, taskstore.ExecutionState{}, entry.ExecutionState, "VerifyApprovedAction must clear execution state")

	select {
	case ev := <-sub:
		assert.Equal(t, "signal_processed", ev.Kind)
		assert.Equal(t, "plan.md", ev.PlanFile)
	case <-time.After(time.Second):
		t.Fatal("expected signal_processed event")
	}
}

func TestDaemon_ExecuteAction_VerifyFailed_PersistsFeedbackAndEmitsEvent(t *testing.T) {
	store := taskstore.NewTestStore(t)
	require.NoError(t, store.Create("proj", taskstore.TaskEntry{
		Filename: "plan.md",
		Status:   taskstore.StatusVerifying,
	}))
	broadcaster := api.NewEventBroadcaster()
	sub := broadcaster.Subscribe()
	t.Cleanup(func() {
		broadcaster.Unsubscribe(sub)
		broadcaster.Close()
	})
	d := &Daemon{logger: slog.Default(), broadcaster: broadcaster}
	e := RepoEntry{Project: "proj", Store: store}

	require.NoError(t, d.executeAction(context.Background(), e, loop.VerifyFailedAction{PlanFile: "plan.md", Feedback: "not ready yet"}))

	entry, err := store.Get("proj", "plan.md")
	require.NoError(t, err)
	assert.Equal(t, "not ready yet", entry.LatestReviewFeedback, "VerifyFailedAction must persist feedback")

	select {
	case ev := <-sub:
		assert.Equal(t, "signal_processed", ev.Kind)
		assert.Equal(t, "plan.md", ev.PlanFile)
	case <-time.After(time.Second):
		t.Fatal("expected signal_processed event")
	}
}

func TestSharedWorktreePaths(t *testing.T) {
	repo := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(repo, ".worktrees", "a"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(repo, ".worktrees", "b"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(repo, ".worktrees", "README"), nil, 0o644))

	paths := sharedWorktreePaths(repo)
	assert.ElementsMatch(t, []string{
		filepath.Join(repo, ".worktrees", "a"),
		filepath.Join(repo, ".worktrees", "b"),
	}, paths)
}

func TestTmuxSpawner_DiscoverOrphans(t *testing.T) {
	s := NewTmuxSpawner(TmuxSpawnerConfig{})
	orphans := s.DiscoverOrphanSessions()
	assert.NotNil(t, orphans)
}

func TestDaemon_TickRepoUsesGateway(t *testing.T) {
	dir := t.TempDir()
	signalsDir := filepath.Join(dir, ".kasmos", "signals")
	require.NoError(t, os.MkdirAll(signalsDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(signalsDir, "planner-finished-gw-plan"), nil, 0o644))

	store := taskstore.NewTestStore(t)
	require.NoError(t, store.Create("test-project", taskstore.TaskEntry{Filename: "gw-plan", Status: taskstore.StatusPlanning}))

	gw, err := taskstore.NewSQLiteSignalGateway(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { _ = gw.Close() })

	entry := RepoEntry{
		Path:          dir,
		Project:       "test-project",
		Store:         store,
		SignalsDir:    signalsDir,
		Processor:     loop.NewProcessor(loop.ProcessorConfig{Store: store, Project: "test-project"}),
		SignalGateway: gw,
	}
	d := &Daemon{
		cfg:         &DaemonConfig{PollInterval: time.Second},
		repos:       NewRepoManager(),
		spawner:     newTmuxSpawner(slog.Default()),
		logger:      slog.Default(),
		broadcaster: api.NewEventBroadcaster(),
	}

	d.tickRepo(context.Background(), entry)

	files, err := os.ReadDir(signalsDir)
	require.NoError(t, err)
	assert.Empty(t, files)

	done, err := gw.List("test-project", taskstore.SignalDone)
	require.NoError(t, err)
	assert.Len(t, done, 1)

	updated, err := store.Get("test-project", "gw-plan")
	require.NoError(t, err)
	assert.Equal(t, taskstore.StatusReady, updated.Status)
}

func TestDaemon_TickRepoGateway_TaskSignalRestoresOrchestrator(t *testing.T) {
	dir := t.TempDir()
	store := taskstore.NewTestStore(t)
	project := "test-project"
	planFile := "gw-wave-plan"
	require.NoError(t, store.Create(project, taskstore.TaskEntry{Filename: planFile, Status: taskstore.StatusImplementing}))
	require.NoError(t, store.SetContent(project, planFile, "# Plan\n\n**Goal:** test\n\n**Architecture:** test\n\n**Tech Stack:** go\n\n**Size:** Small\n\n---\n\n## Wave 1\n\n### Task 1: First\n\nDo first.\n\n### Task 2: Second\n\nDo second."))
	require.NoError(t, store.SetSubtasks(project, planFile, []taskstore.SubtaskEntry{
		{TaskNumber: 1, Title: "First", Status: taskstore.SubtaskStatusRunning},
		{TaskNumber: 2, Title: "Second", Status: taskstore.SubtaskStatusRunning},
	}))

	gw, err := taskstore.NewSQLiteSignalGateway(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { _ = gw.Close() })
	require.NoError(t, gw.Create(project, taskstore.SignalEntry{
		PlanFile:   planFile,
		SignalType: "implement_task_finished",
		Payload:    `{"wave_number":1,"task_number":1}`,
	}))

	entry := RepoEntry{
		Path:          dir,
		Project:       project,
		Store:         store,
		Processor:     loop.NewProcessor(loop.ProcessorConfig{Store: store, Project: project}),
		SignalGateway: gw,
	}
	d := &Daemon{
		cfg:         &DaemonConfig{PollInterval: time.Second},
		repos:       NewRepoManager(),
		spawner:     newTmuxSpawner(slog.Default()),
		logger:      slog.Default(),
		broadcaster: api.NewEventBroadcaster(),
	}

	d.tickRepo(context.Background(), entry)

	done, err := gw.List(project, taskstore.SignalDone)
	require.NoError(t, err)
	assert.Len(t, done, 1)
	assert.Equal(t, "", done[0].Result)

	subtasks, err := store.GetSubtasks(project, planFile)
	require.NoError(t, err)
	require.Len(t, subtasks, 2)
	assert.Equal(t, taskstore.SubtaskStatusComplete, subtasks[0].Status)
	assert.Equal(t, taskstore.SubtaskStatusRunning, subtasks[1].Status)

	orch := entry.Processor.WaveOrchestrator(planFile)
	require.NotNil(t, orch)
	assert.Equal(t, 1, orch.CurrentWaveNumber())
	assert.True(t, orch.IsTaskComplete(1))
	assert.True(t, orch.IsTaskRunning(2))
}

func TestDaemon_TickRepoGateway_InvalidTaskSignalMarkedFailed(t *testing.T) {
	dir := t.TempDir()
	store := taskstore.NewTestStore(t)
	project := "test-project"
	planFile := "gw-bad-task"
	require.NoError(t, store.Create(project, taskstore.TaskEntry{Filename: planFile, Status: taskstore.StatusImplementing}))

	gw, err := taskstore.NewSQLiteSignalGateway(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { _ = gw.Close() })
	require.NoError(t, gw.Create(project, taskstore.SignalEntry{
		PlanFile:   planFile,
		SignalType: "implement_task_finished",
		Payload:    `{"wave_number":1,"task_number":1}`,
	}))

	entry := RepoEntry{
		Path:          dir,
		Project:       project,
		Store:         store,
		Processor:     loop.NewProcessor(loop.ProcessorConfig{Store: store, Project: project}),
		SignalGateway: gw,
	}
	d := &Daemon{
		cfg:         &DaemonConfig{PollInterval: time.Second},
		repos:       NewRepoManager(),
		spawner:     newTmuxSpawner(slog.Default()),
		logger:      slog.Default(),
		broadcaster: api.NewEventBroadcaster(),
	}

	d.tickRepo(context.Background(), entry)

	failed, err := gw.List(project, taskstore.SignalFailed)
	require.NoError(t, err)
	require.Len(t, failed, 1)
	assert.Equal(t, "no active orchestrator / wrong wave / already-finished task", failed[0].Result)

	done, err := gw.List(project, taskstore.SignalDone)
	require.NoError(t, err)
	assert.Empty(t, done)
}

func TestDaemon_TickRepoGateway_InvalidPlannerDraftSignalMarkedFailed(t *testing.T) {
	dir := t.TempDir()
	store := taskstore.NewTestStore(t)
	project := "test-project"
	planFile := "gw-bad-draft"
	require.NoError(t, store.Create(project, taskstore.TaskEntry{Filename: planFile, Status: taskstore.StatusPlanning}))

	gw, err := taskstore.NewSQLiteSignalGateway(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { _ = gw.Close() })
	require.NoError(t, gw.Create(project, taskstore.SignalEntry{
		PlanFile:   planFile,
		SignalType: "planner_draft_finished",
		Payload:    `{"planner_id":"typo-planner"}`,
	}))

	entry := RepoEntry{
		Path:    dir,
		Project: project,
		Store:   store,
		Processor: loop.NewProcessor(loop.ProcessorConfig{
			Store:            store,
			Project:          project,
			PlannerDraftMode: true,
			PlannerProfiles:  []string{"planner-a", "planner-b"},
		}),
		SignalGateway: gw,
	}
	d := &Daemon{
		cfg:         &DaemonConfig{PollInterval: time.Second},
		repos:       NewRepoManager(),
		spawner:     newTmuxSpawner(slog.Default()),
		logger:      slog.Default(),
		broadcaster: api.NewEventBroadcaster(),
	}

	d.tickRepo(context.Background(), entry)

	failed, err := gw.List(project, taskstore.SignalFailed)
	require.NoError(t, err)
	require.Len(t, failed, 1)
	assert.Equal(t, "planner_draft_finished", failed[0].SignalType)
	assert.Contains(t, failed[0].Result, "unknown planner draft profile")
	assert.Contains(t, failed[0].Result, "typo-planner")

	done, err := gw.List(project, taskstore.SignalDone)
	require.NoError(t, err)
	assert.Empty(t, done)
}

func TestDaemon_ExecuteAction_ReviewCycleLimit(t *testing.T) {
	action := loop.ReviewCycleLimitAction{
		PlanFile: "test.md",
		Cycle:    3,
		Limit:    3,
	}
	assert.Equal(t, "review_cycle_limit", action.Kind())
}

func TestDaemon_ExecuteAction_SpawnFixer_EmitsEvent(t *testing.T) {
	// SpawnFixerAction.Kind() must return the expected string.
	action := loop.SpawnFixerAction{
		PlanFile: "fix-me.md",
		Feedback: "address review comments",
	}
	assert.Equal(t, "spawn_fixer", action.Kind())
}

func TestDaemon_ExecuteAction_SpawnFixer_BranchEmpty_Fails(t *testing.T) {
	// When the task store returns no branch, executeAction must propagate the
	// error from SpawnFixer (which fails inside spawnInSharedWorktree with
	// "Branch is required"). This test validates the wiring without real tmux.
	store := taskstore.NewTestStore(t)
	project := "test-project"
	// Task exists but has no branch — simulates a newly created task.
	require.NoError(t, store.Create(project, taskstore.TaskEntry{
		Filename: "fix-me.md",
		Status:   taskstore.StatusImplementing,
	}))

	d := &Daemon{
		repos:       NewRepoManager(),
		spawner:     NewTmuxSpawner(),
		logger:      slog.Default(),
		broadcaster: api.NewEventBroadcaster(),
	}
	e := RepoEntry{
		Path:    t.TempDir(),
		Project: project,
		Store:   store,
	}

	// SpawnFixer will fail with "Branch is required" because the store entry
	// has no branch set. executeAction must return the error and not panic.
	err := d.executeAction(context.Background(), e, loop.SpawnFixerAction{
		PlanFile: "fix-me.md",
		Feedback: "please fix the comments",
	})
	require.Error(t, err, "executeAction must propagate spawn error when branch is empty")
	assert.Contains(t, err.Error(), "Branch is required")
}

func TestDaemon_TaskComplete_FinalWaveTransitionsToReviewing(t *testing.T) {
	store := taskstore.NewTestStore(t)
	project := "test-project"
	planFile := "wave-plan.md"
	require.NoError(t, store.Create(project, taskstore.TaskEntry{
		Filename: planFile,
		Status:   taskstore.StatusImplementing,
	}))

	proc := loop.NewProcessor(loop.ProcessorConfig{Store: store, Project: project})
	proc.RegisterOrchestrator(planFile, 1, []int{1})

	d := &Daemon{
		cfg:         &DaemonConfig{},
		spawner:     NewTmuxSpawner(),
		logger:      slog.Default(),
		broadcaster: api.NewEventBroadcaster(),
	}
	e := RepoEntry{
		Path:      t.TempDir(),
		Project:   project,
		Store:     store,
		Processor: proc,
	}

	actions := proc.ProcessTaskSignals([]taskfsm.TaskSignal{{
		TaskFile:   planFile,
		WaveNumber: 1,
		TaskNumber: 1,
	}})
	require.Len(t, actions, 1)
	taskAction, ok := actions[0].(loop.TaskCompleteAction)
	require.True(t, ok)

	err := d.executeAction(context.Background(), e, taskAction)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Branch is required")

	entry, getErr := store.Get(project, planFile)
	require.NoError(t, getErr)
	assert.Equal(t, taskstore.StatusReviewing, entry.Status)
	assert.Nil(t, proc.WaveOrchestrator(planFile), "orchestrator must be cleared after final wave completion")
}

func TestDaemon_TaskComplete_AutoAdvanceStartsNextWave(t *testing.T) {
	store := taskstore.NewTestStore(t)
	project := "test-project"
	planFile := "multi-wave.md"
	require.NoError(t, store.Create(project, taskstore.TaskEntry{
		Filename: planFile,
		Status:   taskstore.StatusImplementing,
	}))
	require.NoError(t, store.SetContent(project, planFile, `# Feature Plan

## Wave 1
### Task 1: First Thing

Do the first thing.

## Wave 2
### Task 2: Second Thing

Do the second thing.
`))

	proc := loop.NewProcessor(loop.ProcessorConfig{Store: store, Project: project})
	advance := proc.ProcessWaveSignals([]taskfsm.WaveSignal{{TaskFile: planFile, WaveNumber: 1}})
	require.Len(t, advance, 1)
	_, ok := advance[0].(loop.AdvanceWaveAction)
	require.True(t, ok)

	d := &Daemon{
		cfg:         &DaemonConfig{AutoAdvanceWaves: true},
		spawner:     NewTmuxSpawner(),
		logger:      slog.Default(),
		broadcaster: api.NewEventBroadcaster(),
	}
	e := RepoEntry{
		Path:      t.TempDir(),
		Project:   project,
		Store:     store,
		Processor: proc,
	}

	actions := proc.ProcessTaskSignals([]taskfsm.TaskSignal{{
		TaskFile:   planFile,
		WaveNumber: 1,
		TaskNumber: 1,
	}})
	require.Len(t, actions, 1)
	taskAction, ok := actions[0].(loop.TaskCompleteAction)
	require.True(t, ok)

	err := d.executeAction(context.Background(), e, taskAction)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Branch is required")

	orch := proc.WaveOrchestrator(planFile)
	require.NotNil(t, orch)
	assert.Equal(t, 2, orch.CurrentWaveNumber())
	assert.Equal(t, orchestration.WaveStateRunning, orch.State())
}

func TestDaemon_StartWaveTasks_SpawnsWaveTasksConcurrently(t *testing.T) {
	store := taskstore.NewTestStore(t)
	project := "test-project"
	planFile := "concurrent-wave.md"
	require.NoError(t, store.Create(project, taskstore.TaskEntry{
		Filename: planFile,
		Status:   taskstore.StatusImplementing,
		Branch:   "plan/concurrent-wave",
	}))
	require.NoError(t, store.SetContent(project, planFile, `# Feature Plan

## Wave 1
### Task 1: First Thing

Do the first thing.

### Task 2: Second Thing

Do the second thing.
`))

	proc := loop.NewProcessor(loop.ProcessorConfig{Store: store, Project: project})
	actions := proc.ProcessWaveSignals([]taskfsm.WaveSignal{{TaskFile: planFile, WaveNumber: 1}})
	require.Len(t, actions, 1)
	advance, ok := actions[0].(loop.AdvanceWaveAction)
	require.True(t, ok)

	started := make(chan int, 2)
	release := make(chan struct{})
	d := &Daemon{
		spawner:     NewTmuxSpawner(),
		logger:      slog.Default(),
		broadcaster: api.NewEventBroadcaster(),
		spawnWaveTask: func(_ context.Context, opts loop.SpawnOpts, task taskparser.Task, prompt string, _ int, peerCount int) error {
			assert.Equal(t, planFile, opts.PlanFile)
			assert.Equal(t, 1, opts.Wave)
			assert.Equal(t, 2, peerCount)
			assert.NotEmpty(t, prompt)
			started <- task.Number
			<-release
			return nil
		},
	}
	e := RepoEntry{
		Path:      t.TempDir(),
		Project:   project,
		Store:     store,
		Processor: proc,
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- d.executeAction(context.Background(), e, advance)
	}()

	seen := make(map[int]bool, 2)
	deadline := time.After(200 * time.Millisecond)
	for len(seen) < 2 {
		select {
		case taskNum := <-started:
			seen[taskNum] = true
		case <-deadline:
			t.Fatal("expected both wave task spawns to begin before either returned")
		}
	}

	close(release)
	require.NoError(t, <-errCh)
	assert.True(t, seen[1])
	assert.True(t, seen[2])
}

// TestDaemon_StartWaveTasks_LimitOne verifies that with MaxParallelWaveTasks=1,
// only the first task is spawned and the rest remain pending.
func TestDaemon_StartWaveTasks_LimitOne(t *testing.T) {
	store := taskstore.NewTestStore(t)
	project := "test-project"
	planFile := "limited-wave.md"
	require.NoError(t, store.Create(project, taskstore.TaskEntry{
		Filename: planFile,
		Status:   taskstore.StatusImplementing,
		Branch:   "plan/limited-wave",
	}))
	require.NoError(t, store.SetContent(project, planFile, `# Feature Plan

## Wave 1
### Task 1: First

Do first.

### Task 2: Second

Do second.

### Task 3: Third

Do third.
`))

	proc := loop.NewProcessor(loop.ProcessorConfig{Store: store, Project: project})
	actions := proc.ProcessWaveSignals([]taskfsm.WaveSignal{{TaskFile: planFile, WaveNumber: 1}})
	require.Len(t, actions, 1)
	advance, ok := actions[0].(loop.AdvanceWaveAction)
	require.True(t, ok)

	var spawned []int
	var mu sync.Mutex
	d := &Daemon{
		spawner:     NewTmuxSpawner(),
		logger:      slog.Default(),
		broadcaster: api.NewEventBroadcaster(),
		spawnWaveTask: func(_ context.Context, _ loop.SpawnOpts, task taskparser.Task, _ string, _ int, _ int) error {
			mu.Lock()
			spawned = append(spawned, task.Number)
			mu.Unlock()
			return nil
		},
	}
	one := 1
	e := RepoEntry{
		Path:      t.TempDir(),
		Project:   project,
		Store:     store,
		Processor: proc,
		Resources: config.ResolvedResourceControls{MaxParallelWaveTasks: one},
	}

	require.NoError(t, d.executeAction(context.Background(), e, advance))

	mu.Lock()
	defer mu.Unlock()
	assert.Len(t, spawned, 1, "only one task should be spawned under limit=1")
	assert.Equal(t, 1, spawned[0])
}

// TestDaemon_TaskComplete_LaunchesPendingTask verifies that a completing task
// causes the next pending task to be spawned when running under a limit.
func TestDaemon_TaskComplete_LaunchesPendingTask(t *testing.T) {
	store := taskstore.NewTestStore(t)
	project := "test-project"
	planFile := "pending-launch.md"
	require.NoError(t, store.Create(project, taskstore.TaskEntry{
		Filename: planFile,
		Status:   taskstore.StatusImplementing,
		Branch:   "plan/pending-launch",
	}))
	require.NoError(t, store.SetContent(project, planFile, `# Feature Plan

## Wave 1
### Task 1: First

Do first.

### Task 2: Second

Do second.

### Task 3: Third

Do third.
`))

	proc := loop.NewProcessor(loop.ProcessorConfig{Store: store, Project: project})
	// Advance to wave 1 — start the orchestrator.
	actions := proc.ProcessWaveSignals([]taskfsm.WaveSignal{{TaskFile: planFile, WaveNumber: 1}})
	require.Len(t, actions, 1)
	advance, ok := actions[0].(loop.AdvanceWaveAction)
	require.True(t, ok)

	var spawned []int
	var mu sync.Mutex
	d := &Daemon{
		spawner:     NewTmuxSpawner(),
		logger:      slog.Default(),
		broadcaster: api.NewEventBroadcaster(),
		spawnWaveTask: func(_ context.Context, _ loop.SpawnOpts, task taskparser.Task, _ string, _ int, _ int) error {
			mu.Lock()
			spawned = append(spawned, task.Number)
			mu.Unlock()
			return nil
		},
	}
	one := 1
	e := RepoEntry{
		Path:      t.TempDir(),
		Project:   project,
		Store:     store,
		Processor: proc,
		Resources: config.ResolvedResourceControls{MaxParallelWaveTasks: one},
	}

	// Start wave — only task 1 should spawn.
	require.NoError(t, d.executeAction(context.Background(), e, advance))
	mu.Lock()
	assert.Equal(t, []int{1}, spawned)
	mu.Unlock()

	// Complete task 1 — should spawn task 2.
	taskActions := proc.ProcessTaskSignals([]taskfsm.TaskSignal{
		{TaskFile: planFile, WaveNumber: 1, TaskNumber: 1},
	})
	require.Len(t, taskActions, 1)
	taskComplete, ok := taskActions[0].(loop.TaskCompleteAction)
	require.True(t, ok)

	require.NoError(t, d.executeAction(context.Background(), e, taskComplete))
	mu.Lock()
	assert.Equal(t, []int{1, 2}, spawned, "task 2 should launch after task 1 completes")
	mu.Unlock()
}

// TestDaemon_TaskComplete_NoPendingLaunchWhenUnlimited verifies that when no limit
// is configured, the WaveStateRunning case does not attempt to launch pending tasks.
func TestDaemon_TaskComplete_NoPendingLaunchWhenUnlimited(t *testing.T) {
	store := taskstore.NewTestStore(t)
	project := "test-project"
	planFile := "unlimited-wave.md"
	require.NoError(t, store.Create(project, taskstore.TaskEntry{
		Filename: planFile,
		Status:   taskstore.StatusImplementing,
		Branch:   "plan/unlimited-wave",
	}))
	require.NoError(t, store.SetContent(project, planFile, `# Feature Plan

## Wave 1
### Task 1: First

Do first.

### Task 2: Second

Do second.
`))

	proc := loop.NewProcessor(loop.ProcessorConfig{Store: store, Project: project})
	actions := proc.ProcessWaveSignals([]taskfsm.WaveSignal{{TaskFile: planFile, WaveNumber: 1}})
	require.Len(t, actions, 1)
	advance, ok := actions[0].(loop.AdvanceWaveAction)
	require.True(t, ok)

	var spawnCallCount int
	var mu sync.Mutex
	d := &Daemon{
		spawner:     NewTmuxSpawner(),
		logger:      slog.Default(),
		broadcaster: api.NewEventBroadcaster(),
		spawnWaveTask: func(_ context.Context, _ loop.SpawnOpts, _ taskparser.Task, _ string, _ int, _ int) error {
			mu.Lock()
			spawnCallCount++
			mu.Unlock()
			return nil
		},
	}
	// No limit (zero = unlimited).
	e := RepoEntry{
		Path:      t.TempDir(),
		Project:   project,
		Store:     store,
		Processor: proc,
		Resources: config.ResolvedResourceControls{MaxParallelWaveTasks: 0},
	}

	// Both tasks should launch immediately.
	require.NoError(t, d.executeAction(context.Background(), e, advance))
	mu.Lock()
	assert.Equal(t, 2, spawnCallCount, "unlimited: both tasks spawn on wave start")
	mu.Unlock()
}

func TestDaemon_ArchitectCompletion_StartsWaveOneAfterRestart(t *testing.T) {
	store := taskstore.NewTestStore(t)
	project := "test-project"
	planFile := "architect-restart.md"
	require.NoError(t, store.Create(project, taskstore.TaskEntry{
		Filename: planFile,
		Status:   taskstore.StatusImplementing,
		ExecutionState: taskstore.ExecutionState{
			Phase:           string(taskfsm.ExecutionPhaseArchitecting),
			ActiveAgentType: session.AgentTypeElaborator,
		},
	}))
	require.NoError(t, store.SetContent(project, planFile, `# Feature Plan

## Wave 1
### Task 1: First Thing

Do the first thing.
`))

	proc := loop.NewProcessor(loop.ProcessorConfig{Store: store, Project: project})
	actions := proc.ProcessElaborationSignals([]taskfsm.ElaborationSignal{{TaskFile: planFile}})
	require.Len(t, actions, 1)
	advance, ok := actions[0].(loop.AdvanceWaveAction)
	require.True(t, ok)

	d := &Daemon{
		spawner:     NewTmuxSpawner(),
		logger:      slog.Default(),
		broadcaster: api.NewEventBroadcaster(),
	}
	e := RepoEntry{
		Path:      t.TempDir(),
		Project:   project,
		Store:     store,
		Processor: proc,
	}

	err := d.executeAction(context.Background(), e, advance)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Branch is required")

	orch := proc.WaveOrchestrator(planFile)
	require.NotNil(t, orch)
	assert.Equal(t, 1, orch.CurrentWaveNumber())
	assert.Equal(t, orchestration.WaveStateRunning, orch.State())

	entry, getErr := store.Get(project, planFile)
	require.NoError(t, getErr)
	assert.Equal(t, taskstore.ExecutionState{
		Phase:           string(taskfsm.ExecutionPhaseWaveRunning),
		ActiveAgentType: session.AgentTypeCoder,
		ActiveWave:      1,
	}, entry.ExecutionState)
}

func TestDaemon_ExecuteAction_SpawnMaster_PersistsExecutionStateAndEmitsEvent(t *testing.T) {
	store := taskstore.NewTestStore(t)
	const project = "test-project"
	const planFile = "master-plan.md"
	const branch = "plan/master-plan"

	// In real usage, ReviewApproved transitions the task to StatusVerifying
	// before SpawnMasterAction is emitted. Use StatusVerifying here so that
	// normalizeExecutionState preserves the master AgentType field.
	require.NoError(t, store.Create(project, taskstore.TaskEntry{
		Filename: planFile,
		Status:   taskstore.StatusVerifying,
		Branch:   branch,
	}))

	spawnCalled := false
	var broadcaster = api.NewEventBroadcaster()
	sub := broadcaster.Subscribe()
	t.Cleanup(func() { broadcaster.Unsubscribe(sub) })

	d := &Daemon{
		repos:       NewRepoManager(),
		spawner:     NewTmuxSpawner(),
		logger:      slog.Default(),
		broadcaster: broadcaster,
		spawnMaster: func(_ context.Context, opts loop.SpawnOpts) error {
			spawnCalled = true
			assert.Equal(t, planFile, opts.PlanFile)
			assert.Equal(t, branch, opts.Branch)
			return nil
		},
	}
	e := RepoEntry{Path: t.TempDir(), Project: project, Store: store}

	err := d.executeAction(context.Background(), e, loop.SpawnMasterAction{PlanFile: planFile})
	require.NoError(t, err)
	assert.True(t, spawnCalled, "spawnMaster must be called")

	// Verify execution state was persisted (phase is not set; status is tracked at FSM level).
	entry, getErr := store.Get(project, planFile)
	require.NoError(t, getErr)
	assert.Empty(t, entry.ExecutionState.Phase)
	assert.Equal(t, session.AgentTypeMaster, entry.ExecutionState.ActiveAgentType)

	// Verify broadcast event.
	select {
	case ev := <-sub:
		assert.Equal(t, api.EventKindAgentSpawned, ev.Kind)
		assert.Equal(t, session.AgentTypeMaster, ev.AgentType)
		assert.Equal(t, planFile, ev.PlanFile)
	case <-time.After(time.Second):
		t.Fatal("expected agent_spawned event")
	}
}

func TestDaemon_ExecuteAction_SpawnMaster_BranchEmpty_Fails(t *testing.T) {
	store := taskstore.NewTestStore(t)
	const project = "test-project"
	require.NoError(t, store.Create(project, taskstore.TaskEntry{
		Filename: "master-plan.md",
		Status:   taskstore.StatusReviewing,
	}))

	d := &Daemon{
		repos:       NewRepoManager(),
		spawner:     NewTmuxSpawner(),
		logger:      slog.Default(),
		broadcaster: api.NewEventBroadcaster(),
	}
	e := RepoEntry{Path: t.TempDir(), Project: project, Store: store}

	err := d.executeAction(context.Background(), e, loop.SpawnMasterAction{PlanFile: "master-plan.md"})
	require.Error(t, err, "executeAction must propagate spawn error when branch is empty")
	assert.Contains(t, err.Error(), "Branch is required")
}

func TestReapStuckSignals(t *testing.T) {
	gw, err := taskstore.NewSQLiteSignalGateway(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { _ = gw.Close() })

	require.NoError(t, gw.Create("proj", taskstore.SignalEntry{PlanFile: "stuck-plan", SignalType: "implement_finished"}))
	claimed, err := gw.Claim("proj", "worker-1")
	require.NoError(t, err)
	require.NotNil(t, claimed)
	require.NoError(t, gw.BackdateClaimedAt(claimed.ID, 2*time.Minute))

	n := reapStuckSignals([]RepoEntry{{SignalGateway: gw}}, 60*time.Second, slog.Default())
	assert.Equal(t, 1, n)

	pending, err := gw.List("proj", taskstore.SignalPending)
	require.NoError(t, err)
	assert.Len(t, pending, 1)
}

// ---------------------------------------------------------------------------
// SpawnSolo daemon tests
// ---------------------------------------------------------------------------

func TestDaemon_SpawnSolo_ReturnsBeforeSpawnCompletes(t *testing.T) {
	project := "proj"
	started := make(chan struct{}, 1)
	release := make(chan struct{})

	d := &Daemon{
		repos:       NewRepoManager(),
		spawner:     NewTmuxSpawner(),
		logger:      slog.Default(),
		broadcaster: api.NewEventBroadcaster(),
		spawnSolo: func(_ context.Context, opts SpawnSoloOpts) error {
			select {
			case started <- struct{}{}:
			default:
			}
			<-release
			return nil
		},
	}
	d.repos.repos = []RepoEntry{{Path: "/tmp/proj", Project: project}}

	req := api.SpawnSoloRequest{Title: "my-solo", Program: "claude", SoloAgent: true}
	errCh := make(chan error, 1)
	go func() { errCh <- d.SpawnSolo(project, req) }()

	select {
	case err := <-errCh:
		require.NoError(t, err)
	case <-time.After(200 * time.Millisecond):
		t.Fatal("SpawnSolo should return before async spawn completes")
	}

	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("background solo spawn did not start")
	}
	close(release)
}

func TestDaemon_SpawnSolo_ForwardsRepoSDKTranscriptLimits(t *testing.T) {
	project := "proj"
	gotOpts := make(chan SpawnSoloOpts, 1)
	d := &Daemon{
		repos:       NewRepoManager(),
		spawner:     NewTmuxSpawner(),
		logger:      slog.Default(),
		broadcaster: api.NewEventBroadcaster(),
		spawnSolo: func(_ context.Context, opts SpawnSoloOpts) error {
			gotOpts <- opts
			return nil
		},
	}
	d.repos.repos = []RepoEntry{{
		Path:    "/tmp/proj",
		Project: project,
		SDK: config.SDKConfig{
			TranscriptMaxBytes: 0,
			TranscriptMaxTurns: 0,
		},
	}}

	require.NoError(t, d.SpawnSolo(project, api.SpawnSoloRequest{Title: "my-solo", Program: "claude"}))

	select {
	case opts := <-gotOpts:
		assert.True(t, opts.SDKTranscriptLimitsSet)
		assert.Equal(t, int64(0), opts.SDKTranscriptMaxBytes)
		assert.Equal(t, int64(0), opts.SDKTranscriptMaxTurns)
	case <-time.After(time.Second):
		t.Fatal("background solo spawn did not start")
	}
}

func TestDaemon_SpawnSolo_ConflictWhileAsyncSpawnPending(t *testing.T) {
	project := "proj"
	started := make(chan struct{}, 1)
	release := make(chan struct{})

	d := &Daemon{
		repos:       NewRepoManager(),
		spawner:     NewTmuxSpawner(),
		logger:      slog.Default(),
		broadcaster: api.NewEventBroadcaster(),
		spawnSolo: func(_ context.Context, opts SpawnSoloOpts) error {
			select {
			case started <- struct{}{}:
			default:
			}
			<-release
			return nil
		},
	}
	d.repos.repos = []RepoEntry{{Path: "/tmp/proj", Project: project}}

	req := api.SpawnSoloRequest{Title: "my-solo", Program: "claude", SoloAgent: true}
	require.NoError(t, d.SpawnSolo(project, req))

	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("background solo spawn did not start")
	}

	err := d.SpawnSolo(project, req)
	require.Error(t, err)
	assert.ErrorIs(t, err, api.ErrStandaloneConflict)
	close(release)
}

func TestDaemon_SpawnSolo_ProjectNotFound(t *testing.T) {
	d := &Daemon{
		repos:  NewRepoManager(),
		logger: slog.Default(),
	}
	d.repos.repos = []RepoEntry{{Path: "/tmp/proj", Project: "proj"}}

	err := d.SpawnSolo("missing", api.SpawnSoloRequest{Title: "t", Program: "claude"})
	require.Error(t, err)
	assert.ErrorIs(t, err, api.ErrProjectNotFound)
}

func TestDaemon_SpawnSolo_NonSDKProgram_Returns400(t *testing.T) {
	d := &Daemon{
		repos:  NewRepoManager(),
		logger: slog.Default(),
	}
	d.repos.repos = []RepoEntry{{Path: "/tmp/proj", Project: "proj"}}

	// "nvim" is not an SDK-supported program.
	err := d.SpawnSolo("proj", api.SpawnSoloRequest{Title: "t", Program: "nvim"})
	require.Error(t, err)
	assert.ErrorIs(t, err, api.ErrInvalidRequest)
}

func TestDaemonStateAdapter_SpawnSolo_Conflict(t *testing.T) {
	const (
		project  = "proj"
		repoPath = "/tmp/proj"
	)

	d := &Daemon{
		repos:       NewRepoManager(),
		spawner:     NewTmuxSpawner(),
		logger:      slog.Default(),
		broadcaster: api.NewEventBroadcaster(),
	}
	d.repos.repos = []RepoEntry{{Path: repoPath, Project: project}}

	// Block the start so the instance stays in Loading state.
	release := make(chan struct{})
	d.spawner.startOnMain = func(inst *session.Instance) error {
		<-release
		inst.MarkStartedForTest()
		return nil
	}

	req := api.SpawnSoloRequest{Title: "dupe-solo", Program: "claude"}
	adapter := &daemonStateAdapter{d: d}

	// First spawn should succeed.
	require.NoError(t, adapter.SpawnSolo(project, req))

	// Wait until the instance is tracked.
	require.Eventually(t, func() bool {
		tracked := d.spawner.InstancesForProject(repoPath, project)
		return len(tracked) > 0
	}, time.Second, 5*time.Millisecond)

	// Second spawn with the same title returns 409 synchronously while the first
	// request is already tracked.
	err := adapter.SpawnSolo(project, req)
	require.Error(t, err)
	assert.ErrorIs(t, err, api.ErrStandaloneConflict)
	assert.Contains(t, err.Error(), "dupe-solo")
	close(release)
}

func TestDaemonStateAdapter_ListInstances_IncludesSoloAgentFields(t *testing.T) {
	const (
		project  = "proj"
		repoPath = "/tmp/proj"
	)
	spawner := NewTmuxSpawner()
	d := &Daemon{
		repos:       NewRepoManager(),
		spawner:     spawner,
		logger:      slog.Default(),
		broadcaster: api.NewEventBroadcaster(),
	}
	d.repos.repos = []RepoEntry{{Path: repoPath, Project: project}}

	// Manually track a standalone SDK instance.
	key := instanceKeyForStandalone(repoPath, "sdk-solo-agent")
	inst := &session.Instance{
		Title:           "sdk-solo-agent",
		Path:            repoPath,
		SoloAgent:       true,
		SDKSpeedTier:    "fast",
		SkipPermissions: true,
		ResourceProfile: "interactive",
		Status:          session.Loading,
	}
	spawner.mu.Lock()
	spawner.instances[key] = inst
	spawner.planFileByKey[key] = ""
	spawner.agentTypeByKey[key] = session.AgentTypeMaster
	spawner.projectByKey[key] = project
	spawner.mu.Unlock()

	adapter := &daemonStateAdapter{d: d}
	statuses := adapter.ListInstances(project)
	require.Len(t, statuses, 1)
	assert.True(t, statuses[0].SoloAgent)
	assert.Equal(t, "fast", statuses[0].SDKSpeedTier)
	assert.Equal(t, "interactive", statuses[0].ResourceProfile)
	require.NotNil(t, statuses[0].SkipPermissions)
	assert.True(t, *statuses[0].SkipPermissions)
	assert.Equal(t, "sdk-solo-agent", statuses[0].Title)
	assert.True(t, statuses[0].Loading)
	assert.True(t, statuses[0].Active)
}

func TestDaemonStateAdapter_ActivePlans_IgnoresStandaloneInstances(t *testing.T) {
	const (
		project  = "proj"
		repoPath = "/tmp/proj"
	)
	spawner := NewTmuxSpawner()
	d := &Daemon{
		repos:   NewRepoManager(),
		spawner: spawner,
		logger:  slog.Default(),
	}
	d.repos.repos = []RepoEntry{{Path: repoPath, Project: project}}

	// Track a standalone instance (no plan file).
	soloKey := instanceKeyForStandalone(repoPath, "solo-agent")
	spawner.mu.Lock()
	spawner.instances[soloKey] = &session.Instance{Title: "solo-agent", Path: repoPath}
	spawner.planFileByKey[soloKey] = ""
	spawner.projectByKey[soloKey] = project
	spawner.mu.Unlock()

	// Track a real plan instance.
	planKey := instanceKey(repoPath, "feature.md", session.AgentTypeCoder)
	spawner.mu.Lock()
	spawner.instances[planKey] = &session.Instance{Title: "feature-coder", Path: repoPath, TaskFile: "feature.md"}
	spawner.planFileByKey[planKey] = "feature.md"
	spawner.projectByKey[planKey] = project
	spawner.mu.Unlock()

	adapter := &daemonStateAdapter{d: d}
	counts := adapter.activePlansByProject()
	assert.Equal(t, 1, counts[project], "standalone instances must not inflate the active plan count")
}
