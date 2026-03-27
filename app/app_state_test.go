package app

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"charm.land/bubbles/v2/spinner"
	"github.com/kastheco/kasmos/config"
	"github.com/kastheco/kasmos/config/taskstate"
	"github.com/kastheco/kasmos/session"
	"github.com/kastheco/kasmos/ui"
	"github.com/kastheco/kasmos/ui/overlay"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSpawnTaskAgent_PatchesMainBranchOpencodeConfig(t *testing.T) {
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

func TestSpawnArchitectPass_PatchesMainBranchOpencodeConfig(t *testing.T) {
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
	prompt := buildChatAboutTaskPrompt("architect-plan-location", taskstate.TaskEntry{
		Status:      taskstate.StatusReady,
		Description: "architect-plan-location",
		Branch:      "plan/architect-plan-location",
		Topic:       "bugs",
	}, "is this still needed given the switch to an mcp server?")

	assert.Contains(t, prompt, "MCP `task_show`")
	assert.Contains(t, prompt, `filename: "architect-plan-location"`)
	assert.Contains(t, prompt, "fall back to `kas task show architect-plan-location`")
	assert.Contains(t, prompt, "## User Question")
}

func TestSyncSharedWorktreeScaffold_WritesHarnessFilesForConfiguredProfiles(t *testing.T) {
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
	assert.FileExists(t, filepath.Join(dir, ".claude", ".mcp.json"))
	assert.FileExists(t, filepath.Join(dir, ".claude", "agents", "reviewer.md"))
	assert.FileExists(t, filepath.Join(dir, ".opencode", "agents", "fixer.md"))
	assert.FileExists(t, filepath.Join(dir, ".agents", "skills", "kasmos-fixer", "SKILL.md"))
	assert.FileExists(t, filepath.Join(dir, "opencode.jsonc"))
}
