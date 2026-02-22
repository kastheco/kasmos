package check

// SkillStatus represents the state of a single skill entry.
type SkillStatus int

const (
	StatusSynced  SkillStatus = iota // symlink exists, valid target
	StatusSkipped                    // source is symlink, intentionally not synced
	StatusMissing                    // source exists, no link in harness
	StatusOrphan                     // link in harness, no source
	StatusBroken                     // symlink exists, target doesn't resolve
)

func (s SkillStatus) String() string {
	switch s {
	case StatusSynced:
		return "synced"
	case StatusSkipped:
		return "skipped"
	case StatusMissing:
		return "missing"
	case StatusOrphan:
		return "orphan"
	case StatusBroken:
		return "broken"
	default:
		return "unknown"
	}
}

// SkillEntry is one skill's audit result for one harness.
type SkillEntry struct {
	Name   string
	Status SkillStatus
	Detail string // e.g. symlink target, error message
}

// HarnessResult holds audit results for one harness.
type HarnessResult struct {
	Name      string
	Installed bool
	Skills    []SkillEntry
}

// ProjectSkillEntry is one embedded skill's status in the project.
type ProjectSkillEntry struct {
	Name          string
	InCanonical   bool                   // exists in .agents/skills/
	HarnessStatus map[string]SkillStatus // harness name → status
}

// SuperpowersResult holds superpowers check for one harness.
type SuperpowersResult struct {
	Name      string
	Installed bool
	Detail    string
}

// AuditResult is the complete output of kq check.
type AuditResult struct {
	Global      []HarnessResult
	Project     []ProjectSkillEntry
	Superpowers []SuperpowersResult
	InProject   bool // whether cwd is a kq project
}

// Summary returns (ok, total) counts across all checks.
func (r *AuditResult) Summary() (int, int) {
	ok, total := 0, 0
	for _, h := range r.Global {
		for _, s := range h.Skills {
			if s.Status == StatusSkipped {
				continue // don't count intentional skips
			}
			total++
			if s.Status == StatusSynced {
				ok++
			}
		}
	}
	for _, p := range r.Project {
		if !p.InCanonical {
			total++
			continue
		}
		for _, st := range p.HarnessStatus {
			total++
			if st == StatusSynced {
				ok++
			}
		}
	}
	for _, sp := range r.Superpowers {
		total++
		if sp.Installed {
			ok++
		}
	}
	return ok, total
}
