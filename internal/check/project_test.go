package check

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/kastheco/kasmos/internal/initcmd/scaffold"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func bundledSkillNames(t *testing.T) []string {
	t.Helper()
	names, err := scaffold.BundledSkillNames()
	require.NoError(t, err)
	return names
}

func TestAuditProject_SyncedAndMissing(t *testing.T) {
	dir := t.TempDir()

	// Create .agents/skills/ with kasmos-coder and kasmos-planner (not all skills)
	agentsSkills := filepath.Join(dir, ".agents", "skills")
	for _, name := range []string{"kasmos-coder", "kasmos-planner"} {
		skillDir := filepath.Join(agentsSkills, name)
		require.NoError(t, os.MkdirAll(skillDir, 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("test"), 0o644))
	}

	// Create .claude/skills/ with kasmos-coder symlinked
	claudeSkills := filepath.Join(dir, ".claude", "skills")
	require.NoError(t, os.MkdirAll(claudeSkills, 0o755))
	coderTarget := filepath.Join("..", "..", ".agents", "skills", "kasmos-coder")
	require.NoError(t, os.Symlink(coderTarget, filepath.Join(claudeSkills, "kasmos-coder")))

	// .opencode/skills/ is empty (no symlinks)
	opencodeSkills := filepath.Join(dir, ".opencode", "skills")
	require.NoError(t, os.MkdirAll(opencodeSkills, 0o755))

	results := AuditProject(dir, []string{"claude", "opencode"})

	byName := make(map[string]ProjectSkillEntry)
	for _, r := range results {
		byName[r.Name] = r
	}

	// kasmos-coder: in canonical, claude synced, opencode missing
	coder := byName["kasmos-coder"]
	assert.True(t, coder.InCanonical, "kasmos-coder should be in canonical")
	assert.Equal(t, StatusSynced, coder.HarnessStatus["claude"], "kasmos-coder: claude should be synced")
	assert.Equal(t, StatusMissing, coder.HarnessStatus["opencode"], "kasmos-coder: opencode should be missing")

	// kasmos-planner: in canonical, both missing (no symlinks created)
	planner := byName["kasmos-planner"]
	assert.True(t, planner.InCanonical, "kasmos-planner should be in canonical")
	assert.Equal(t, StatusMissing, planner.HarnessStatus["claude"], "kasmos-planner: claude should be missing")
	assert.Equal(t, StatusMissing, planner.HarnessStatus["opencode"], "kasmos-planner: opencode should be missing")

	// kasmos-reviewer is bundled but missing from .agents/skills/.
	reviewer, exists := byName["kasmos-reviewer"]
	require.True(t, exists, "kasmos-reviewer should appear in results (bundled scaffold skill)")
	assert.False(t, reviewer.InCanonical, "kasmos-reviewer should be marked missing from canonical")
}

func TestAuditProject_AllSynced(t *testing.T) {
	dir := t.TempDir()

	skillNames := bundledSkillNames(t)

	agentsSkills := filepath.Join(dir, ".agents", "skills")
	for _, name := range skillNames {
		skillDir := filepath.Join(agentsSkills, name)
		require.NoError(t, os.MkdirAll(skillDir, 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("test"), 0o644))
	}

	// Symlink all skills for claude
	claudeSkills := filepath.Join(dir, ".claude", "skills")
	require.NoError(t, os.MkdirAll(claudeSkills, 0o755))
	for _, name := range skillNames {
		target := filepath.Join("..", "..", ".agents", "skills", name)
		require.NoError(t, os.Symlink(target, filepath.Join(claudeSkills, name)))
	}

	results := AuditProject(dir, []string{"claude"})

	assert.Len(t, results, len(skillNames))
	for _, r := range results {
		assert.True(t, r.InCanonical, "%s should be in canonical", r.Name)
		assert.True(t, r.HasSkillMD, "%s should have SKILL.md", r.Name)
		assert.Equal(t, StatusSynced, r.HarnessStatus["claude"], "%s: claude should be synced", r.Name)
	}
}

func TestAuditProject_IgnoresUnbundledCanonicalSkills(t *testing.T) {
	dir := t.TempDir()
	agentsSkills := filepath.Join(dir, ".agents", "skills")
	require.NoError(t, os.MkdirAll(agentsSkills, 0o755))

	require.NoError(t, os.MkdirAll(filepath.Join(agentsSkills, "kasmos-coder"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(agentsSkills, "kasmos-coder", "SKILL.md"), []byte("test"), 0o644))
	require.NoError(t, os.MkdirAll(filepath.Join(agentsSkills, "my-custom-skill"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(agentsSkills, "tui-design"), 0o755))

	results := AuditProject(dir, []string{"claude"})

	byName := make(map[string]ProjectSkillEntry)
	for _, r := range results {
		byName[r.Name] = r
	}
	_, customExists := byName["my-custom-skill"]
	assert.False(t, customExists, "non-bundled custom skills should be ignored by audit")
	_, tuiExists := byName["tui-design"]
	assert.False(t, tuiExists, "non-bundled optional skills should be ignored by audit")
	entry, ok := byName["kasmos-coder"]
	require.True(t, ok, "bundled skills should still be audited")
	assert.True(t, entry.InCanonical)
}

func TestAuditProject_SkillMDCheck(t *testing.T) {
	dir := t.TempDir()
	agentsSkills := filepath.Join(dir, ".agents", "skills")

	// Skill with SKILL.md
	withSkillMDDir := filepath.Join(agentsSkills, "kasmos-coder")
	require.NoError(t, os.MkdirAll(withSkillMDDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(withSkillMDDir, "SKILL.md"), []byte("test"), 0o644))

	// Skill without SKILL.md
	withoutSkillMDDir := filepath.Join(agentsSkills, "kasmos-planner")
	require.NoError(t, os.MkdirAll(withoutSkillMDDir, 0o755))

	results := AuditProject(dir, []string{"claude"})

	byName := make(map[string]ProjectSkillEntry)
	for _, r := range results {
		byName[r.Name] = r
	}

	assert.True(t, byName["kasmos-coder"].HasSkillMD, "kasmos-coder should have HasSkillMD=true")
	assert.False(t, byName["kasmos-planner"].HasSkillMD, "kasmos-planner should have HasSkillMD=false")
}

func TestAuditProject_DetectsNonSymlinkCopy(t *testing.T) {
	dir := t.TempDir()
	agentsSkills := filepath.Join(dir, ".agents", "skills")
	skillDir := filepath.Join(agentsSkills, "kasmos-coder")
	require.NoError(t, os.MkdirAll(skillDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("test"), 0o644))

	// Create a non-symlink directory in harness dir (copy, not symlink)
	claudeSkillPath := filepath.Join(dir, ".claude", "skills", "kasmos-coder")
	require.NoError(t, os.MkdirAll(claudeSkillPath, 0o755))

	results := AuditProject(dir, []string{"claude"})

	byName := make(map[string]ProjectSkillEntry)
	for _, r := range results {
		byName[r.Name] = r
	}

	skill := byName["kasmos-coder"]
	assert.True(t, skill.InCanonical)
	assert.Equal(t, StatusCopy, skill.HarnessStatus["claude"], "non-symlink dir should be StatusCopy")
}

func TestAuditProject_SymlinkedSkillIncluded(t *testing.T) {
	// A bundled skill symlink in .agents/skills/ pointing to a real directory must be
	// included in the audit, not silently skipped.
	dir := t.TempDir()
	agentsSkills := filepath.Join(dir, ".agents", "skills")
	require.NoError(t, os.MkdirAll(agentsSkills, 0o755))

	// Create an external skill directory and symlink it into .agents/skills/.
	externalSkill := filepath.Join(dir, "external-skill-src")
	require.NoError(t, os.MkdirAll(externalSkill, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(externalSkill, "SKILL.md"), []byte("test"), 0o644))
	require.NoError(t, os.Symlink(externalSkill, filepath.Join(agentsSkills, "kasmos-coder")))

	results := AuditProject(dir, []string{"claude"})

	byName := make(map[string]ProjectSkillEntry)
	for _, r := range results {
		byName[r.Name] = r
	}
	require.Contains(t, byName, "kasmos-coder")
	assert.True(t, byName["kasmos-coder"].InCanonical)
	assert.True(t, byName["kasmos-coder"].HasSkillMD, "SKILL.md through symlink should be found")
}

func TestAuditProject_DanglingCanonicalSymlinkMarkedMissing(t *testing.T) {
	// A dangling bundled skill symlink in .agents/skills/ should be treated as missing.
	dir := t.TempDir()
	agentsSkills := filepath.Join(dir, ".agents", "skills")
	require.NoError(t, os.MkdirAll(agentsSkills, 0o755))
	require.NoError(t, os.Symlink("/nonexistent-target", filepath.Join(agentsSkills, "kasmos-coder")))

	results := AuditProject(dir, []string{"claude"})
	byName := make(map[string]ProjectSkillEntry)
	for _, r := range results {
		byName[r.Name] = r
	}
	require.Contains(t, byName, "kasmos-coder")
	assert.False(t, byName["kasmos-coder"].InCanonical, "dangling canonical symlink should be treated as missing")
}

func TestAuditProject_MissingCanonicalDir(t *testing.T) {
	// .agents/ exists (project) but .agents/skills/ is absent — should surface
	// a synthetic unhealthy entry rather than silently returning nil.
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, ".agents"), 0o755))
	// Deliberately do NOT create .agents/skills/

	results := AuditProject(dir, []string{"claude"})

	require.Len(t, results, 1, "expected one synthetic entry for missing canonical dir")
	assert.Equal(t, ".agents/skills", results[0].Name)
	assert.False(t, results[0].InCanonical, "synthetic entry must not be marked as in-canonical")
}

func TestAuditProject_BrokenSymlink(t *testing.T) {
	dir := t.TempDir()

	agentsSkills := filepath.Join(dir, ".agents", "skills")
	skillDir := filepath.Join(agentsSkills, "kasmos-coder")
	require.NoError(t, os.MkdirAll(skillDir, 0o755))

	claudeSkills := filepath.Join(dir, ".claude", "skills")
	require.NoError(t, os.MkdirAll(claudeSkills, 0o755))
	// Broken symlink
	require.NoError(t, os.Symlink("/nonexistent", filepath.Join(claudeSkills, "kasmos-coder")))

	results := AuditProject(dir, []string{"claude"})

	byName := make(map[string]ProjectSkillEntry)
	for _, r := range results {
		byName[r.Name] = r
	}

	coder := byName["kasmos-coder"]
	assert.True(t, coder.InCanonical)
	assert.Equal(t, StatusBroken, coder.HarnessStatus["claude"], "kasmos-coder: claude should be broken")
}
