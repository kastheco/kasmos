package daemon

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kastheco/kasmos/config/taskfsm"
	"github.com/kastheco/kasmos/config/taskparser"
	"github.com/kastheco/kasmos/config/taskstore"
	"github.com/kastheco/kasmos/daemon/api"
	"github.com/kastheco/kasmos/orchestration"
	"github.com/kastheco/kasmos/orchestration/loop"
	"github.com/kastheco/kasmos/session"
	tmuxpkg "github.com/kastheco/kasmos/session/tmux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
		Program:       "true",
		ExecutionMode: session.ExecutionModeHeadless,
		TaskFile:      planFile,
		AgentType:     session.AgentTypeElaborator,
	})
	require.NoError(t, err)
	require.NoError(t, inst.StartOnMainBranch())
	require.Eventually(t, func() bool { return !inst.TmuxAlive() }, time.Second, 10*time.Millisecond)

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
		Program:       "true",
		ExecutionMode: session.ExecutionModeHeadless,
		TaskFile:      planFile,
		AgentType:     session.AgentTypeCoder,
		WaveNumber:    1,
		TaskNumber:    1,
	})
	require.NoError(t, err)
	require.NoError(t, inst.StartOnMainBranch())
	require.Eventually(t, func() bool { return !inst.TmuxAlive() }, time.Second, 10*time.Millisecond)
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
		Program:       "true",
		ExecutionMode: session.ExecutionModeHeadless,
		TaskFile:      planFile,
		AgentType:     session.AgentTypeElaborator,
	})
	require.NoError(t, err)
	require.NoError(t, inst.StartOnMainBranch())
	require.Eventually(t, func() bool { return !inst.TmuxAlive() }, time.Second, 10*time.Millisecond)

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
