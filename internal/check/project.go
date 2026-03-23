package check

import (
	"os"
	"path/filepath"
	"sort"

	"github.com/kastheco/kasmos/internal/initcmd/scaffold"
)

// AuditProject checks scaffold-bundled project skills and verifies harness
// project skill dirs have valid symlinks or copies. Repo-local optional skills
// outside the scaffold bundle are ignored by this audit.
func AuditProject(dir string, harnessNames []string) []ProjectSkillEntry {
	canonicalDir := filepath.Join(dir, ".agents", "skills")

	if _, err := os.Stat(canonicalDir); err != nil {
		// Surface the missing/unreadable canonical dir as an unhealthy entry.
		// AuditProject is only called when InProject=true (.agents/ exists), so a
		// missing or unreadable .agents/skills/ directory is itself a health issue.
		return []ProjectSkillEntry{{
			Name:          ".agents/skills",
			InCanonical:   false,
			HarnessStatus: map[string]SkillStatus{},
		}}
	}

	bundledSkills, err := scaffold.BundledSkillNames()
	if err != nil {
		return []ProjectSkillEntry{{
			Name:          ".agents/skills",
			InCanonical:   false,
			HarnessStatus: map[string]SkillStatus{},
		}}
	}

	results := make([]ProjectSkillEntry, 0, len(bundledSkills))
	for _, skillName := range bundledSkills {
		skillPath := filepath.Join(canonicalDir, skillName)

		entry := ProjectSkillEntry{
			Name:          skillName,
			HarnessStatus: make(map[string]SkillStatus),
		}

		entry.InCanonical, entry.HasSkillMD = canonicalSkillStatus(skillPath)
		if !entry.InCanonical {
			results = append(results, entry)
			continue
		}

		// Check each harness's project skill dir for a symlink.
		for _, harnessName := range harnessNames {
			if harnessName == "codex" {
				// Codex reads .agents/skills/ natively — always synced if in canonical.
				entry.HarnessStatus[harnessName] = StatusSynced
				continue
			}

			harnessSkillsDir := filepath.Join(dir, "."+harnessName, "skills")
			link := filepath.Join(harnessSkillsDir, skillName)

			lfi, err := os.Lstat(link)
			if err != nil {
				if os.IsNotExist(err) {
					entry.HarnessStatus[harnessName] = StatusMissing
				} else {
					entry.HarnessStatus[harnessName] = StatusBroken
				}
				continue
			}

			if lfi.Mode()&os.ModeSymlink == 0 {
				// Non-symlink directory — functional but may drift from source.
				entry.HarnessStatus[harnessName] = StatusCopy
				continue
			}

			// Symlink exists — verify target resolves.
			target, err := os.Readlink(link)
			if err != nil {
				entry.HarnessStatus[harnessName] = StatusBroken
				continue
			}

			resolvedTarget := target
			if !filepath.IsAbs(target) {
				resolvedTarget = filepath.Join(harnessSkillsDir, target)
			}

			if _, err := os.Stat(resolvedTarget); err != nil {
				entry.HarnessStatus[harnessName] = StatusBroken
			} else {
				entry.HarnessStatus[harnessName] = StatusSynced
			}
		}

		results = append(results, entry)
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Name < results[j].Name
	})

	return results
}

func canonicalSkillStatus(skillPath string) (bool, bool) {
	info, err := os.Lstat(skillPath)
	if err != nil {
		return false, false
	}

	if !info.IsDir() {
		if info.Mode()&os.ModeSymlink == 0 {
			return false, false
		}
		resolved, statErr := os.Stat(skillPath)
		if statErr != nil || !resolved.IsDir() {
			return false, false
		}
	}

	_, err = os.Stat(filepath.Join(skillPath, "SKILL.md"))
	return true, err == nil
}
