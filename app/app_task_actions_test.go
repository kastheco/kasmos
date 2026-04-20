package app

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"github.com/kastheco/kasmos/config"
	"github.com/kastheco/kasmos/config/auditlog"
	"github.com/kastheco/kasmos/config/taskfsm"
	"github.com/kastheco/kasmos/config/taskparser"
	"github.com/kastheco/kasmos/config/taskstate"
	"github.com/kastheco/kasmos/config/taskstore"
	"github.com/kastheco/kasmos/daemon/api"
	"github.com/kastheco/kasmos/orchestration"
	"github.com/kastheco/kasmos/orchestration/loop"
	"github.com/kastheco/kasmos/session"
	gitpkg "github.com/kastheco/kasmos/session/git"
	"github.com/kastheco/kasmos/ui"
	"github.com/kastheco/kasmos/ui/overlay"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildPlanPrompt(t *testing.T) {
	prompt := buildPlanningPrompt("auth-refactor", "Auth Refactor", "Refactor JWT auth", "myproject")
	if !strings.Contains(prompt, "Plan Auth Refactor") {
		t.Fatalf("prompt missing title")
	}
	if !strings.Contains(prompt, "Goal: Refactor JWT auth") {
		t.Fatalf("prompt missing goal")
	}
	// Wave headers are required for kasmos orchestration — the prompt must
	// instruct the planner to include them.
	assert.Contains(t, prompt, "Wave", "plan prompt must mention Wave headers for kasmos orchestration")
	assert.Contains(t, prompt, "kasmos-planner", "plan prompt must reference the kasmos-planner skill")
	assert.Contains(t, prompt, "task_update_content", "plan prompt must include MCP content storage")
	assert.Contains(t, prompt, "planner-finished", "plan prompt must include planner completion signal")
	assert.Contains(t, prompt, `project: "myproject"`, "plan prompt must include project in MCP examples")
}

func TestBuildWaveAnnotationPrompt(t *testing.T) {
	prompt := orchestration.BuildWaveAnnotationPrompt("my-feature", "myproject")
	assert.Contains(t, prompt, "task_show", "prompt must reference MCP task_show")
	assert.Contains(t, prompt, "## Wave", "prompt must mention ## Wave header format")
	assert.Contains(t, prompt, "task_update_content", "prompt must instruct the planner to store content via MCP")
	assert.Contains(t, prompt, "planner-finished", "prompt must include planner completion signal")
	assert.Contains(t, prompt, `project: "myproject"`, "prompt must include project in MCP examples")
	assert.NotContains(t, prompt, "The plan at docs/plans/", "prompt must not reference disk path for reading")
}

func TestBuildWaveAnnotationPrompt_SingleWaveFallback(t *testing.T) {
	prompt := orchestration.BuildWaveAnnotationPrompt("trivial", "myproject")
	// Even trivial plans must be wrapped in at least ## Wave 1
	assert.Contains(t, prompt, "## Wave 1", "prompt must specify ## Wave 1 as the minimum structure")
}

func TestBuildImplementPrompt(t *testing.T) {
	prompt := buildImplementPrompt("auth-refactor", "myproject")
	assert.Contains(t, prompt, "kas task show auth-refactor")
	assert.Contains(t, prompt, `project: "myproject"`)
	assert.NotContains(t, prompt, "docs/plans/")
	assert.NotContains(t, prompt, "kasmos-coder", "implement prompt must not reference skill to avoid skill-load overhead")
}

func TestSoloAgentPrompt_ContainsTestScopingRule(t *testing.T) {
	prompt := buildSoloPrompt("auth-refactor", "Refactor JWT auth", "auth-refactor", "myproject")
	assert.Contains(t, prompt, "-run Test")
	assert.Contains(t, prompt, "Do not load skills")
}

func TestBuildSoloPrompt_WithDescription(t *testing.T) {
	prompt := buildSoloPrompt("auth-refactor", "Refactor JWT auth", "auth-refactor", "myproject")
	assert.Contains(t, prompt, "kas task show auth-refactor")
	assert.Contains(t, prompt, `project: "myproject"`)
	assert.NotContains(t, prompt, "docs/plans/")
}

func TestBuildSoloPrompt_StubOnly(t *testing.T) {
	prompt := buildSoloPrompt("quick-fix", "Fix the login bug", "", "myproject")
	assert.NotContains(t, prompt, "kas task show")
	assert.NotContains(t, prompt, "docs/plans/")
}

func TestAgentTypeForSubItem(t *testing.T) {
	tests := map[string]string{
		"plan":      session.AgentTypePlanner,
		"implement": session.AgentTypeCoder,
		"review":    session.AgentTypeReviewer,
		"solo":      session.AgentTypeCoder,
	}
	for action, want := range tests {
		got, ok := agentTypeForSubItem(action)
		if !ok {
			t.Fatalf("agentTypeForSubItem(%q) returned ok=false", action)
		}
		if got != want {
			t.Fatalf("agentTypeForSubItem(%q) = %q, want %q", action, got, want)
		}
	}
}

func TestIsLocked_AllowsSoloStage(t *testing.T) {
	assert.False(t, isLocked(taskstate.StatusReady, "solo"),
		"solo stage should be triggerable like implement/review")
}

// TestSpawnPlanAgent_ReviewerSetsIsReviewer verifies that spawnTaskAgent sets
// IsReviewer=true on the created instance when the action is "review", so that
// the reviewer completion check in the metadata tick handler (which gates on
// inst.IsReviewer) can detect when the reviewer session exits.
//
// This is a regression test for the bug where spawnTaskAgent set AgentType but
// not IsReviewer, causing sidebar-spawned reviewers to never trigger plan completion.
func TestSpawnPlanAgent_ReviewerSetsIsReviewer(t *testing.T) {
	dir := t.TempDir()

	// Set up a minimal git repo so shared.Setup() can open it.
	for _, cmd := range [][]string{
		{"git", "init", dir},
		{"git", "-C", dir, "config", "user.email", "test@test.com"},
		{"git", "-C", dir, "config", "user.name", "Test"},
		{"git", "-C", dir, "commit", "--allow-empty", "-m", "init"},
	} {
		out, err := exec.Command(cmd[0], cmd[1:]...).CombinedOutput()
		if err != nil {
			t.Skipf("git setup failed (%v): %s", err, out)
		}
	}

	plansDir := filepath.Join(dir, "docs", "plans")
	if err := os.MkdirAll(plansDir, 0o755); err != nil {
		t.Fatal(err)
	}
	ps, err := newTestPlanState(t, plansDir)
	if err != nil {
		t.Fatal(err)
	}
	planFile := "test.md"
	if err := ps.Register(planFile, "test plan", "plan/test", time.Now()); err != nil {
		t.Fatal(err)
	}

	sp := spinner.New(spinner.WithSpinner(spinner.Dot))
	list := ui.NewNavigationPanel(&sp)
	h := &home{
		taskState:          ps,
		activeRepoPath:     dir,
		program:            "opencode",
		nav:                list,
		menu:               ui.NewMenu(),
		instanceFinalizers: make(map[*session.Instance]func()),
	}

	h.spawnTaskAgent(planFile, "review", "review prompt")

	instances := list.GetInstances()
	if len(instances) == 0 {
		t.Fatal("expected instance to be added to list after spawnTaskAgent(review)")
	}
	inst := instances[len(instances)-1]
	if inst.AgentType != session.AgentTypeReviewer {
		t.Fatalf("AgentType = %q, want %q", inst.AgentType, session.AgentTypeReviewer)
	}
	if !inst.IsReviewer {
		t.Fatal("spawnTaskAgent(review) must set IsReviewer=true on the created instance")
	}
}

// TestSpawnPlanAgent_PlannerUsesMainBranch verifies that spawnTaskAgent for the
// "plan" action does NOT create a git worktree — the planner runs on main and
// commits plan files there directly.
func TestSpawnPlanAgent_PlannerUsesMainBranch(t *testing.T) {
	dir := t.TempDir()

	for _, cmd := range [][]string{
		{"git", "init", dir},
		{"git", "-C", dir, "config", "user.email", "test@test.com"},
		{"git", "-C", dir, "config", "user.name", "Test"},
		{"git", "-C", dir, "commit", "--allow-empty", "-m", "init"},
	} {
		out, err := exec.Command(cmd[0], cmd[1:]...).CombinedOutput()
		if err != nil {
			t.Skipf("git setup failed (%v): %s", err, out)
		}
	}

	plansDir := filepath.Join(dir, "docs", "plans")
	if err := os.MkdirAll(plansDir, 0o755); err != nil {
		t.Fatal(err)
	}
	ps, err := newTestPlanState(t, plansDir)
	if err != nil {
		t.Fatal(err)
	}
	planFile := "test-planner.md"
	if err := ps.Register(planFile, "test plan", "plan/test-planner", time.Now()); err != nil {
		t.Fatal(err)
	}

	sp := spinner.New(spinner.WithSpinner(spinner.Dot))
	list := ui.NewNavigationPanel(&sp)
	h := &home{
		taskState:          ps,
		activeRepoPath:     dir,
		program:            "opencode",
		nav:                list,
		menu:               ui.NewMenu(),
		instanceFinalizers: make(map[*session.Instance]func()),
	}

	h.spawnTaskAgent(planFile, "plan", "plan prompt")

	instances := list.GetInstances()
	if len(instances) == 0 {
		t.Fatal("expected instance to be added to list after spawnTaskAgent(plan)")
	}
	inst := instances[len(instances)-1]
	if inst.AgentType != session.AgentTypePlanner {
		t.Fatalf("AgentType = %q, want %q", inst.AgentType, session.AgentTypePlanner)
	}
	// Planner should have no branch assigned — it runs on main, not a worktree branch.
	if inst.Branch != "" {
		t.Fatalf("planner instance must have empty Branch (runs on main), got %q", inst.Branch)
	}
}

// TestSpawnTaskAgent_PatchesSharedWorktreeOpencodeConfig verifies that when
// spawnTaskAgent is called for an "implement" (coder) action, PatchWorktreeConfig is
// applied to the SHARED WORKTREE path — not the main repo — so the agent running inside
// the worktree reads the correct model/temperature/effort from its own opencode.jsonc.
func TestSpawnTaskAgent_PatchesSharedWorktreeOpencodeConfig(t *testing.T) {
	dir := t.TempDir()

	// Build a git repo with root opencode.jsonc committed so the worktree
	// inherits the file when git worktree add creates it.
	configPath := filepath.Join(dir, "opencode.jsonc")
	require.NoError(t, os.WriteFile(configPath, []byte(`{"agent":{"coder":{"model":"anthropic/old-coder","temperature":0.1,"reasoningEffort":"low"}}}`), 0o644))

	for _, cmd := range [][]string{
		{"git", "init", dir},
		{"git", "-C", dir, "config", "user.email", "test@test.com"},
		{"git", "-C", dir, "config", "user.name", "Test"},
		{"git", "-C", dir, "add", "."},
		{"git", "-C", dir, "commit", "-m", "init with opencode config"},
	} {
		out, err := exec.Command(cmd[0], cmd[1:]...).CombinedOutput()
		if err != nil {
			t.Skipf("git setup failed (%v): %s", err, out)
		}
	}

	plansDir := filepath.Join(dir, "docs", "plans")
	require.NoError(t, os.MkdirAll(plansDir, 0o755))
	ps, err := newTestPlanState(t, plansDir)
	require.NoError(t, err)
	planFile := "shared-wt-patch"
	require.NoError(t, ps.Register(planFile, "shared wt patch test", "plan/shared-wt-patch", time.Now()))

	coderTemp := 0.8
	sp := spinner.New(spinner.WithSpinner(spinner.Dot))
	m := &home{
		taskState:      ps,
		activeRepoPath: dir,
		program:        "opencode",
		nav:            ui.NewNavigationPanel(&sp),
		menu:           ui.NewMenu(),
		toastManager:   overlay.NewToastManager(&sp),
		appConfig: &config.Config{
			PhaseRoles: map[string]string{
				"implementing": "coder",
			},
			Profiles: map[string]config.AgentProfile{
				"coder": {
					Program:     "opencode",
					Model:       "claude-opus-4-6",
					Temperature: &coderTemp,
					Effort:      "high",
					Enabled:     true,
				},
			},
		},
		instanceFinalizers: make(map[*session.Instance]func()),
	}

	_, cmd := m.spawnTaskAgent(planFile, "implement", "implement prompt")
	if cmd != nil {
		_ = cmd()
	}

	// The shared worktree path is derived from the plan branch.
	branch := gitpkg.TaskBranchFromFile(planFile)
	worktreePath := gitpkg.TaskWorktreePath(dir, branch)
	worktreeConfigPath := filepath.Join(worktreePath, "opencode.jsonc")

	data, err := os.ReadFile(worktreeConfigPath)
	require.NoError(t, err, "worktree opencode.jsonc must exist after shared worktree setup")

	var cfg map[string]any
	require.NoError(t, json.Unmarshal(data, &cfg))
	agentCfg, ok := cfg["agent"].(map[string]any)
	require.True(t, ok, "agent block must exist")
	coderCfg, ok := agentCfg["coder"].(map[string]any)
	require.True(t, ok, "coder block must exist")
	assert.Equal(t, "anthropic/claude-opus-4-6", coderCfg["model"], "worktree opencode.jsonc must have patched model")
	assert.InDelta(t, coderTemp, coderCfg["temperature"].(float64), 0.0001, "worktree opencode.jsonc must have patched temperature")
	assert.Equal(t, "high", coderCfg["reasoningEffort"], "worktree opencode.jsonc must have patched reasoningEffort")
}

func TestSpawnTaskAgent_PlanUsesDaemonWhenRepoManaged(t *testing.T) {
	oldManaged := repoManagedByDaemon
	oldSpawner := spawnPlannerWithDaemon
	t.Cleanup(func() {
		repoManagedByDaemon = oldManaged
		spawnPlannerWithDaemon = oldSpawner
	})
	repoManagedByDaemon = func(string) bool { return true }

	dir := t.TempDir()
	plansDir := filepath.Join(dir, "docs", "plans")
	require.NoError(t, os.MkdirAll(plansDir, 0o755))
	ps, err := newTestPlanState(t, plansDir)
	require.NoError(t, err)
	planFile := "daemon-planner.md"
	require.NoError(t, ps.Register(planFile, "daemon planner test", "plan/daemon-planner", time.Now()))

	fakeInst, err := session.NewInstance(session.InstanceOptions{
		Title:     "daemon-planner-plan",
		Path:      dir,
		Program:   "opencode",
		AgentType: session.AgentTypePlanner,
		TaskFile:  planFile,
	})
	require.NoError(t, err)
	fakeInst.MarkStartedForTest()
	fakeInst.SetStatus(session.Running)

	called := false
	spawnPlannerWithDaemon = func(repoPath, project, file, title, prompt, program string) (*session.Instance, error) {
		called = true
		assert.Equal(t, dir, repoPath)
		assert.Equal(t, filepath.Base(dir), project)
		assert.Equal(t, planFile, file)
		assert.Equal(t, taskstate.DisplayName(planFile)+"-plan", title)
		assert.Equal(t, "plan prompt", prompt)
		assert.Equal(t, "opencode", program)
		return fakeInst, nil
	}

	sp := spinner.New(spinner.WithSpinner(spinner.Dot))
	h := &home{
		taskState:          ps,
		activeRepoPath:     dir,
		taskStoreProject:   filepath.Base(dir),
		program:            "opencode",
		nav:                ui.NewNavigationPanel(&sp),
		menu:               ui.NewMenu(),
		tabbedWindow:       ui.NewTabbedWindow(ui.NewPreviewPane(), ui.NewInfoPane()),
		toastManager:       overlay.NewToastManager(&sp),
		instanceFinalizers: make(map[*session.Instance]func()),
	}

	_, cmd := h.spawnTaskAgent(planFile, "plan", "plan prompt")
	require.NotNil(t, cmd)
	msg := cmd()
	var started daemonPlannerStartedMsg
	if batch, ok := msg.(tea.BatchMsg); ok {
		for _, sub := range batch {
			if dm, ok := sub().(daemonPlannerStartedMsg); ok {
				started = dm
				break
			}
		}
	} else if dm, ok := msg.(daemonPlannerStartedMsg); ok {
		started = dm
	}
	require.NotNil(t, started.instance)

	updatedModel, _ := h.Update(started)
	updated := updatedModel.(*home)
	require.True(t, called)
	instances := updated.nav.GetInstances()
	require.Len(t, instances, 1)
	assert.Equal(t, session.AgentTypePlanner, instances[0].AgentType)
	assert.Equal(t, planFile, instances[0].TaskFile)
	assert.Equal(t, fakeInst.Title, instances[0].Title)
}

func TestWaitForDaemonPlannerInstance_SkipsExitedPlaceholder(t *testing.T) {
	oldRestore := restoreInstanceFromData
	t.Cleanup(func() {
		restoreInstanceFromData = oldRestore
	})

	attempts := 0
	restoreInstanceFromData = func(data session.InstanceData) (*session.Instance, error) {
		attempts++
		inst, err := session.NewInstance(session.InstanceOptions{
			Title:         data.Title,
			Path:          data.Path,
			Program:       data.Program,
			ExecutionMode: data.ExecutionMode,
			TaskFile:      data.TaskFile,
			AgentType:     data.AgentType,
		})
		if err != nil {
			return nil, err
		}
		inst.MarkStartedForTest()
		inst.SetStatus(session.Running)
		if attempts == 1 {
			inst.Exited = true
			inst.SetStatus(session.Ready)
			return inst, nil
		}
		return inst, nil
	}

	inst, err := waitForDaemonPlannerInstance("", session.InstanceData{
		Title:         "planner-test",
		Path:          t.TempDir(),
		Program:       "opencode",
		ExecutionMode: session.ExecutionModeTmux,
		TaskFile:      "planner-test.md",
		AgentType:     session.AgentTypePlanner,
	})
	require.NoError(t, err)
	require.NotNil(t, inst)
	assert.False(t, inst.Exited)
	assert.GreaterOrEqual(t, attempts, 2)
}

func TestWaitForDaemonPlannerInstance_ToleratesSlowStartup(t *testing.T) {
	oldRestore := restoreInstanceFromData
	t.Cleanup(func() {
		restoreInstanceFromData = oldRestore
	})

	attempts := 0
	restoreInstanceFromData = func(data session.InstanceData) (*session.Instance, error) {
		attempts++
		inst, err := session.NewInstance(session.InstanceOptions{
			Title:         data.Title,
			Path:          data.Path,
			Program:       data.Program,
			ExecutionMode: data.ExecutionMode,
			TaskFile:      data.TaskFile,
			AgentType:     data.AgentType,
		})
		if err != nil {
			return nil, err
		}
		inst.MarkStartedForTest()
		inst.SetStatus(session.Running)
		if attempts <= 25 {
			inst.Exited = true
			inst.SetStatus(session.Ready)
			return inst, nil
		}
		return inst, nil
	}

	inst, err := waitForDaemonPlannerInstance("", session.InstanceData{
		Title:         "planner-test",
		Path:          t.TempDir(),
		Program:       "opencode",
		ExecutionMode: session.ExecutionModeTmux,
		TaskFile:      "planner-test.md",
		AgentType:     session.AgentTypePlanner,
	})
	require.NoError(t, err)
	require.NotNil(t, inst)
	assert.False(t, inst.Exited)
	assert.GreaterOrEqual(t, attempts, 26)
}

func TestWaitForDaemonPlannerInstance_UsesDaemonLoadingPlaceholder(t *testing.T) {
	oldRestore := restoreInstanceFromData
	oldListInstances := listDaemonInstances
	t.Cleanup(func() {
		restoreInstanceFromData = oldRestore
		listDaemonInstances = oldListInstances
	})

	restoreInstanceFromData = func(data session.InstanceData) (*session.Instance, error) {
		return nil, assert.AnError
	}
	listDaemonInstances = func(project string) ([]api.InstanceStatus, error) {
		require.Equal(t, "proj", project)
		return []api.InstanceStatus{{
			Title:   "planner-test",
			Plan:    "planner-test.md",
			Role:    session.AgentTypePlanner,
			Active:  true,
			Loading: true,
			Program: "opencode",
		}}, nil
	}

	inst, err := waitForDaemonPlannerInstance("proj", session.InstanceData{
		Title:         "planner-test",
		Path:          t.TempDir(),
		Program:       "opencode",
		ExecutionMode: session.ExecutionModeTmux,
		TaskFile:      "planner-test.md",
		AgentType:     session.AgentTypePlanner,
		Status:        session.Loading,
	})
	require.NoError(t, err)
	require.NotNil(t, inst)
	assert.Equal(t, session.Loading, inst.Status)
	assert.False(t, inst.Exited)
	assert.False(t, inst.Started())
}

// TestSpawnWaveTasks_SDKProfileFallsBackToTmuxForUnsupportedProgram verifies that
// a coder profile with execution_mode="sdk" (or the legacy "headless" alias) results
// in an instance with ExecutionModeTmux when the program does not support SDK transport.
// The final resolved mode is always the actual process host so UI and livepreview state
// are consistent.
func TestSpawnWaveTasks_SDKProfileFallsBackToTmuxForUnsupportedProgram(t *testing.T) {
	dir := t.TempDir()
	for _, cmd := range [][]string{
		{"git", "init", dir},
		{"git", "-C", dir, "config", "user.email", "test@test.com"},
		{"git", "-C", dir, "config", "user.name", "Test"},
		{"git", "-C", dir, "commit", "--allow-empty", "-m", "init"},
	} {
		out, err := exec.Command(cmd[0], cmd[1:]...).CombinedOutput()
		if err != nil {
			t.Skipf("git setup failed (%v): %s", err, out)
		}
	}

	planDoc := "# test\n\n## Wave 1\n\n### Task 1: implement sdk execution\n\nDo it.\n"
	parsed, err := taskparser.Parse(planDoc)
	require.NoError(t, err)
	require.Len(t, parsed.Waves, 1)

	orch := orchestration.NewWaveOrchestrator("test.md", parsed)
	tasks := orch.StartNextWave()
	require.Len(t, tasks, 1)

	// spawnWaveTasks persists execution state before spawning; provide a minimal
	// store and taskState so that write does not short-circuit the spawn.
	store := taskstore.NewTestSQLiteStore(t)
	require.NoError(t, store.Create("test", taskstore.TaskEntry{
		Filename: "test.md",
		Status:   taskstore.StatusImplementing,
		Branch:   "plan/test",
		ExecutionState: taskstore.ExecutionState{
			Phase:           string(taskfsm.ExecutionPhaseWaveRunning),
			ActiveAgentType: session.AgentTypeCoder,
			ActiveWave:      1,
		},
	}))
	ps, err := taskstate.Load(store, "test", "")
	require.NoError(t, err)

	sp := spinner.New(spinner.WithSpinner(spinner.Dot))
	list := ui.NewNavigationPanel(&sp)
	h := &home{
		activeRepoPath:     dir,
		program:            "opencode",
		nav:                list,
		menu:               ui.NewMenu(),
		toastManager:       overlay.NewToastManager(&sp),
		instanceFinalizers: make(map[*session.Instance]func()),
		taskState:          ps,
		taskStore:          store,
		taskStoreProject:   "test",
		appConfig: &config.Config{
			PhaseRoles: map[string]string{"implementing": "coder"},
			Profiles: map[string]config.AgentProfile{
				"coder": {
					Program:       "opencode",
					Enabled:       true,
					ExecutionMode: config.ExecutionModeSDK,
				},
			},
		},
	}

	entry := taskstate.TaskEntry{Branch: "plan/test"}
	_, _ = h.spawnWaveTasks(orch, tasks, entry)

	instances := list.GetInstances()
	require.Len(t, instances, 1)
	// opencode does not support SDK transport; the resolved mode falls back to tmux.
	assert.Equal(t, session.ExecutionModeTmux, instances[0].ExecutionMode)
}

// TestSpawnWaveTasks_PatchesSharedWorktreeOpencodeConfig verifies that spawnWaveTasks
// patches the SHARED WORKTREE's opencode.jsonc, not the main repo's, so coder agents
// spawned by wave orchestration read the correct config from their worktree.
func TestSpawnWaveTasks_PatchesSharedWorktreeOpencodeConfig(t *testing.T) {
	dir := t.TempDir()

	configPath := filepath.Join(dir, "opencode.jsonc")
	require.NoError(t, os.WriteFile(configPath, []byte(`{"agent":{"coder":{"model":"anthropic/old-wave-coder","temperature":0.2,"reasoningEffort":"low"}}}`), 0o644))

	for _, cmd := range [][]string{
		{"git", "init", dir},
		{"git", "-C", dir, "config", "user.email", "test@test.com"},
		{"git", "-C", dir, "config", "user.name", "Test"},
		{"git", "-C", dir, "add", "."},
		{"git", "-C", dir, "commit", "-m", "init with opencode config"},
	} {
		out, err := exec.Command(cmd[0], cmd[1:]...).CombinedOutput()
		if err != nil {
			t.Skipf("git setup failed (%v): %s", err, out)
		}
	}

	plansDir := filepath.Join(dir, "docs", "plans")
	require.NoError(t, os.MkdirAll(plansDir, 0o755))
	ps, err := newTestPlanState(t, plansDir)
	require.NoError(t, err)
	const planFile = "wave-wt-patch"
	require.NoError(t, ps.Register(planFile, "wave wt patch test", "plan/wave-wt-patch", time.Now()))

	coderTemp := 0.75
	sp := spinner.New(spinner.WithSpinner(spinner.Dot))
	m := &home{
		taskState:      ps,
		activeRepoPath: dir,
		program:        "opencode",
		nav:            ui.NewNavigationPanel(&sp),
		menu:           ui.NewMenu(),
		toastManager:   overlay.NewToastManager(&sp),
		appConfig: &config.Config{
			PhaseRoles: map[string]string{
				"implementing": "coder",
			},
			Profiles: map[string]config.AgentProfile{
				"coder": {
					Program:     "opencode",
					Model:       "claude-sonnet-4-6",
					Temperature: &coderTemp,
					Effort:      "medium",
					Enabled:     true,
				},
			},
		},
		instanceFinalizers: make(map[*session.Instance]func()),
	}

	plan := &taskparser.Plan{
		Waves: []taskparser.Wave{
			{Number: 1, Tasks: []taskparser.Task{{Number: 1, Title: "Task 1", Body: "do it"}}},
		},
	}
	orch := orchestration.NewWaveOrchestrator(planFile, plan)

	entry, ok := ps.Entry(planFile)
	require.True(t, ok)

	_, cmd := m.spawnWaveTasks(orch, plan.Waves[0].Tasks, entry)
	if cmd != nil {
		_ = cmd()
	}

	branch := gitpkg.TaskBranchFromFile(planFile)
	worktreePath := gitpkg.TaskWorktreePath(dir, branch)
	worktreeConfigPath := filepath.Join(worktreePath, "opencode.jsonc")

	data, err := os.ReadFile(worktreeConfigPath)
	require.NoError(t, err, "worktree opencode.jsonc must exist after shared worktree setup")

	var cfg map[string]any
	require.NoError(t, json.Unmarshal(data, &cfg))
	agentCfg, ok := cfg["agent"].(map[string]any)
	require.True(t, ok, "agent block must exist")
	coderCfg, ok := agentCfg["coder"].(map[string]any)
	require.True(t, ok, "coder block must exist")
	assert.Equal(t, "anthropic/claude-sonnet-4-6", coderCfg["model"], "worktree opencode.jsonc must have patched model")
	assert.InDelta(t, coderTemp, coderCfg["temperature"].(float64), 0.0001, "worktree opencode.jsonc must have patched temperature")
	assert.Equal(t, "medium", coderCfg["reasoningEffort"], "worktree opencode.jsonc must have patched reasoningEffort")
}

func TestExecuteContextAction_MergePlanPreflightStopsBeforeKillingInstances(t *testing.T) {
	dir := t.TempDir()
	runGit := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		out, err := cmd.CombinedOutput()
		require.NoErrorf(t, err, "git %v failed: %s", args, string(out))
	}

	runGit("init")
	runGit("config", "user.email", "test@test.com")
	runGit("config", "user.name", "Test")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "tracked.txt"), []byte("base\n"), 0o644))
	runGit("add", "tracked.txt")
	runGit("commit", "-m", "init")
	runGit("checkout", "-b", "plan/merge-guard")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "tracked.txt"), []byte("branch change\n"), 0o644))
	runGit("add", "tracked.txt")
	runGit("commit", "-m", "branch change")
	runGit("checkout", "-")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "tracked.txt"), []byte("dirty local change\n"), 0o644))

	plansDir := filepath.Join(dir, "docs", "plans")
	require.NoError(t, os.MkdirAll(plansDir, 0o755))
	ps, err := newTestPlanState(t, plansDir)
	require.NoError(t, err)

	const planFile = "merge-guard"
	require.NoError(t, ps.Register(planFile, "merge guard", "plan/merge-guard", time.Now()))
	seedPlanStatus(t, ps, planFile, taskstate.StatusReviewing)

	spin := spinner.New(spinner.WithSpinner(spinner.Dot))
	inst, err := session.NewInstance(session.InstanceOptions{
		Title:    "merge-guard-reviewer",
		Path:     dir,
		Program:  "opencode",
		TaskFile: planFile,
	})
	require.NoError(t, err)

	h := &home{
		taskState:      ps,
		taskStateDir:   plansDir,
		nav:            ui.NewNavigationPanel(&spin),
		menu:           ui.NewMenu(),
		tabbedWindow:   ui.NewTabbedWindow(ui.NewPreviewPane(), ui.NewInfoPane()),
		toastManager:   overlay.NewToastManager(&spin),
		overlays:       overlay.NewManager(),
		activeRepoPath: dir,
		allInstances:   []*session.Instance{inst},
	}

	h.updateSidebarTasks()
	require.True(t, h.nav.SelectByID(ui.SidebarPlanPrefix+planFile))

	model, cmd := h.executeContextAction("merge_plan")
	updated := model.(*home)
	require.Nil(t, cmd)
	require.Equal(t, stateConfirm, updated.state)
	require.NotNil(t, updated.pendingConfirmAction)

	msg := updated.pendingConfirmAction()
	mergeErr, ok := msg.(error)
	require.True(t, ok, "pending confirm action must return the preflight error, got %T", msg)
	assert.ErrorContains(t, mergeErr, "uncommitted changes overlap")
	assert.Len(t, updated.allInstances, 1, "preflight must stop before any bound instance is removed")

	entry, ok := updated.taskState.Entry(planFile)
	require.True(t, ok)
	assert.Equal(t, taskstate.StatusReviewing, entry.Status)
}

// TestFSM_PlanLifecycleStages verifies that the FSM produces the correct status for
// each stage in the plan lifecycle (plan→implement→review→done).
func TestFSM_PlanLifecycleStages(t *testing.T) {
	dir := t.TempDir()
	plansDir := filepath.Join(dir, "docs", "plans")
	if err := os.MkdirAll(plansDir, 0o755); err != nil {
		t.Fatal(err)
	}

	ps, err := newTestPlanState(t, plansDir)
	if err != nil {
		t.Fatal(err)
	}
	planFile := "test.md"
	if err := ps.Register(planFile, "test plan", "plan/test", time.Now()); err != nil {
		t.Fatal(err)
	}

	f := newFSMForTest(t, plansDir)

	stages := []struct {
		event      string
		wantStatus taskstate.Status
	}{
		{"plan_start", taskstate.StatusPlanning},
		{"planner_finished", taskstate.StatusReady},
		{"implement_start", taskstate.StatusImplementing},
		{"implement_finished", taskstate.StatusReviewing},
		{"review_approved", taskstate.StatusVerifying},
		{"verify_approved", taskstate.StatusDone},
	}

	for _, tc := range stages {
		if err := f.TransitionByName(planFile, tc.event); err != nil {
			t.Fatalf("TransitionByName(%q, %q) error: %v", planFile, tc.event, err)
		}
		reloaded, _ := newTestPlanState(t, plansDir)
		entry, ok := reloaded.Entry(planFile)
		if !ok {
			t.Fatalf("plan entry missing after %q", tc.event)
		}
		if entry.Status != tc.wantStatus {
			t.Errorf("after %q: got status %q, want %q", tc.event, entry.Status, tc.wantStatus)
		}
	}
}

func TestSpawnPlanAgent_SoloSetsSoloAgentFlag(t *testing.T) {
	dir := t.TempDir()

	for _, cmd := range [][]string{
		{"git", "init", dir},
		{"git", "-C", dir, "config", "user.email", "test@test.com"},
		{"git", "-C", dir, "config", "user.name", "Test"},
		{"git", "-C", dir, "commit", "--allow-empty", "-m", "init"},
	} {
		out, err := exec.Command(cmd[0], cmd[1:]...).CombinedOutput()
		if err != nil {
			t.Skipf("git setup failed (%v): %s", err, out)
		}
	}

	plansDir := filepath.Join(dir, "docs", "plans")
	require.NoError(t, os.MkdirAll(plansDir, 0o755))
	ps, err := newTestPlanState(t, plansDir)
	require.NoError(t, err)
	planFile := "test-solo.md"
	require.NoError(t, ps.Register(planFile, "test solo", "plan/test-solo", time.Now()))

	sp := spinner.New(spinner.WithSpinner(spinner.Dot))
	list := ui.NewNavigationPanel(&sp)
	h := &home{
		taskState:          ps,
		activeRepoPath:     dir,
		program:            "opencode",
		nav:                list,
		menu:               ui.NewMenu(),
		instanceFinalizers: make(map[*session.Instance]func()),
	}

	h.spawnTaskAgent(planFile, "solo", "solo prompt")

	instances := list.GetInstances()
	require.NotEmpty(t, instances, "expected instance after spawnTaskAgent(solo)")
	inst := instances[len(instances)-1]
	assert.True(t, inst.SoloAgent, "solo agent must have SoloAgent=true")
	assert.Equal(t, session.AgentTypeCoder, inst.AgentType)
}

func TestSpawnPlanAgent_SoloTitlesArePlanScoped(t *testing.T) {
	dir := t.TempDir()

	for _, cmd := range [][]string{
		{"git", "init", dir},
		{"git", "-C", dir, "config", "user.email", "test@test.com"},
		{"git", "-C", dir, "config", "user.name", "Test"},
		{"git", "-C", dir, "commit", "--allow-empty", "-m", "init"},
	} {
		out, err := exec.Command(cmd[0], cmd[1:]...).CombinedOutput()
		if err != nil {
			t.Skipf("git setup failed (%v): %s", err, out)
		}
	}

	plansDir := filepath.Join(dir, "docs", "plans")
	require.NoError(t, os.MkdirAll(plansDir, 0o755))
	ps, err := newTestPlanState(t, plansDir)
	require.NoError(t, err)

	const firstPlan = "wrong-timezone"
	const secondPlan = "rename-solo-agent-label"
	require.NoError(t, ps.Register(firstPlan, "wrong timezone", "plan/wrong-timezone", time.Now()))
	require.NoError(t, ps.Register(secondPlan, "rename solo agent label", "plan/rename-solo-agent-label", time.Now()))

	sp := spinner.New(spinner.WithSpinner(spinner.Dot))
	list := ui.NewNavigationPanel(&sp)
	h := &home{
		taskState:          ps,
		activeRepoPath:     dir,
		program:            "opencode",
		nav:                list,
		menu:               ui.NewMenu(),
		instanceFinalizers: make(map[*session.Instance]func()),
	}

	h.spawnTaskAgent(firstPlan, "solo", "first solo prompt")
	h.spawnTaskAgent(secondPlan, "solo", "second solo prompt")

	instances := list.GetInstances()
	require.Len(t, instances, 2, "expected two solo instances")

	assert.Equal(t, "wrong-timezone-solo", instances[0].Title)
	assert.Equal(t, "rename-solo-agent-label-solo", instances[1].Title)
	assert.NotEqual(t, instances[0].Title, instances[1].Title,
		"solo instance titles must be unique so tmux sessions do not collide")
}

// setupTopicConflictHome creates a home with two plans in the same topic,
// one already implementing, for testing the concurrency gate.
func setupTopicConflictHome(t *testing.T) (*home, string) {
	t.Helper()
	dir := t.TempDir()

	for _, cmd := range [][]string{
		{"git", "init", dir},
		{"git", "-C", dir, "config", "user.email", "test@test.com"},
		{"git", "-C", dir, "config", "user.name", "Test"},
		{"git", "-C", dir, "commit", "--allow-empty", "-m", "init"},
	} {
		out, err := exec.Command(cmd[0], cmd[1:]...).CombinedOutput()
		if err != nil {
			t.Skipf("git setup failed (%v): %s", err, out)
		}
	}

	plansDir := filepath.Join(dir, "docs", "plans")
	require.NoError(t, os.MkdirAll(plansDir, 0o755))

	ps, err := newTestPlanState(t, plansDir)
	require.NoError(t, err)

	const (
		targetPlan   = "solo-target.md"
		conflictPlan = "conflict.md"
		topic        = "shared-topic"
	)

	require.NoError(t, ps.Create(targetPlan, "target", "plan/solo-target", topic, time.Now()))
	require.NoError(t, ps.Create(conflictPlan, "conflict", "plan/conflict", topic, time.Now()))
	seedPlanStatus(t, ps, targetPlan, taskstate.StatusReady)
	seedPlanStatus(t, ps, conflictPlan, taskstate.StatusImplementing)

	h := waveFlowHome(t, ps, plansDir, make(map[string]*orchestration.WaveOrchestrator))
	h.fsm = newFSMForTest(t, plansDir).TaskStateMachine
	h.activeRepoPath = dir
	h.program = "opencode"
	return h, targetPlan
}

func TestTriggerPlanStage_SoloRespectsTopicConcurrencyGate(t *testing.T) {
	h, targetPlan := setupTopicConflictHome(t)

	model, _ := h.triggerTaskStage(targetPlan, "solo")
	updated := model.(*home)

	assert.Equal(t, stateConfirm, updated.state,
		"solo stage must show topic concurrency confirmation when another plan in topic is implementing")
	require.True(t, updated.overlays.IsActive(),
		"confirmation overlay must be shown for solo topic conflict")
	require.NotNil(t, updated.pendingConfirmAction,
		"confirm action must be set for solo topic conflict")
}

// TestTopicConcurrencyConfirm_ReturnsPlanStageConfirmedMsg verifies that
// confirming the topic-concurrency dialog returns a taskStageConfirmedMsg
// (not just a taskRefreshMsg), so the actual stage execution is triggered.
func TestTopicConcurrencyConfirm_ReturnsPlanStageConfirmedMsg(t *testing.T) {
	for _, stage := range []string{"solo", "implement"} {
		t.Run(stage, func(t *testing.T) {
			h, targetPlan := setupTopicConflictHome(t)

			model, _ := h.triggerTaskStage(targetPlan, stage)
			updated := model.(*home)

			require.Equal(t, stateConfirm, updated.state,
				"must show confirmation dialog for topic conflict")
			require.NotNil(t, updated.pendingConfirmAction,
				"pending confirm action must be set")

			// Execute the pending confirm action and check the returned message.
			msg := updated.pendingConfirmAction()
			stageMsg, ok := msg.(taskStageConfirmedMsg)
			require.True(t, ok,
				"confirm action must return taskStageConfirmedMsg, got %T", msg)
			assert.Equal(t, targetPlan, stageMsg.planFile,
				"taskStageConfirmedMsg must carry the correct plan file")
			assert.Equal(t, stage, stageMsg.stage,
				"taskStageConfirmedMsg must carry the correct stage")
		})
	}
}

func TestExecuteContextAction_SetStatusForceOverridesWithoutFSM(t *testing.T) {
	dir := t.TempDir()
	plansDir := filepath.Join(dir, "docs", "plans")
	require.NoError(t, os.MkdirAll(plansDir, 0o755))

	ps, err := newTestPlanState(t, plansDir)
	require.NoError(t, err)

	planFile := "test-set-status.md"
	require.NoError(t, ps.Register(planFile, "test set status", "plan/test-set-status", time.Now()))
	seedPlanStatus(t, ps, planFile, taskstate.StatusImplementing)

	sp := spinner.New(spinner.WithSpinner(spinner.Dot))
	h := &home{
		taskState:      ps,
		taskStateDir:   plansDir,
		fsm:            newFSMForTest(t, plansDir).TaskStateMachine,
		nav:            ui.NewNavigationPanel(&sp),
		menu:           ui.NewMenu(),
		tabbedWindow:   ui.NewTabbedWindow(ui.NewPreviewPane(), ui.NewInfoPane()),
		toastManager:   overlay.NewToastManager(&sp),
		overlays:       overlay.NewManager(),
		activeRepoPath: dir,
	}

	h.updateSidebarTasks()
	require.True(t, h.nav.SelectByID(ui.SidebarPlanPrefix+planFile))

	// Simulate: context menu selected "set_status", which sets up the picker
	_, _ = h.executeContextAction("set_status")
	assert.Equal(t, stateSetStatus, h.state, "set_status action should enter stateSetStatus")
	assert.True(t, h.overlays.IsActive(), "picker overlay should be created for status selection")
	assert.Equal(t, planFile, h.pendingSetStatusTask, "pending plan file should be stored")
}

func TestHandleKeyPress_SetStatusPlannedCreatesPlannedReadyForDaemonManagedRepo(t *testing.T) {
	dir := t.TempDir()
	plansDir := filepath.Join(dir, "docs", "plans")
	require.NoError(t, os.MkdirAll(plansDir, 0o755))

	oldManaged := repoManagedByDaemon
	repoManagedByDaemon = func(repoPath string) bool {
		return filepath.Clean(repoPath) == filepath.Clean(dir)
	}
	t.Cleanup(func() {
		repoManagedByDaemon = oldManaged
	})

	ps, err := newTestPlanState(t, plansDir)
	require.NoError(t, err)

	planFile := "test-set-status-planned.md"
	require.NoError(t, ps.Register(planFile, "test set status planned", "plan/test-set-status-planned", time.Now()))

	sp := spinner.New(spinner.WithSpinner(spinner.Dot))
	h := &home{
		taskState:      ps,
		taskStateDir:   plansDir,
		fsm:            newFSMForTest(t, plansDir).TaskStateMachine,
		nav:            ui.NewNavigationPanel(&sp),
		menu:           ui.NewMenu(),
		tabbedWindow:   ui.NewTabbedWindow(ui.NewPreviewPane(), ui.NewInfoPane()),
		toastManager:   overlay.NewToastManager(&sp),
		overlays:       overlay.NewManager(),
		activeRepoPath: dir,
	}

	h.updateSidebarTasks()
	require.True(t, h.nav.SelectByID(ui.SidebarPlanPrefix+planFile))

	_, _ = h.executeContextAction("set_status")
	require.Equal(t, stateSetStatus, h.state)

	model, _ := h.handleKeyPress(tea.KeyPressMsg{Text: taskstate.ManualOverridePlanned})
	updated := model.(*home)
	model, _ = updated.handleKeyPress(tea.KeyPressMsg{Code: tea.KeyEnter})
	updated = model.(*home)

	assert.Equal(t, stateDefault, updated.state)
	assert.False(t, updated.overlays.IsActive())
	assert.Empty(t, updated.pendingSetStatusTask)

	entry, ok := ps.Entry(planFile)
	require.True(t, ok)
	assert.True(t, taskstate.IsPlannedReady(entry))
	assert.False(t, taskstate.IsDraftReady(entry))
}

func TestExecuteTaskStage_BlocksWhenDaemonUnavailable(t *testing.T) {
	dir := t.TempDir()
	plansDir := filepath.Join(dir, "docs", "plans")
	require.NoError(t, os.MkdirAll(plansDir, 0o755))

	ps, err := newTestPlanState(t, plansDir)
	require.NoError(t, err)

	planFile := "daemon-block-plan.md"
	require.NoError(t, ps.Register(planFile, "daemon block", "plan/daemon-block-plan", time.Now()))

	sp := spinner.New(spinner.WithSpinner(spinner.Dot))
	h := &home{
		taskState:      ps,
		taskStateDir:   plansDir,
		fsm:            newFSMForTest(t, plansDir).TaskStateMachine,
		nav:            ui.NewNavigationPanel(&sp),
		menu:           ui.NewMenu(),
		tabbedWindow:   ui.NewTabbedWindow(ui.NewPreviewPane(), ui.NewInfoPane()),
		toastManager:   overlay.NewToastManager(&sp),
		overlays:       overlay.NewManager(),
		activeRepoPath: dir,
		daemonStatusChecker: func(string) daemonStatusMsg {
			return daemonStatusMsg{message: "start it with kas daemon start"}
		},
	}

	model, cmd := h.executeTaskStage(planFile, "plan")
	updated := model.(*home)

	require.Nil(t, cmd)
	assert.Equal(t, stateConfirm, updated.state)
	assert.True(t, updated.overlays.IsActive())
	assert.Nil(t, updated.pendingConfirmAction)

	entry, ok := updated.taskState.Entry(planFile)
	require.True(t, ok)
	assert.Equal(t, taskstate.StatusReady, entry.Status)
	co, ok := updated.overlays.Current().(*overlay.ConfirmationOverlay)
	require.True(t, ok)
	assert.Contains(t, co.View(), "kas daemon start")
}

func TestSpawnAdHocAgent_BlocksWhenDaemonUnavailable(t *testing.T) {
	spin := spinner.New(spinner.WithSpinner(spinner.Dot))
	h := &home{
		ctx:            context.Background(),
		state:          stateDefault,
		appConfig:      config.DefaultConfig(),
		nav:            ui.NewNavigationPanel(&spin),
		menu:           ui.NewMenu(),
		auditPane:      ui.NewAuditPane(),
		tabbedWindow:   ui.NewTabbedWindow(ui.NewPreviewPane(), ui.NewInfoPane()),
		toastManager:   overlay.NewToastManager(&spin),
		overlays:       overlay.NewManager(),
		activeRepoPath: t.TempDir(),
		program:        "opencode",
		daemonStatusChecker: func(string) daemonStatusMsg {
			return daemonStatusMsg{message: "register it with kas daemon add /tmp/repo"}
		},
	}

	model, cmd := h.spawnAdHocAgent("my-agent", "", "", "", session.ExecutionModeTmux, "")
	updated := model.(*home)

	require.Nil(t, cmd)
	assert.Empty(t, updated.nav.GetInstances())
	assert.Equal(t, stateConfirm, updated.state)
	assert.True(t, updated.overlays.IsActive())
	co, ok := updated.overlays.Current().(*overlay.ConfirmationOverlay)
	require.True(t, ok)
	assert.Contains(t, co.View(), "kas daemon add")
}

func TestMergeInstance_UsesSelectedInstanceTask(t *testing.T) {
	dir := t.TempDir()
	runGit := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		out, err := cmd.CombinedOutput()
		require.NoErrorf(t, err, "git %v failed: %s", args, string(out))
	}

	runGit("init")
	runGit("config", "user.email", "test@test.com")
	runGit("config", "user.name", "Test")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "tracked.txt"), []byte("base\n"), 0o644))
	runGit("add", "tracked.txt")
	runGit("commit", "-m", "init")
	runGit("checkout", "-b", "plan/merge-instance")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "tracked.txt"), []byte("branch change\n"), 0o644))
	runGit("add", "tracked.txt")
	runGit("commit", "-m", "branch change")
	runGit("checkout", "-")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "tracked.txt"), []byte("dirty local change\n"), 0o644))

	plansDir := filepath.Join(dir, "docs", "plans")
	require.NoError(t, os.MkdirAll(plansDir, 0o755))
	ps, err := newTestPlanState(t, plansDir)
	require.NoError(t, err)

	const planFile = "merge-instance"
	require.NoError(t, ps.Register(planFile, "merge instance", "plan/merge-instance", time.Now()))
	seedPlanStatus(t, ps, planFile, taskstate.StatusReviewing)

	spin := spinner.New(spinner.WithSpinner(spinner.Dot))
	inst, err := session.NewInstance(session.InstanceOptions{
		Title:     "merge-instance-reviewer",
		Path:      dir,
		Program:   "claude",
		TaskFile:  planFile,
		AgentType: session.AgentTypeReviewer,
	})
	require.NoError(t, err)

	h := &home{
		taskState:      ps,
		taskStateDir:   plansDir,
		nav:            ui.NewNavigationPanel(&spin),
		menu:           ui.NewMenu(),
		tabbedWindow:   ui.NewTabbedWindow(ui.NewPreviewPane(), ui.NewInfoPane()),
		toastManager:   overlay.NewToastManager(&spin),
		overlays:       overlay.NewManager(),
		activeRepoPath: dir,
		allInstances:   []*session.Instance{inst},
	}

	h.nav.AddInstance(inst)
	h.updateSidebarTasks()
	require.True(t, h.nav.SelectInstance(inst))

	model, cmd := h.executeContextAction("merge_instance")
	updated := model.(*home)
	require.Nil(t, cmd)
	require.Equal(t, stateConfirm, updated.state)
	require.NotNil(t, updated.pendingConfirmAction)

	msg := updated.pendingConfirmAction()
	mergeErr, ok := msg.(error)
	require.True(t, ok, "pending confirm action must return the preflight error, got %T", msg)
	assert.ErrorContains(t, mergeErr, "uncommitted changes overlap")
	assert.Len(t, updated.allInstances, 1, "preflight must stop before any bound instance is removed")

	entry, ok := updated.taskState.Entry(planFile)
	require.True(t, ok)
	assert.Equal(t, taskstate.StatusReviewing, entry.Status)
}

func TestToggleAutoAdvanceWaves(t *testing.T) {
	m := &home{
		appConfig: &config.Config{AutoAdvanceWaves: false},
	}
	assert.False(t, m.appConfig.AutoAdvanceWaves)

	// Simulate executing the toggle action
	m.appConfig.AutoAdvanceWaves = !m.appConfig.AutoAdvanceWaves
	assert.True(t, m.appConfig.AutoAdvanceWaves)

	// Toggle back
	m.appConfig.AutoAdvanceWaves = !m.appConfig.AutoAdvanceWaves
	assert.False(t, m.appConfig.AutoAdvanceWaves)
}

func TestExecuteContextAction_ToggleAutoAdvancePlanner(t *testing.T) {
	var sp spinner.Model
	m := &home{
		appConfig:    &config.Config{AutoAdvance: false, AutoAdvanceWaves: true},
		toastManager: overlay.NewToastManager(&sp),
	}

	model, cmd := m.executeContextAction("toggle_auto_advance_planner")
	updated := model.(*home)

	require.NotNil(t, cmd)
	assert.True(t, updated.appConfig.AutoAdvance)
	assert.True(t, updated.appConfig.AutoAdvanceWaves)
	assert.Contains(t, updated.toastManager.View(), "auto-advance planner: on")
}

func TestExecuteContextAction_ToggleAutoAdvance_LegacyAliasTargetsWaves(t *testing.T) {
	var sp spinner.Model
	m := &home{
		appConfig:    &config.Config{AutoAdvance: false, AutoAdvanceWaves: false},
		toastManager: overlay.NewToastManager(&sp),
	}

	model, cmd := m.executeContextAction("toggle_auto_advance")
	updated := model.(*home)

	require.NotNil(t, cmd)
	assert.False(t, updated.appConfig.AutoAdvance)
	assert.True(t, updated.appConfig.AutoAdvanceWaves)
	assert.Contains(t, updated.toastManager.View(), "auto-advance waves: on")
}

func TestToggleAutoReviewFix(t *testing.T) {
	m := &home{
		appConfig: &config.Config{AutoReviewFix: false},
	}
	assert.False(t, m.appConfig.AutoReviewFix)

	m.appConfig.AutoReviewFix = !m.appConfig.AutoReviewFix
	assert.True(t, m.appConfig.AutoReviewFix)

	m.appConfig.AutoReviewFix = !m.appConfig.AutoReviewFix
	assert.False(t, m.appConfig.AutoReviewFix)
}

func TestEnsureProcessor_RefreshesReviewFixConfig(t *testing.T) {
	store := taskstore.NewTestSQLiteStore(t)
	require.NoError(t, store.Create("proj", taskstore.TaskEntry{
		Filename: "disabled.md",
		Status:   taskstore.StatusReviewing,
	}))
	require.NoError(t, store.Create("proj", taskstore.TaskEntry{
		Filename: "enabled.md",
		Status:   taskstore.StatusReviewing,
	}))

	h := &home{
		appConfig:             &config.Config{AutoReviewFix: false, MaxReviewFixCycles: 2},
		taskStore:             store,
		taskStoreProject:      "proj",
		taskStateDir:          t.TempDir(),
		pendingReviewFeedback: make(map[string]string),
	}

	proc := h.ensureProcessor()
	require.NotNil(t, proc)
	actions := proc.ProcessFSMSignals([]taskfsm.Signal{{
		Event:    taskfsm.ReviewChangesRequested,
		TaskFile: "disabled.md",
		Body:     "fix this",
	}})
	require.Len(t, actions, 1)
	_, ok := actions[0].(loop.ReviewChangesAction)
	assert.True(t, ok)

	h.appConfig.AutoReviewFix = true
	h.appConfig.MaxReviewFixCycles = 4
	proc = h.ensureProcessor()
	actions = proc.ProcessFSMSignals([]taskfsm.Signal{{
		Event:    taskfsm.ReviewChangesRequested,
		TaskFile: "enabled.md",
		Body:     "fix this",
	}})

	var foundFixer, foundIncrement bool
	for _, action := range actions {
		if _, ok := action.(loop.SpawnFixerAction); ok {
			foundFixer = true
		}
		if _, ok := action.(loop.IncrementReviewCycleAction); ok {
			foundIncrement = true
		}
	}
	assert.True(t, foundFixer)
	assert.True(t, foundIncrement)
}

func TestStartFixer_UsesPersistedLatestReviewFeedback(t *testing.T) {
	store := taskstore.NewTestSQLiteStore(t)
	const planFile = "feature"
	const feedback = "Round 4 — changes required\n\n- [app/app.go:1603] keep the re-review loop stateful"
	require.NoError(t, store.Create("proj", taskstore.TaskEntry{
		Filename:             planFile,
		Status:               taskstore.StatusImplementing,
		Branch:               "plan/feature",
		ReviewCycle:          4,
		LatestReviewFeedback: feedback,
	}))

	ps, err := taskstate.Load(store, "proj", t.TempDir())
	require.NoError(t, err)

	sp := spinner.New(spinner.WithSpinner(spinner.Dot))
	nav := ui.NewNavigationPanel(&sp)
	nav.SetTopicsAndPlans(nil, []ui.PlanDisplay{{Filename: planFile, Status: string(taskstate.StatusImplementing)}}, nil)
	require.True(t, nav.SelectByID(ui.SidebarPlanPrefix+planFile))

	h := &home{
		ctx:                   context.Background(),
		state:                 stateDefault,
		appConfig:             config.DefaultConfig(),
		nav:                   nav,
		menu:                  ui.NewMenu(),
		tabbedWindow:          ui.NewTabbedWindow(ui.NewPreviewPane(), ui.NewInfoPane()),
		toastManager:          overlay.NewToastManager(&sp),
		overlays:              overlay.NewManager(),
		taskState:             ps,
		taskStore:             store,
		taskStoreProject:      "proj",
		pendingReviewFeedback: make(map[string]string),
		activeRepoPath:        t.TempDir(),
		program:               "claude",
	}

	_, _ = h.executeContextAction("start_fixer")

	var fixer *session.Instance
	for _, inst := range h.nav.GetInstances() {
		if inst.TaskFile == planFile && inst.AgentType == session.AgentTypeFixer {
			fixer = inst
			break
		}
	}
	require.NotNil(t, fixer, "manual start_fixer should spawn a fixer instance from implementing")
	assert.Contains(t, fixer.QueuedPrompt, feedback)
	assert.Contains(t, fixer.QueuedPrompt, "Current fix round: 4")
}

func TestAdvanceReviewCycle_CapturesFeedbackForDaemonManagedRepo(t *testing.T) {
	dir := t.TempDir()
	plansDir := filepath.Join(dir, "docs", "plans")
	require.NoError(t, os.MkdirAll(plansDir, 0o755))

	oldManaged := repoManagedByDaemon
	repoManagedByDaemon = func(repoPath string) bool {
		return filepath.Clean(repoPath) == filepath.Clean(dir)
	}
	t.Cleanup(func() {
		repoManagedByDaemon = oldManaged
	})

	ps, err := newTestPlanState(t, plansDir)
	require.NoError(t, err)
	const planFile = "feature"
	require.NoError(t, ps.Create(planFile, "feature", "plan/feature", "", time.Now()))
	for i := 0; i < 5; i++ {
		require.NoError(t, ps.IncrementReviewCycle(planFile))
	}

	h := newTestHome()
	h.taskState = ps
	h.taskStateDir = plansDir
	h.fsm = newPlanFSMForTest(t, plansDir)
	h.activeRepoPath = dir
	h.taskStoreProject = "test"
	h.pendingReviewFeedback = make(map[string]string)
	reviewer := &session.Instance{Title: "feature-review-6", Path: dir, Program: "opencode", TaskFile: planFile, AgentType: session.AgentTypeReviewer, ReviewCycle: 6, CachedContent: "round six findings"}
	h.nav.AddInstance(reviewer)
	h.updateSidebarTasks()
	h.nav.SelectInstance(reviewer)

	_, cmd := h.executeContextAction("advance_review_cycle")
	require.NotNil(t, cmd)
	require.IsType(t, overlay.ToastTickMsg{}, cmd())

	cycle, err := h.taskState.ReviewCycle(planFile)
	require.NoError(t, err)
	assert.Equal(t, 6, cycle)
	entry, ok := h.taskState.Entry(planFile)
	require.True(t, ok)
	assert.Equal(t, "round six findings", entry.LatestReviewFeedback)
}

func TestManualRecovery_NoSelectionNoOp(t *testing.T) {
	h := newTestHome()
	h.taskStoreProject = "test"

	assert.Nil(t, h.emitSelectedInstanceSignal(taskfsm.PlannerFinished, "planner finished signal queued"))

	_, cmd := h.executeContextAction("advance_review_cycle")
	assert.Nil(t, cmd)
}

func TestEmitSelectedInstanceSignal_QueuesExpectedGatewayRows(t *testing.T) {
	tests := []struct {
		name                  string
		status                taskstate.Status
		executionPhase        string // persisted execution phase required for FSM gating
		executionAgentType    string // required alongside executionPhase by normalizeExecutionState
		agentType             string
		event                 taskfsm.Event
		successToast          string
		cachedContent         string
		wantSignalType        string
		wantPayloadBody       string
		wantPersistedFeedback string
	}{
		{
			name:           "planner finished",
			status:         taskstate.StatusPlanning,
			agentType:      session.AgentTypePlanner,
			event:          taskfsm.PlannerFinished,
			successToast:   "planner finished signal queued",
			wantSignalType: "planner_finished",
		},
		{
			name:               "architect finished",
			status:             taskstate.StatusImplementing,
			executionPhase:     string(taskfsm.ExecutionPhaseArchitecting),
			executionAgentType: session.AgentTypeElaborator,
			agentType:          session.AgentTypeElaborator,
			event:              taskfsm.ArchitectFinished,
			successToast:       "architect pass finished signal queued",
			wantSignalType:     "elaborator_finished",
		},
		{
			name:               "implement finished",
			status:             taskstate.StatusImplementing,
			executionPhase:     string(taskfsm.ExecutionPhaseSingleAgentImplementing),
			executionAgentType: session.AgentTypeCoder,
			agentType:          session.AgentTypeCoder,
			event:              taskfsm.ImplementFinished,
			successToast:       "implement finished signal queued",
			wantSignalType:     "implement_finished",
		},
		{
			name:                  "review approved",
			status:                taskstate.StatusReviewing,
			agentType:             session.AgentTypeReviewer,
			event:                 taskfsm.ReviewApproved,
			successToast:          "review approved signal queued",
			cachedContent:         "ship it",
			wantSignalType:        "review_approved",
			wantPayloadBody:       "ship it",
			wantPersistedFeedback: "ship it",
		},
		{
			name:                  "review changes requested",
			status:                taskstate.StatusReviewing,
			agentType:             session.AgentTypeReviewer,
			event:                 taskfsm.ReviewChangesRequested,
			successToast:          "review changes requested signal queued",
			cachedContent:         "please fix the failing edge case",
			wantSignalType:        "review_changes_requested",
			wantPayloadBody:       "please fix the failing edge case",
			wantPersistedFeedback: "please fix the failing edge case",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("HOME", t.TempDir())
			dir := t.TempDir()
			t.Chdir(dir)
			plansDir := filepath.Join(dir, "docs", "plans")
			require.NoError(t, os.MkdirAll(plansDir, 0o755))
			ps, err := newTestPlanState(t, plansDir)
			require.NoError(t, err)

			const planFile = "feature"
			require.NoError(t, ps.Create(planFile, "feature", "plan/feature", "", time.Now()))
			if tt.executionPhase != "" {
				require.NoError(t, ps.ForceSetLifecycle(planFile, tt.status, taskstore.ExecutionState{
					Phase:           tt.executionPhase,
					ActiveAgentType: tt.executionAgentType,
				}))
			} else {
				require.NoError(t, ps.ForceSetStatus(planFile, tt.status))
			}

			h := newTestHome()
			h.taskState = ps
			h.taskStateDir = plansDir
			h.taskStoreProject = "test"
			h.pendingReviewFeedback = make(map[string]string)
			reviewer := &session.Instance{
				Title:         tt.name,
				Path:          dir,
				Program:       "opencode",
				TaskFile:      planFile,
				AgentType:     tt.agentType,
				CachedContent: tt.cachedContent,
			}
			h.nav.AddInstance(reviewer)
			h.updateSidebarTasks()
			h.nav.SelectInstance(reviewer)

			cmd := h.emitSelectedInstanceSignal(tt.event, tt.successToast)
			require.NotNil(t, cmd)
			msg := cmd()
			result, ok := msg.(manualSignalResultMsg)
			require.True(t, ok)
			require.NoError(t, result.err)
			assert.Equal(t, tt.wantSignalType, result.signalType)
			assert.Equal(t, tt.successToast, result.successToast)

			signals := listPendingAuthoritativeSignals(t, "test")
			require.Len(t, signals, 1)
			assert.Equal(t, planFile, signals[0].PlanFile)
			assert.Equal(t, tt.wantSignalType, signals[0].SignalType)
			assert.Equal(t, tt.wantPayloadBody, decodeSignalPayloadBody(t, signals[0].Payload))

			entry, exists := h.taskState.Entry(planFile)
			require.True(t, exists)
			assert.Equal(t, tt.wantPersistedFeedback, entry.LatestReviewFeedback)

			model, updateCmd := h.Update(msg)
			updated := model.(*home)
			require.NotNil(t, updateCmd)
			assert.Contains(t, updated.toastManager.View(), tt.successToast)
		})
	}
}

func TestEmitSelectedInstanceSignal_RejectsPlannerFinishedWithoutWaveHeaders(t *testing.T) {
	dir := t.TempDir()
	plansDir := filepath.Join(dir, "docs", "plans")
	require.NoError(t, os.MkdirAll(plansDir, 0o755))

	store, ps, _ := newSharedStoreForTest(t, plansDir)
	const planFile = "needs-waves"
	require.NoError(t, ps.Register(planFile, "needs waves", "plan/needs-waves", time.Now()))
	seedPlanStatus(t, ps, planFile, taskstate.StatusPlanning)
	require.NoError(t, store.SetContent("test", planFile, "# Plan\n\n### Task 1: Missing waves\n\nDo it.\n"))

	h := newTestHome()
	h.taskState = ps
	h.taskStateDir = plansDir
	h.taskStore = store
	h.taskStoreProject = "test"
	h.fsm = newPlanFSMForTest(t, plansDir)
	h.activeRepoPath = dir
	h.pendingReviewFeedback = make(map[string]string)
	planner := &session.Instance{Title: "needs-waves-plan", Path: dir, Program: "opencode", TaskFile: planFile, AgentType: session.AgentTypePlanner}

	_, _, err := h.prepareSelectedInstanceSignal(planner, taskfsm.PlannerFinished)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not implementation-ready")
	assert.Contains(t, err.Error(), "no wave headers found")
}

func listPendingAuthoritativeSignals(t *testing.T, project string) []taskstore.SignalEntry {
	t.Helper()
	gw, err := taskstore.OpenAuthoritativeSignalGateway(project)
	require.NoError(t, err)
	t.Cleanup(func() { _ = gw.Close() })
	signals, err := gw.List(project, taskstore.SignalPending)
	require.NoError(t, err)
	return signals
}

func decodeSignalPayloadBody(t *testing.T, payload string) string {
	t.Helper()
	if payload == "" {
		return ""
	}
	var body struct {
		Body string `json:"body"`
	}
	require.NoError(t, json.Unmarshal([]byte(payload), &body))
	return body.Body
}

func TestMarkReviewChangesRequested_QueuesGatewaySignal(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	dir := t.TempDir()
	t.Chdir(dir)
	plansDir := filepath.Join(dir, "docs", "plans")
	require.NoError(t, os.MkdirAll(plansDir, 0o755))
	ps, err := newTestPlanState(t, plansDir)
	require.NoError(t, err)
	const planFile = "feature"
	require.NoError(t, ps.Create(planFile, "feature", "plan/feature", "", time.Now()))
	// The reviewer signal is only offered when the task is in reviewing state.
	require.NoError(t, ps.ForceSetStatus(planFile, taskstate.StatusReviewing))

	h := newTestHome()
	h.taskState = ps
	h.taskStateDir = plansDir
	h.fsm = newPlanFSMForTest(t, plansDir)
	h.activeRepoPath = dir
	h.taskStoreProject = "test"
	h.pendingReviewFeedback = make(map[string]string)
	reviewer := &session.Instance{Title: "feature-review-1", Path: dir, Program: "opencode", TaskFile: planFile, AgentType: session.AgentTypeReviewer, ReviewCycle: 1, CachedContent: "review findings"}
	h.nav.AddInstance(reviewer)
	h.updateSidebarTasks()
	h.nav.SelectInstance(reviewer)

	_, cmd := h.executeContextAction("mark_review_changes_requested")
	require.NotNil(t, cmd)
	msg := cmd()
	result, ok := msg.(manualSignalResultMsg)
	require.True(t, ok)
	require.NoError(t, result.err)

	signals := listPendingAuthoritativeSignals(t, "test")
	require.Len(t, signals, 1)
	assert.Equal(t, "review_changes_requested", signals[0].SignalType)
	assert.Contains(t, signals[0].Payload, "review findings")
}

func TestViewSelectedPlan_ReadsFromStore(t *testing.T) {
	store := taskstore.NewTestSQLiteStore(t)
	planFile := "test.md"
	content := "# My Plan\n\n## Wave 1\n\n### Task 1: Do thing\n"
	require.NoError(t, store.Create("proj", taskstore.TaskEntry{
		Filename: planFile,
		Status:   taskstore.StatusReady,
		Content:  content,
	}))

	ps, err := taskstate.Load(store, "proj", t.TempDir())
	require.NoError(t, err)

	sp := spinner.New(spinner.WithSpinner(spinner.Dot))
	nav := ui.NewNavigationPanel(&sp)
	nav.SetTopicsAndPlans(nil, []ui.PlanDisplay{{Filename: planFile, Status: string(taskstate.StatusReady)}}, nil)
	require.True(t, nav.SelectByID(ui.SidebarPlanPrefix+planFile))

	h := &home{
		taskState:        ps,
		taskStore:        store,
		taskStoreProject: "proj",
		taskStateDir:     t.TempDir(),
		nav:              nav,
		tabbedWindow:     ui.NewTabbedWindow(ui.NewPreviewPane(), ui.NewInfoPane()),
	}

	_, cmd := h.viewSelectedPlan()
	require.NotNil(t, cmd)

	msg := cmd()
	renderedMsg, ok := msg.(planRenderedMsg)
	require.True(t, ok, "expected planRenderedMsg, got %T", msg)
	require.NoError(t, renderedMsg.err)
	assert.Equal(t, planFile, renderedMsg.planFile)
}

func TestLoadTaskState_InvalidatesCachedRenderedPlan(t *testing.T) {
	store := taskstore.NewTestSQLiteStore(t)
	planFile := "test.md"
	content := "# Draft\n\nInitial body\n"
	require.NoError(t, store.Create("proj", taskstore.TaskEntry{
		Filename: planFile,
		Status:   taskstore.StatusReady,
		Content:  content,
	}))

	plansDir := t.TempDir()
	ps, err := taskstate.Load(store, "proj", plansDir)
	require.NoError(t, err)

	sp := spinner.New(spinner.WithSpinner(spinner.Dot))
	nav := ui.NewNavigationPanel(&sp)
	nav.SetTopicsAndPlans(nil, []ui.PlanDisplay{{Filename: planFile, Status: string(taskstate.StatusReady)}}, nil)
	require.True(t, nav.SelectByID(ui.SidebarPlanPrefix+planFile))

	h := &home{
		taskState:          ps,
		taskStore:          store,
		taskStoreProject:   "proj",
		taskStateDir:       plansDir,
		nav:                nav,
		tabbedWindow:       ui.NewTabbedWindow(ui.NewPreviewPane(), ui.NewInfoPane()),
		cachedPlanFile:     planFile,
		cachedPlanRendered: "stale rendered content",
	}

	require.NoError(t, store.SetContent("proj", planFile, "# Updated\n\n## Wave 1\n\n### Task 1: Fresh\n"))
	h.loadTaskState()

	assert.Empty(t, h.cachedPlanFile)
	assert.Empty(t, h.cachedPlanRendered)

	_, cmd := h.viewSelectedPlan()
	require.NotNil(t, cmd, "viewSelectedPlan must re-read store content after task state reload invalidates cache")

	msg := cmd()
	renderedMsg, ok := msg.(planRenderedMsg)
	require.True(t, ok, "expected planRenderedMsg, got %T", msg)
	require.NoError(t, renderedMsg.err)
	assert.Equal(t, planFile, renderedMsg.planFile)
	ansiRE := regexp.MustCompile(`\x1b\[[0-9;]*m`)
	plain := ansiRE.ReplaceAllString(renderedMsg.rendered, "")
	assert.Contains(t, plain, "Updated")
	assert.Contains(t, plain, "Wave 1")
}

// TestImplementActionReadsFromStore verifies that the "implement" action reads plan
// content from the task store database, not from a file on disk. The test creates
// a task entry with valid wave-header content in the task store and deliberately omits
// any .md file on disk. A non-nil WaveOrchestrator in the home model after
// executeTaskStage proves that the plan was read from the DB and parsed successfully.
func TestImplementActionReadsFromStore(t *testing.T) {
	dir := t.TempDir()
	plansDir := filepath.Join(dir, "docs", "plans")
	require.NoError(t, os.MkdirAll(plansDir, 0o755))
	threshold := 0

	const planFile = "test-implement-from-db.md"
	const planContent = "# Plan\n\n**Goal:** Test DB read\n\n## Wave 1\n\n### Task 1: Do the thing\n\nDo it.\n"

	// Create task in store WITH content and a branch (branch avoids the backfill
	// path in executeTaskStage that would call store.Update and inadvertently clear
	// the content field). No file is written to disk.
	store := taskstore.NewTestSQLiteStore(t)
	require.NoError(t, store.Create("proj", taskstore.TaskEntry{
		Filename: planFile,
		Status:   taskstore.StatusReady,
		Branch:   "plan/test-implement-from-db",
		Content:  planContent,
	}))

	ps, err := taskstate.Load(store, "proj", plansDir)
	require.NoError(t, err)
	fsm := taskfsm.New(store, "proj", plansDir)

	sp := spinner.New(spinner.WithSpinner(spinner.Dot))
	h := &home{
		appConfig:          &config.Config{BlueprintSkipThresholdValue: &threshold},
		taskState:          ps,
		taskStore:          store,
		taskStoreProject:   "proj",
		taskStateDir:       plansDir,
		fsm:                fsm,
		nav:                ui.NewNavigationPanel(&sp),
		menu:               ui.NewMenu(),
		tabbedWindow:       ui.NewTabbedWindow(ui.NewPreviewPane(), ui.NewInfoPane()),
		toastManager:       overlay.NewToastManager(&sp),
		waveOrchestrators:  make(map[string]*orchestration.WaveOrchestrator),
		instanceFinalizers: make(map[*session.Instance]func()),
		activeRepoPath:     dir,
		program:            "opencode",
	}

	// No plan file on disk — content must come from the task store.
	model, _ := h.executeTaskStage(planFile, "implement")
	updated := model.(*home)

	// The WaveOrchestrator is created before spawnWaveTasks (which may fail on git).
	// A non-nil orchestrator proves the plan was read from the DB and parsed successfully.
	assert.NotNil(t, updated.waveOrchestrators[planFile],
		"implement action must read plan content from store, not disk")
}

// TestSoloActionChecksStoreNotDisk verifies that the "solo" action determines
// whether to include a plan file reference in its prompt by checking for content
// in the task store, rather than checking for a file on disk. The test stores
// content in the DB without writing any .md file. When the prompt contains
// "kas task show <planFile>", it proves the store check (not os.Stat) was used.
func TestSoloActionChecksStoreNotDisk(t *testing.T) {
	dir := t.TempDir()
	plansDir := filepath.Join(dir, "docs", "plans")
	require.NoError(t, os.MkdirAll(plansDir, 0o755))

	const planFile = "test-solo-from-db.md"
	const planContent = "# Plan\n\n**Goal:** Test solo DB check\n\n## Wave 1\n\n### Task 1: Solo task\n\nDo it.\n"

	// Create task in store WITH content and a branch (branch avoids the backfill
	// path in executeTaskStage that would call store.Update and inadvertently clear
	// the content field). No file is written to disk.
	store := taskstore.NewTestSQLiteStore(t)
	require.NoError(t, store.Create("proj", taskstore.TaskEntry{
		Filename: planFile,
		Status:   taskstore.StatusReady,
		Branch:   "plan/test-solo-from-db",
		Content:  planContent,
		// planned-ready so fsmSetImplementing accepts it when the solo stage runs.
		ExecutionState: taskstore.ExecutionState{Phase: "planned"},
	}))

	ps, err := taskstate.Load(store, "proj", plansDir)
	require.NoError(t, err)
	fsm := taskfsm.New(store, "proj", plansDir)

	sp := spinner.New(spinner.WithSpinner(spinner.Dot))
	h := &home{
		taskState:          ps,
		taskStore:          store,
		taskStoreProject:   "proj",
		taskStateDir:       plansDir,
		fsm:                fsm,
		nav:                ui.NewNavigationPanel(&sp),
		menu:               ui.NewMenu(),
		tabbedWindow:       ui.NewTabbedWindow(ui.NewPreviewPane(), ui.NewInfoPane()),
		toastManager:       overlay.NewToastManager(&sp),
		waveOrchestrators:  make(map[string]*orchestration.WaveOrchestrator),
		instanceFinalizers: make(map[*session.Instance]func()),
		activeRepoPath:     dir,
		program:            "opencode",
	}

	// No plan file on disk — the store check must find content and set refFile.
	model, _ := h.executeTaskStage(planFile, "solo")
	updated := model.(*home)

	// The solo agent must have been spawned with a prompt referencing kas task show <planFile>
	// because the store has content. If os.Stat were used instead, no disk file means
	// refFile="" and the prompt would omit the plan file reference.
	instances := updated.nav.GetInstances()
	require.NotEmpty(t, instances, "solo stage must spawn an agent instance")
	soloInst := instances[len(instances)-1]
	assert.Contains(t, soloInst.QueuedPrompt, "kas task show "+planFile,
		"solo prompt must reference plan file when store has content (not disk)")
}

func TestExecuteContextAction_MarkPlanDoneFromReadyTransitionsToDone(t *testing.T) {
	dir := t.TempDir()
	plansDir := filepath.Join(dir, "docs", "plans")
	require.NoError(t, os.MkdirAll(plansDir, 0o755))

	ps, err := newTestPlanState(t, plansDir)
	require.NoError(t, err)

	planFile := "review-approval-gate.md"
	require.NoError(t, ps.Register(planFile, "review approval gate", "plan/review-approval-gate", time.Now()))
	seedPlanStatus(t, ps, planFile, taskstate.StatusReady)
	// planned-ready so mark_plan_done can walk ready→implementing→reviewing→done.
	require.NoError(t, ps.SetExecutionState(planFile, taskstore.ExecutionState{Phase: "planned"}))

	sp := spinner.New(spinner.WithSpinner(spinner.Dot))
	h := &home{
		taskState:      ps,
		taskStateDir:   plansDir,
		fsm:            newFSMForTest(t, plansDir).TaskStateMachine,
		nav:            ui.NewNavigationPanel(&sp),
		menu:           ui.NewMenu(),
		tabbedWindow:   ui.NewTabbedWindow(ui.NewPreviewPane(), ui.NewInfoPane()),
		toastManager:   overlay.NewToastManager(&sp),
		activeRepoPath: dir,
	}

	h.updateSidebarTasks()
	require.True(t, h.nav.SelectByID(ui.SidebarPlanPrefix+planFile), "plan should be selectable in sidebar")

	_, _ = h.executeContextAction("mark_plan_done")

	reloaded, err := newTestPlanState(t, plansDir)
	require.NoError(t, err)
	entry, ok := reloaded.Entry(planFile)
	require.True(t, ok)
	assert.Equal(t, taskstate.StatusDone, entry.Status,
		"mark_plan_done should walk ready->implementing->reviewing->done")
}

func TestExecuteTaskStage_Implement_ReusesPersistedArchitectingState(t *testing.T) {
	dir := t.TempDir()
	plansDir := filepath.Join(dir, "docs", "plans")
	require.NoError(t, os.MkdirAll(plansDir, 0o755))

	const planFile = "architecting-from-state"
	const planContent = "# Plan\n\n**Goal:** reuse architect state\n\n## Wave 1\n\n### Task 1: First\n\nDo it.\n"

	store := taskstore.NewTestSQLiteStore(t)
	require.NoError(t, store.Create("proj", taskstore.TaskEntry{
		Filename: planFile,
		Status:   taskstore.StatusImplementing,
		Branch:   "plan/architecting-from-state",
		Content:  planContent,
		ExecutionState: taskstore.ExecutionState{
			Phase:           string(taskfsm.ExecutionPhaseArchitecting),
			ActiveAgentType: session.AgentTypeElaborator,
		},
	}))

	ps, err := taskstate.Load(store, "proj", plansDir)
	require.NoError(t, err)
	threshold := 0

	sp := spinner.New(spinner.WithSpinner(spinner.Dot))
	h := &home{
		appConfig:          &config.Config{BlueprintSkipThresholdValue: &threshold},
		taskState:          ps,
		taskStore:          store,
		taskStoreProject:   "proj",
		taskStateDir:       plansDir,
		fsm:                taskfsm.New(store, "proj", plansDir),
		nav:                ui.NewNavigationPanel(&sp),
		menu:               ui.NewMenu(),
		tabbedWindow:       ui.NewTabbedWindow(ui.NewPreviewPane(), ui.NewInfoPane()),
		toastManager:       overlay.NewToastManager(&sp),
		waveOrchestrators:  make(map[string]*orchestration.WaveOrchestrator),
		instanceFinalizers: make(map[*session.Instance]func()),
		activeRepoPath:     dir,
		program:            "opencode",
	}

	model, _ := h.executeTaskStage(planFile, "implement")
	updated := model.(*home)
	orch, ok := updated.waveOrchestrators[planFile]
	require.True(t, ok)
	assert.Equal(t, orchestration.WaveStateElaborating, orch.State())
	assert.Empty(t, updated.nav.GetInstances(), "persisted architecting state must not spawn a duplicate architect")
	entry, err := store.Get("proj", planFile)
	require.NoError(t, err)
	assert.Equal(t, taskstore.ExecutionState{Phase: string(taskfsm.ExecutionPhaseArchitecting), ActiveAgentType: session.AgentTypeElaborator}, entry.ExecutionState)
}

// ── readiness review TUI wiring ──────────────────────────────────────────────

func TestEnsureProcessor_RefreshesReadinessReviewConfig(t *testing.T) {
	store := taskstore.NewTestSQLiteStore(t)
	require.NoError(t, store.Create("proj", taskstore.TaskEntry{
		Filename: "feature.md",
		Status:   taskstore.StatusReviewing,
	}))

	h := &home{
		appConfig:             &config.Config{AutoReadinessReview: false},
		taskStore:             store,
		taskStoreProject:      "proj",
		taskStateDir:          t.TempDir(),
		pendingReviewFeedback: make(map[string]string),
	}

	proc := h.ensureProcessor()
	require.NotNil(t, proc)

	// Simulate a reviewer approval with AutoReadinessReview disabled — must pass through.
	require.NoError(t, store.Create("proj", taskstore.TaskEntry{
		Filename: "alpha.md",
		Status:   taskstore.StatusReviewing,
	}))
	actions := proc.ProcessFSMSignals([]taskfsm.Signal{{
		Event:    taskfsm.ReviewApproved,
		TaskFile: "alpha.md",
	}})
	_, isApproved := findAction[loop.ReviewApprovedAction](actions)
	assert.True(t, isApproved, "with readiness disabled, ReviewApproved should pass through")

	// Enable readiness review and refresh the processor.
	h.appConfig.AutoReadinessReview = true
	proc = h.ensureProcessor()

	require.NoError(t, store.Create("proj", taskstore.TaskEntry{
		Filename: "beta.md",
		Status:   taskstore.StatusReviewing,
	}))
	actions = proc.ProcessFSMSignals([]taskfsm.Signal{{
		Event:    taskfsm.ReviewApproved,
		TaskFile: "beta.md",
	}})
	_, isSpawnMaster := findAction[loop.SpawnMasterAction](actions)
	assert.True(t, isSpawnMaster, "with readiness enabled, ReviewApproved should produce SpawnMasterAction")
}

// findAction is a generic helper that searches a slice of loop.Action for a value of type T.
func findAction[T loop.Action](actions []loop.Action) (T, bool) {
	for _, a := range actions {
		if v, ok := a.(T); ok {
			return v, true
		}
	}
	var zero T
	return zero, false
}

func TestTaskLifecycleItems_StartReviewAvailableDuringReviewing(t *testing.T) {
	// In the new FSM, StatusReviewing tasks always offer "start_review".
	// Verification is tracked by StatusVerifying (a separate FSM status).
	reviewingEntry := taskstate.TaskEntry{
		Status: taskstate.StatusReviewing,
		ExecutionState: taskstore.ExecutionState{
			ActiveAgentType: session.AgentTypeReviewer,
		},
	}

	items := taskLifecycleItems(reviewingEntry)
	var actions []string
	for _, item := range items {
		actions = append(actions, item.Action)
	}
	assert.Contains(t, actions, "start_review",
		"start_review must be available for reviewing tasks in the new FSM")
}

func TestInstanceSignalItems_MasterAgent_HasReadinessSignals(t *testing.T) {
	inst := &session.Instance{
		Title:     "my-plan-verify-1",
		TaskFile:  "my-plan.md",
		AgentType: session.AgentTypeMaster,
	}
	entry := taskstate.TaskEntry{Status: taskstate.StatusVerifying}

	items := instanceSignalItems(inst, entry, true)

	var actions []string
	for _, item := range items {
		actions = append(actions, item.Action)
	}
	assert.Contains(t, actions, "mark_verify_approved", "master instance must offer mark_verify_approved action")
	assert.Contains(t, actions, "mark_verify_failed", "master instance must offer mark_verify_failed action")
}

func TestExecuteContextAction_MarkReadinessApproved(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	dir := t.TempDir()
	t.Chdir(dir)
	plansDir := filepath.Join(dir, "docs", "plans")
	require.NoError(t, os.MkdirAll(plansDir, 0o755))
	ps, err := newTestPlanState(t, plansDir)
	require.NoError(t, err)
	const planFile = "feature"
	require.NoError(t, ps.Create(planFile, "feature", "plan/feature", "", time.Now()))
	require.NoError(t, ps.ForceSetLifecycle(planFile, taskstate.StatusVerifying, taskstore.ExecutionState{
		ActiveAgentType: session.AgentTypeMaster,
	}))

	h := newTestHome()
	h.taskState = ps
	h.taskStateDir = plansDir
	h.activeRepoPath = dir
	h.taskStoreProject = "test"
	h.pendingReviewFeedback = make(map[string]string)
	master := &session.Instance{
		Title:     "my-plan-verify-1",
		Path:      dir,
		Program:   "opencode",
		TaskFile:  planFile,
		AgentType: session.AgentTypeMaster,
	}
	h.nav.AddInstance(master)
	h.updateSidebarTasks()
	h.nav.SelectInstance(master)

	_, cmd := h.executeContextAction("mark_verify_approved")
	require.NotNil(t, cmd)
	msg := cmd()
	result, ok := msg.(manualSignalResultMsg)
	require.True(t, ok)
	require.NoError(t, result.err)

	signals := listPendingAuthoritativeSignals(t, "test")
	require.Len(t, signals, 1)
	assert.Equal(t, "verify_approved", signals[0].SignalType)
}

func TestExecuteContextAction_MarkReadinessChangesRequested(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	dir := t.TempDir()
	t.Chdir(dir)
	plansDir := filepath.Join(dir, "docs", "plans")
	require.NoError(t, os.MkdirAll(plansDir, 0o755))
	ps, err := newTestPlanState(t, plansDir)
	require.NoError(t, err)
	const planFile = "feature"
	require.NoError(t, ps.Create(planFile, "feature", "plan/feature", "", time.Now()))
	require.NoError(t, ps.ForceSetLifecycle(planFile, taskstate.StatusVerifying, taskstore.ExecutionState{
		ActiveAgentType: session.AgentTypeMaster,
	}))

	h := newTestHome()
	h.taskState = ps
	h.taskStateDir = plansDir
	h.activeRepoPath = dir
	h.taskStoreProject = "test"
	h.pendingReviewFeedback = make(map[string]string)
	master := &session.Instance{
		Title:     "my-plan-verify-1",
		Path:      dir,
		Program:   "opencode",
		TaskFile:  planFile,
		AgentType: session.AgentTypeMaster,
	}
	h.nav.AddInstance(master)
	h.updateSidebarTasks()
	h.nav.SelectInstance(master)

	_, cmd := h.executeContextAction("mark_verify_failed")
	require.NotNil(t, cmd)
	msg := cmd()
	result, ok := msg.(manualSignalResultMsg)
	require.True(t, ok)
	require.NoError(t, result.err)

	signals := listPendingAuthoritativeSignals(t, "test")
	require.Len(t, signals, 1)
	assert.Equal(t, "verify_failed", signals[0].SignalType)
}

// TestEmitSelectedInstanceSignal_RejectsStaleLifecycleState verifies that
// emitSelectedInstanceSignal returns a lifecycleActionRejectedMsg when the
// task's FSM state has drifted since the menu was opened.
//
// Scenario: the reviewer instance sees StatusReviewing at menu-build time.
// Before the signal is emitted, the daemon regresses the task back to
// StatusImplementing (e.g. a manual intervention).  The stale guard must
// detect this and cancel the signal without writing a gateway row.
func TestEmitSelectedInstanceSignal_RejectsStaleLifecycleState(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	dir := t.TempDir()
	t.Chdir(dir)
	plansDir := filepath.Join(dir, "docs", "plans")
	require.NoError(t, os.MkdirAll(plansDir, 0o755))

	store, ps, fsm := newSharedStoreForTest(t, plansDir)
	const planFile = "stale-signal"
	require.NoError(t, ps.Create(planFile, "stale signal", "plan/stale-signal", "", time.Now()))
	// Put the task into reviewing so mark_review_approved is a valid action.
	require.NoError(t, ps.ForceSetStatus(planFile, taskstate.StatusReviewing))

	h := newTestHome()
	h.taskState = ps
	h.taskStore = store
	h.taskStateDir = plansDir
	h.activeRepoPath = dir
	h.taskStoreProject = "test"
	h.fsm = fsm
	h.pendingReviewFeedback = make(map[string]string)

	reviewer := &session.Instance{
		Title:     "stale-signal-review-1",
		Path:      dir,
		Program:   "opencode",
		TaskFile:  planFile,
		AgentType: session.AgentTypeReviewer,
	}
	h.nav.AddInstance(reviewer)
	h.updateSidebarTasks()
	h.nav.SelectInstance(reviewer)

	// Simulate the daemon advancing the task back to implementing while the
	// menu was open (state drift).
	require.NoError(t, ps.ForceSetStatus(planFile, taskstate.StatusImplementing))

	// emitSelectedInstanceSignal must detect the stale state and reject with
	// a lifecycleActionRejectedMsg — no error toast, no gateway row.
	cmd := h.emitSelectedInstanceSignal(taskfsm.ReviewApproved, "review approved signal queued")
	require.NotNil(t, cmd, "stale signal must return a non-nil Cmd (the rejection Cmd)")
	msg := cmd()

	rejected, ok := msg.(lifecycleActionRejectedMsg)
	require.True(t, ok, "expected lifecycleActionRejectedMsg, got %T: %v", msg, msg)
	assert.Contains(t, rejected.message, "task state changed",
		"rejection message must describe the state-change reason")

	// No gateway row must have been written.
	signals := listPendingAuthoritativeSignals(t, "test")
	assert.Empty(t, signals, "stale signal rejection must not write any gateway rows")
}

// TestExecuteContextAction_StartReviewRejectsStaleState verifies that
// start_review does not spawn a reviewer agent when the task has advanced to a
// terminal state (Done) between menu-open and action execution.
//
// The guard is exercised through the FSM: refreshTaskEntry reloads the DB row,
// and fsmSetReviewing fails because Done→implementing is not an allowed
// transition, so no agent is ever created.
func TestExecuteContextAction_StartReviewRejectsStaleState(t *testing.T) {
	dir := t.TempDir()
	plansDir := filepath.Join(dir, "docs", "plans")
	require.NoError(t, os.MkdirAll(plansDir, 0o755))

	// Create task in store before loading ps so the Plans map is populated.
	store := taskstore.NewTestSQLiteStore(t)
	const planFile = "stale-review"
	require.NoError(t, store.Create("test", taskstore.TaskEntry{
		Filename: planFile,
		Status:   taskstore.StatusImplementing,
		Branch:   "plan/stale-review",
	}))

	ps, err := taskstate.Load(store, "test", plansDir)
	require.NoError(t, err)
	fsm := taskfsm.New(store, "test", plansDir)

	// Advance to Done — simulating what the daemon would do while the context
	// menu was open (the TUI still shows implementing, but the DB says done).
	require.NoError(t, fsm.Transition(planFile, taskfsm.ImplementFinished))
	require.NoError(t, fsm.Transition(planFile, taskfsm.ReviewApproved))
	require.NoError(t, fsm.Transition(planFile, taskfsm.VerifyApproved))
	// Verify precondition: task must be Done before the stale action.
	ps2, err2 := taskstate.Load(store, "test", plansDir)
	require.NoError(t, err2)
	entry, ok := ps2.Entry(planFile)
	require.True(t, ok)
	require.Equal(t, taskstate.StatusDone, entry.Status, "precondition: task must be Done before the stale action")

	sp := spinner.New(spinner.WithSpinner(spinner.Dot))
	h := &home{
		ctx:          context.Background(),
		state:        stateDefault,
		appConfig:    config.DefaultConfig(),
		nav:          ui.NewNavigationPanel(&sp),
		menu:         ui.NewMenu(),
		tabbedWindow: ui.NewTabbedWindow(ui.NewPreviewPane(), ui.NewInfoPane()),
		toastManager: overlay.NewToastManager(&sp),
		overlays:     overlay.NewManager(),
		daemonStatusChecker: func(string) daemonStatusMsg {
			return daemonStatusMsg{ready: true}
		},
		taskState:          ps,
		taskStore:          store,
		taskStoreProject:   "test",
		taskStateDir:       plansDir,
		fsm:                fsm,
		activeRepoPath:     dir,
		program:            "opencode",
		instanceFinalizers: make(map[*session.Instance]func()),
	}
	h.updateSidebarTasks()
	require.True(t, h.nav.SelectByID(ui.SidebarPlanPrefix+planFile))

	instancesBefore := len(h.nav.GetInstances())

	_, cmd := h.executeContextAction("start_review")
	// The rejection may be an error Cmd (FSM transition failed) or a
	// lifecycleActionRejectedMsg — either way the key invariant is that no
	// new agent was spawned and the FSM state did not regress.
	_ = cmd

	assert.Equal(t, instancesBefore, len(h.nav.GetInstances()),
		"stale start_review must not spawn any agent instances")

	// Reload from store to confirm FSM state was not mutated.
	reloaded, err := taskstate.Load(store, "test", plansDir)
	require.NoError(t, err)
	freshEntry, exists := reloaded.Entry(planFile)
	require.True(t, exists)
	assert.Equal(t, taskstate.StatusDone, freshEntry.Status,
		"stale start_review must not mutate FSM state from Done")
}

// TestExecuteContextAction_StartFixerRejectsStaleState verifies that
// start_fixer returns a lifecycleActionRejectedMsg and does not spawn a fixer
// agent when the task has advanced to Done between menu-open and execution.
func TestExecuteContextAction_StartFixerRejectsStaleState(t *testing.T) {
	dir := t.TempDir()
	plansDir := filepath.Join(dir, "docs", "plans")
	require.NoError(t, os.MkdirAll(plansDir, 0o755))

	// Create task in store before loading ps so the Plans map is populated.
	store := taskstore.NewTestSQLiteStore(t)
	const planFile = "stale-fixer"
	require.NoError(t, store.Create("test", taskstore.TaskEntry{
		Filename: planFile,
		Status:   taskstore.StatusReviewing,
		Branch:   "plan/stale-fixer",
	}))

	ps, err := taskstate.Load(store, "test", plansDir)
	require.NoError(t, err)
	fsm := taskfsm.New(store, "test", plansDir)

	// Simulate daemon advancing to Done while confirmation/menu was open.
	require.NoError(t, fsm.Transition(planFile, taskfsm.ReviewApproved))
	require.NoError(t, fsm.Transition(planFile, taskfsm.VerifyApproved))
	// Verify precondition via fresh load.
	ps2, err2 := taskstate.Load(store, "test", plansDir)
	require.NoError(t, err2)
	entry, ok := ps2.Entry(planFile)
	require.True(t, ok)
	require.Equal(t, taskstate.StatusDone, entry.Status, "precondition: task must be Done before the stale action")

	sp := spinner.New(spinner.WithSpinner(spinner.Dot))
	h := &home{
		ctx:          context.Background(),
		state:        stateDefault,
		appConfig:    config.DefaultConfig(),
		nav:          ui.NewNavigationPanel(&sp),
		menu:         ui.NewMenu(),
		tabbedWindow: ui.NewTabbedWindow(ui.NewPreviewPane(), ui.NewInfoPane()),
		toastManager: overlay.NewToastManager(&sp),
		overlays:     overlay.NewManager(),
		daemonStatusChecker: func(string) daemonStatusMsg {
			return daemonStatusMsg{ready: true}
		},
		taskState:             ps,
		taskStore:             store,
		taskStoreProject:      "test",
		taskStateDir:          plansDir,
		fsm:                   fsm,
		activeRepoPath:        dir,
		program:               "opencode",
		pendingReviewFeedback: make(map[string]string),
		instanceFinalizers:    make(map[*session.Instance]func()),
	}
	h.updateSidebarTasks()
	require.True(t, h.nav.SelectByID(ui.SidebarPlanPrefix+planFile))

	instancesBefore := len(h.nav.GetInstances())

	_, cmd := h.executeContextAction("start_fixer")
	require.NotNil(t, cmd, "stale start_fixer must return a rejection Cmd")
	msg := cmd()

	// The rejection must be via lifecycleActionRejectedMsg, not an error toast.
	rejected, ok := msg.(lifecycleActionRejectedMsg)
	require.True(t, ok, "expected lifecycleActionRejectedMsg, got %T: %v", msg, msg)
	assert.Contains(t, rejected.message, "task state changed",
		"rejection message must indicate that task state changed")

	assert.Equal(t, instancesBefore, len(h.nav.GetInstances()),
		"stale start_fixer must not spawn any fixer instances")
}

// TestExecuteContextAction_CancelPlanRevalidatesOnConfirm verifies that the
// cancel confirm action re-fetches task state before committing the cancel.
//
// Scenario: the user opens the cancel dialog when the task is implementing.
// While the confirmation overlay is visible, the daemon advances the task to
// Done.  When the user confirms, the cancelAction closure must detect the
// stale state and surface a lifecycleActionRejectedMsg rather than
// erroneously cancelling an already-done task.
func TestExecuteContextAction_CancelPlanRevalidatesOnConfirm(t *testing.T) {
	dir := t.TempDir()
	plansDir := filepath.Join(dir, "docs", "plans")
	require.NoError(t, os.MkdirAll(plansDir, 0o755))

	// Create task in store before loading ps so the Plans map is populated.
	store := taskstore.NewTestSQLiteStore(t)
	const planFile = "stale-cancel"
	require.NoError(t, store.Create("test", taskstore.TaskEntry{
		Filename: planFile,
		Status:   taskstore.StatusImplementing,
		Branch:   "plan/stale-cancel",
	}))

	ps, err := taskstate.Load(store, "test", plansDir)
	require.NoError(t, err)
	fsm := taskfsm.New(store, "test", plansDir)

	sp := spinner.New(spinner.WithSpinner(spinner.Dot))
	h := &home{
		ctx:          context.Background(),
		state:        stateDefault,
		appConfig:    config.DefaultConfig(),
		nav:          ui.NewNavigationPanel(&sp),
		menu:         ui.NewMenu(),
		tabbedWindow: ui.NewTabbedWindow(ui.NewPreviewPane(), ui.NewInfoPane()),
		toastManager: overlay.NewToastManager(&sp),
		overlays:     overlay.NewManager(),
		daemonStatusChecker: func(string) daemonStatusMsg {
			return daemonStatusMsg{ready: true}
		},
		taskState:          ps,
		taskStore:          store,
		taskStoreProject:   "test",
		taskStateDir:       plansDir,
		fsm:                fsm,
		activeRepoPath:     dir,
		program:            "opencode",
		instanceFinalizers: make(map[*session.Instance]func()),
	}
	h.updateSidebarTasks()
	require.True(t, h.nav.SelectByID(ui.SidebarPlanPrefix+planFile))

	// Trigger cancel_plan to install the pendingConfirmAction.
	_, confirmCmd := h.executeContextAction("cancel_plan")
	// confirmAction returns nil (the confirm overlay is shown synchronously).
	assert.Nil(t, confirmCmd, "confirmAction must return nil Cmd")
	require.NotNil(t, h.pendingConfirmAction, "cancel_plan must set pendingConfirmAction")

	// Simulate daemon advancing to Done while the confirmation overlay is open.
	require.NoError(t, fsm.Transition(planFile, taskfsm.ImplementFinished))
	require.NoError(t, fsm.Transition(planFile, taskfsm.ReviewApproved))
	require.NoError(t, fsm.Transition(planFile, taskfsm.VerifyApproved))

	// User confirms — the pending action must detect the stale state.
	msg := h.pendingConfirmAction()

	rejected, ok := msg.(lifecycleActionRejectedMsg)
	require.True(t, ok, "expected lifecycleActionRejectedMsg after state drift, got %T: %v", msg, msg)
	assert.Contains(t, rejected.message, "task state changed",
		"rejection message must indicate the task state changed")

	// The task must still be Done, not Cancelled.
	reloaded, err := taskstate.Load(store, "test", plansDir)
	require.NoError(t, err)
	freshEntry, exists := reloaded.Entry(planFile)
	require.True(t, exists)
	assert.Equal(t, taskstate.StatusDone, freshEntry.Status,
		"cancel on a Done task must be rejected; task must remain Done not Cancelled")
}

// TestInstanceSignalItems_ReviewerAgent_WhenVerifying_NoSignals is a negative
// case: a reviewer instance paired with a StatusVerifying task must NOT be
// offered mark_review_approved or mark_review_changes_requested because the
// FSM does not allow ReviewApproved from StatusVerifying.
//
// This mirrors the positive case in TestInstanceSignalItems_MasterAgent_HasReadinessSignals
// and verifies that instanceSignalItems and emitSelectedInstanceSignal share the
// same FSM-gating rules — so stale menus can never offer actions the action
// handler would reject.
func TestInstanceSignalItems_ReviewerAgent_WhenVerifying_NoSignals(t *testing.T) {
	inst := &session.Instance{
		Title:     "stale-reviewer-1",
		TaskFile:  "my-plan.md",
		AgentType: session.AgentTypeReviewer,
	}
	// StatusVerifying is entered after the reviewer approved — the reviewer
	// signal items must no longer be offered at this point.
	entry := taskstate.TaskEntry{Status: taskstate.StatusVerifying}

	items := instanceSignalItems(inst, entry, true)

	var actions []string
	for _, item := range items {
		actions = append(actions, item.Action)
	}
	assert.NotContains(t, actions, "mark_review_approved",
		"reviewer must NOT offer mark_review_approved when task is verifying")
	assert.NotContains(t, actions, "mark_review_changes_requested",
		"reviewer must NOT offer mark_review_changes_requested when task is verifying")
}

// TestExecuteContextAction_StartVerify_TransitionsAndSpawnsMaster verifies that
// executing start_verify from implementing or reviewing:
//   - transitions the task to verifying in the store
//   - registers a master instance synchronously (before the cmd fires)
//   - emits at least one EventPlanTransition audit entry mentioning "verifying"
func TestExecuteContextAction_StartVerify_TransitionsAndSpawnsMaster(t *testing.T) {
	cases := []struct {
		name          string
		initialStatus taskstore.Status
		branch        string
		planFile      string
	}{
		{
			name:          "from implementing",
			initialStatus: taskstore.StatusImplementing,
			branch:        "plan/start-verify-impl",
			planFile:      "start-verify-impl",
		},
		{
			name:          "from reviewing",
			initialStatus: taskstore.StatusReviewing,
			branch:        "plan/start-verify-review",
			planFile:      "start-verify-review",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			plansDir := filepath.Join(dir, "docs", "plans")
			require.NoError(t, os.MkdirAll(plansDir, 0o755))

			store := taskstore.NewTestSQLiteStore(t)
			require.NoError(t, store.Create("test", taskstore.TaskEntry{
				Filename: tc.planFile,
				Status:   tc.initialStatus,
				Branch:   tc.branch,
			}))

			ps, err := taskstate.Load(store, "test", plansDir)
			require.NoError(t, err)
			fsm := taskfsm.New(store, "test", plansDir)

			logger, err := auditlog.NewSQLiteLogger(":memory:")
			require.NoError(t, err)
			defer logger.Close()

			sp := spinner.New(spinner.WithSpinner(spinner.Dot))
			h := &home{
				ctx:          context.Background(),
				state:        stateDefault,
				appConfig:    config.DefaultConfig(),
				nav:          ui.NewNavigationPanel(&sp),
				menu:         ui.NewMenu(),
				tabbedWindow: ui.NewTabbedWindow(ui.NewPreviewPane(), ui.NewInfoPane()),
				toastManager: overlay.NewToastManager(&sp),
				overlays:     overlay.NewManager(),
				daemonStatusChecker: func(string) daemonStatusMsg {
					return daemonStatusMsg{ready: true}
				},
				taskState:          ps,
				taskStore:          store,
				taskStoreProject:   "test",
				taskStateDir:       plansDir,
				fsm:                fsm,
				activeRepoPath:     dir,
				program:            "opencode",
				auditLogger:        logger,
				instanceFinalizers: make(map[*session.Instance]func()),
			}
			h.updateSidebarTasks()
			require.True(t, h.nav.SelectByID(ui.SidebarPlanPrefix+tc.planFile))

			_, cmd := h.executeContextAction("start_verify")
			require.NotNil(t, cmd, "start_verify from %s must return a non-nil Cmd", tc.name)

			// spawnMaster adds the instance synchronously before returning the Cmd.
			instances := h.nav.GetInstances()
			var masterCount int
			for _, inst := range instances {
				if inst.AgentType == session.AgentTypeMaster && inst.TaskFile == tc.planFile {
					masterCount++
				}
			}
			assert.Equal(t, 1, masterCount,
				"start_verify from %s must register exactly one master instance", tc.name)

			// Reload from store: the task must now be verifying.
			reloaded, err := taskstate.Load(store, "test", plansDir)
			require.NoError(t, err)
			freshEntry, ok := reloaded.Entry(tc.planFile)
			require.True(t, ok)
			assert.Equal(t, taskstate.StatusVerifying, freshEntry.Status,
				"start_verify from %s must advance the task to verifying in the store", tc.name)

			// At least one plan-transition audit event mentioning "verifying" must exist.
			events, err := logger.Query(auditlog.QueryFilter{
				Project: "test",
				Kinds:   []auditlog.EventKind{auditlog.EventPlanTransition},
				Limit:   20,
			})
			require.NoError(t, err)
			var foundVerifyingAudit bool
			for _, ev := range events {
				if strings.Contains(ev.Message, "verifying") {
					foundVerifyingAudit = true
					break
				}
			}
			assert.True(t, foundVerifyingAudit,
				"start_verify from %s must emit a plan-transition audit entry mentioning 'verifying'", tc.name)
		})
	}
}

// TestExecuteContextAction_StartVerify_FromVerifyingSpawnsMasterWithoutTransitionAudit
// verifies that executing start_verify when the task is already verifying:
//   - does not write any EventPlanTransition audit entries
//   - still registers a master instance when no live verifier exists
//   - leaves the store state as verifying
func TestExecuteContextAction_StartVerify_FromVerifyingSpawnsMasterWithoutTransitionAudit(t *testing.T) {
	dir := t.TempDir()
	plansDir := filepath.Join(dir, "docs", "plans")
	require.NoError(t, os.MkdirAll(plansDir, 0o755))

	store := taskstore.NewTestSQLiteStore(t)
	const planFile = "start-verify-verifying"
	require.NoError(t, store.Create("test", taskstore.TaskEntry{
		Filename: planFile,
		Status:   taskstore.StatusReviewing,
		Branch:   "plan/start-verify-verifying",
	}))

	// Advance to verifying before building the home (simulates daemon doing it
	// while the TUI was open — the home will load an already-verifying state).
	seedFSM := taskfsm.New(store, "test", plansDir)
	require.NoError(t, seedFSM.Transition(planFile, taskfsm.ReviewApproved))

	ps, err := taskstate.Load(store, "test", plansDir)
	require.NoError(t, err)
	entry, ok := ps.Entry(planFile)
	require.True(t, ok)
	require.Equal(t, taskstate.StatusVerifying, entry.Status, "precondition: task must be verifying")

	fsm := taskfsm.New(store, "test", plansDir)

	logger, err := auditlog.NewSQLiteLogger(":memory:")
	require.NoError(t, err)
	defer logger.Close()

	sp := spinner.New(spinner.WithSpinner(spinner.Dot))
	h := &home{
		ctx:          context.Background(),
		state:        stateDefault,
		appConfig:    config.DefaultConfig(),
		nav:          ui.NewNavigationPanel(&sp),
		menu:         ui.NewMenu(),
		tabbedWindow: ui.NewTabbedWindow(ui.NewPreviewPane(), ui.NewInfoPane()),
		toastManager: overlay.NewToastManager(&sp),
		overlays:     overlay.NewManager(),
		daemonStatusChecker: func(string) daemonStatusMsg {
			return daemonStatusMsg{ready: true}
		},
		taskState:          ps,
		taskStore:          store,
		taskStoreProject:   "test",
		taskStateDir:       plansDir,
		fsm:                fsm,
		activeRepoPath:     dir,
		program:            "opencode",
		auditLogger:        logger,
		instanceFinalizers: make(map[*session.Instance]func()),
	}
	h.updateSidebarTasks()
	require.True(t, h.nav.SelectByID(ui.SidebarPlanPrefix+planFile))

	_, cmd := h.executeContextAction("start_verify")
	require.NotNil(t, cmd, "start_verify from verifying must return a non-nil Cmd")

	// One master instance must be registered (no live verifier existed).
	instances := h.nav.GetInstances()
	var masterCount int
	for _, inst := range instances {
		if inst.AgentType == session.AgentTypeMaster && inst.TaskFile == planFile {
			masterCount++
		}
	}
	assert.Equal(t, 1, masterCount, "start_verify from verifying must register one master instance")

	// Store state must remain verifying — no extra FSM transition.
	reloaded, err := taskstate.Load(store, "test", plansDir)
	require.NoError(t, err)
	freshEntry, ok := reloaded.Entry(planFile)
	require.True(t, ok)
	assert.Equal(t, taskstate.StatusVerifying, freshEntry.Status,
		"start_verify from verifying must not change the store state")

	// No plan-transition audit event must have been emitted by the action.
	events, err := logger.Query(auditlog.QueryFilter{
		Project: "test",
		Kinds:   []auditlog.EventKind{auditlog.EventPlanTransition},
		Limit:   10,
	})
	require.NoError(t, err)
	assert.Empty(t, events,
		"start_verify from verifying must not emit any plan-transition audit entries")
}

// TestExecuteContextAction_StartVerifyRejectsStaleState verifies that start_verify
// returns a lifecycleActionRejectedMsg and does not spawn a master agent when the
// task has advanced to Done between menu-open and execution.
func TestExecuteContextAction_StartVerifyRejectsStaleState(t *testing.T) {
	dir := t.TempDir()
	plansDir := filepath.Join(dir, "docs", "plans")
	require.NoError(t, os.MkdirAll(plansDir, 0o755))

	// Create task in store before loading ps so the Plans map is populated.
	store := taskstore.NewTestSQLiteStore(t)
	const planFile = "stale-verify"
	require.NoError(t, store.Create("test", taskstore.TaskEntry{
		Filename: planFile,
		Status:   taskstore.StatusImplementing,
		Branch:   "plan/stale-verify",
	}))

	ps, err := taskstate.Load(store, "test", plansDir)
	require.NoError(t, err)
	fsm := taskfsm.New(store, "test", plansDir)

	// Simulate the daemon advancing the task to Done while the context menu was
	// open — ps still holds the stale implementing snapshot.
	require.NoError(t, fsm.Transition(planFile, taskfsm.ImplementFinished))
	require.NoError(t, fsm.Transition(planFile, taskfsm.ReviewApproved))
	require.NoError(t, fsm.Transition(planFile, taskfsm.VerifyApproved))

	// Verify precondition via a fresh load.
	ps2, err2 := taskstate.Load(store, "test", plansDir)
	require.NoError(t, err2)
	preEntry, preOK := ps2.Entry(planFile)
	require.True(t, preOK)
	require.Equal(t, taskstate.StatusDone, preEntry.Status,
		"precondition: task must be Done before the stale action")

	// Build the home with the stale ps (implementing), DB is already Done.
	sp := spinner.New(spinner.WithSpinner(spinner.Dot))
	h := &home{
		ctx:          context.Background(),
		state:        stateDefault,
		appConfig:    config.DefaultConfig(),
		nav:          ui.NewNavigationPanel(&sp),
		menu:         ui.NewMenu(),
		tabbedWindow: ui.NewTabbedWindow(ui.NewPreviewPane(), ui.NewInfoPane()),
		toastManager: overlay.NewToastManager(&sp),
		overlays:     overlay.NewManager(),
		daemonStatusChecker: func(string) daemonStatusMsg {
			return daemonStatusMsg{ready: true}
		},
		taskState:          ps,
		taskStore:          store,
		taskStoreProject:   "test",
		taskStateDir:       plansDir,
		fsm:                fsm,
		activeRepoPath:     dir,
		program:            "opencode",
		instanceFinalizers: make(map[*session.Instance]func()),
	}
	h.updateSidebarTasks()
	require.True(t, h.nav.SelectByID(ui.SidebarPlanPrefix+planFile))

	instancesBefore := len(h.nav.GetInstances())

	_, cmd := h.executeContextAction("start_verify")
	require.NotNil(t, cmd, "stale start_verify must return a rejection Cmd")
	msg := cmd()

	// The rejection must arrive as a lifecycleActionRejectedMsg.
	rejected, ok := msg.(lifecycleActionRejectedMsg)
	require.True(t, ok, "expected lifecycleActionRejectedMsg, got %T: %v", msg, msg)
	assert.Contains(t, rejected.message, "task state changed",
		"rejection message must indicate that the task state changed")

	assert.Equal(t, instancesBefore, len(h.nav.GetInstances()),
		"stale start_verify must not spawn any master instances")

	// Reload from store: the task must still be Done.
	reloaded, err := taskstate.Load(store, "test", plansDir)
	require.NoError(t, err)
	freshEntry, exists := reloaded.Entry(planFile)
	require.True(t, exists)
	assert.Equal(t, taskstate.StatusDone, freshEntry.Status,
		"stale start_verify must not mutate FSM state from Done")
}

// TestExecuteContextAction_StartVerify_ClearsLatestReviewFeedback verifies that
// moving a task from reviewing to verifying via the manual start_verify action
// clears any persisted reviewer feedback (both in-memory and on the taskstate
// entry), mirroring the cleanup performed by the signal-driven
// loop.ReviewApprovedAction path. Stale feedback left on a verifying/done task
// is surfaced by cmd/status.go and reused by start_fixer, so leaking it across
// manual verify would leave reviewer findings attached to work that has already
// been approved.
func TestExecuteContextAction_StartVerify_ClearsLatestReviewFeedback(t *testing.T) {
	dir := t.TempDir()
	plansDir := filepath.Join(dir, "docs", "plans")
	require.NoError(t, os.MkdirAll(plansDir, 0o755))

	store := taskstore.NewTestSQLiteStore(t)
	const planFile = "start-verify-clears-feedback"
	const feedback = "reviewer flagged missing null check"
	require.NoError(t, store.Create("test", taskstore.TaskEntry{
		Filename: planFile,
		Status:   taskstore.StatusReviewing,
		Branch:   "plan/start-verify-clears-feedback",
	}))

	ps, err := taskstate.Load(store, "test", plansDir)
	require.NoError(t, err)
	require.NoError(t, ps.SetLatestReviewFeedback(planFile, feedback))
	preEntry, ok := ps.Entry(planFile)
	require.True(t, ok)
	require.Equal(t, feedback, preEntry.LatestReviewFeedback,
		"precondition: feedback must be persisted before manual verify")

	fsm := taskfsm.New(store, "test", plansDir)

	sp := spinner.New(spinner.WithSpinner(spinner.Dot))
	h := &home{
		ctx:          context.Background(),
		state:        stateDefault,
		appConfig:    config.DefaultConfig(),
		nav:          ui.NewNavigationPanel(&sp),
		menu:         ui.NewMenu(),
		tabbedWindow: ui.NewTabbedWindow(ui.NewPreviewPane(), ui.NewInfoPane()),
		toastManager: overlay.NewToastManager(&sp),
		overlays:     overlay.NewManager(),
		daemonStatusChecker: func(string) daemonStatusMsg {
			return daemonStatusMsg{ready: true}
		},
		taskState:             ps,
		taskStore:             store,
		taskStoreProject:      "test",
		taskStateDir:          plansDir,
		fsm:                   fsm,
		activeRepoPath:        dir,
		program:               "opencode",
		pendingReviewFeedback: map[string]string{planFile: feedback},
		instanceFinalizers:    make(map[*session.Instance]func()),
	}
	h.updateSidebarTasks()
	require.True(t, h.nav.SelectByID(ui.SidebarPlanPrefix+planFile))

	_, cmd := h.executeContextAction("start_verify")
	require.NotNil(t, cmd, "start_verify must return a non-nil Cmd")

	// Persisted feedback must have been cleared on the live taskstate.
	liveEntry, ok := h.taskState.Entry(planFile)
	require.True(t, ok)
	assert.Empty(t, liveEntry.LatestReviewFeedback,
		"start_verify must clear persisted reviewer feedback on the live taskstate")

	// Reload from the store to confirm the clear was persisted, not just
	// mutated in memory.
	reloaded, err := taskstate.Load(store, "test", plansDir)
	require.NoError(t, err)
	freshEntry, ok := reloaded.Entry(planFile)
	require.True(t, ok)
	assert.Equal(t, taskstate.StatusVerifying, freshEntry.Status,
		"start_verify must advance the task to verifying")
	assert.Empty(t, freshEntry.LatestReviewFeedback,
		"start_verify must clear persisted reviewer feedback in the store")

	// Pending in-memory feedback (used by start_fixer) must also be cleared.
	_, pendingStillThere := h.pendingReviewFeedback[planFile]
	assert.False(t, pendingStillThere,
		"start_verify must clear pendingReviewFeedback so start_fixer cannot reuse stale feedback")
}

// TestExecuteContextAction_StartVerify_PreemptsLiveCoderAndFixer verifies that
// triggering start_verify on an implementing task with live coder/fixer
// sessions removes those sessions before spawning the master agent. The
// regression: spawnMaster only cleans up prior master/reviewer instances, so
// without explicit preemption a coder or fixer would keep writing to the same
// shared worktree while the readiness pass runs.
func TestExecuteContextAction_StartVerify_PreemptsLiveCoderAndFixer(t *testing.T) {
	cases := []struct {
		name      string
		agentType string
	}{
		{name: "live coder", agentType: session.AgentTypeCoder},
		{name: "live fixer", agentType: session.AgentTypeFixer},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			plansDir := filepath.Join(dir, "docs", "plans")
			require.NoError(t, os.MkdirAll(plansDir, 0o755))

			store := taskstore.NewTestSQLiteStore(t)
			const planFile = "start-verify-preempt"
			require.NoError(t, store.Create("test", taskstore.TaskEntry{
				Filename: planFile,
				Status:   taskstore.StatusImplementing,
				Branch:   "plan/start-verify-preempt",
			}))

			ps, err := taskstate.Load(store, "test", plansDir)
			require.NoError(t, err)
			fsm := taskfsm.New(store, "test", plansDir)

			sp := spinner.New(spinner.WithSpinner(spinner.Dot))
			h := &home{
				ctx:          context.Background(),
				state:        stateDefault,
				appConfig:    config.DefaultConfig(),
				nav:          ui.NewNavigationPanel(&sp),
				menu:         ui.NewMenu(),
				tabbedWindow: ui.NewTabbedWindow(ui.NewPreviewPane(), ui.NewInfoPane()),
				toastManager: overlay.NewToastManager(&sp),
				overlays:     overlay.NewManager(),
				daemonStatusChecker: func(string) daemonStatusMsg {
					return daemonStatusMsg{ready: true}
				},
				taskState:          ps,
				taskStore:          store,
				taskStoreProject:   "test",
				taskStateDir:       plansDir,
				fsm:                fsm,
				activeRepoPath:     dir,
				program:            "opencode",
				instanceFinalizers: make(map[*session.Instance]func()),
			}

			// Seed a running implementation agent for this plan before the
			// user triggers start_verify. killExistingPlanAgent only inspects
			// TaskFile + AgentType, so a bare struct is sufficient.
			liveAgent := &session.Instance{
				Title:     "start-verify-preempt-" + tc.agentType,
				Path:      dir,
				Program:   "opencode",
				TaskFile:  planFile,
				AgentType: tc.agentType,
			}
			h.nav.AddInstance(liveAgent)

			h.updateSidebarTasks()
			require.True(t, h.nav.SelectByID(ui.SidebarPlanPrefix+planFile))

			// Precondition: the live agent is tracked before the action runs.
			instancesBefore := h.nav.GetInstances()
			var preCount int
			for _, inst := range instancesBefore {
				if inst.TaskFile == planFile && inst.AgentType == tc.agentType {
					preCount++
				}
			}
			require.Equal(t, 1, preCount,
				"precondition: one %s must be live before start_verify", tc.agentType)

			_, cmd := h.executeContextAction("start_verify")
			require.NotNil(t, cmd, "start_verify must return a non-nil Cmd")

			// The live implementation agent must be gone; a master must be
			// registered synchronously (spawnMaster adds it before returning).
			instancesAfter := h.nav.GetInstances()
			var liveCount, masterCount int
			for _, inst := range instancesAfter {
				if inst.TaskFile != planFile {
					continue
				}
				switch inst.AgentType {
				case tc.agentType:
					liveCount++
				case session.AgentTypeMaster:
					masterCount++
				}
			}
			assert.Equal(t, 0, liveCount,
				"start_verify must preempt the live %s so it cannot race the master in the shared worktree", tc.agentType)
			assert.Equal(t, 1, masterCount,
				"start_verify must register exactly one master instance after preemption")

			// The store must now reflect verifying — the manual walk from
			// implementing to verifying succeeded.
			reloaded, err := taskstate.Load(store, "test", plansDir)
			require.NoError(t, err)
			freshEntry, ok := reloaded.Entry(planFile)
			require.True(t, ok)
			assert.Equal(t, taskstate.StatusVerifying, freshEntry.Status,
				"start_verify from implementing must walk the task to verifying")
		})
	}
}

// TestSpawnAdHocAgent_SDKRoutesThroughDaemonWhenManaged verifies that when
// the repo is managed by the daemon and the instance uses SDK mode, spawnAdHocAgent
// delegates to the spawnSoloWithDaemon seam instead of calling inst.StartOnMainBranch.
func TestSpawnAdHocAgent_SDKRoutesThroughDaemonWhenManaged(t *testing.T) {
	oldManaged := repoManagedByDaemon
	oldSpawner := spawnSoloWithDaemon
	t.Cleanup(func() {
		repoManagedByDaemon = oldManaged
		spawnSoloWithDaemon = oldSpawner
	})
	repoManagedByDaemon = func(string) bool { return true }

	var capturedReq api.SpawnSoloRequest
	var capturedProject string
	spawnSoloWithDaemon = func(project string, req api.SpawnSoloRequest) error {
		capturedProject = project
		capturedReq = req
		return nil
	}

	spin := spinner.New(spinner.WithSpinner(spinner.Dot))
	h := &home{
		ctx:              context.Background(),
		state:            stateDefault,
		taskStoreProject: "myproject",
		appConfig: &config.Config{
			Profiles: map[string]config.AgentProfile{
				"master": {Program: "claude", Enabled: true, ExecutionMode: config.ExecutionModeSDK},
			},
		},
		nav:            ui.NewNavigationPanel(&spin),
		menu:           ui.NewMenu(),
		tabbedWindow:   ui.NewTabbedWindow(ui.NewPreviewPane(), ui.NewInfoPane()),
		toastManager:   overlay.NewToastManager(&spin),
		overlays:       overlay.NewManager(),
		activeRepoPath: t.TempDir(),
		program:        "claude",
		daemonStatusChecker: func(string) daemonStatusMsg {
			return daemonStatusMsg{ready: true}
		},
		instanceFinalizers: make(map[*session.Instance]func()),
	}

	_, cmd := h.spawnAdHocAgent("solo-agent", "", "", "claude", session.ExecutionModeSDK, "")
	require.NotNil(t, cmd)

	// Execute the returned cmd to get the instanceStartedMsg.
	msg := cmd()
	var started instanceStartedMsg
	if batch, ok := msg.(tea.BatchMsg); ok {
		for _, sub := range batch {
			if sm, ok := sub().(instanceStartedMsg); ok {
				started = sm
				break
			}
		}
	} else {
		started, _ = msg.(instanceStartedMsg)
	}

	require.NotNil(t, started.instance)
	assert.NoError(t, started.err)
	assert.Equal(t, "myproject", capturedProject)
	assert.Equal(t, "solo-agent", capturedReq.Title)
	assert.Equal(t, "claude", capturedReq.Program)
	assert.False(t, started.instance.Started(), "daemon SDK placeholder must not be started locally")
}

// TestSpawnTaskAgent_SoloSDKRoutesThroughDaemonWhenManaged verifies that when
// the solo action uses SDK mode and the repo is daemon-managed, the spawn is
// routed through spawnSoloWithDaemon with SoloAgent=true.
func TestSpawnTaskAgent_SoloSDKRoutesThroughDaemonWhenManaged(t *testing.T) {
	oldManaged := repoManagedByDaemon
	oldSpawner := spawnSoloWithDaemon
	t.Cleanup(func() {
		repoManagedByDaemon = oldManaged
		spawnSoloWithDaemon = oldSpawner
	})
	repoManagedByDaemon = func(string) bool { return true }

	var capturedReq api.SpawnSoloRequest
	spawnSoloWithDaemon = func(project string, req api.SpawnSoloRequest) error {
		capturedReq = req
		return nil
	}

	dir := t.TempDir()
	plansDir := filepath.Join(dir, "docs", "plans")
	require.NoError(t, os.MkdirAll(plansDir, 0o755))
	ps, err := newTestPlanState(t, plansDir)
	require.NoError(t, err)
	planFile := "my-task.md"
	require.NoError(t, ps.Register(planFile, "My Task", "plan/my-task", time.Now()))

	spin := spinner.New(spinner.WithSpinner(spinner.Dot))
	h := &home{
		ctx:              context.Background(),
		state:            stateDefault,
		taskState:        ps,
		taskStoreProject: "myproject",
		activeRepoPath:   dir,
		appConfig: &config.Config{
			PhaseRoles: map[string]string{"implementing": "coder"},
			Profiles: map[string]config.AgentProfile{
				"coder": {Program: "claude", Enabled: true, ExecutionMode: config.ExecutionModeSDK},
			},
		},
		nav:          ui.NewNavigationPanel(&spin),
		menu:         ui.NewMenu(),
		tabbedWindow: ui.NewTabbedWindow(ui.NewPreviewPane(), ui.NewInfoPane()),
		toastManager: overlay.NewToastManager(&spin),
		overlays:     overlay.NewManager(),
		program:      "claude",
		daemonStatusChecker: func(string) daemonStatusMsg {
			return daemonStatusMsg{ready: true}
		},
		instanceFinalizers: make(map[*session.Instance]func()),
	}

	_, cmd := h.spawnTaskAgent(planFile, "solo", "")
	require.NotNil(t, cmd)

	msg := cmd()
	var started instanceStartedMsg
	if batch, ok := msg.(tea.BatchMsg); ok {
		for _, sub := range batch {
			if sm, ok := sub().(instanceStartedMsg); ok {
				started = sm
				break
			}
		}
	} else {
		started, _ = msg.(instanceStartedMsg)
	}

	require.NotNil(t, started.instance)
	assert.NoError(t, started.err)
	assert.True(t, capturedReq.SoloAgent, "SoloAgent must be set on the daemon request")
	assert.Equal(t, planFile, capturedReq.TaskFile, "TaskFile must be forwarded to daemon request")
	assert.True(t, started.instance.SoloAgent, "local placeholder must keep SoloAgent=true")
}
