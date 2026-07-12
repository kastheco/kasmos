package app

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"charm.land/bubbles/v2/spinner"
	"github.com/kastheco/kasmos/config"
	"github.com/kastheco/kasmos/config/taskfsm"
	"github.com/kastheco/kasmos/config/taskstate"
	"github.com/kastheco/kasmos/config/taskstore"
	"github.com/kastheco/kasmos/session"
	"github.com/kastheco/kasmos/ui"
	"github.com/kastheco/kasmos/ui/overlay"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSpawnTaskAgent_PatchesMainBranchOpencodeConfig(t *testing.T) {
	t.Parallel()
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
	planFile := "plan-branch-patch.md"
	require.NoError(t, ps.Register(planFile, "test plan", "plan/patch", time.Now()))

	configPath := filepath.Join(dir, "opencode.jsonc")
	require.NoError(t, os.WriteFile(configPath, []byte(`{"agent":{"planner":{"model":"anthropic/old","temperature":0.1,"reasoningEffort":"low"}}}`), 0o644))

	planTemp := 0.7
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
				"planning": "planner",
			},
			Profiles: map[string]config.AgentProfile{
				"planner": {
					Program:     "opencode",
					Model:       "claude-opus-4-6",
					Temperature: &planTemp,
					Effort:      "high",
					Enabled:     true,
				},
			},
		},
		instanceFinalizers: make(map[*session.Instance]func()),
	}

	_, cmd := m.spawnTaskAgent(planFile, "plan", "plan prompt")
	if cmd != nil {
		_ = cmd()
	}

	var cfg map[string]any
	data, err := os.ReadFile(configPath)
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(data, &cfg))
	agentCfg, ok := cfg["agent"].(map[string]any)
	require.True(t, ok)
	plannerCfg, ok := agentCfg["planner"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "anthropic/claude-opus-4-6", plannerCfg["model"])
	assert.InDelta(t, planTemp, plannerCfg["temperature"].(float64), 0.0001)
	assert.Equal(t, "high", plannerCfg["reasoningEffort"])
}

func TestSpawnMasterBuildsPromptForResolvedRangeWithoutFeedback(t *testing.T) {
	dir := t.TempDir()
	for _, args := range [][]string{
		{"init", "-b", "main", dir},
		{"-C", dir, "config", "user.email", "test@test.com"},
		{"-C", dir, "config", "user.name", "Test"},
		{"-C", dir, "commit", "--allow-empty", "-m", "base"},
		{"-C", dir, "checkout", "-b", "plan/range-prompt"},
		{"-C", dir, "commit", "--allow-empty", "-m", "head"},
		{"-C", dir, "checkout", "main"},
	} {
		out, err := exec.Command("git", args...).CombinedOutput()
		require.NoError(t, err, "%s", out)
	}
	baseOut, err := exec.Command("git", "-C", dir, "rev-parse", "main").Output()
	require.NoError(t, err)
	headOut, err := exec.Command("git", "-C", dir, "rev-parse", "plan/range-prompt").Output()
	require.NoError(t, err)
	base, head := strings.TrimSpace(string(baseOut)), strings.TrimSpace(string(headOut))

	plansDir := filepath.Join(dir, "docs", "plans")
	require.NoError(t, os.MkdirAll(plansDir, 0o755))
	ps, err := newTestPlanState(t, plansDir)
	require.NoError(t, err)
	require.NoError(t, ps.Register("range-prompt", "test", "plan/range-prompt", time.Now()))

	_ = ps
	spec := buildMasterSpecForBranch(dir, "plan/range-prompt", "range-prompt", "test", 0, 37, 5)
	prompt := spec.Prompt
	assert.Contains(t, prompt, base)
	assert.Contains(t, prompt, head)
	assert.Contains(t, prompt, "37")
	assert.Contains(t, prompt, "5")
	assert.NotContains(t, prompt, "Feedback:")
}

func TestSpawnArchitectPass_PatchesMainBranchOpencodeConfig(t *testing.T) {
	t.Parallel()
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
	planFile := "architect-branch-patch.md"
	require.NoError(t, ps.Register(planFile, "architect test plan", "plan/architect", time.Now()))
	seedPlanStatus(t, ps, planFile, taskstate.StatusImplementing)

	configPath := filepath.Join(dir, "opencode.jsonc")
	require.NoError(t, os.WriteFile(configPath, []byte(`{"agent":{"architect":{"model":"anthropic/old","temperature":0.1,"reasoningEffort":"low"}}}`), 0o644))

	planTemp := 0.65
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
				"elaborating": "architect",
			},
			Profiles: map[string]config.AgentProfile{
				"architect": {
					Program:     "opencode",
					Model:       "claude-opus-4-6",
					Temperature: &planTemp,
					Effort:      "low",
					Enabled:     true,
				},
			},
		},
		instanceFinalizers: make(map[*session.Instance]func()),
	}

	_, cmd := m.spawnElaborator(planFile)
	if cmd != nil {
		_ = cmd()
	}
	entry, ok := ps.Entry(planFile)
	require.True(t, ok)
	assert.Equal(t, taskstore.ExecutionState{Phase: string(taskfsm.ExecutionPhaseArchitecting), ActiveAgentType: session.AgentTypeElaborator}, entry.ExecutionState)

	var cfg map[string]any
	data, err := os.ReadFile(configPath)
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(data, &cfg))
	agentCfg, ok := cfg["agent"].(map[string]any)
	require.True(t, ok)
	elabCfg, ok := agentCfg["architect"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "anthropic/claude-opus-4-6", elabCfg["model"])
	assert.InDelta(t, planTemp, elabCfg["temperature"].(float64), 0.0001)
	assert.Equal(t, "low", elabCfg["reasoningEffort"])
}

func TestSpawnTaskAgent_ReviewKeepsReviewerCompatibilityMirrorSynced(t *testing.T) {
	t.Parallel()
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
	planFile := "review-agent.md"
	require.NoError(t, ps.Register(planFile, "review test plan", "plan/review-agent", time.Now()))
	require.NoError(t, ps.SetBranch(planFile, "plan/review-agent"))

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
				"quality_review": session.AgentTypeReviewer,
			},
			Profiles: map[string]config.AgentProfile{
				session.AgentTypeReviewer: {
					Program: "opencode",
					Enabled: true,
				},
			},
		},
		instanceFinalizers: make(map[*session.Instance]func()),
	}

	_, cmd := m.spawnTaskAgent(planFile, "review", "review prompt")
	require.NotNil(t, cmd)

	instances := m.nav.GetInstances()
	require.Len(t, instances, 1)
	inst := instances[0]
	assert.Equal(t, session.AgentTypeReviewer, inst.AgentType)
	assert.True(t, inst.IsReviewer, "new reviewer sessions must keep the reviewer compatibility mirror in sync")
	assert.Equal(t, 1, inst.ReviewCycle)
}

func TestBuildChatAboutTaskPrompt_UsesMCPFirst(t *testing.T) {
	t.Parallel()
	prompt := buildChatAboutTaskPrompt("architect-plan-location", "myproject", taskstate.TaskEntry{
		Status:      taskstate.StatusReady,
		Description: "architect-plan-location",
		Branch:      "plan/architect-plan-location",
		Topic:       "bugs",
	}, "is this still needed given the switch to an mcp server?")

	assert.Contains(t, prompt, "MCP `task_show`")
	assert.Contains(t, prompt, `filename: "architect-plan-location"`)
	assert.Contains(t, prompt, `project: "myproject"`)
	assert.Contains(t, prompt, "fall back to `kas task show architect-plan-location`")
	assert.Contains(t, prompt, "## User Question")
}

func TestSpawnReviewer_SkipsDuplicatePendingSameCycle(t *testing.T) {
	t.Parallel()
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
	planFile := "review-dedupe.md"
	require.NoError(t, ps.Register(planFile, "review dedupe", "plan/review-dedupe", time.Now()))
	require.NoError(t, ps.SetBranch(planFile, "plan/review-dedupe"))
	seedPlanStatus(t, ps, planFile, taskstate.StatusReviewing)
	require.NoError(t, ps.IncrementReviewCycle(planFile))

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
				"quality_review": session.AgentTypeReviewer,
			},
			Profiles: map[string]config.AgentProfile{
				session.AgentTypeReviewer: {
					Program: "opencode",
					Enabled: true,
				},
			},
		},
		instanceFinalizers: make(map[*session.Instance]func()),
	}

	cmd1 := m.spawnReviewer(planFile)
	require.NotNil(t, cmd1)
	assert.Len(t, m.instanceFinalizers, 1)
	entry, ok := ps.Entry(planFile)
	require.True(t, ok)
	assert.Equal(t, taskstore.ExecutionState{Phase: string(taskfsm.ExecutionPhaseReviewing), ActiveAgentType: session.AgentTypeReviewer}, entry.ExecutionState)

	cmd2 := m.spawnReviewer(planFile)
	assert.Nil(t, cmd2)
	assert.Len(t, m.instanceFinalizers, 1)
}

func TestDedupeInstancesByRepoAndTitle_KeepsNewest(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	older, err := session.NewInstance(session.InstanceOptions{Title: "feature-review-3", Path: dir, Program: "opencode", TaskFile: "feature", AgentType: session.AgentTypeReviewer})
	require.NoError(t, err)
	older.CreatedAt = time.Now().Add(-time.Minute)

	newer, err := session.NewInstance(session.InstanceOptions{Title: "feature-review-3", Path: dir, Program: "opencode", TaskFile: "feature", AgentType: session.AgentTypeReviewer})
	require.NoError(t, err)
	newer.CreatedAt = time.Now()

	other, err := session.NewInstance(session.InstanceOptions{Title: "feature-fix-2", Path: dir, Program: "opencode", TaskFile: "feature", AgentType: session.AgentTypeFixer})
	require.NoError(t, err)

	deduped := dedupeInstancesByRepoAndTitle([]*session.Instance{older, newer, other})
	require.Len(t, deduped, 2)
	assert.Same(t, newer, deduped[0])
	assert.Same(t, other, deduped[1])
}

func TestAdoptOrphanSession_BindsPersistedReviewerCycle(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	plansDir := filepath.Join(dir, "docs", "plans")
	require.NoError(t, os.MkdirAll(plansDir, 0o755))
	ps, err := newTestPlanState(t, plansDir)
	require.NoError(t, err)
	planFile := "repo-owned-wakeword-training"
	require.NoError(t, ps.Register(planFile, "repo owned wakeword training", "plan/wakeword", time.Now()))
	require.NoError(t, ps.IncrementReviewCycle(planFile))
	require.NoError(t, ps.IncrementReviewCycle(planFile))
	require.NoError(t, ps.ForceSetLifecycle(planFile, taskstate.StatusReviewing, taskstore.ExecutionState{Phase: string(taskfsm.ExecutionPhaseReviewing), ActiveAgentType: session.AgentTypeReviewer}))

	sp := spinner.New(spinner.WithSpinner(spinner.Dot))
	m := &home{
		taskState:      ps,
		activeRepoPath: dir,
		program:        "opencode",
		nav:            ui.NewNavigationPanel(&sp),
		menu:           ui.NewMenu(),
		toastManager:   overlay.NewToastManager(&sp),
	}

	model, _ := m.adoptOrphanSession(overlay.TmuxBrowserItem{Title: "repo-owned-wakeword-training-review-3", Name: "kas_repo-owned-wakeword-training-review-3"})
	updated := model.(*home)
	instances := updated.nav.GetInstances()
	require.Len(t, instances, 1)
	assert.Equal(t, planFile, instances[0].TaskFile)
	assert.Equal(t, session.AgentTypeReviewer, instances[0].AgentType)
	assert.Equal(t, "plan/wakeword", instances[0].Branch)
	assert.Equal(t, 3, instances[0].ReviewCycle)
}

func TestAdoptOrphanSession_BindsPlannerDraftProfile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	plansDir := filepath.Join(dir, "docs", "plans")
	require.NoError(t, os.MkdirAll(plansDir, 0o755))
	ps, err := newTestPlanState(t, plansDir)
	require.NoError(t, err)
	planFile := "feature"
	require.NoError(t, ps.Register(planFile, "feature", "plan/feature", time.Now()))
	seedPlanStatus(t, ps, planFile, taskstate.StatusPlanning)

	sp := spinner.New(spinner.WithSpinner(spinner.Dot))
	m := &home{
		taskState:      ps,
		activeRepoPath: dir,
		program:        "opencode",
		nav:            ui.NewNavigationPanel(&sp),
		menu:           ui.NewMenu(),
		toastManager:   overlay.NewToastManager(&sp),
	}

	model, _ := m.adoptOrphanSession(overlay.TmuxBrowserItem{Title: "feature-plan-planner-a", Name: "kas_feature-plan-planner-a"})
	updated := model.(*home)
	instances := updated.nav.GetInstances()
	require.Len(t, instances, 1)
	assert.Equal(t, planFile, instances[0].TaskFile)
	assert.Equal(t, session.AgentTypePlanner, instances[0].AgentType)
	assert.Equal(t, "planner-a", instances[0].PlannerProfile)
}

func TestAdoptOrphanSession_BindsPlanMetadataFromTitle(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	plansDir := filepath.Join(dir, "docs", "plans")
	require.NoError(t, os.MkdirAll(plansDir, 0o755))
	ps, err := newTestPlanState(t, plansDir)
	require.NoError(t, err)
	planFile := "repo-owned-wakeword-training"
	require.NoError(t, ps.Register(planFile, "repo owned wakeword training", "plan/wakeword", time.Now()))
	require.NoError(t, ps.ForceSetLifecycle(planFile, taskstate.StatusReviewing, taskstore.ExecutionState{Phase: string(taskfsm.ExecutionPhaseReviewing), ActiveAgentType: session.AgentTypeReviewer}))

	sp := spinner.New(spinner.WithSpinner(spinner.Dot))
	m := &home{
		taskState:      ps,
		activeRepoPath: dir,
		program:        "opencode",
		nav:            ui.NewNavigationPanel(&sp),
		menu:           ui.NewMenu(),
		toastManager:   overlay.NewToastManager(&sp),
	}

	model, _ := m.adoptOrphanSession(overlay.TmuxBrowserItem{Title: "repo-owned-wakeword-training-review-1", Name: "kas_repo-owned-wakeword-training-review-1"})
	updated := model.(*home)
	instances := updated.nav.GetInstances()
	require.Len(t, instances, 1)
	assert.Equal(t, planFile, instances[0].TaskFile)
	assert.Equal(t, session.AgentTypeReviewer, instances[0].AgentType)
	assert.Equal(t, 1, instances[0].ReviewCycle)
	assert.Equal(t, "plan/wakeword", instances[0].Branch)
}

func TestAdoptOrphanSession_BindsStalePhaseTitleToPlan(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	plansDir := filepath.Join(dir, "docs", "plans")
	require.NoError(t, os.MkdirAll(plansDir, 0o755))
	ps, err := newTestPlanState(t, plansDir)
	require.NoError(t, err)
	planFile := "feature"
	require.NoError(t, ps.Register(planFile, "feature", "plan/feature", time.Now()))
	require.NoError(t, ps.ForceSetLifecycle(planFile, taskstate.StatusReviewing, taskstore.ExecutionState{Phase: string(taskfsm.ExecutionPhaseReviewing), ActiveAgentType: session.AgentTypeReviewer}))

	sp := spinner.New(spinner.WithSpinner(spinner.Dot))
	m := &home{
		taskState:      ps,
		activeRepoPath: dir,
		program:        "opencode",
		nav:            ui.NewNavigationPanel(&sp),
		menu:           ui.NewMenu(),
		toastManager:   overlay.NewToastManager(&sp),
	}

	model, _ := m.adoptOrphanSession(overlay.TmuxBrowserItem{Title: "feature-fix-1", Name: "kas_feature-fix-1"})
	updated := model.(*home)
	instances := updated.nav.GetInstances()
	require.Len(t, instances, 1)
	assert.Equal(t, planFile, instances[0].TaskFile)
	assert.Equal(t, session.AgentTypeFixer, instances[0].AgentType)
	assert.Equal(t, "plan/feature", instances[0].Branch)
	assert.Equal(t, 1, instances[0].ReviewCycle)
}

func TestAdoptOrphanSession_BindsActiveWaveMetadata(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	plansDir := filepath.Join(dir, "docs", "plans")
	require.NoError(t, os.MkdirAll(plansDir, 0o755))

	store := taskstore.NewTestSQLiteStore(t)
	planFile := "feature"
	content := "**Goal:** test\n\n## Wave 1\n\n### Task 1: First\n\nDo first.\n\n## Wave 2\n\n### Task 2: Second\n\nDo second.\n"
	require.NoError(t, store.Create("proj", taskstore.TaskEntry{Filename: planFile, Status: taskstore.StatusReady, Branch: "plan/feature", Content: content}))
	ps, err := taskstate.Load(store, "proj", plansDir)
	require.NoError(t, err)
	seedPlanStatus(t, ps, planFile, taskstate.StatusImplementing)
	require.NoError(t, ps.SetExecutionState(planFile, taskstore.ExecutionState{Phase: string(taskfsm.ExecutionPhaseWaveRunning), ActiveAgentType: session.AgentTypeCoder, ActiveWave: 2}))

	sp := spinner.New(spinner.WithSpinner(spinner.Dot))
	m := &home{
		taskState:        ps,
		taskStore:        store,
		taskStoreProject: "proj",
		activeRepoPath:   dir,
		program:          "opencode",
		nav:              ui.NewNavigationPanel(&sp),
		menu:             ui.NewMenu(),
		toastManager:     overlay.NewToastManager(&sp),
	}

	model, _ := m.adoptOrphanSession(overlay.TmuxBrowserItem{Title: "feature-W2-T2", Name: "kas_feature-W2-T2"})
	updated := model.(*home)
	instances := updated.nav.GetInstances()
	require.Len(t, instances, 1)
	assert.Equal(t, planFile, instances[0].TaskFile)
	assert.Equal(t, session.AgentTypeCoder, instances[0].AgentType)
	assert.Equal(t, 2, instances[0].WaveNumber)
	assert.Equal(t, 2, instances[0].TaskNumber)
	assert.Equal(t, "plan/feature", instances[0].Branch)
}

func TestSyncSharedWorktreeScaffold_WritesHarnessFilesForConfiguredProfiles(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	reviewerTemp := 0.2
	fixerTemp := 0.1

	m := &home{
		appConfig: &config.Config{
			Profiles: map[string]config.AgentProfile{
				"reviewer": {
					Program:     "claude",
					Model:       "claude-sonnet-4-6",
					Temperature: &reviewerTemp,
					Effort:      "medium",
					Enabled:     true,
				},
				"fixer": {
					Program:     "opencode",
					Model:       "openai/gpt-5.4",
					Temperature: &fixerTemp,
					Effort:      "high",
					Enabled:     true,
				},
			},
		},
	}

	require.NoError(t, m.syncSharedWorktreeScaffold(dir))
	assert.FileExists(t, filepath.Join(dir, ".mcp.json"))
	assert.FileExists(t, filepath.Join(dir, ".claude", "agents", "reviewer.md"))
	assert.FileExists(t, filepath.Join(dir, ".opencode", "agents", "fixer.md"))
	assert.FileExists(t, filepath.Join(dir, ".agents", "skills", "kasmos-fixer", "SKILL.md"))
	assert.FileExists(t, filepath.Join(dir, "opencode.jsonc"))
}

func TestProfileForAgent_MasterUsesReadinessReviewProfile(t *testing.T) {
	t.Parallel()
	temp := 0.0
	m := &home{
		program: "opencode",
		appConfig: &config.Config{
			PhaseRoles: map[string]string{
				"readiness_review": "master",
			},
			Profiles: map[string]config.AgentProfile{
				"master": {
					Program:     "claude",
					Enabled:     true,
					Temperature: &temp,
				},
			},
		},
	}
	profile := m.profileForAgent(session.AgentTypeMaster)
	assert.Equal(t, "claude", profile.Program,
		"master agent must resolve via the readiness_review phase profile")
}

func TestHome_SkipPermissionsForAgent_UsesProfileResolver(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name              string
		agentType         string
		phase             string
		profileName       string
		permissionDefault string
		want              bool
	}{
		{"coder inherits prompt default", session.AgentTypeCoder, "implementing", "coder", "", false},
		{"coder prompts", session.AgentTypeCoder, "implementing", "coder", "prompt", false},
		{"coder bypasses", session.AgentTypeCoder, "implementing", "coder", "bypass", true},
		{"reviewer inherits prompt default", session.AgentTypeReviewer, "quality_review", "reviewer", "", false},
		{"reviewer prompts", session.AgentTypeReviewer, "quality_review", "reviewer", "prompt", false},
		{"reviewer bypasses", session.AgentTypeReviewer, "quality_review", "reviewer", "bypass", true},
		{"chat inherits prompt default", "", "", "chat", "", false},
		{"chat prompts", "", "", "chat", "prompt", false},
		{"chat bypasses", "", "", "chat", "bypass", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			phaseRoles := map[string]string{}
			if tt.phase != "" {
				phaseRoles[tt.phase] = tt.profileName
			}
			m := &home{
				program: "opencode",
				appConfig: &config.Config{
					PhaseRoles: phaseRoles,
					Profiles: map[string]config.AgentProfile{
						tt.profileName: {
							Program:           "opencode",
							Enabled:           true,
							PermissionDefault: tt.permissionDefault,
						},
					},
				},
			}

			assert.Equal(t, tt.want, m.skipPermissionsForAgent(tt.agentType))
		})
	}
}

func newPausedMasterInstance(t *testing.T, planFile string) *session.Instance {
	t.Helper()
	inst, err := session.FromInstanceData(session.InstanceData{
		Title:     "master-agent",
		Path:      t.TempDir(),
		Program:   "opencode",
		Status:    session.Paused,
		AgentType: session.AgentTypeMaster,
		TaskFile:  planFile,
	})
	require.NoError(t, err)
	// Move to Running so the test can verify Pause() transitions back.
	inst.SetStatus(session.Running)
	return inst
}

func TestHandleReviewChangesRequested_PausesMasterInstance(t *testing.T) {
	t.Parallel()
	h := newTestHome()
	h.pendingReviewFeedback = make(map[string]string)
	planFile := "test-plan.md"

	masterInst := newPausedMasterInstance(t, planFile)
	h.nav.AddInstance(masterInst)

	h.handleReviewChangesRequested(planFile, "needs changes")

	assert.True(t, masterInst.Paused(),
		"master instance must be paused when review changes are requested")
}
