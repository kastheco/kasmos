package check

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/kastheco/kasmos/internal/initcmd/harness"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAudit_InProject(t *testing.T) {
	home := t.TempDir()
	projectDir := t.TempDir()

	// Mark as kas project by creating .agents/
	require.NoError(t, os.MkdirAll(filepath.Join(projectDir, ".agents", "skills"), 0o755))

	registry := harness.NewRegistry()
	result := Audit(home, projectDir, registry)

	assert.True(t, result.InProject, "should detect kas project")
	assert.NotNil(t, result.Global)
	assert.NotNil(t, result.Project)
}

func TestAudit_NotInProject(t *testing.T) {
	home := t.TempDir()
	projectDir := t.TempDir()
	// No .agents/ dir — not a kas project

	registry := harness.NewRegistry()
	result := Audit(home, projectDir, registry)

	assert.False(t, result.InProject, "should not detect kas project")
	assert.Nil(t, result.Project, "project skills should be nil when not in project")
}

func TestAudit_Summary_AllSynced(t *testing.T) {
	home := t.TempDir()
	projectDir := t.TempDir()

	// Set up global skills for claude: one real skill synced
	agentsSkills := filepath.Join(home, ".agents", "skills")
	skillDir := filepath.Join(agentsSkills, "my-skill")
	require.NoError(t, os.MkdirAll(skillDir, 0o755))

	claudeSkills := filepath.Join(home, ".claude", "skills")
	require.NoError(t, os.MkdirAll(claudeSkills, 0o755))
	target, err := filepath.Rel(claudeSkills, skillDir)
	require.NoError(t, err)
	require.NoError(t, os.Symlink(target, filepath.Join(claudeSkills, "my-skill")))

	// Not a kas project — no project skills counted
	registry := harness.NewRegistry()
	result := Audit(home, projectDir, registry)

	ok, total := result.Summary()
	// Global: 1 synced (my-skill for claude) + opencode missing + codex synced
	assert.GreaterOrEqual(t, total, 1, "should have at least one check")
	assert.GreaterOrEqual(t, ok, 0)
	assert.LessOrEqual(t, ok, total)
}

func TestAudit_GlobalResultsPerHarness(t *testing.T) {
	home := t.TempDir()
	projectDir := t.TempDir()

	registry := harness.NewRegistry()
	result := Audit(home, projectDir, registry)

	// Should have one HarnessResult per registered harness
	harnessNames := registry.All()
	assert.Len(t, result.Global, len(harnessNames))

	byName := make(map[string]HarnessResult)
	for _, h := range result.Global {
		byName[h.Name] = h
	}
	for _, name := range harnessNames {
		_, ok := byName[name]
		assert.True(t, ok, "should have global result for harness %s", name)
	}
}

func TestStatusCopy_String(t *testing.T) { assert.Equal(t, "copy", StatusCopy.String()) }

func TestSummary_CopyCountsAsOK(t *testing.T) {
	result := &AuditResult{Project: []ProjectSkillEntry{{Name: "skill-a", InCanonical: true, HasSkillMD: true, HarnessStatus: map[string]SkillStatus{"claude": StatusCopy}}}, InProject: true}
	ok, total := result.Summary()
	assert.Equal(t, 2, ok)
	assert.Equal(t, 2, total)
}

func TestSummary_MissingSkillMDCountsAsNotOK(t *testing.T) {
	result := &AuditResult{Project: []ProjectSkillEntry{{Name: "skill-a", InCanonical: true, HasSkillMD: false, HarnessStatus: map[string]SkillStatus{"claude": StatusSynced}}}, InProject: true}
	ok, total := result.Summary()
	assert.Equal(t, 1, ok)
	assert.Equal(t, 2, total)
}

func TestAudit_PopulatesBinaryPath(t *testing.T) {
	home := t.TempDir()
	projectDir := t.TempDir()
	registry := harness.NewRegistry()
	result := Audit(home, projectDir, registry)

	assert.NotNil(t, result.BinaryPath, "BinaryPath should always be populated")
}

func TestAuditAgentCommands_ValidatesEnabledProfiles(t *testing.T) {
	projectDir := t.TempDir()
	configDir := filepath.Join(projectDir, ".kasmos")
	require.NoError(t, os.MkdirAll(configDir, 0o755))

	codexPath := filepath.Join(t.TempDir(), "codex")
	require.NoError(t, os.WriteFile(codexPath, []byte("#!/bin/sh\n"), 0o755))
	brokenPath := filepath.Join(t.TempDir(), "broken")
	require.NoError(t, os.WriteFile(brokenPath, []byte("#!/bin/sh\n"), 0o644))

	require.NoError(t, os.WriteFile(filepath.Join(configDir, "config.toml"), []byte(fmt.Sprintf(`
[agents.coder]
enabled = true
program = "codex"

[agents.reviewer]
enabled = true
program = %q

[agents.disabled]
enabled = false
program = "missing"
`, brokenPath)), 0o644))

	prev := SetResolveAgentCommandPathForTest(func(name string) (string, error) {
		if name == "codex" {
			return codexPath, nil
		}
		return "", fmt.Errorf("%s missing", name)
	})
	t.Cleanup(func() { SetResolveAgentCommandPathForTest(prev) })

	result := AuditAgentCommands(projectDir)
	require.NotNil(t, result)
	require.Empty(t, result.LoadError)
	require.Len(t, result.Entries, 2)

	assert.Equal(t, "coder", result.Entries[0].Role)
	assert.True(t, result.Entries[0].Healthy)
	assert.Equal(t, codexPath, result.Entries[0].Resolved)

	assert.Equal(t, "reviewer", result.Entries[1].Role)
	assert.False(t, result.Entries[1].Healthy)
	assert.Contains(t, result.Entries[1].Detail, "not executable")
}

func TestSummary_AgentCommandFailureCountsAsUnhealthy(t *testing.T) {
	result := &AuditResult{
		AgentCommands: &AgentCommandResult{
			Entries: []AgentCommandStatus{
				{Role: "coder", Healthy: true},
				{Role: "reviewer", Healthy: false},
			},
		},
	}
	ok, total := result.Summary()
	assert.Equal(t, 1, ok)
	assert.Equal(t, 2, total)
}

func TestSummary_BinaryPathMismatchCountsAsUnhealthy(t *testing.T) {
	result := &AuditResult{
		BinaryPath: &BinaryPathResult{
			RunningExecutable: "/usr/local/bin/kas",
			RunningCanonical:  "/usr/local/bin/kas",
			References: []BinaryPathReference{
				{
					File:       ".mcp.json",
					Label:      "mcpServers.kasmos",
					RawPath:    "/old/kas",
					Normalized: "/old/kas",
					Healthy:    false,
				},
			},
		},
	}
	ok, total := result.Summary()
	// 1 running (ok) + 1 mismatch (not ok)
	assert.Equal(t, 1, ok)
	assert.Equal(t, 2, total)
}

func TestSummary_MissingServiceFilesNotCountedAgainstHealth(t *testing.T) {
	result := &AuditResult{
		BinaryPath: &BinaryPathResult{
			RunningExecutable: "/usr/local/bin/kas",
			RunningCanonical:  "/usr/local/bin/kas",
			References: []BinaryPathReference{
				{
					File:         "kasmos.service",
					Label:        "ExecStart",
					Note:         "not installed",
					NotInstalled: true,
				},
			},
		},
	}
	ok, total := result.Summary()
	// 1 running ok, not-installed refs excluded
	assert.Equal(t, 1, ok)
	assert.Equal(t, 1, total)
}

func TestSummary_NilBinaryPathNotPanics(t *testing.T) {
	result := &AuditResult{}
	ok, total := result.Summary()
	// Should not panic; binary path adds 0 to both counts
	assert.Equal(t, 0, ok)
	assert.Equal(t, 0, total)
}
